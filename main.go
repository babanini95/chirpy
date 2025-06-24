package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/babanini95/chirpy/internal/auth"
	"github.com/babanini95/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	queries        *database.Queries
}

type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

type authReqBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func main() {
	// load .env
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	// get db url
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	// get generated queries
	dbQueries := database.New(db)

	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}
	// store generated queries in apiCfg
	apiCfg := &apiConfig{
		queries: dbQueries,
	}
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileServerHandler))
	mux.Handle("/assets", http.FileServer(http.Dir("./assets/logo.png")))

	mux.HandleFunc("GET /api/healthz", readinessHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.getAllChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpId}", apiCfg.getChirpById)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirpsHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginHandler)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshTokenHandler)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeAccessTokenHandler)
	mux.HandleFunc("PUT /api/users", apiCfg.updateEmailAndPasswordHandler)

	mux.HandleFunc("GET /admin/metrics", apiCfg.writeNumberOfRequestHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	srv.ListenAndServe()
}

func (cfg *apiConfig) writeNumberOfRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	htmlString := fmt.Sprintf(`
<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>
`, cfg.fileserverHits.Load())
	w.Write(fmt.Appendf(nil, "%v", htmlString))
	w.WriteHeader(200)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	err := godotenv.Load()
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	isDev := os.Getenv("PLATFORM") == "dev"
	if !isDev {
		respondWithError(w, 403, "Forbidden")
		return
	}

	err = cfg.queries.ResetUser(r.Context())
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	cfg.fileserverHits.Store(0)
	w.WriteHeader(200)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	cfg.fileserverHits.Add(1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	reqData := authReqBody{}
	err := decoder.Decode(&reqData)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(reqData.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	params := database.CreateUserParams{
		Email:          reqData.Email,
		HashedPassword: hashedPassword,
	}
	user, err := cfg.queries.CreateUser(r.Context(), params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	jsonUser := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	respondWithJson(w, 201, jsonUser)

}

func (cfg *apiConfig) createChirpsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	userId, err := auth.ValidateJWT(token, os.Getenv("SECRET_KEY"))
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	type reqBody struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	reqData := reqBody{}
	err = decoder.Decode(&reqData)
	if err != nil {
		respondWithError(w, 400, "invalid request body")
		return
	}

	if len(reqData.Body) > 140 {
		respondWithError(w, 400, "chirp is too long")
		return
	}

	cleanedBody := censorChirp(reqData.Body, []string{"kerfuffle", "sharbert", "fornax"})
	params := database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: userId,
	}
	c, err := cfg.queries.CreateChirp(r.Context(), params)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	respPayload := Chirp{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		UserID:    c.UserID,
	}
	respondWithJson(w, 201, respPayload)
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	cs, err := cfg.queries.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	chirps := make([]Chirp, len(cs))
	for i, c := range cs {
		chirps[i] = Chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		}
	}

	respondWithJson(w, 200, chirps)
}

func (cfg *apiConfig) getChirpById(w http.ResponseWriter, r *http.Request) {
	chirpId, err := uuid.Parse(r.PathValue("chirpId"))
	if err != nil {
		respondWithError(w, 404, err.Error())
		return
	}

	c, err := cfg.queries.GetChirpById(r.Context(), chirpId)
	if err != nil {
		respondWithError(w, 404, err.Error())
		return
	}
	chirp := Chirp{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		UserID:    c.UserID,
	}
	respondWithJson(w, 200, chirp)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	reqData := authReqBody{}
	err := decoder.Decode(&reqData)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}

	user, err := cfg.queries.GetUserByEmail(r.Context(), reqData.Email)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	err = auth.CheckPassword(user.HashedPassword, reqData.Password)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	refreshTokenExpAt := time.Now().AddDate(0, 0, 60)
	refreshToken, _ := auth.MakeRefreshToken()
	saveRefreshTokenParams := database.SaveRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: refreshTokenExpAt,
	}
	_, err = cfg.queries.SaveRefreshToken(r.Context(), saveRefreshTokenParams)
	if err != nil {
		respondWithError(w, 500, "Can not save refresh token")
		return
	}

	accessTokenExpDuration := time.Hour
	token, err := auth.MakeJWT(user.ID, os.Getenv("SECRET_KEY"), accessTokenExpDuration)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	respData := User{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
	}
	respondWithJson(w, 200, respData)
}

func (cfg *apiConfig) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	tokenDb, err := cfg.queries.GetUserFromRefreshTokens(r.Context(), token)
	if err != nil {
		respondWithError(w, 401, "invalid refresh token")
		return
	}

	if tokenDb.ExpiresAt.Before(time.Now()) {
		respondWithError(w, 401, "refresh token expired")
		return
	}

	// make jwt
	jwt, err := auth.MakeJWT(tokenDb.UserID, os.Getenv("SECRET_KEY"), time.Hour)
	if err != nil {
		respondWithError(w, 500, err.Error())
	}

	type respData struct {
		Token string `json:"token"`
	}

	respBody := respData{Token: jwt}
	respondWithJson(w, 200, respBody)
}

func (cfg *apiConfig) revokeAccessTokenHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	err = cfg.queries.RevokeRefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	w.WriteHeader(204)
}

func (cfg *apiConfig) updateEmailAndPasswordHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	accessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	userId, err := auth.ValidateJWT(accessToken, os.Getenv("SECRET_KEY"))
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	reqBody := authReqBody{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}

	user, _ := cfg.queries.GetUserByEmail(r.Context(), reqBody.Email)
	if userId != user.ID {
		respondWithError(w, 401, "can not update others")
		return
	}

	hashPwd, err := auth.HashPassword(reqBody.Password)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}

	updatedUser, err := cfg.queries.UpdateEmailAndPassword(
		r.Context(),
		database.UpdateEmailAndPasswordParams{
			Email:          reqBody.Email,
			HashedPassword: hashPwd,
			ID:             userId,
		},
	)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	respBody := User{
		ID:        userId,
		CreatedAt: updatedUser.CreatedAt,
		UpdatedAt: updatedUser.UpdatedAt,
		Email:     updatedUser.Email,
	}
	respondWithJson(w, 200, respBody)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) error {
	resp, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(resp)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, message string) error {
	return respondWithJson(w, code, map[string]string{"error": message})
}

func censorChirp(chirp string, profane []string) string {

	separateWords := strings.Split(chirp, " ")
	for i, word := range separateWords {
		for _, badWord := range profane {
			if strings.EqualFold(word, badWord) {
				separateWords[i] = "****"
			}
		}
	}

	cleanChirp := strings.Join(separateWords, " ")

	return cleanChirp
}

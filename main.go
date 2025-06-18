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
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)
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

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type reqBody struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	reqData := reqBody{}
	err := decoder.Decode(&reqData)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	user, err := cfg.queries.CreateUser(r.Context(), reqData.Email)
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

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type reqBody struct {
		Body string `json:"body"`
	}

	type resBody struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(r.Body)
	reqData := reqBody{}
	err := decoder.Decode(&reqData)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	if len(reqData.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	profane := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedBody := censorChirp(reqData.Body, profane)

	resLoad := resBody{CleanedBody: cleanedBody}
	respondWithJson(w, 200, resLoad)
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

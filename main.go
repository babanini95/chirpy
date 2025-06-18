package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/babanini95/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	queries        *database.Queries
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
	mux.HandleFunc("GET /admin/metrics", apiCfg.writeNumberOfRequestHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)

	srv.ListenAndServe()
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
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

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(200)
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

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	cfg.fileserverHits.Add(1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
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

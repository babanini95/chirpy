package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	srv := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	mux.Handle("/", http.FileServer(http.Dir(".")))

	srv.ListenAndServe()
}

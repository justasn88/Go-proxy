package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("pong"))
		if err != nil {
			log.Printf("failed to write data: %v", err)
		}
	})

	log.Printf("API Server is starting on port :%s", apiPort)
	if err := http.ListenAndServe(":"+apiPort, mux); err != nil {
		log.Fatalf("API server crashed: %v", err)
	}
}

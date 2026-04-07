package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			log.Printf("Failed to write bytes: %v", err)
		}
	})

	log.Println("Target server is running on :9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Fatalf("Target server crashed: %v", err)
	}
}

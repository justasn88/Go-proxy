package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Target server is running on :9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Fatalf("Target server crashed: %v", err)
	}
}

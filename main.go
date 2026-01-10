package main

import (
	"awesomeProject11/proxy"
	"awesomeProject11/repo"
	"log"
	"net/http"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	allowedUser := map[string]string{"user": "pass"}

	Repository := repo.NewMemoryRepo(allowedUser)

	server := &proxy.Server{
		Repo: Repository,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(server.ProxyHandler)))
}

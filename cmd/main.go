package main

import (
	"awesomeProject11/internal/proxy"
	"awesomeProject11/internal/repo"
	"log"
	"net/http"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	allowedUser := map[string]string{"user": "pass"}

	redisClient, err := repo.CreateRedisCache(0)
	if err != nil {
		log.Fatal("Failed to create cache: %v", err)
	}
	log.Println("Succesfully connected to Redis Cache")

	Repository := repo.NewRedisRepo(redisClient, allowedUser)

	server := &proxy.Server{
		Repo: Repository,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(server.ProxyHandler)))
}

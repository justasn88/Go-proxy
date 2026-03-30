package main

import (
	"awesomeProject11/internal/proxy"
	"awesomeProject11/internal/repo"
	"log"
	"net/http"
	"os"
)

func main() {

	pgDSN := os.Getenv("POSTGRES_DNS")

	if pgDSN == "" {
		pgDSN = "postgres://proxy_user:proxy_password@localhost:5432/proxy_db?sslmode=disable"
	}

	pgDB, err := repo.ConnectPostgres(pgDSN)
	if err != nil {
		log.Fatalf("PosgreSQL connection error: %v", err)
	}
	defer func() {
		err := pgDB.Close()
		if err != nil {
			log.Printf("Failed to close PosgreSQL db: %v", err)
		}
	}()

	allowedUsers, err := repo.LoadCredentialsFromDB(pgDB)
	if err != nil {
		log.Printf("Failed to load users from DB: %v", err)
	}

	redisClient, err := repo.CreateRedisCache(0)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	log.Println("Successfully connected to Redis Cache")

	asyncLogger := repo.NewAsyncLogger(pgDB, 2000)

	Repository := repo.NewRedisRepo(redisClient, allowedUsers, asyncLogger)

	server := &proxy.Server{
		Repo: Repository,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(server.ProxyHandler)))
}

package repo

import (
	"database/sql"
	"fmt"
	"log"
)

type PosgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) *PosgresRepo {
	return &PosgresRepo{
		db: db,
	}
}

func ConnectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres DB: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to reach DB (Ping error): %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	log.Println("successfully connected to PostgreSQL")
	return db, nil
}

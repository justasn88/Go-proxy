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

func LoadCredentialsFromDB(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT username, password FROM users")
	if err != nil {
		return nil, fmt.Errorf("failed to read users from db: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	creds := make(map[string]string)
	for rows.Next() {
		var username, password string
		if err := rows.Scan(&username, &password); err != nil {
			return nil, fmt.Errorf("error when reading row: %v", err)
		}
		creds[username] = password
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return creds, nil
}

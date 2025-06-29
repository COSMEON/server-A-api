package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// NewDatabase creates a new database connection
func NewDatabase() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "user=postgres password=password dbname=postgres sslmode=disable"
	}
	log.Printf("Connecting to database: %s", dbURL)

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	return db
}

// InitDB initializes the database schema
func InitDB(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS codebases (
		id UUID PRIMARY KEY,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		file_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS files (
		id SERIAL PRIMARY KEY,
		codebase_id UUID REFERENCES codebases(id) ON DELETE CASCADE,
		file_path TEXT NOT NULL,
		file_name TEXT NOT NULL,
		file_size BIGINT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_files_codebase_id ON files(codebase_id);
	`

	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
}

package main

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("postgres", "user=postgres password=Ayan46@93#Yo dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS codebases (
		uuid UUID PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS files (
		id SERIAL PRIMARY KEY,
		codebase_id UUID REFERENCES codebases(uuid) ON DELETE CASCADE,
		path TEXT NOT NULL,
		name TEXT NOT NULL,
		size BIGINT NOT NULL,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
}

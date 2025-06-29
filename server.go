package main

import (
	"os"
)

// NewServer creates a new server instance with database connection
func NewServer() *Server {
	db := NewDatabase()
	InitDB(db)

	storageServerURL := os.Getenv("SERVER_B_URL")
	if storageServerURL == "" {
		storageServerURL = "http://localhost:8081"
	}

	return &Server{
		db:               db,
		storageServerURL: storageServerURL,
	}
}

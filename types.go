package main

import (
	"database/sql"
	"time"
)

// UploadResponse represents the response structure for file uploads
type UploadResponse struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message"`
	DirectoryID   string   `json:"directory_id,omitempty"`
	UploadedFiles []string `json:"uploaded_files,omitempty"`
}

// FileInfo represents metadata about a file
type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// Codebase represents a codebase record in the database
type Codebase struct {
	ID        string    `json:"directory_id"`
	CreatedAt time.Time `json:"created_at"`
	FileCount int       `json:"file_count"`
}

// Server holds the database connection and configuration
type Server struct {
	db               *sql.DB
	storageServerURL string
}

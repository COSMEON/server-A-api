package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	MaxUploadSize = 100 << 20 // 100MB
)

// uploadCodebase handles file upload requests
func (s *Server) uploadCodebase(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)

	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		respondWithError(w, http.StatusBadRequest, "File too large or invalid form data")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		respondWithError(w, http.StatusBadRequest, "No files uploaded")
		return
	}

	// Generate UUID for the new codebase
	codebaseID := uuid.New().String()

	// Forward files to storage server
	uploadedFiles, err := s.forwardFilesToStorage(codebaseID, files, r)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store files: %v", err))
		return
	}

	// Store metadata in database
	tx, err := s.db.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database transaction failed")
		return
	}
	defer tx.Rollback()

	// Insert codebase record
	_, err = tx.Exec("INSERT INTO codebases (id, file_count) VALUES ($1, $2)",
		codebaseID, len(uploadedFiles))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save codebase metadata")
		return
	}

	// Insert file records
	for _, fileInfo := range uploadedFiles {
		_, err = tx.Exec(`INSERT INTO files (codebase_id, file_path, file_name, file_size) 
			VALUES ($1, $2, $3, $4)`,
			codebaseID, fileInfo.Path, fileInfo.Name, fileInfo.Size)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to save file metadata")
			return
		}
	}

	if err = tx.Commit(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	var filePaths []string
	var totalSize int64
	for _, f := range uploadedFiles {
		filePaths = append(filePaths, f.Path)
		totalSize += f.Size
	}

	response := UploadResponse{
		Success:       true,
		Message:       fmt.Sprintf("Successfully uploaded %d files (%d bytes total)", len(uploadedFiles), totalSize),
		DirectoryID:   codebaseID,
		UploadedFiles: filePaths,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// listCodebases handles requests to list all codebases
func (s *Server) listCodebases(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, created_at, file_count FROM codebases ORDER BY created_at DESC")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query codebases")
		return
	}
	defer rows.Close()

	var codebases []Codebase
	for rows.Next() {
		var cb Codebase
		if err := rows.Scan(&cb.ID, &cb.CreatedAt, &cb.FileCount); err != nil {
			continue
		}
		codebases = append(codebases, cb)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"codebases": codebases,
	})
}

// getCodebaseFiles handles requests to get files in a specific codebase
func (s *Server) getCodebaseFiles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	codebaseID := vars["id"]

	if _, err := uuid.Parse(codebaseID); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid directory ID")
		return
	}

	// Check if codebase exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM codebases WHERE id = $1)", codebaseID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Codebase not found")
		return
	}

	// Get files from database
	rows, err := s.db.Query("SELECT file_path, file_name, file_size FROM files WHERE codebase_id = $1", codebaseID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to query files")
		return
	}
	defer rows.Close()

	var files []FileInfo
	for rows.Next() {
		var f FileInfo
		if err := rows.Scan(&f.Path, &f.Name, &f.Size); err != nil {
			continue
		}
		files = append(files, f)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"directory_id": codebaseID,
		"files":        files,
	})
}

// readFileContent handles requests to read file content from storage
func (s *Server) readFileContent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	codebaseID := vars["id"]
	filePath := r.URL.Query().Get("file")

	if _, err := uuid.Parse(codebaseID); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid directory ID")
		return
	}

	if filePath == "" {
		respondWithError(w, http.StatusBadRequest, "File path is required")
		return
	}

	// Verify file exists in database first
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE codebase_id = $1 AND file_path = $2)",
		codebaseID, filePath).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking file existence: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if !exists {
		respondWithError(w, http.StatusNotFound, "File not found in codebase")
		return
	}

	// Forward request to storage server with proper URL encoding
	log.Printf("Fetching file content for codebase %s, file %s", codebaseID, filePath)
	encodedFilePath := url.QueryEscape(filePath)
	requestURL := fmt.Sprintf("%s/content/%s?file=%s", s.storageServerURL, codebaseID, encodedFilePath)

	resp, err := http.Get(requestURL)
	if err != nil {
		log.Printf("Error fetching file content from storage server: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Storage server unavailable")
		return
	}
	defer resp.Body.Close()

	// Check if storage server returned an error
	if resp.StatusCode != http.StatusOK {
		log.Printf("Storage server returned status %d for file %s", resp.StatusCode, filePath)

		// Try to read error response from storage server
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			log.Printf("Storage server error response: %s", string(body))
		}

		if resp.StatusCode == http.StatusNotFound {
			respondWithError(w, http.StatusNotFound, "File not found in storage")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve file from storage")
		}
		return
	}

	// Copy response headers and body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// downloadFile handles file download requests
func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	codebaseID := vars["id"]
	filePath := r.URL.Query().Get("file")

	if _, err := uuid.Parse(codebaseID); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid directory ID")
		return
	}

	if filePath == "" {
		respondWithError(w, http.StatusBadRequest, "File path is required")
		return
	}

	// Verify file exists in database first
	var exists bool
	var fileName string
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE codebase_id = $1 AND file_path = $2), file_name FROM files WHERE codebase_id = $1 AND file_path = $2",
		codebaseID, filePath).Scan(&exists, &fileName)
	if err != nil {
		// Try a simpler query if the complex one fails
		err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE codebase_id = $1 AND file_path = $2)",
			codebaseID, filePath).Scan(&exists)
		if err != nil {
			log.Printf("Database error checking file existence: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}
		// Get filename separately if needed
		if exists {
			s.db.QueryRow("SELECT file_name FROM files WHERE codebase_id = $1 AND file_path = $2",
				codebaseID, filePath).Scan(&fileName)
		}
	}

	if !exists {
		respondWithError(w, http.StatusNotFound, "File not found in codebase")
		return
	}

	// Forward request to storage server with proper URL encoding
	encodedFilePath := url.QueryEscape(filePath)
	requestURL := fmt.Sprintf("%s/download/%s?file=%s", s.storageServerURL, codebaseID, encodedFilePath)

	resp, err := http.Get(requestURL)
	if err != nil {
		log.Printf("Error downloading file from storage server: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Storage server unavailable")
		return
	}
	defer resp.Body.Close()

	// Check if storage server returned an error
	if resp.StatusCode != http.StatusOK {
		log.Printf("Storage server returned status %d for file download %s", resp.StatusCode, filePath)

		if resp.StatusCode == http.StatusNotFound {
			respondWithError(w, http.StatusNotFound, "File not found in storage")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve file from storage")
		}
		return
	}

	// Copy headers from storage server response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Ensure proper filename in Content-Disposition if not set by storage server
	if w.Header().Get("Content-Disposition") == "" && fileName != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// downloadZip handles ZIP download requests for entire codebases
func (s *Server) downloadZip(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	codebaseID := vars["id"]

	if _, err := uuid.Parse(codebaseID); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid directory ID")
		return
	}

	// Check if codebase exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM codebases WHERE id = $1)", codebaseID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Codebase not found")
		return
	}

	// Forward request to storage server
	requestURL := fmt.Sprintf("%s/zip/%s", s.storageServerURL, codebaseID)
	resp, err := http.Get(requestURL)
	if err != nil {
		log.Printf("Error downloading ZIP from storage server: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Storage server unavailable")
		return
	}
	defer resp.Body.Close()

	// Check if storage server returned an error
	if resp.StatusCode != http.StatusOK {
		log.Printf("Storage server returned status %d for ZIP download", resp.StatusCode)

		if resp.StatusCode == http.StatusNotFound {
			respondWithError(w, http.StatusNotFound, "Codebase not found in storage")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve ZIP from storage")
		}
		return
	}

	// Copy headers and response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Ensure proper filename in Content-Disposition if not set
	if w.Header().Get("Content-Disposition") == "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"codebase-%s.zip\"", codebaseID))
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// healthCheck handles health check requests
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

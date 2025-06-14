package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var serverBURL = getServerBURL()

func getServerBURL() string {
	url := os.Getenv("SERVER_B_URL")
	if url == "" {
		return "http://localhost:8081"
	}
	return url
}

type UploadResponse struct {
	Success     bool   `json:"success"`
	UUID        string `json:"uuid,omitempty"`
	DirectoryID string `json:"directory_id,omitempty"`
	Message     string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type CodebaseListResponse struct {
	Success   bool       `json:"success"`
	Codebases []Codebase `json:"codebases,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type CodebaseDetailsResponse struct {
	Success bool   `json:"success"`
	Files   []File `json:"files,omitempty"`
	Error   string `json:"error,omitempty"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Success: false,
		Error:   message,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		respondWithError(w, http.StatusBadRequest, "No files provided")
		return
	}

	codebaseUUID := uuid.New().String()

	// Insert codebase into database
	_, err = db.Exec("INSERT INTO codebases (uuid, name) VALUES ($1, $2)", codebaseUUID, "UploadedCodebase")
	if err != nil {
		log.Printf("Database error: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Prepare multipart form for Server B
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("Failed to open file %s: %v", fileHeader.Filename, err)
			continue
		}

		part, err := writer.CreateFormFile("files", fileHeader.Filename)
		if err != nil {
			file.Close()
			continue
		}

		_, err = io.Copy(part, file)
		file.Close()
		if err != nil {
			continue
		}

		// Insert file metadata into database
		_, err = db.Exec("INSERT INTO files (codebase_id, path, name, size) VALUES ($1, $2, $3, $4)",
			codebaseUUID, fileHeader.Filename, filepath.Base(fileHeader.Filename), fileHeader.Size)
		if err != nil {
			log.Printf("DB insert error for file %s: %v", fileHeader.Filename, err)
		}
	}
	writer.Close()

	// Send to Server B
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/upload/%s", serverBURL, codebaseUUID), body)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create request for Server B")
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to upload to Server B: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to upload to Server B")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithError(w, http.StatusInternalServerError, "Server B upload failed")
		return
	}

	respondWithJSON(w, http.StatusOK, UploadResponse{
		Success:     true,
		UUID:        codebaseUUID,
		DirectoryID: codebaseUUID,
		Message:     "Upload successful",
	})
}

func ListCodebasesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT uuid, name FROM codebases ORDER BY created_at DESC")
	if err != nil {
		log.Printf("Database query error: %v", err)
		respondWithJSON(w, http.StatusOK, CodebaseListResponse{
			Success: false,
			Error:   "Database error",
		})
		return
	}
	defer rows.Close()

	var codebases []Codebase
	for rows.Next() {
		var cb Codebase
		err := rows.Scan(&cb.UUID, &cb.Name)
		if err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}
		codebases = append(codebases, cb)
	}

	respondWithJSON(w, http.StatusOK, CodebaseListResponse{
		Success:   true,
		Codebases: codebases,
	})
}

func GetCodebaseDetailsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]

	rows, err := db.Query("SELECT path, name, size FROM files WHERE codebase_id = $1 ORDER BY path", uuid)
	if err != nil {
		log.Printf("Database query error: %v", err)
		respondWithJSON(w, http.StatusOK, CodebaseDetailsResponse{
			Success: false,
			Error:   "Database error",
		})
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		err := rows.Scan(&f.Path, &f.Name, &f.Size)
		if err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}
		files = append(files, f)
	}

	respondWithJSON(w, http.StatusOK, CodebaseDetailsResponse{
		Success: true,
		Files:   files,
	})
}

func GetFileMetadataHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	filepath := mux.Vars(r)["filepath"]

	row := db.QueryRow("SELECT name, size FROM files WHERE codebase_id = $1 AND path = $2", uuid, filepath)
	var f File
	err := row.Scan(&f.Name, &f.Size)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "File not found")
		return
	}
	f.Path = filepath
	respondWithJSON(w, http.StatusOK, f)
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	codebaseUUID := mux.Vars(r)["uuid"]
	filePath := mux.Vars(r)["filepath"]

	url := fmt.Sprintf("%s/download/%s/%s", serverBURL, codebaseUUID, filePath)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to get file from Server B: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error retrieving file from Server B")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithError(w, http.StatusNotFound, "File not found on Server B")
		return
	}

	// Copy headers from Server B response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// If no content-disposition header, set one
	if w.Header().Get("Content-Disposition") == "" {
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func DownloadZipHandler(w http.ResponseWriter, r *http.Request) {
	codebaseUUID := mux.Vars(r)["uuid"]
	url := fmt.Sprintf("%s/download-zip/%s", serverBURL, codebaseUUID)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to get ZIP from Server B: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error retrieving ZIP from Server B")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithError(w, http.StatusNotFound, "ZIP not found on Server B")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+codebaseUUID+".zip")
	io.Copy(w, resp.Body)
}

// Add missing handlers that the frontend expects
func GetFileContentHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	filePath := r.URL.Query().Get("file")

	if filePath == "" {
		respondWithError(w, http.StatusBadRequest, "File parameter is required")
		return
	}

	// Check if file exists in database
	row := db.QueryRow("SELECT name, size FROM files WHERE codebase_id = $1 AND path = $2", uuid, filePath)
	var f File
	err := row.Scan(&f.Name, &f.Size)
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "File not found",
		})
		return
	}

	// Get file content from Server B
	url := fmt.Sprintf("%s/download/%s/%s", serverBURL, uuid, filePath)
	resp, err := http.Get(url)
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Error retrieving file from Server B",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "File not found on Server B",
		})
		return
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Error reading file content",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"file": map[string]interface{}{
			"path":    filePath,
			"name":    f.Name,
			"size":    f.Size,
			"content": string(content),
		},
	})
}

func DownloadFileByQueryHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	filePath := r.URL.Query().Get("file")

	if filePath == "" {
		respondWithError(w, http.StatusBadRequest, "File parameter is required")
		return
	}

	url := fmt.Sprintf("%s/download/%s/%s", serverBURL, uuid, filePath)
	resp, err := http.Get(url)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving file from Server B")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	io.Copy(w, resp.Body)
}

func DownloadZipByQueryHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	url := fmt.Sprintf("%s/download-zip/%s", serverBURL, uuid)

	resp, err := http.Get(url)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving ZIP from Server B")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respondWithError(w, http.StatusNotFound, "ZIP not found")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=codebase-"+uuid+".zip")
	io.Copy(w, resp.Body)
}

func enableCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}

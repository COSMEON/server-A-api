package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const serverBURL = "http://localhost:8081"

type UploadResponse struct {
	UUID string `json:"uuid"`
}

/*func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}*/

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["files"]
	codebaseUUID := uuid.New().String()

	_, err := db.Exec("INSERT INTO codebases (uuid, name) VALUES ($1, $2)", codebaseUUID, "UploadedCodebase")
	if err != nil {
		http.Error(w, "Database error", 500)
		//respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		part, _ := writer.CreateFormFile("files", fileHeader.Filename)
		io.Copy(part, file)
		file.Close()

		// Insert metadata into SQL
		_, err = db.Exec("INSERT INTO files (codebase_id, path, name, size) VALUES ($1, $2, $3, $4)",
			codebaseUUID, fileHeader.Filename, filepath.Base(fileHeader.Filename), fileHeader.Size)
		if err != nil {
			log.Println("DB insert error:", err)
		}
	}
	writer.Close()

	// Send to Server B
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/upload/%s", serverBURL, codebaseUUID), body)
	if err != nil {
		http.Error(w, "Request creation failed", 500)
		//respondWithError(w, http.StatusInternalServerError, "Request creation failed")
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Upload to Server B failed", 500)
		//respondWithError(w, http.StatusInternalServerError, "Upload to Server B failed")
		return
	}
	defer resp.Body.Close()

	json.NewEncoder(w).Encode(UploadResponse{UUID: codebaseUUID})
}

func ListCodebasesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT uuid, name FROM codebases")
	if err != nil {
		http.Error(w, "DB error", 500)
		//respondWithError(w, http.StatusInternalServerError, "DB error")
		return
	}
	defer rows.Close()

	var codebases []Codebase
	for rows.Next() {
		var cb Codebase
		rows.Scan(&cb.UUID, &cb.Name)
		codebases = append(codebases, cb)
	}
	json.NewEncoder(w).Encode(codebases)
}

func GetCodebaseDetailsHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	rows, err := db.Query("SELECT path, name, size FROM files WHERE codebase_id = $1", uuid)
	if err != nil {
		http.Error(w, "DB error", 500)
		//respondWithError(w, http.StatusInternalServerError, "DB error")
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		rows.Scan(&f.Path, &f.Name, &f.Size)
		files = append(files, f)
	}
	json.NewEncoder(w).Encode(files)
}

func GetFileMetadataHandler(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	path := mux.Vars(r)["filepath"]
	row := db.QueryRow("SELECT name, size FROM files WHERE codebase_id = $1 AND path = $2", uuid, path)
	var f File
	err := row.Scan(&f.Name, &f.Size)
	if err != nil {
		http.Error(w, "File not found", 404)
		//respondWithError(w, http.StatusNotFound, "File not found")
		return
	}
	f.Path = path
	json.NewEncoder(w).Encode(f)
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	codebaseUUID := mux.Vars(r)["uuid"]
	filePath := mux.Vars(r)["filepath"]

	url := fmt.Sprintf("%s/download/%s/%s", serverBURL, codebaseUUID, filePath)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Error retrieving file from Server B", 500)
		//respondWithError(w, http.StatusInternalServerError, "Error retrieving file from Server B")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

func DownloadZipHandler(w http.ResponseWriter, r *http.Request) {
	codebaseUUID := mux.Vars(r)["uuid"]
	url := fmt.Sprintf("%s/download-zip/%s", serverBURL, codebaseUUID)

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Error retrieving ZIP from Server B", 500)
		//respondWithError(w, http.StatusInternalServerError, "Error retrieving ZIP from Server B")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+codebaseUUID+".zip")
	w.Header().Set("Content-Type", "application/zip")
	io.Copy(w, resp.Body)
}

func enableCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}
		h.ServeHTTP(w, r)
	})
}

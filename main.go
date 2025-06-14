package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize database
	InitDB()

	// Create router
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/health", HealthCheckHandler).Methods("GET")
	r.HandleFunc("/upload", UploadHandler).Methods("POST")
	r.HandleFunc("/codebases", ListCodebasesHandler).Methods("GET")
	r.HandleFunc("/codebases/{uuid}", GetCodebaseDetailsHandler).Methods("GET")
	r.HandleFunc("/codebases/{uuid}/content", GetFileContentHandler).Methods("GET")
	r.HandleFunc("/codebases/{uuid}/download", DownloadFileByQueryHandler).Methods("GET")
	r.HandleFunc("/codebases/{uuid}/zip", DownloadZipByQueryHandler).Methods("GET")

	// Alternative routes (path-based)
	r.HandleFunc("/file/{uuid}/{filepath:.*}", GetFileMetadataHandler).Methods("GET")
	r.HandleFunc("/download/{uuid}/{filepath:.*}", DownloadFileHandler).Methods("GET")
	r.HandleFunc("/download-zip/{uuid}", DownloadZipHandler).Methods("GET")

	// Serve static files (HTML, CSS, JS)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./")))

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Apply CORS middleware
	handler := enableCORS(r)

	log.Printf("Server A running on http://localhost:%s", port)
	log.Printf("Server B URL: %s", getServerBURL())
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

// SetupRoutes configures all the API routes
func SetupRoutes(server *Server) *mux.Router {
	r := mux.NewRouter()
	r.Use(enableCORS)

	// API routes
	r.HandleFunc("/upload", server.uploadCodebase).Methods("POST", "OPTIONS")
	r.HandleFunc("/codebases", server.listCodebases).Methods("GET")
	r.HandleFunc("/codebases/{id}", server.getCodebaseFiles).Methods("GET")
	r.HandleFunc("/codebases/{id}/content", server.readFileContent).Methods("GET")
	r.HandleFunc("/codebases/{id}/download", server.downloadFile).Methods("GET")
	r.HandleFunc("/codebases/{id}/zip", server.downloadZip).Methods("GET")
	r.HandleFunc("/health", server.healthCheck).Methods("GET")

	// Serve static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	return r
}

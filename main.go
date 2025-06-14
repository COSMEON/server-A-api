package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	InitDB()

	r := mux.NewRouter()
	r.HandleFunc("/health", HealthCheckHandler).Methods("GET")
	r.HandleFunc("/upload", UploadHandler).Methods("POST")
	r.HandleFunc("/codebases", ListCodebasesHandler).Methods("GET")
	r.HandleFunc("/codebases/{uuid}", GetCodebaseDetailsHandler).Methods("GET")
	r.HandleFunc("/file/{uuid}/{filepath:.*}", GetFileMetadataHandler).Methods("GET")
	r.HandleFunc("/download/{uuid}/{filepath:.*}", DownloadFileHandler).Methods("GET")
	r.HandleFunc("/download-zip/{uuid}", DownloadZipHandler).Methods("GET")

	http.Handle("/", enableCORS(r))
	log.Println("Server A running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

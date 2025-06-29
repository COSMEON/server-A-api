package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	server := NewServer()
	defer server.db.Close()

	router := SetupRoutes(server)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server A starting on port %s", port)
	log.Printf("Storage server URL: %s", server.storageServerURL)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

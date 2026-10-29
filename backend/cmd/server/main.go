package main

import (
	"backend/internal/api"
	"backend/internal/config"
	"log"
	"net/http"
)

func main() {
	// Load configuration
	_, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Println("Config loaded successfully")

	// Create a new HTTP server
	mux := api.NewMux()
	log.Println("Routes registered")

	log.Println("Starting server on :8000")
	err = http.ListenAndServe(":8000", mux)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

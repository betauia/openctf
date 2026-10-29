package main

import (
	"log"
	"backend/internal/config"
	// "backend/internal/db"
)

func main() {
	// Load configuration
	_, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Println("Config loaded successfully")

	// Test if environment variables are set
	log.Println("Testing environment variables...")
	log.Printf("DATABASE_URL: %s", config.GetEnv("DATABASE_URL"))
	log.Printf("INSTANCE_PROVIDER: %s", config.GetEnv("INSTANCE_PROVIDER"))
	log.Printf("SECRET_KEY: %s", config.GetEnv("SECRET_KEY"))

}

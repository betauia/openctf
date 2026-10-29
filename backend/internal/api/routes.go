package api

import (
	"backend/internal/health"
	"log"
	"net/http"
)

func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	log.Println("Registering routes")

	// Register routes
	// auth.RegisterAuthRoutes(mux)
	health.RegisterHealthRoutes(mux)

	return mux
}

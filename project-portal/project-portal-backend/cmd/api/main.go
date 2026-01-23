package main

import (
	"carbon-scribe/project-portal/project-portal-backend/internal/auth"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	mux := http.NewServeMux()

	// Create a new Gin engine
	r := gin.Default()

	// Initialize the Auth service and handler
	authService := auth.NewAuthService()
	authHandler := auth.NewHandler(authService)

	// Register all auth routes
	auth.RegisterRoutes(mux, authHandler)

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))

	// Start server
	r.Run(":8080")
}

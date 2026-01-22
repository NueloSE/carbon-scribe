package main

import (
	"log"
	"os"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"carbon-scribe/project-portal/project-portal-backend/internal/compliance/audit"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Database Connection
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=carbon_scribe port=5432 sslmode=disable"
	}
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 2. Initialize Repositories & Services
	complianceRepo := compliance.NewRepository(db)
	complianceService := compliance.NewService(complianceRepo)
	complianceHandler := compliance.NewHandler(complianceService)

	// 3. Setup Router
	r := gin.Default()
	
	// Middleware
	r.Use(audit.Middleware(complianceService))

	// Register Routes
	api := r.Group("/api/v1")
	complianceHandler.RegisterRoutes(api)

	// 4. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

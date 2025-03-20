package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"sample-golang/pkg/api"
	"sample-golang/pkg/clients/airtable"
	"sample-golang/pkg/clients/shortio"
	"sample-golang/pkg/clients/textmagic"
	"sample-golang/pkg/config"
	"sample-golang/pkg/middleware"
	"sample-golang/pkg/services"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize API clients
	textMagicClient := textmagic.NewClient(cfg.TextMagicUsername, cfg.TextMagicAPIKey)
	airtableClient := airtable.NewClient(cfg.AirtableAPIKey, cfg.AirtableBaseID)
	shortIOClient := shortio.NewClient(cfg.ShortIOAPIKey, cfg.ShortIODomain)

	// Initialize services
	submissionService := services.NewLandingSubmissionService(
		textMagicClient,
		airtableClient,
		shortIOClient,
		cfg,
	)

	// Set Gin to release mode in production
	gin.SetMode(gin.DebugMode)

	// Create a new Gin router with default middleware
	router := gin.Default()

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Initialize handlers
	handlers := api.NewHandlers(submissionService)

	// Register routes
	router.POST("/webhook/framer-submission", handlers.HandleLandingSubmission)
	router.GET("/health", handlers.HealthCheck)

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start the server
	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

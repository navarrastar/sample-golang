package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"sample-golang/pkg/models"
	"sample-golang/pkg/services"
)

// Handlers contains all HTTP handlers for the API
type Handlers struct {
	submissionService services.LandingSubmissionService
}

// NewHandlers creates a new Handlers instance
func NewHandlers(submissionService services.LandingSubmissionService) *Handlers {
	return &Handlers{
		submissionService: submissionService,
	}
}

// HealthCheck handler for monitoring
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// Processes incoming webhook requests from Framer
func (h *Handlers) HandleLandingSubmission(c *gin.Context) {
	var landingData models.LandingFormData

	// Read the request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request"})
		return
	}

	// Log the raw request for debugging
	log.Printf("Received webhook body: %s", string(body))

	// Bind JSON to struct
	if err := json.Unmarshal(body, &landingData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Validate required fields
	if landingData.First == "" || landingData.Last == "" || landingData.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	// Process the form data
	go h.submissionService.ProcessLandingSubmission(landingData)

	// Send a success response
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Form submission received and processing",
	})
}

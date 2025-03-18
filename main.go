package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/verify/v2"
)

// Twilio configuration
var TWILIO_ACCOUNT_SID string = os.Getenv("TWILIO_ACCOUNT_SID")
var TWILIO_AUTH_TOKEN string = os.Getenv("TWILIO_AUTH_TOKEN")
var VERIFY_SERVICE_SID string = os.Getenv("VERIFY_SERVICE_SID")

var client *twilio.RestClient

// Registration request structure
type RegistrationRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	PhoneNumber string `json:"phoneNumber"`
}

// OTP verification request structure
type VerificationRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTP         string `json:"otp"`
}

func main() {
	// Initialize Twilio client
	client = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: TWILIO_ACCOUNT_SID,
		Password: TWILIO_AUTH_TOKEN,
	})

	// Create a Gin router
	r := gin.Default()

	// Configure CORS to allow requests from your Framer app
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // In production, restrict this to your app's domain
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Rate limiting middleware (simple implementation)
	phoneRateLimits := make(map[string]time.Time)

	// Define API endpoints
	r.POST("/send-otp", func(c *gin.Context) {
		var req RegistrationRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request format"})
			return
		}

		// Basic validation
		if req.PhoneNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Phone number is required"})
			return
		}

		// Check rate limit (3 minutes between requests for the same number)
		lastRequest, exists := phoneRateLimits[req.PhoneNumber]
		if exists && time.Since(lastRequest) < 3*time.Minute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":    false,
				"error":      "Please wait before requesting another code",
				"retryAfter": int((3*time.Minute - time.Since(lastRequest)).Seconds()),
			})
			return
		}

		// Update rate limit timestamp
		phoneRateLimits[req.PhoneNumber] = time.Now()

		// Send verification via Twilio
		params := &openapi.CreateVerificationParams{}
		params.SetTo(req.PhoneNumber)
		params.SetChannel("sms")

		resp, err := client.VerifyV2.CreateVerification(VERIFY_SERVICE_SID, params)
		if err != nil {
			log.Printf("Twilio error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to send verification code"})
			return
		}

		log.Printf("Sent verification to %s, SID: %s", req.PhoneNumber, *resp.Sid)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.POST("/verify-otp", func(c *gin.Context) {
		var req VerificationRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"verified": false, "error": "Invalid request format"})
			return
		}

		// Basic validation
		if req.PhoneNumber == "" || req.OTP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"verified": false, "error": "Phone number and OTP are required"})
			return
		}

		// Verify the code with Twilio
		params := &openapi.CreateVerificationCheckParams{}
		params.SetTo(req.PhoneNumber)
		params.SetCode(req.OTP)

		resp, err := client.VerifyV2.CreateVerificationCheck(VERIFY_SERVICE_SID, params)
		if err != nil {
			log.Printf("Twilio error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"verified": false, "error": "Verification failed"})
			return
		}

		// Check if verification was successful
		if *resp.Status == "approved" {
			log.Printf("Verification successful for %s", req.PhoneNumber)

			// Here you would typically:
			// 1. Create the user account in your database
			// 2. Start a session or generate a JWT token
			// 3. Return the token or session ID

			c.JSON(http.StatusOK, gin.H{
				"verified": true,
				"user": gin.H{
					"phoneNumber": req.PhoneNumber,
					// Include other user details as needed
				},
			})
		} else {
			log.Printf("Verification failed for %s, status: %s", req.PhoneNumber, *resp.Status)
			c.JSON(http.StatusOK, gin.H{"verified": false, "error": "Invalid verification code"})
		}
	})

	// Start the server
	fmt.Println("Server starting on port 8080...")
	r.Run(":8080")
}

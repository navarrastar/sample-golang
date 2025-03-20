package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"bytes"

	"github.com/gin-gonic/gin"

	"github.com/joho/godotenv"
)

// Global variables for clients and config
var (
	textMagicClient TextMagicClient
	airtableClient  AirtableClient
	shortIOClient   ShortIOClient
	config          Config
)

// RawFormData represents the actual data structure coming from Framer form
type RawFormData struct {
	First string `json:"first" binding:"required"`
	Last  string `json:"last" binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

// ProcessedFormData represents the processed data after transformations
type ProcessedFormData struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	ID        string `json:"id"` // Hashed phone number
}

// Configuration struct
type Config struct {
	TextMagicAPIKey      string
	TextMagicUsername    string
	AirtableAPIKey       string
	AirtableBaseID       string
	AirtablePartialTable string
	AirtableR2ETable     string
	ShortIOAPIKey        string
	ShortIODomain        string
}

// API client interfaces
type TextMagicClient interface {
	GetOrCreateContact(phone, firstName, lastName string) (string, error)
	SendMessage(contactID, message string) error
}

type AirtableClient interface {
	RecordExists(table, phoneHash string) (bool, error)
	CreateRecord(table string, data map[string]interface{}) error
}

type ShortIOClient interface {
	CreateShortLink(originalURL string) (string, error)
}

// hashString creates a SHA-256 hash of the input string
func hashString(input string) string {
	// Create a new SHA-256 hash
	h := sha256.New()
	h.Write([]byte(input))

	// Return the hex-encoded hash
	return hex.EncodeToString(h.Sum(nil))
}

// TextMagic client implementation
type textMagicClientImpl struct {
	apiKey   string
	username string
	baseURL  string
}

func NewTextMagicClient(username, apiKey string) TextMagicClient {
	return &textMagicClientImpl{
		apiKey:   apiKey,
		username: username,
		baseURL:  "https://rest.textmagic.com/api/v2",
	}
}

func (c *textMagicClientImpl) GetOrCreateContact(phone, firstName, lastName string) (string, error) {
	// First, try to search for existing contact by phone number
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	// Add "1" to the beginning of the phone number if not already present
	if !strings.HasPrefix(phone, "1") {
		phone = "1" + phone
	}

	fmt.Println("Phone number after cleaning:", phone)

	searchURL := fmt.Sprintf("%s/contacts/search?query=%s", c.baseURL, url.QueryEscape(phone))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication headers
	req.SetBasicAuth(c.username, c.apiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error searching for contact: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from TextMagic API: %s", string(body))
	}

	// Parse response
	var searchResponse struct {
		Page      int `json:"page"`
		Limit     int `json:"limit"`
		Total     int `json:"total"`
		Resources []struct {
			ID int `json:"id"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	// If contact exists, return the ID
	if searchResponse.Total > 0 {
		contactID := fmt.Sprintf("%d", searchResponse.Resources[0].ID)
		log.Printf("Found existing TextMagic contact with ID: %s", contactID)
		return contactID, nil
	}

	// Create new contact if not found
	createURL := fmt.Sprintf("%s/contacts", c.baseURL)

	// Create payload
	payload := map[string]interface{}{
		"phone":     phone,
		"firstName": firstName,
		"lastName":  lastName,
		"lists":     "4344890", // Customers List ID
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating payload: %w", err)
	}

	createReq, err := http.NewRequest("POST", createURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication headers
	createReq.SetBasicAuth(c.username, c.apiKey)
	createReq.Header.Add("Content-Type", "application/json")

	createResp, err := client.Do(createReq)
	if err != nil {
		return "", fmt.Errorf("error creating contact: %w", err)
	}
	defer createResp.Body.Close()

	// Read response body
	createBody, err := io.ReadAll(createResp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// Check for duplicate contact error (400 status code)
	if createResp.StatusCode == http.StatusBadRequest {
		// Parse error response
		var errorResponse struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Errors  struct {
				Fields struct {
					Phone []string `json:"phone"`
				} `json:"fields"`
			} `json:"errors"`
		}

		if err := json.Unmarshal(createBody, &errorResponse); err == nil {
			// Check if this is the "Phone number already exists" error
			for _, msg := range errorResponse.Errors.Fields.Phone {
				if strings.Contains(msg, "already exists in your contacts") {
					// Search again to get the ID of the existing contact
					return c.findContactByPhone(phone)
				}
			}
		}

		return "", fmt.Errorf("error from TextMagic API: %s", string(createBody))
	}

	if createResp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("error from TextMagic API: %s", string(createBody))
	}

	// Parse response
	var createResponse struct {
		ID int `json:"id"`
	}

	if err := json.Unmarshal(createBody, &createResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	contactID := fmt.Sprintf("%d", createResponse.ID)
	log.Printf("Created new TextMagic contact with ID: %s", contactID)
	return contactID, nil
}

// Helper function to find a contact by phone number
func (c *textMagicClientImpl) findContactByPhone(phone string) (string, error) {
	searchURL := fmt.Sprintf("%s/contacts/search?query=%s", c.baseURL, url.QueryEscape(phone))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error searching for contact: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from TextMagic API: %s", string(body))
	}

	var searchResponse struct {
		Resources []struct {
			ID int `json:"id"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if len(searchResponse.Resources) == 0 {
		return "", fmt.Errorf("contact with phone %s not found", phone)
	}

	contactID := fmt.Sprintf("%d", searchResponse.Resources[0].ID)
	log.Printf("Found existing TextMagic contact with ID: %s", contactID)
	return contactID, nil
}

func (c *textMagicClientImpl) SendMessage(contactID, message string) error {
	sendURL := fmt.Sprintf("%s/messages", c.baseURL)

	// Create payload
	payload := map[string]interface{}{
		"contacts": contactID,
		"text":     message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating payload: %w", err)
	}

	req, err := http.NewRequest("POST", sendURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication headers
	req.SetBasicAuth(c.username, c.apiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error from TextMagic API: %s", string(body))
	}

	log.Printf("Successfully sent message to contact ID: %s", contactID)
	return nil
}

// Airtable client implementation
type airtableClientImpl struct {
	apiKey string
	baseID string
}

func NewAirtableClient(apiKey, baseID string) AirtableClient {
	return &airtableClientImpl{
		apiKey: apiKey,
		baseID: baseID,
	}
}

func (c *airtableClientImpl) RecordExists(table, phoneHash string) (bool, error) {
	// URL for filtering records by phone hash
	url := fmt.Sprintf("https://api.airtable.com/v0/%s/%s?filterByFormula={hash}=\"%s\"",
		c.baseID, url.PathEscape(table), url.QueryEscape(phoneHash))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication header
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("error checking Airtable: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("error from Airtable API: %s", string(body))
	}

	// Parse response
	var response struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("error parsing response: %w", err)
	}

	// Record exists if we got any records back
	exists := len(response.Records) > 0
	log.Printf("Airtable record check for hash %s in table %s: exists=%v", phoneHash, table, exists)

	return exists, nil
}

func (c *airtableClientImpl) CreateRecord(table string, data map[string]interface{}) error {
	url := fmt.Sprintf("https://api.airtable.com/v0/%s/%s", c.baseID, url.PathEscape(table))

	// Format data for Airtable API
	payload := map[string]interface{}{
		"records": []map[string]interface{}{
			{
				"fields": data,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication and content type headers
	req.Header.Add("Authorization", "Bearer "+c.apiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error creating Airtable record: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error from Airtable API: %s", string(body))
	}

	log.Printf("Successfully created record in Airtable table: %s", table)
	return nil
}

// Short.io client implementation
type shortIOClientImpl struct {
	apiKey string
	domain string
}

func NewShortIOClient(apiKey, domain string) ShortIOClient {
	return &shortIOClientImpl{
		apiKey: apiKey,
		domain: domain,
	}
}

func (c *shortIOClientImpl) CreateShortLink(originalURL string) (string, error) {
	url := "https://api.short.io/links"

	// Create payload
	payload := map[string]interface{}{
		"originalURL": originalURL,
		"domain":      c.domain,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("Authorization", c.apiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error creating short link: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("error from Short.io API: %s", string(body))
	}

	// Parse response
	var response struct {
		ShortURL string `json:"shortURL"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	log.Printf("Created short link: %s -> %s", originalURL, response.ShortURL)
	return response.ShortURL, nil
}

func main() {
	// Load configuration from environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	config = Config{
		TextMagicAPIKey:      os.Getenv("TEXTMAGIC_API_KEY"),
		TextMagicUsername:    os.Getenv("TEXTMAGIC_USERNAME"),
		AirtableAPIKey:       os.Getenv("AIRTABLE_API_KEY"),
		AirtableBaseID:       os.Getenv("AIRTABLE_BASE_ID"),
		AirtablePartialTable: os.Getenv("AIRTABLE_PARTIAL_TABLE"),
		AirtableR2ETable:     os.Getenv("AIRTABLE_R2E_TABLE"),
		ShortIOAPIKey:        os.Getenv("SHORTIO_API_KEY"),
		ShortIODomain:        os.Getenv("SHORTIO_DOMAIN"),
	}

	// Initialize API clients
	textMagicClient = NewTextMagicClient(config.TextMagicUsername, config.TextMagicAPIKey)
	airtableClient = NewAirtableClient(config.AirtableAPIKey, config.AirtableBaseID)
	shortIOClient = NewShortIOClient(config.ShortIOAPIKey, config.ShortIODomain)

	// Set Gin to release mode in production
	gin.SetMode(gin.DebugMode)

	// Create a new Gin router with default middleware
	// Logger and Recovery middleware already attached
	router := gin.Default()

	// Add CORS middleware
	router.Use(corsMiddleware())

	// Register routes
	router.POST("/webhook/framer-submission", handleFramerSubmission)
	router.GET("/health", healthCheck)

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

// corsMiddleware handles CORS preflight requests
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// healthCheck handler for monitoring
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// handleFramerSubmission processes incoming webhook requests from Framer
func handleFramerSubmission(c *gin.Context) {
	// Define the raw form data structure
	var rawData RawFormData

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
	if err := json.Unmarshal(body, &rawData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Validate required fields
	if rawData.First == "" || rawData.Last == "" || rawData.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	// Process the form data
	go processSubmission(rawData)

	// Send a success response
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Form submission received and processing",
	})
}

// Extended data processing function
func processSubmission(data RawFormData) {
	// Hash the phone number
	phoneHash := hashString(data.Phone)

	log.Printf("Processing submission for %s %s (%s)", data.First, data.Last, phoneHash)

	// Get or create TextMagic contact
	textMagicContactID, err := textMagicClient.GetOrCreateContact(data.Phone, data.First, data.Last)
	if err != nil {
		log.Printf("Error with TextMagic API: %v", err)
		return
	}

	// Check if record exists in Partial table
	exists, err := airtableClient.RecordExists(config.AirtablePartialTable, phoneHash)
	if err != nil {
		log.Printf("Error checking Partial table: %v", err)
		return
	}

	if !exists {
		// Parse the TextMagic contact ID as an integer for Airtable
		contactIDInt, err := strconv.ParseInt(textMagicContactID, 10, 64)
		if err != nil {
			log.Printf("Error converting contact ID to number: %v", err)
			return
		}

		// Create new record in Airtable
		record := map[string]interface{}{
			"first":      data.First,
			"last":       data.Last,
			"phone":      data.Phone,
			"hash":       phoneHash,
			"Contact ID": contactIDInt, // Now sending as integer, not string
		}

		if err := airtableClient.CreateRecord(config.AirtablePartialTable, record); err != nil {
			log.Printf("Error creating Airtable record: %v", err)
			return
		}

		// Set timer
		go func(phoneHash, firstName, lastName, contactID string) {
			log.Printf("Setting timer for %s", phoneHash)
			// Wait for 15 minutes
			time.Sleep(15 * time.Minute)

			// Check if record exists in R2E table
			exists, err := airtableClient.RecordExists(config.AirtableR2ETable, phoneHash)
			if err != nil {
				log.Printf("Error checking second Airtable table: %v", err)
				return
			}

			if !exists {
				// Create Short.io link
				params := url.Values{}
				params.Add("first", firstName)
				params.Add("last", lastName)
				params.Add("id", phoneHash)

				targetURL := fmt.Sprintf("https://forms.democracyOS.com/t/bj1RaePxL2us?%s", params.Encode())
				shortLink, err := shortIOClient.CreateShortLink(targetURL)
				if err != nil {
					log.Printf("Error creating short link: %v", err)
					return
				}

				// Send message via TextMagic
				message := fmt.Sprintf("Hello %s! Finish signing up for DemocracyOS here: %s", firstName, shortLink)
				if err := textMagicClient.SendMessage(contactID, message); err != nil {
					log.Printf("Error sending message: %v", err)
					return
				}

				log.Printf("Successfully sent message with short link to %s %s", firstName, lastName)
			} else {
				log.Printf("Skipping message for %s as they already exist in the R2E table", phoneHash)
			}
		}(phoneHash, data.First, data.Last, textMagicContactID)
	} else {
		log.Printf("Skipping processing for %s as they already exist in the Partial table", phoneHash)
	}
}

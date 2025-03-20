package textmagic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Client defines the interface for interacting with TextMagic API
type Client interface {
	GetOrCreateContact(phone, firstName, lastName string) (string, error)
	SendMessage(contactID, message string) error
}

type clientImpl struct {
	apiKey   string
	username string
	baseURL  string
}

// NewClient creates a new TextMagic client
func NewClient(username, apiKey string) Client {
	return &clientImpl{
		apiKey:   apiKey,
		username: username,
		baseURL:  "https://rest.textmagic.com/api/v2",
	}
}

func (c *clientImpl) GetOrCreateContact(phone, firstName, lastName string) (string, error) {
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
func (c *clientImpl) findContactByPhone(phone string) (string, error) {
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

func (c *clientImpl) SendMessage(contactID, message string) error {
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

package shortio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Client defines the interface for interacting with Short.io API
type Client interface {
	CreateShortLink(originalURL string) (string, error)
}

type clientImpl struct {
	apiKey string
	domain string
}

// NewClient creates a new Short.io client
func NewClient(apiKey, domain string) Client {
	return &clientImpl{
		apiKey: apiKey,
		domain: domain,
	}
}

func (c *clientImpl) CreateShortLink(originalURL string) (string, error) {
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

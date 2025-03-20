package airtable

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// Client defines the interface for interacting with Airtable API
type Client interface {
	RecordExists(table, phoneHash string) (bool, error)
	CreateRecord(table string, data map[string]interface{}) error
}

type clientImpl struct {
	apiKey string
	baseID string
}

// NewClient creates a new Airtable client
func NewClient(apiKey, baseID string) Client {
	return &clientImpl{
		apiKey: apiKey,
		baseID: baseID,
	}
}

func (c *clientImpl) RecordExists(table, phoneHash string) (bool, error) {
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

func (c *clientImpl) CreateRecord(table string, data map[string]interface{}) error {
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

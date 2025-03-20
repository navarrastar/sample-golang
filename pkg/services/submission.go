package services

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"sample-golang/pkg/clients/airtable"
	"sample-golang/pkg/clients/shortio"
	"sample-golang/pkg/clients/textmagic"
	"sample-golang/pkg/config"
	"sample-golang/pkg/models"
	"sample-golang/pkg/utils"
)

// LandingSubmissionService defines the interface for handling form submissions
type LandingSubmissionService interface {
	ProcessLandingSubmission(data models.LandingFormData)
}

type landingSubmissionServiceImpl struct {
	textMagicClient textmagic.Client
	airtableClient  airtable.Client
	shortIOClient   shortio.Client
	config          *config.Config
}

// NewLandingSubmissionService creates a new submission service
func NewLandingSubmissionService(
	textMagicClient textmagic.Client,
	airtableClient airtable.Client,
	shortIOClient shortio.Client,
	config *config.Config,
) LandingSubmissionService {
	return &landingSubmissionServiceImpl{
		textMagicClient: textMagicClient,
		airtableClient:  airtableClient,
		shortIOClient:   shortIOClient,
		config:          config,
	}
}

// ProcessLandingSubmission handles the entire submission workflow
func (s *landingSubmissionServiceImpl) ProcessLandingSubmission(data models.LandingFormData) {
	// Hash the phone number
	phoneHash := utils.HashString(data.Phone)

	log.Printf("Processing submission for %s %s (%s)", data.First, data.Last, phoneHash)

	// Get or create TextMagic contact
	textMagicContactID, err := s.textMagicClient.GetOrCreateContact(data.Phone, data.First, data.Last)
	if err != nil {
		log.Printf("Error with TextMagic API: %v", err)
		return
	}

	// Check if record exists in Partial table
	existsInPartial, err := s.airtableClient.RecordExists(s.config.AirtablePartialTable, phoneHash)
	if err != nil {
		log.Printf("Error checking Partial table: %v", err)
		return
	}

	// Check if record exists in R2E table
	existsInR2E, err := s.airtableClient.RecordExists(s.config.AirtableR2ETable, phoneHash)
	if err != nil {
		log.Printf("Error checking R2E table: %v", err)
		return
	}

	if !existsInPartial && !existsInR2E {
		// Parse the TextMagic contact ID as an integer for Airtable
		contactIDInt, err := strconv.ParseInt(textMagicContactID, 10, 64)
		if err != nil {
			log.Printf("Error converting contact ID to number: %v", err)
			return
		}

		// Create new record in partial
		record := map[string]interface{}{
			"first":      data.First,
			"last":       data.Last,
			"phone":      data.Phone,
			"hash":       phoneHash,
			"Contact ID": contactIDInt, // Sending as integer, not string
		}

		if err := s.airtableClient.CreateRecord(s.config.AirtablePartialTable, record); err != nil {
			log.Printf("Error creating Airtable record: %v", err)
			return
		}

		// Set timer
		go s.scheduleFollowup(phoneHash, data.First, data.Last, textMagicContactID)

	} else if existsInPartial && existsInR2E {
		log.Printf("Skipping processing for %s as they already exist in both R2E and Partial tables", phoneHash)
	} else if existsInPartial {
		log.Printf("Skipping processing for %s as they already exist in the Partial table", phoneHash)
	} else if existsInR2E {
		log.Printf("Skipping processing for %s as they already exist in the R2E table", phoneHash)
	}
}

// scheduleFollowup waits 15 minutes then checks if the user needs a followup message
func (s *landingSubmissionServiceImpl) scheduleFollowup(phoneHash, firstName, lastName, contactID string) {
	log.Printf("Setting timer for %s", phoneHash)
	// Wait for 15 minutes
	time.Sleep(15 * time.Minute)

	// Check if record exists in R2E table
	existsInR2E, err := s.airtableClient.RecordExists(s.config.AirtableR2ETable, phoneHash)
	if err != nil {
		log.Printf("Error checking second Airtable table: %v", err)
		return
	}

	if !existsInR2E {
		// Create Short.io link
		params := url.Values{}
		params.Add("first", firstName)
		params.Add("last", lastName)
		params.Add("id", phoneHash)

		targetURL := fmt.Sprintf("https://forms.democracyOS.com/t/bj1RaePxL2us?%s", params.Encode())
		shortLink, err := s.shortIOClient.CreateShortLink(targetURL)
		if err != nil {
			log.Printf("Error creating short link: %v", err)
			return
		}

		// Send message via TextMagic
		message := fmt.Sprintf("Hello %s! Finish signing up for DemocracyOS here: %s", firstName, shortLink)
		if err := s.textMagicClient.SendMessage(contactID, message); err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		log.Printf("Successfully sent reminder to %s %s", firstName, lastName)
	} else {
		log.Printf("Skipping message for %s as they already exist in the R2E table", phoneHash)
	}
}

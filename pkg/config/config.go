package config

import (
	"os"
)

// Config holds all application configuration values
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

// LoadConfig reads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		TextMagicAPIKey:      os.Getenv("TEXTMAGIC_API_KEY"),
		TextMagicUsername:    os.Getenv("TEXTMAGIC_USERNAME"),
		AirtableAPIKey:       os.Getenv("AIRTABLE_API_KEY"),
		AirtableBaseID:       os.Getenv("AIRTABLE_BASE_ID"),
		AirtablePartialTable: os.Getenv("AIRTABLE_PARTIAL_TABLE"),
		AirtableR2ETable:     os.Getenv("AIRTABLE_R2E_TABLE"),
		ShortIOAPIKey:        os.Getenv("SHORTIO_API_KEY"),
		ShortIODomain:        os.Getenv("SHORTIO_DOMAIN"),
	}
}

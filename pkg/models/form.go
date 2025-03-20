package models

// Represents the data structure coming from Landing Page form
type LandingFormData struct {
	First string `json:"first" binding:"required"`
	Last  string `json:"last" binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

// HashedLandingFormData represents the processed data after transformations
type HashedLandingFormData struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	ID        string `json:"id"` // Hashed phone number
}

package services

import (
	"errors"
	"sync"
	"time"

	"sample-golang/pkg/clients/twilio"
)

var (
	ErrVerificationExpired = errors.New("verification expired")
	ErrInvalidCode         = errors.New("invalid verification code")
)

type PendingVerification struct {
	Phone     string
	Data      interface{}
	ExpiresAt time.Time
}

type VerificationService struct {
	twilioClient twilio.Client
	pending      map[string]*PendingVerification
	mu           sync.RWMutex
	timeout      time.Duration
}

func NewVerificationService(twilioClient twilio.Client) *VerificationService {
	return &VerificationService{
		twilioClient: twilioClient,
		pending:      make(map[string]*PendingVerification),
		timeout:      10 * time.Minute,
	}
}

func (s *VerificationService) InitiateVerification(phone string, data interface{}) error {
	if err := s.twilioClient.SendVerificationCode(phone); err != nil {
		return err
	}

	s.mu.Lock()
	s.pending[phone] = &PendingVerification{
		Phone:     phone,
		Data:      data,
		ExpiresAt: time.Now().Add(s.timeout),
	}
	s.mu.Unlock()

	// Start cleanup goroutine
	go func() {
		time.Sleep(s.timeout)
		s.mu.Lock()
		delete(s.pending, phone)
		s.mu.Unlock()
	}()

	return nil
}

func (s *VerificationService) VerifyCode(phone, code string) (interface{}, error) {
	s.mu.RLock()
	verification, exists := s.pending[phone]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrVerificationExpired
	}

	if time.Now().After(verification.ExpiresAt) {
		s.mu.Lock()
		delete(s.pending, phone)
		s.mu.Unlock()
		return nil, ErrVerificationExpired
	}

	verified, err := s.twilioClient.CheckVerificationCode(phone, code)
	if err != nil {
		return nil, err
	}

	if !verified {
		return nil, ErrInvalidCode
	}

	s.mu.Lock()
	data := verification.Data
	delete(s.pending, phone)
	s.mu.Unlock()

	return data, nil
}

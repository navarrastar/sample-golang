package twilio

import (
	"fmt"
	"log"

	"github.com/twilio/twilio-go"
	verify "github.com/twilio/twilio-go/rest/verify/v2"
)

// Client defines the interface for interacting with Twilio Verify API
type Client interface {
	SendVerificationCode(phoneNumber string) error
	CheckVerificationCode(phoneNumber, code string) (bool, error)
}

type clientImpl struct {
	client    *twilio.RestClient
	serviceID string
}

// NewClient creates a new Twilio client
func NewClient(accountSid, authToken, serviceID string) Client {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSid,
		Password: authToken,
	})

	return &clientImpl{
		client:    client,
		serviceID: serviceID,
	}
}

func (c *clientImpl) SendVerificationCode(phoneNumber string) error {
	params := &verify.CreateVerificationParams{}
	params.SetTo(phoneNumber)
	params.SetChannel("sms")

	resp, err := c.client.VerifyV2.CreateVerification(c.serviceID, params)
	if err != nil {
		return fmt.Errorf("error sending verification code: %w", err)
	}

	log.Printf("Sent verification code to: %s, status: %s", phoneNumber, *resp.Status)
	return nil
}

func (c *clientImpl) CheckVerificationCode(phoneNumber, code string) (bool, error) {
	params := &verify.CreateVerificationCheckParams{}
	params.SetTo(phoneNumber)
	params.SetCode(code)

	resp, err := c.client.VerifyV2.CreateVerificationCheck(c.serviceID, params)
	if err != nil {
		return false, fmt.Errorf("error checking verification code: %w", err)
	}

	verified := *resp.Status == "approved"
	log.Printf("Verification check for %s: %v", phoneNumber, verified)
	return verified, nil
}

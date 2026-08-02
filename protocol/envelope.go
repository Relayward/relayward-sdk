// Package protocol defines the common envelope and error vocabulary used by Relayward messages.
package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/Relayward/relayward-sdk/contract"
)

type Envelope struct {
	APIVersion     string          `json:"api_version"`
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	SentAt         time.Time       `json:"sent_at"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Payload        json.RawMessage `json:"payload"`
}

var (
	idPattern             = regexp.MustCompile(`^[0-9a-f]{32}$`)
	typePattern           = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_-]*)+$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

func NewEnvelope(messageType string, payload any) (Envelope, error) {
	return NewEnvelopeFor(contract.ControlAPIVersion, messageType, payload)
}

func NewEnvelopeFor(apiVersion, messageType string, payload any) (Envelope, error) {
	id, err := NewID()
	if err != nil {
		return Envelope{}, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode payload: %w", err)
	}
	value := Envelope{
		APIVersion: apiVersion,
		ID:         id,
		Type:       messageType,
		SentAt:     time.Now().UTC(),
		Payload:    encoded,
	}
	if err := ValidateEnvelope(value); err != nil {
		return Envelope{}, err
	}
	return value, nil
}

func NewID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate message id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func ValidateIdempotencyKey(value string) error {
	if !idempotencyKeyPattern.MatchString(value) {
		return fmt.Errorf("idempotency_key: invalid key")
	}
	return nil
}

func ValidateEnvelope(value Envelope) error {
	if !contract.IsMessageAPIVersion(value.APIVersion) {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	return validateEnvelopeFields(value)
}

func ValidateEnvelopeVersion(value Envelope, apiVersion string) error {
	if !contract.IsMessageAPIVersion(apiVersion) || value.APIVersion != apiVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	return validateEnvelopeFields(value)
}

func validateEnvelopeFields(value Envelope) error {
	if !idPattern.MatchString(value.ID) {
		return fmt.Errorf("id: must contain 32 lowercase hexadecimal characters")
	}
	if !typePattern.MatchString(value.Type) || len(value.Type) > 128 {
		return fmt.Errorf("type: invalid message type")
	}
	if value.SentAt.IsZero() {
		return fmt.Errorf("sent_at: must be set")
	}
	if value.CorrelationID != "" && !idPattern.MatchString(value.CorrelationID) {
		return fmt.Errorf("correlation_id: invalid message id")
	}
	if value.IdempotencyKey != "" {
		if err := ValidateIdempotencyKey(value.IdempotencyKey); err != nil {
			return err
		}
	}
	if len(value.Payload) == 0 || !json.Valid(value.Payload) {
		return fmt.Errorf("payload: must be valid JSON")
	}
	return nil
}

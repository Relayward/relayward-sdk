package protocol

import (
	"encoding/json"
	"testing"
)

func TestNewEnvelope(t *testing.T) {
	value, err := NewEnvelope("system.hello", map[string]string{"name": "agent"})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := ValidateEnvelope(value); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
	if got := string(value.Payload); got != `{"name":"agent"}` {
		t.Fatalf("payload = %s", got)
	}
}

func TestValidateEnvelopeRejectsInvalidIdempotencyKey(t *testing.T) {
	value, err := NewEnvelope("command.apply", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	value.IdempotencyKey = "contains spaces"
	if err := ValidateEnvelope(value); err == nil {
		t.Fatal("ValidateEnvelope() error = nil, want invalid key error")
	}
}

func TestProblemValidate(t *testing.T) {
	problem := Problem{Code: ErrorUnavailable, Message: "try later", Retryable: true}
	if err := problem.Validate(); err != nil {
		t.Fatalf("Problem.Validate() error = %v", err)
	}
	problem.Code = "other"
	if err := problem.Validate(); err == nil {
		t.Fatal("Problem.Validate() error = nil, want invalid code")
	}
}

package agentv1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/Relayward/relayward-sdk/protocol"
)

const (
	CommandStatusSucceeded = "succeeded"
	CommandStatusFailed    = "failed"

	MaximumCommandPayloadBytes = 512 << 10
	MaximumCommandOutputBytes  = 256 << 10
	MaximumCommandLifetime     = 24 * time.Hour
	MaximumCommandExecution    = 10 * time.Minute
)

var commandKindPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)+$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Command struct {
	Kind      string          `json:"kind"`
	IssuedAt  time.Time       `json:"issued_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Payload   json.RawMessage `json:"payload"`
}

type CommandResult struct {
	CommandID     string            `json:"command_id"`
	RequestSHA256 string            `json:"request_sha256"`
	Status        string            `json:"status"`
	CompletedAt   time.Time         `json:"completed_at"`
	Problem       *protocol.Problem `json:"problem,omitempty"`
	Output        json.RawMessage   `json:"output,omitempty"`
}

type CommandResultAck struct {
	CommandID     string    `json:"command_id"`
	RequestSHA256 string    `json:"request_sha256"`
	ServerTime    time.Time `json:"server_time"`
}

func ValidateCommand(value Command) error {
	if !commandKindPattern.MatchString(value.Kind) || len(value.Kind) > 128 {
		return fmt.Errorf("kind: invalid command kind")
	}
	if value.IssuedAt.IsZero() {
		return fmt.Errorf("issued_at: must be set")
	}
	if value.ExpiresAt.IsZero() || !value.ExpiresAt.After(value.IssuedAt) {
		return fmt.Errorf("expires_at: must be after issued_at")
	}
	if value.ExpiresAt.Sub(value.IssuedAt) > MaximumCommandLifetime {
		return fmt.Errorf("expires_at: command lifetime exceeds %s", MaximumCommandLifetime)
	}
	if len(value.Payload) == 0 || len(value.Payload) > MaximumCommandPayloadBytes || !json.Valid(value.Payload) {
		return fmt.Errorf("payload: must contain at most %d bytes of valid JSON", MaximumCommandPayloadBytes)
	}
	return nil
}

func CommandDigest(value Command) (string, error) {
	if err := ValidateCommand(value); err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode command digest input: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateCommandResult(value CommandResult) error {
	if err := protocol.ValidateIdempotencyKey(value.CommandID); err != nil {
		return fmt.Errorf("command_id: invalid command ID")
	}
	if !digestPattern.MatchString(value.RequestSHA256) {
		return fmt.Errorf("request_sha256: invalid SHA-256 digest")
	}
	if value.CompletedAt.IsZero() {
		return fmt.Errorf("completed_at: must be set")
	}
	switch value.Status {
	case CommandStatusSucceeded:
		if value.Problem != nil {
			return fmt.Errorf("problem: must be absent for a successful command")
		}
	case CommandStatusFailed:
		if value.Problem == nil {
			return fmt.Errorf("problem: required for a failed command")
		}
		if err := value.Problem.Validate(); err != nil {
			return fmt.Errorf("problem: %w", err)
		}
	default:
		return fmt.Errorf("status: unsupported value %q", value.Status)
	}
	if len(value.Output) > MaximumCommandOutputBytes || (len(value.Output) > 0 && !json.Valid(value.Output)) {
		return fmt.Errorf("output: must contain at most %d bytes of valid JSON", MaximumCommandOutputBytes)
	}
	return nil
}

func ValidateCommandResultAck(value CommandResultAck) error {
	if err := protocol.ValidateIdempotencyKey(value.CommandID); err != nil {
		return fmt.Errorf("command_id: invalid command ID")
	}
	if !digestPattern.MatchString(value.RequestSHA256) {
		return fmt.Errorf("request_sha256: invalid SHA-256 digest")
	}
	if value.ServerTime.IsZero() {
		return fmt.Errorf("server_time: must be set")
	}
	return nil
}

func NewCommandEnvelope(commandID string, value Command) (protocol.Envelope, error) {
	if err := protocol.ValidateIdempotencyKey(commandID); err != nil {
		return protocol.Envelope{}, fmt.Errorf("command_id: invalid command ID")
	}
	envelope, err := NewEnvelope(MessageCenterCommand, value)
	if err != nil {
		return protocol.Envelope{}, err
	}
	envelope.IdempotencyKey = commandID
	if err := ValidateEnvelope(envelope); err != nil {
		return protocol.Envelope{}, err
	}
	return envelope, nil
}

func NewCommandResultEnvelope(value CommandResult) (protocol.Envelope, error) {
	if err := ValidateCommandResult(value); err != nil {
		return protocol.Envelope{}, err
	}
	envelope, err := NewEnvelope(MessageAgentCommandResult, value)
	if err != nil {
		return protocol.Envelope{}, err
	}
	envelope.IdempotencyKey = value.CommandID
	if err := ValidateEnvelope(envelope); err != nil {
		return protocol.Envelope{}, err
	}
	return envelope, nil
}

func NewCommandResultAckEnvelope(resultMessageID string, value CommandResultAck) (protocol.Envelope, error) {
	if err := ValidateCommandResultAck(value); err != nil {
		return protocol.Envelope{}, err
	}
	envelope, err := NewEnvelope(MessageCenterCommandResultAck, value)
	if err != nil {
		return protocol.Envelope{}, err
	}
	envelope.CorrelationID = resultMessageID
	if err := ValidateEnvelope(envelope); err != nil {
		return protocol.Envelope{}, err
	}
	return envelope, nil
}

package agentv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Relayward/relayward-sdk/protocol"
)

const (
	MessageAgentHello             = "agent.hello"
	MessageCenterHello            = "center.hello"
	MessageAgentHeartbeat         = "agent.heartbeat"
	MessageCenterHeartbeatAck     = "center.heartbeat_ack"
	MessageCenterCommand          = "center.command"
	MessageAgentCommandResult     = "agent.command_result"
	MessageCenterCommandResultAck = "center.command_result_ack"
	MessageProtocolError          = "protocol.error"

	CapabilityControlHeartbeat = "control.heartbeat"
	CapabilityControlCommands  = "control.commands"
	CapabilityEventQueue       = "event.queue"

	DefaultHeartbeatInterval = 30 * time.Second
	MinimumHeartbeatInterval = 5 * time.Second
	MaximumHeartbeatInterval = 5 * time.Minute
	MaximumMessageBytes      = 1 << 20
)

type AgentHello struct {
	NodeID       string    `json:"node_id"`
	AgentVersion string    `json:"agent_version"`
	StartedAt    time.Time `json:"started_at"`
	Capabilities []string  `json:"capabilities"`
}

type CenterHello struct {
	SessionID                string    `json:"session_id"`
	HeartbeatIntervalSeconds int       `json:"heartbeat_interval_seconds"`
	ServerTime               time.Time `json:"server_time"`
}

type Heartbeat struct {
	SessionID    string    `json:"session_id"`
	AgentVersion string    `json:"agent_version"`
	ObservedAt   time.Time `json:"observed_at"`
}

type HeartbeatAck struct {
	MessageID  string             `json:"message_id"`
	ServerTime time.Time          `json:"server_time"`
	Command    *protocol.Envelope `json:"command,omitempty"`
}

func NewEnvelope(messageType string, payload any) (protocol.Envelope, error) {
	return protocol.NewEnvelopeFor(APIVersion, messageType, payload)
}

func DecodeEnvelopePayload[T any](value protocol.Envelope) (T, error) {
	var payload T
	if err := decodeStrictBytes(value.Payload, &payload); err != nil {
		return payload, fmt.Errorf("decode %s payload: %w", value.Type, err)
	}
	return payload, nil
}

func ValidateEnvelope(value protocol.Envelope) error {
	if err := protocol.ValidateEnvelopeVersion(value, APIVersion); err != nil {
		return err
	}
	switch value.Type {
	case MessageAgentHello:
		payload, err := DecodeEnvelopePayload[AgentHello](value)
		if err != nil {
			return err
		}
		return ValidateAgentHello(payload)
	case MessageCenterHello:
		payload, err := DecodeEnvelopePayload[CenterHello](value)
		if err != nil {
			return err
		}
		return ValidateCenterHello(payload)
	case MessageAgentHeartbeat:
		payload, err := DecodeEnvelopePayload[Heartbeat](value)
		if err != nil {
			return err
		}
		return ValidateHeartbeat(payload)
	case MessageCenterHeartbeatAck:
		payload, err := DecodeEnvelopePayload[HeartbeatAck](value)
		if err != nil {
			return err
		}
		return ValidateHeartbeatAck(payload)
	case MessageCenterCommand:
		payload, err := DecodeEnvelopePayload[Command](value)
		if err != nil {
			return err
		}
		if err := protocol.ValidateIdempotencyKey(value.IdempotencyKey); err != nil {
			return fmt.Errorf("center command: %w", err)
		}
		return ValidateCommand(payload)
	case MessageAgentCommandResult:
		payload, err := DecodeEnvelopePayload[CommandResult](value)
		if err != nil {
			return err
		}
		if value.IdempotencyKey != payload.CommandID {
			return fmt.Errorf("idempotency_key: must match command_id")
		}
		return ValidateCommandResult(payload)
	case MessageCenterCommandResultAck:
		payload, err := DecodeEnvelopePayload[CommandResultAck](value)
		if err != nil {
			return err
		}
		if value.CorrelationID == "" {
			return fmt.Errorf("correlation_id: required for command result acknowledgement")
		}
		return ValidateCommandResultAck(payload)
	case MessageProtocolError:
		payload, err := DecodeEnvelopePayload[protocol.Problem](value)
		if err != nil {
			return err
		}
		return payload.Validate()
	default:
		return fmt.Errorf("type: unsupported Agent message type %q", value.Type)
	}
}

func ValidateAgentHello(value AgentHello) error {
	if err := ValidateNodeID(value.NodeID); err != nil {
		return err
	}
	if err := validateAgentVersion(value.AgentVersion); err != nil {
		return err
	}
	if value.StartedAt.IsZero() {
		return fmt.Errorf("started_at: must be set")
	}
	return validateCapabilities(value.Capabilities)
}

func ValidateCenterHello(value CenterHello) error {
	if !messageIDPattern.MatchString(value.SessionID) {
		return fmt.Errorf("session_id: invalid session ID")
	}
	interval := time.Duration(value.HeartbeatIntervalSeconds) * time.Second
	if interval < MinimumHeartbeatInterval || interval > MaximumHeartbeatInterval {
		return fmt.Errorf("heartbeat_interval_seconds: must be between %d and %d", int(MinimumHeartbeatInterval.Seconds()), int(MaximumHeartbeatInterval.Seconds()))
	}
	if value.ServerTime.IsZero() {
		return fmt.Errorf("server_time: must be set")
	}
	return nil
}

func ValidateHeartbeat(value Heartbeat) error {
	if !messageIDPattern.MatchString(value.SessionID) {
		return fmt.Errorf("session_id: invalid session ID")
	}
	if err := validateAgentVersion(value.AgentVersion); err != nil {
		return err
	}
	if value.ObservedAt.IsZero() {
		return fmt.Errorf("observed_at: must be set")
	}
	return nil
}

func ValidateHeartbeatAck(value HeartbeatAck) error {
	if !messageIDPattern.MatchString(value.MessageID) {
		return fmt.Errorf("message_id: invalid message ID")
	}
	if value.ServerTime.IsZero() {
		return fmt.Errorf("server_time: must be set")
	}
	if value.Command != nil {
		if value.Command.Type != MessageCenterCommand {
			return fmt.Errorf("command: must contain a center command envelope")
		}
		if err := ValidateEnvelope(*value.Command); err != nil {
			return fmt.Errorf("command: %w", err)
		}
	}
	return nil
}

func decodeStrictBytes(raw json.RawMessage, destination any) error {
	return decodeStrict(bytes.NewReader(raw), destination)
}

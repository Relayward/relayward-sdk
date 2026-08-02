package agentv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Relayward/relayward-sdk/contract"
)

const (
	CommandAgentUpdate        = "agent.update"
	AgentUpdateStateActivated = "activated"
)

type AgentUpdateCommand struct {
	Version string `json:"version"`
}

type AgentUpdateOutput struct {
	Version string `json:"version"`
	State   string `json:"state"`
}

func NewAgentUpdateCommand(version string, issuedAt, expiresAt time.Time) (Command, error) {
	payload := AgentUpdateCommand{Version: version}
	if err := ValidateAgentUpdateCommand(payload); err != nil {
		return Command{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Command{}, fmt.Errorf("encode Agent update command: %w", err)
	}
	value := Command{Kind: CommandAgentUpdate, IssuedAt: issuedAt.UTC(), ExpiresAt: expiresAt.UTC(), Payload: raw}
	if err := ValidateCommand(value); err != nil {
		return Command{}, err
	}
	return value, nil
}

func DecodeAgentUpdateCommand(value Command) (AgentUpdateCommand, error) {
	if err := ValidateCommand(value); err != nil {
		return AgentUpdateCommand{}, err
	}
	if value.Kind != CommandAgentUpdate {
		return AgentUpdateCommand{}, fmt.Errorf("kind: must be %q", CommandAgentUpdate)
	}
	var payload AgentUpdateCommand
	if err := decodeStrict(bytes.NewReader(value.Payload), &payload); err != nil {
		return AgentUpdateCommand{}, fmt.Errorf("decode Agent update command: %w", err)
	}
	if err := ValidateAgentUpdateCommand(payload); err != nil {
		return AgentUpdateCommand{}, err
	}
	return payload, nil
}

func ValidateAgentUpdateCommand(value AgentUpdateCommand) error {
	if err := contract.ValidateSemanticVersion(value.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	return nil
}

func EncodeAgentUpdateOutput(value AgentUpdateOutput) (json.RawMessage, error) {
	if err := ValidateAgentUpdateOutput(value); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode Agent update output: %w", err)
	}
	return raw, nil
}

func DecodeAgentUpdateOutput(raw json.RawMessage) (AgentUpdateOutput, error) {
	var value AgentUpdateOutput
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return AgentUpdateOutput{}, fmt.Errorf("decode Agent update output: %w", err)
	}
	if err := ValidateAgentUpdateOutput(value); err != nil {
		return AgentUpdateOutput{}, err
	}
	return value, nil
}

func ValidateAgentUpdateOutput(value AgentUpdateOutput) error {
	if err := contract.ValidateSemanticVersion(value.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	if value.State != AgentUpdateStateActivated {
		return fmt.Errorf("state: unsupported value %q", value.State)
	}
	return nil
}

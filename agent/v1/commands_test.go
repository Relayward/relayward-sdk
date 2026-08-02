package agentv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Relayward/relayward-sdk/protocol"
)

func testCommand() Command {
	issuedAt := time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)
	return Command{
		Kind:      "agent.update",
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(10 * time.Minute),
		Payload:   json.RawMessage(`{"version":"0.2.0"}`),
	}
}

func TestCommandEnvelopeAndDigest(t *testing.T) {
	command := testCommand()
	digest, err := CommandDigest(command)
	if err != nil || digest != "3a91d020b542bec0dc31a88a20f85981909144107f015a07d31522c46b6b667a" {
		t.Fatalf("CommandDigest() = %q, %v", digest, err)
	}
	envelope, err := NewCommandEnvelope("command-1", command)
	if err != nil {
		t.Fatalf("NewCommandEnvelope() error = %v", err)
	}
	if envelope.IdempotencyKey != "command-1" || envelope.Type != MessageCenterCommand {
		t.Fatalf("command envelope = %+v", envelope)
	}
	if err := ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
	decoded, err := DecodeEnvelopePayload[Command](envelope)
	if err != nil {
		t.Fatalf("DecodeEnvelopePayload() error = %v", err)
	}
	decodedDigest, err := CommandDigest(decoded)
	if err != nil || decodedDigest != digest {
		t.Fatalf("decoded digest = %q, %v, want %q", decodedDigest, err, digest)
	}
}

func TestCommandResultAndAcknowledgementEnvelopes(t *testing.T) {
	digest, _ := CommandDigest(testCommand())
	result := CommandResult{
		CommandID: "command-1", RequestSHA256: digest, Status: CommandStatusSucceeded,
		CompletedAt: time.Now().UTC(), Output: json.RawMessage(`{"version":"0.2.0"}`),
	}
	resultEnvelope, err := NewCommandResultEnvelope(result)
	if err != nil {
		t.Fatalf("NewCommandResultEnvelope() error = %v", err)
	}
	ack, err := NewCommandResultAckEnvelope(resultEnvelope.ID, CommandResultAck{
		CommandID: "command-1", RequestSHA256: digest, ServerTime: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewCommandResultAckEnvelope() error = %v", err)
	}
	if ack.CorrelationID != resultEnvelope.ID || ack.Type != MessageCenterCommandResultAck {
		t.Fatalf("command acknowledgement envelope = %+v", ack)
	}
	if err := ValidateEnvelope(ack); err != nil {
		t.Fatalf("ValidateEnvelope(ack) error = %v", err)
	}
	ack.CorrelationID = ""
	if err := ValidateEnvelope(ack); err == nil {
		t.Fatal("ValidateEnvelope() accepted an uncorrelated command result acknowledgement")
	}
}

func TestFailedCommandRequiresValidatedProblem(t *testing.T) {
	digest, _ := CommandDigest(testCommand())
	result := CommandResult{
		CommandID: "command-1", RequestSHA256: digest, Status: CommandStatusFailed,
		CompletedAt: time.Now().UTC(),
	}
	if err := ValidateCommandResult(result); err == nil || !strings.Contains(err.Error(), "problem") {
		t.Fatalf("ValidateCommandResult() error = %v", err)
	}
	result.Problem = &protocol.Problem{Code: protocol.ErrorUnsupported, Message: "unsupported command", Retryable: false}
	if err := ValidateCommandResult(result); err != nil {
		t.Fatalf("ValidateCommandResult() error = %v", err)
	}
	result.Status = CommandStatusSucceeded
	if err := ValidateCommandResult(result); err == nil {
		t.Fatal("ValidateCommandResult() accepted a problem on success")
	}
}

func TestCommandValidationBounds(t *testing.T) {
	command := testCommand()
	command.ExpiresAt = command.IssuedAt.Add(MaximumCommandLifetime + time.Second)
	if err := ValidateCommand(command); err == nil || !strings.Contains(err.Error(), "lifetime") {
		t.Fatalf("ValidateCommand() lifetime error = %v", err)
	}
	command = testCommand()
	command.Payload = json.RawMessage(`{"value":"` + strings.Repeat("x", MaximumCommandPayloadBytes) + `"}`)
	if err := ValidateCommand(command); err == nil || !strings.Contains(err.Error(), "payload") {
		t.Fatalf("ValidateCommand() payload error = %v", err)
	}
}

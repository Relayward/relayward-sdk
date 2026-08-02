package agentv1

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testEvent(t *testing.T, sequence uint64) Event {
	t.Helper()
	value, err := NewEvent(
		"123e4567-e89b-42d3-a456-426614174000",
		"0123456789abcdef0123456789abcdef",
		sequence,
		"system.test",
		time.Date(2026, time.August, 2, 9, 0, 0, 123000000, time.UTC),
		map[string]any{"ok": true},
	)
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}
	return value
}

func TestEventIDFixedVectorAndContentValidation(t *testing.T) {
	value := testEvent(t, 1)
	const want = "fb9ce8fc9da2e1c27a30b3704cb51b0c902c54fc0f4c285d2f1cb0cb7e2f53a6"
	if value.EventID != want {
		t.Fatalf("EventID = %q, want %q", value.EventID, want)
	}
	if err := ValidateEventForNode("123e4567-e89b-42d3-a456-426614174000", "0123456789abcdef0123456789abcdef", value); err != nil {
		t.Fatalf("ValidateEventForNode() error = %v", err)
	}
	value.Payload = json.RawMessage(`{"ok":false}`)
	if err := ValidateEventForNode("123e4567-e89b-42d3-a456-426614174000", "0123456789abcdef0123456789abcdef", value); err == nil || !strings.Contains(err.Error(), "event_id") {
		t.Fatalf("ValidateEventForNode() content error = %v", err)
	}
}

func TestEventBatchRoundTripAndBounds(t *testing.T) {
	value := EventBatch{
		APIVersion: APIVersion, NodeID: "123e4567-e89b-42d3-a456-426614174000", StreamID: "0123456789abcdef0123456789abcdef",
		FirstSequence: 1, LastSequence: 2, Events: []Event{testEvent(t, 1), testEvent(t, 2)},
	}
	if err := ValidateEventBatch(value); err != nil {
		t.Fatalf("ValidateEventBatch() error = %v", err)
	}
	raw, _ := json.Marshal(value)
	decoded, err := DecodeEventBatch(bytes.NewReader(raw))
	if err != nil || decoded.LastSequence != 2 {
		t.Fatalf("DecodeEventBatch() = %+v, %v", decoded, err)
	}
	value.Events[1] = testEvent(t, 3)
	value.LastSequence = 3
	if err := ValidateEventBatch(value); err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("ValidateEventBatch() gap error = %v", err)
	}
}

func TestEventBatchAckValidation(t *testing.T) {
	value := EventBatchAck{
		APIVersion: APIVersion, StreamID: "0123456789abcdef0123456789abcdef",
		HighestContiguousSequence: 2, ServerTime: time.Now().UTC(),
	}
	if err := ValidateEventBatchAck(value); err != nil {
		t.Fatalf("ValidateEventBatchAck() error = %v", err)
	}
	value.HighestContiguousSequence = 0
	if err := ValidateEventBatchAck(value); err == nil {
		t.Fatal("ValidateEventBatchAck() accepted sequence zero")
	}
	value.HighestContiguousSequence = MaximumEventSequence + 1
	if err := ValidateEventBatchAck(value); err == nil {
		t.Fatal("ValidateEventBatchAck() accepted a sequence beyond the storage limit")
	}
}

func TestEventSequenceStorageLimit(t *testing.T) {
	if _, err := NewEvent(
		"123e4567-e89b-42d3-a456-426614174000",
		"0123456789abcdef0123456789abcdef",
		MaximumEventSequence+1,
		"system.test",
		time.Now().UTC(),
		map[string]bool{"ok": true},
	); err == nil {
		t.Fatal("NewEvent() accepted a sequence beyond the storage limit")
	}
}

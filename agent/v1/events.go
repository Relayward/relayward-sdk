package agentv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"math"
	"regexp"
	"time"
)

const (
	MaximumEventPayloadBytes         = 256 << 10
	MaximumEventBatchEvents          = 500
	MaximumEventBatchCompressedBytes = 1 << 20
	MaximumEventBatchExpandedBytes   = 4 << 20
	MaximumEventSequence             = uint64(math.MaxInt64)
)

var eventKindPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)+$`)

type Event struct {
	Sequence   uint64          `json:"sequence"`
	EventID    string          `json:"event_id"`
	Kind       string          `json:"kind"`
	ObservedAt time.Time       `json:"observed_at"`
	Payload    json.RawMessage `json:"payload"`
}

type EventBatch struct {
	APIVersion    string  `json:"api_version"`
	NodeID        string  `json:"node_id"`
	StreamID      string  `json:"stream_id"`
	FirstSequence uint64  `json:"first_sequence"`
	LastSequence  uint64  `json:"last_sequence"`
	Events        []Event `json:"events"`
}

type EventBatchAck struct {
	APIVersion                string    `json:"api_version"`
	StreamID                  string    `json:"stream_id"`
	HighestContiguousSequence uint64    `json:"highest_contiguous_sequence"`
	ServerTime                time.Time `json:"server_time"`
}

func NewEvent(nodeID, streamID string, sequence uint64, kind string, observedAt time.Time, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	value := Event{Sequence: sequence, Kind: kind, ObservedAt: observedAt.UTC(), Payload: raw}
	id, err := EventID(nodeID, streamID, value)
	if err != nil {
		return Event{}, err
	}
	value.EventID = id
	return value, nil
}

func EventID(nodeID, streamID string, value Event) (string, error) {
	if err := ValidateNodeID(nodeID); err != nil {
		return "", err
	}
	if !messageIDPattern.MatchString(streamID) {
		return "", fmt.Errorf("stream_id: invalid stream ID")
	}
	if err := validateEventContent(value); err != nil {
		return "", err
	}
	compact := make([]byte, 0, len(value.Payload))
	buffer := bytes.NewBuffer(compact)
	if err := json.Compact(buffer, value.Payload); err != nil {
		return "", fmt.Errorf("payload: invalid JSON")
	}
	digest := sha256.New()
	writeDigestField(digest, []byte(APIVersion))
	writeDigestField(digest, []byte(nodeID))
	writeDigestField(digest, []byte(streamID))
	var numeric [8]byte
	binary.BigEndian.PutUint64(numeric[:], value.Sequence)
	_, _ = digest.Write(numeric[:])
	writeDigestField(digest, []byte(value.Kind))
	writeDigestField(digest, []byte(value.ObservedAt.UTC().Format(time.RFC3339Nano)))
	writeDigestField(digest, buffer.Bytes())
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func ValidateEvent(value Event) error {
	if !digestPattern.MatchString(value.EventID) {
		return fmt.Errorf("event_id: invalid event ID")
	}
	return validateEventContent(value)
}

func ValidateEventForNode(nodeID, streamID string, value Event) error {
	if err := ValidateEvent(value); err != nil {
		return err
	}
	expected, err := EventID(nodeID, streamID, value)
	if err != nil {
		return err
	}
	if value.EventID != expected {
		return fmt.Errorf("event_id: does not match event content")
	}
	return nil
}

func ValidateEventBatch(value EventBatch) error {
	if value.APIVersion != APIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	if err := ValidateNodeID(value.NodeID); err != nil {
		return err
	}
	if !messageIDPattern.MatchString(value.StreamID) {
		return fmt.Errorf("stream_id: invalid stream ID")
	}
	if len(value.Events) == 0 || len(value.Events) > MaximumEventBatchEvents {
		return fmt.Errorf("events: must contain 1 to %d events", MaximumEventBatchEvents)
	}
	if value.FirstSequence != value.Events[0].Sequence || value.LastSequence != value.Events[len(value.Events)-1].Sequence {
		return fmt.Errorf("sequence bounds do not match events")
	}
	for index, event := range value.Events {
		if err := ValidateEventForNode(value.NodeID, value.StreamID, event); err != nil {
			return fmt.Errorf("events[%d]: %w", index, err)
		}
		if index > 0 && event.Sequence != value.Events[index-1].Sequence+1 {
			return fmt.Errorf("events[%d]: sequence is not contiguous", index)
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode event batch: %w", err)
	}
	if len(raw) > MaximumEventBatchExpandedBytes {
		return fmt.Errorf("event batch exceeds %d bytes", MaximumEventBatchExpandedBytes)
	}
	return nil
}

func ValidateEventBatchForNode(nodeID string, value EventBatch) error {
	if err := ValidateEventBatch(value); err != nil {
		return err
	}
	if value.NodeID != nodeID {
		return fmt.Errorf("node_id: does not match authenticated node")
	}
	return nil
}

func ValidateEventBatchAck(value EventBatchAck) error {
	if value.APIVersion != APIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	if !messageIDPattern.MatchString(value.StreamID) {
		return fmt.Errorf("stream_id: invalid stream ID")
	}
	if value.HighestContiguousSequence == 0 || value.HighestContiguousSequence > MaximumEventSequence {
		return fmt.Errorf("highest_contiguous_sequence: must be between 1 and %d", MaximumEventSequence)
	}
	if value.ServerTime.IsZero() {
		return fmt.Errorf("server_time: must be set")
	}
	return nil
}

func DecodeEventBatch(reader io.Reader) (EventBatch, error) {
	var value EventBatch
	if err := decodeStrict(reader, &value); err != nil {
		return EventBatch{}, fmt.Errorf("decode event batch: %w", err)
	}
	if err := ValidateEventBatch(value); err != nil {
		return EventBatch{}, err
	}
	return value, nil
}

func DecodeEventBatchAck(reader io.Reader) (EventBatchAck, error) {
	var value EventBatchAck
	if err := decodeStrict(reader, &value); err != nil {
		return EventBatchAck{}, fmt.Errorf("decode event batch acknowledgement: %w", err)
	}
	if err := ValidateEventBatchAck(value); err != nil {
		return EventBatchAck{}, err
	}
	return value, nil
}

func validateEventContent(value Event) error {
	if value.Sequence == 0 || value.Sequence > MaximumEventSequence {
		return fmt.Errorf("sequence: must be between 1 and %d", MaximumEventSequence)
	}
	if !eventKindPattern.MatchString(value.Kind) || len(value.Kind) > 128 {
		return fmt.Errorf("kind: invalid event kind")
	}
	if value.ObservedAt.IsZero() {
		return fmt.Errorf("observed_at: must be set")
	}
	if len(value.Payload) == 0 || len(value.Payload) > MaximumEventPayloadBytes || !json.Valid(value.Payload) {
		return fmt.Errorf("payload: must contain at most %d bytes of valid JSON", MaximumEventPayloadBytes)
	}
	return nil
}

func writeDigestField(digest hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}

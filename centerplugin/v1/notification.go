package centerpluginv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const EventNotificationRequest = "notification.request"

type NotificationSeverity string

const (
	NotificationInfo     NotificationSeverity = "info"
	NotificationWarning  NotificationSeverity = "warning"
	NotificationError    NotificationSeverity = "error"
	NotificationCritical NotificationSeverity = "critical"
)

type NotificationRequest struct {
	Severity NotificationSeverity `json:"severity"`
	Subject  string               `json:"subject"`
	Body     string               `json:"body"`
	DedupKey string               `json:"dedup_key,omitempty"`
}

func DecodeNotificationRequest(raw []byte) (NotificationRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value NotificationRequest
	if err := decoder.Decode(&value); err != nil {
		return NotificationRequest{}, fmt.Errorf("decode notification request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return NotificationRequest{}, fmt.Errorf("decode notification request: trailing JSON value")
		}
		return NotificationRequest{}, fmt.Errorf("decode notification request: %w", err)
	}
	if err := ValidateNotificationRequest(value); err != nil {
		return NotificationRequest{}, err
	}
	return value, nil
}

func ValidateNotificationRequest(value NotificationRequest) error {
	switch value.Severity {
	case NotificationInfo, NotificationWarning, NotificationError, NotificationCritical:
	default:
		return fmt.Errorf("severity: unsupported value %q", value.Severity)
	}
	if err := validateNotificationText("subject", value.Subject, 120, false); err != nil {
		return err
	}
	if err := validateNotificationText("body", value.Body, 4096, true); err != nil {
		return err
	}
	if value.DedupKey != "" && !sourceEventIDPattern.MatchString(value.DedupKey) {
		return fmt.Errorf("dedup_key: invalid deduplication key")
	}
	return nil
}

func validateNotificationText(field, value string, maximum int, multiline bool) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return fmt.Errorf("%s: must contain 1 to %d trimmed characters", field, maximum)
	}
	for _, character := range value {
		if unicode.IsControl(character) && !(multiline && (character == '\n' || character == '\t')) {
			return fmt.Errorf("%s: contains an unsupported control character", field)
		}
	}
	return nil
}

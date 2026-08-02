package agentv1

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	nodeIDPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	messageIDPattern    = regexp.MustCompile(`^[0-9a-f]{32}$`)
	agentVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	capabilityPattern   = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
)

func validateAgentVersion(value string) error {
	if !agentVersionPattern.MatchString(value) {
		return fmt.Errorf("agent_version: invalid version")
	}
	return nil
}

func validateHostname(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 253 {
		return fmt.Errorf("hostname: must contain 1 to 253 characters")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("hostname: must not contain control characters")
		}
	}
	return nil
}

func validateCapabilities(values []string) error {
	if len(values) > 64 {
		return fmt.Errorf("capabilities: must contain at most 64 values")
	}
	previous := ""
	for index, value := range values {
		if !capabilityPattern.MatchString(value) || len(value) > 64 {
			return fmt.Errorf("capabilities[%d]: invalid capability", index)
		}
		if index > 0 && value <= previous {
			return fmt.Errorf("capabilities: values must be sorted and unique")
		}
		previous = value
	}
	return nil
}

func validateDisplayName(field, value string, maximum int) error {
	if value == "" || value != strings.TrimSpace(value) || utf8.RuneCountInString(value) > maximum {
		return fmt.Errorf("%s: must contain 1 to %d characters", field, maximum)
	}
	return nil
}

func validSecret(value, prefix string) bool {
	if len(value) != len(prefix)+43 || !strings.HasPrefix(value, prefix) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value[len(prefix):])
	return err == nil && len(decoded) == 32
}

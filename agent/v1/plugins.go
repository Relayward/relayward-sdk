package agentv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/Relayward/relayward-sdk/contract"
)

const (
	CommandPluginReconcile = "plugin.reconcile"
	EventPluginStatus      = "plugin.status"

	PluginStateRunning = "running"
	PluginStateStopped = "stopped"
	PluginStateAbsent  = "absent"
	PluginStateFailed  = "failed"

	PluginHealthHealthy   = "healthy"
	PluginHealthUnhealthy = "unhealthy"
	PluginHealthUnknown   = "unknown"

	MaximumPluginArtifactBytes      int64 = 256 << 20
	MaximumPluginConfigurationBytes       = 384 << 10
	MaximumPluginDownloadURLBytes         = 4096
	MaximumPluginStatusReasonBytes        = 512
)

type PluginArtifact struct {
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
}

type PluginReconcileCommand struct {
	PluginID      string          `json:"plugin_id"`
	Generation    uint64          `json:"generation"`
	DesiredState  string          `json:"desired_state"`
	Version       string          `json:"version,omitempty"`
	Artifact      *PluginArtifact `json:"artifact,omitempty"`
	Configuration json.RawMessage `json:"configuration,omitempty"`
}

type PluginReconcileOutput struct {
	PluginID            string `json:"plugin_id"`
	Generation          uint64 `json:"generation"`
	State               string `json:"state"`
	Version             string `json:"version,omitempty"`
	ConfigurationSHA256 string `json:"configuration_sha256,omitempty"`
}

type PluginStatusEvent struct {
	PluginID            string   `json:"plugin_id"`
	Generation          uint64   `json:"generation"`
	State               string   `json:"state"`
	Version             string   `json:"version,omitempty"`
	ConfigurationSHA256 string   `json:"configuration_sha256,omitempty"`
	Health              string   `json:"health"`
	Reason              string   `json:"reason,omitempty"`
	RestartCount        uint64   `json:"restart_count"`
	Capabilities        []string `json:"capabilities,omitempty"`
}

func NewPluginReconcileCommand(value PluginReconcileCommand, issuedAt, expiresAt time.Time) (Command, error) {
	if err := ValidatePluginReconcileCommand(value); err != nil {
		return Command{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return Command{}, fmt.Errorf("encode plugin reconcile command: %w", err)
	}
	command := Command{Kind: CommandPluginReconcile, IssuedAt: issuedAt.UTC(), ExpiresAt: expiresAt.UTC(), Payload: payload}
	if err := ValidateCommand(command); err != nil {
		return Command{}, err
	}
	return command, nil
}

func DecodePluginReconcileCommand(value Command) (PluginReconcileCommand, error) {
	if err := ValidateCommand(value); err != nil {
		return PluginReconcileCommand{}, err
	}
	if value.Kind != CommandPluginReconcile {
		return PluginReconcileCommand{}, fmt.Errorf("kind: must be %q", CommandPluginReconcile)
	}
	var payload PluginReconcileCommand
	if err := decodeStrict(bytes.NewReader(value.Payload), &payload); err != nil {
		return PluginReconcileCommand{}, fmt.Errorf("decode plugin reconcile command: %w", err)
	}
	if err := ValidatePluginReconcileCommand(payload); err != nil {
		return PluginReconcileCommand{}, err
	}
	return payload, nil
}

func ValidatePluginReconcileCommand(value PluginReconcileCommand) error {
	if err := validatePluginIdentity(value.PluginID, value.Generation); err != nil {
		return err
	}
	switch value.DesiredState {
	case PluginStateAbsent:
		if value.Version != "" || value.Artifact != nil || len(value.Configuration) != 0 {
			return fmt.Errorf("desired_state: absent plugins must not include a version, artifact, or configuration")
		}
		return nil
	case PluginStateRunning, PluginStateStopped:
	default:
		return fmt.Errorf("desired_state: unsupported value %q", value.DesiredState)
	}
	if err := contract.ValidateSemanticVersion(value.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	if value.Artifact == nil {
		return fmt.Errorf("artifact: required unless desired_state is absent")
	}
	if err := ValidatePluginArtifact(*value.Artifact); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	if err := ValidatePluginConfiguration(value.Configuration); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	return nil
}

func ValidatePluginArtifact(value PluginArtifact) error {
	if len(value.DownloadURL) == 0 || len(value.DownloadURL) > MaximumPluginDownloadURLBytes {
		return fmt.Errorf("download_url: must contain 1 to %d bytes", MaximumPluginDownloadURLBytes)
	}
	parsed, err := url.ParseRequestURI(value.DownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("download_url: must be an HTTPS URL without credentials or a fragment")
	}
	if value.Size < 1 || value.Size > MaximumPluginArtifactBytes {
		return fmt.Errorf("size: must be between 1 and %d", MaximumPluginArtifactBytes)
	}
	if !digestPattern.MatchString(value.SHA256) {
		return fmt.Errorf("sha256: invalid SHA-256 digest")
	}
	return nil
}

func ValidatePluginConfiguration(raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > MaximumPluginConfigurationBytes || !json.Valid(raw) {
		return fmt.Errorf("must contain at most %d bytes of valid JSON", MaximumPluginConfigurationBytes)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return fmt.Errorf("must be a JSON object")
	}
	return nil
}

func PluginConfigurationDigest(raw json.RawMessage) (string, error) {
	if err := ValidatePluginConfiguration(raw); err != nil {
		return "", err
	}
	compact := bytes.NewBuffer(make([]byte, 0, len(raw)))
	if err := json.Compact(compact, raw); err != nil {
		return "", fmt.Errorf("compact plugin configuration: %w", err)
	}
	digest := sha256.Sum256(compact.Bytes())
	return hex.EncodeToString(digest[:]), nil
}

func EncodePluginReconcileOutput(value PluginReconcileOutput) (json.RawMessage, error) {
	if err := ValidatePluginReconcileOutput(value); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode plugin reconcile output: %w", err)
	}
	return raw, nil
}

func DecodePluginReconcileOutput(raw json.RawMessage) (PluginReconcileOutput, error) {
	var value PluginReconcileOutput
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return PluginReconcileOutput{}, fmt.Errorf("decode plugin reconcile output: %w", err)
	}
	if err := ValidatePluginReconcileOutput(value); err != nil {
		return PluginReconcileOutput{}, err
	}
	return value, nil
}

func ValidatePluginReconcileOutput(value PluginReconcileOutput) error {
	if err := validatePluginIdentity(value.PluginID, value.Generation); err != nil {
		return err
	}
	return validateObservedPluginState(value.State, value.Version, value.ConfigurationSHA256)
}

func ValidatePluginStatusEvent(value PluginStatusEvent) error {
	if err := validatePluginIdentity(value.PluginID, value.Generation); err != nil {
		return err
	}
	if err := validateStatusReason(value.Reason); err != nil {
		return err
	}
	if err := validateCapabilities(value.Capabilities); err != nil {
		return err
	}
	switch value.State {
	case PluginStateRunning:
		if value.Health != PluginHealthHealthy {
			return fmt.Errorf("health: running plugins must be healthy")
		}
		return validateObservedPluginState(value.State, value.Version, value.ConfigurationSHA256)
	case PluginStateStopped, PluginStateAbsent:
		if value.Health != PluginHealthUnknown {
			return fmt.Errorf("health: stopped or absent plugins must use unknown")
		}
		if len(value.Capabilities) != 0 {
			return fmt.Errorf("capabilities: stopped or absent plugins must not report capabilities")
		}
		return validateObservedPluginState(value.State, value.Version, value.ConfigurationSHA256)
	case PluginStateFailed:
		if value.Health != PluginHealthUnhealthy {
			return fmt.Errorf("health: failed plugins must be unhealthy")
		}
		if value.Reason == "" {
			return fmt.Errorf("reason: required for failed plugins")
		}
		if len(value.Capabilities) != 0 {
			return fmt.Errorf("capabilities: failed plugins must not report capabilities")
		}
		if (value.Version == "") != (value.ConfigurationSHA256 == "") {
			return fmt.Errorf("version and configuration_sha256 must either both be present or both be absent")
		}
		if value.Version != "" {
			if err := contract.ValidateSemanticVersion(value.Version); err != nil {
				return fmt.Errorf("version: %w", err)
			}
			if !digestPattern.MatchString(value.ConfigurationSHA256) {
				return fmt.Errorf("configuration_sha256: invalid SHA-256 digest")
			}
		}
		return nil
	default:
		return fmt.Errorf("state: unsupported value %q", value.State)
	}
}

func DecodePluginStatusEvent(raw json.RawMessage) (PluginStatusEvent, error) {
	var value PluginStatusEvent
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return PluginStatusEvent{}, fmt.Errorf("decode plugin status event: %w", err)
	}
	if err := ValidatePluginStatusEvent(value); err != nil {
		return PluginStatusEvent{}, err
	}
	return value, nil
}

func validatePluginIdentity(pluginID string, generation uint64) error {
	if err := contract.ValidatePluginID(pluginID); err != nil {
		return fmt.Errorf("plugin_id: %w", err)
	}
	if generation == 0 || generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	return nil
}

func validateObservedPluginState(state, version, configurationSHA256 string) error {
	switch state {
	case PluginStateAbsent:
		if version != "" || configurationSHA256 != "" {
			return fmt.Errorf("state: absent plugins must not include version or configuration_sha256")
		}
		return nil
	case PluginStateRunning, PluginStateStopped:
	default:
		return fmt.Errorf("state: unsupported value %q", state)
	}
	if err := contract.ValidateSemanticVersion(version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	if !digestPattern.MatchString(configurationSHA256) {
		return fmt.Errorf("configuration_sha256: invalid SHA-256 digest")
	}
	return nil
}

func validateStatusReason(value string) error {
	if len(value) > MaximumPluginStatusReasonBytes || value != strings.TrimSpace(value) {
		return fmt.Errorf("reason: must contain at most %d trimmed bytes", MaximumPluginStatusReasonBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("reason: must not contain control characters")
		}
	}
	return nil
}

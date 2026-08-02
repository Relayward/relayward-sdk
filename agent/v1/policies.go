package agentv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Relayward/relayward-sdk/contract"
	policyv1 "github.com/Relayward/relayward-sdk/policy/v1"
)

const (
	CommandPolicyReconcile = "policy.reconcile"
	EventPolicyStatus      = "policy.status"

	PolicyReasonActive        = "active"
	PolicyReasonDisabled      = "administrator_disabled"
	PolicyReasonExpired       = "expired"
	PolicyReasonQuotaExceeded = "quota_exceeded"

	MaximumPoliciesPerSnapshot = 4096
	MaximumBindingsPerPolicy   = 256
)

type ServiceBinding struct {
	PluginID  string `json:"plugin_id"`
	ServiceID string `json:"service_id"`
}

type AuthorizationPolicy struct {
	AuthorizationID       string             `json:"authorization_id"`
	StartedAt             time.Time          `json:"started_at"`
	Enabled               bool               `json:"enabled"`
	TrafficLimitBytes     *uint64            `json:"traffic_limit_bytes,omitempty"`
	Reset                 policyv1.ResetRule `json:"reset"`
	CurrentPeriod         policyv1.Period    `json:"current_period"`
	ExpiresAt             *time.Time         `json:"expires_at,omitempty"`
	SoftIPLimit           *uint32            `json:"soft_ip_limit,omitempty"`
	ActivityWindowSeconds uint32             `json:"activity_window_seconds"`
	BlockDurationSeconds  uint32             `json:"block_duration_seconds"`
	Bindings              []ServiceBinding   `json:"bindings"`
}

type PolicyReconcileCommand struct {
	Generation     uint64                `json:"generation"`
	Authorizations []AuthorizationPolicy `json:"authorizations"`
}

type PolicyReconcileOutput struct {
	Generation         uint64 `json:"generation"`
	AuthorizationCount uint32 `json:"authorization_count"`
}

type PolicyStatusEvent struct {
	Generation      uint64          `json:"generation"`
	AuthorizationID string          `json:"authorization_id"`
	Period          policyv1.Period `json:"period"`
	UploadBytes     uint64          `json:"upload_bytes"`
	DownloadBytes   uint64          `json:"download_bytes"`
	ServicesEnabled bool            `json:"services_enabled"`
	Reason          string          `json:"reason"`
	ActiveIPCount   uint32          `json:"active_ip_count"`
	BlockedIPCount  uint32          `json:"blocked_ip_count"`
}

func NewPolicyReconcileCommand(value PolicyReconcileCommand, issuedAt, expiresAt time.Time) (Command, error) {
	if err := ValidatePolicyReconcileCommand(value); err != nil {
		return Command{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return Command{}, fmt.Errorf("encode policy reconcile command: %w", err)
	}
	command := Command{Kind: CommandPolicyReconcile, IssuedAt: issuedAt.UTC(), ExpiresAt: expiresAt.UTC(), Payload: payload}
	if err := ValidateCommand(command); err != nil {
		return Command{}, err
	}
	return command, nil
}

func DecodePolicyReconcileCommand(value Command) (PolicyReconcileCommand, error) {
	if err := ValidateCommand(value); err != nil {
		return PolicyReconcileCommand{}, err
	}
	if value.Kind != CommandPolicyReconcile {
		return PolicyReconcileCommand{}, fmt.Errorf("kind: must be %q", CommandPolicyReconcile)
	}
	var payload PolicyReconcileCommand
	if err := decodeStrict(bytes.NewReader(value.Payload), &payload); err != nil {
		return PolicyReconcileCommand{}, fmt.Errorf("decode policy reconcile command: %w", err)
	}
	if err := ValidatePolicyReconcileCommand(payload); err != nil {
		return PolicyReconcileCommand{}, err
	}
	return payload, nil
}

func ValidatePolicyReconcileCommand(value PolicyReconcileCommand) error {
	if value.Generation == 0 || value.Generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if len(value.Authorizations) > MaximumPoliciesPerSnapshot {
		return fmt.Errorf("authorizations: must contain at most %d values", MaximumPoliciesPerSnapshot)
	}
	previous := ""
	for index, authorization := range value.Authorizations {
		if err := ValidateAuthorizationPolicy(authorization); err != nil {
			return fmt.Errorf("authorizations[%d]: %w", index, err)
		}
		if index > 0 && authorization.AuthorizationID <= previous {
			return fmt.Errorf("authorizations: values must be sorted by authorization_id and unique")
		}
		previous = authorization.AuthorizationID
	}
	return nil
}

func ValidateAuthorizationPolicy(value AuthorizationPolicy) error {
	if err := policyv1.ValidateIdentifier("authorization_id", value.AuthorizationID); err != nil {
		return err
	}
	if value.StartedAt.IsZero() {
		return fmt.Errorf("started_at: required")
	}
	if value.TrafficLimitBytes != nil && *value.TrafficLimitBytes > math.MaxInt64 {
		return fmt.Errorf("traffic_limit_bytes: must not exceed %d", int64(math.MaxInt64))
	}
	if err := policyv1.ValidateResetRule(value.Reset); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	if err := policyv1.ValidatePeriod(value.CurrentPeriod); err != nil {
		return fmt.Errorf("current_period: %w", err)
	}
	computed, err := policyv1.CurrentPeriod(value.Reset, value.StartedAt, value.CurrentPeriod.StartsAt)
	if err != nil || !policyv1.SamePeriod(computed, value.CurrentPeriod) {
		return fmt.Errorf("current_period: does not match reset rule")
	}
	if value.ExpiresAt != nil && (value.ExpiresAt.IsZero() || !value.ExpiresAt.After(value.StartedAt)) {
		return fmt.Errorf("expires_at: must be after started_at")
	}
	if value.SoftIPLimit != nil && (*value.SoftIPLimit < 1 || *value.SoftIPLimit > 1024) {
		return fmt.Errorf("soft_ip_limit: must be between 1 and 1024")
	}
	if value.ActivityWindowSeconds < 60 || value.ActivityWindowSeconds > 86400 {
		return fmt.Errorf("activity_window_seconds: must be between 60 and 86400")
	}
	if value.BlockDurationSeconds < 60 || value.BlockDurationSeconds > 604800 {
		return fmt.Errorf("block_duration_seconds: must be between 60 and 604800")
	}
	if len(value.Bindings) > MaximumBindingsPerPolicy {
		return fmt.Errorf("bindings: must contain at most %d values", MaximumBindingsPerPolicy)
	}
	previous := ""
	for index, binding := range value.Bindings {
		if err := contract.ValidatePluginID(binding.PluginID); err != nil {
			return fmt.Errorf("bindings[%d].plugin_id: %w", index, err)
		}
		if !componentIDPattern.MatchString(binding.ServiceID) {
			return fmt.Errorf("bindings[%d].service_id: invalid service ID", index)
		}
		key := binding.PluginID + "\x00" + binding.ServiceID
		if index > 0 && key <= previous {
			return fmt.Errorf("bindings: values must be sorted by plugin_id and service_id and unique")
		}
		previous = key
	}
	return nil
}

func EncodePolicyReconcileOutput(value PolicyReconcileOutput) (json.RawMessage, error) {
	if err := ValidatePolicyReconcileOutput(value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func DecodePolicyReconcileOutput(raw json.RawMessage) (PolicyReconcileOutput, error) {
	var value PolicyReconcileOutput
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return PolicyReconcileOutput{}, fmt.Errorf("decode policy reconcile output: %w", err)
	}
	if err := ValidatePolicyReconcileOutput(value); err != nil {
		return PolicyReconcileOutput{}, err
	}
	return value, nil
}

func ValidatePolicyReconcileOutput(value PolicyReconcileOutput) error {
	if value.Generation == 0 || value.Generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if value.AuthorizationCount > MaximumPoliciesPerSnapshot {
		return fmt.Errorf("authorization_count: must not exceed %d", MaximumPoliciesPerSnapshot)
	}
	return nil
}

func ValidatePolicyStatusEvent(value PolicyStatusEvent) error {
	if value.Generation == 0 || value.Generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if err := policyv1.ValidateIdentifier("authorization_id", value.AuthorizationID); err != nil {
		return err
	}
	if err := policyv1.ValidatePeriod(value.Period); err != nil {
		return err
	}
	if value.UploadBytes > math.MaxInt64 || value.DownloadBytes > math.MaxInt64 {
		return fmt.Errorf("traffic bytes: must not exceed %d", int64(math.MaxInt64))
	}
	switch value.Reason {
	case PolicyReasonActive:
		if !value.ServicesEnabled {
			return fmt.Errorf("services_enabled: active policies must enable services")
		}
	case PolicyReasonDisabled, PolicyReasonExpired, PolicyReasonQuotaExceeded:
		if value.ServicesEnabled {
			return fmt.Errorf("services_enabled: enforced policies must disable services")
		}
	default:
		return fmt.Errorf("reason: unsupported value %q", value.Reason)
	}
	return nil
}

func DecodePolicyStatusEvent(raw json.RawMessage) (PolicyStatusEvent, error) {
	var value PolicyStatusEvent
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return PolicyStatusEvent{}, fmt.Errorf("decode policy status event: %w", err)
	}
	if err := ValidatePolicyStatusEvent(value); err != nil {
		return PolicyStatusEvent{}, err
	}
	return value, nil
}

func SortPolicies(values []AuthorizationPolicy) {
	sort.Slice(values, func(i, j int) bool { return values[i].AuthorizationID < values[j].AuthorizationID })
	for index := range values {
		sort.Slice(values[index].Bindings, func(i, j int) bool {
			first, second := values[index].Bindings[i], values[index].Bindings[j]
			return first.PluginID < second.PluginID || first.PluginID == second.PluginID && first.ServiceID < second.ServiceID
		})
	}
}

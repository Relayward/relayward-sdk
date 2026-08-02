package agentv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	policyv1 "github.com/Relayward/relayward-sdk/policy/v1"
)

func TestPolicyReconcileCommandRoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	value := testPolicyReconcile(t, now)
	command, err := NewPolicyReconcileCommand(value, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewPolicyReconcileCommand() error = %v", err)
	}
	decoded, err := DecodePolicyReconcileCommand(command)
	if err != nil || decoded.Generation != value.Generation || len(decoded.Authorizations) != 1 {
		t.Fatalf("DecodePolicyReconcileCommand() = %+v, %v", decoded, err)
	}
	command.Payload = json.RawMessage(`{"generation":2,"authorizations":[],"unknown":true}`)
	if _, err := DecodePolicyReconcileCommand(command); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestPolicyValidationRejectsUnsortedAndMismatchedPeriod(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	value := testPolicyReconcile(t, now)
	authorization := value.Authorizations[0]
	authorization.Bindings = []ServiceBinding{
		{PluginID: "io.relayward.z", ServiceID: "main"},
		{PluginID: "io.relayward.a", ServiceID: "main"},
	}
	value.Authorizations[0] = authorization
	if err := ValidatePolicyReconcileCommand(value); err == nil || !strings.Contains(err.Error(), "sorted") {
		t.Fatalf("unsorted bindings error = %v", err)
	}
	authorization.Bindings = nil
	authorization.CurrentPeriod.StartsAt = authorization.CurrentPeriod.StartsAt.Add(time.Hour)
	value.Authorizations[0] = authorization
	if err := ValidatePolicyReconcileCommand(value); err == nil {
		t.Fatal("ValidatePolicyReconcileCommand() accepted mismatched period")
	}
}

func TestPolicyOutputAndStatus(t *testing.T) {
	output := PolicyReconcileOutput{Generation: 2, AuthorizationCount: 1}
	raw, err := EncodePolicyReconcileOutput(output)
	if err != nil {
		t.Fatalf("EncodePolicyReconcileOutput() error = %v", err)
	}
	decoded, err := DecodePolicyReconcileOutput(raw)
	if err != nil || decoded != output {
		t.Fatalf("DecodePolicyReconcileOutput() = %+v, %v", decoded, err)
	}
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	policy := testPolicyReconcile(t, now).Authorizations[0]
	status := PolicyStatusEvent{
		Generation: 2, AuthorizationID: policy.AuthorizationID, Period: policy.CurrentPeriod,
		ServicesEnabled: true, Reason: PolicyReasonActive,
	}
	if err := ValidatePolicyStatusEvent(status); err != nil {
		t.Fatalf("ValidatePolicyStatusEvent() error = %v", err)
	}
	status.Reason = PolicyReasonExpired
	if err := ValidatePolicyStatusEvent(status); err == nil {
		t.Fatal("ValidatePolicyStatusEvent() accepted enabled expired services")
	}
}

func testPolicyReconcile(t *testing.T, now time.Time) PolicyReconcileCommand {
	t.Helper()
	started := now.Add(-24 * time.Hour)
	rule := policyv1.ResetRule{Kind: policyv1.ResetDaily, Timezone: "UTC"}
	period, err := policyv1.CurrentPeriod(rule, started, now)
	if err != nil {
		t.Fatalf("CurrentPeriod() error = %v", err)
	}
	limit := uint64(1 << 30)
	softLimit := uint32(3)
	return PolicyReconcileCommand{Generation: 2, Authorizations: []AuthorizationPolicy{{
		AuthorizationID: "123e4567-e89b-42d3-a456-426614174000", StartedAt: started, Enabled: true,
		TrafficLimitBytes: &limit, Reset: rule, CurrentPeriod: period, SoftIPLimit: &softLimit,
		ActivityWindowSeconds: 600, BlockDurationSeconds: 1800,
		Bindings: []ServiceBinding{{PluginID: "io.relayward.test", ServiceID: "main"}},
	}}}
}

package agentv1

import (
	"encoding/json"
	"testing"
	"time"

	policyv1 "github.com/Relayward/relayward-sdk/policy/v1"
)

func TestStandardTelemetryValidation(t *testing.T) {
	period, err := policyv1.CurrentPeriod(policyv1.ResetRule{Kind: policyv1.ResetDaily, Timezone: "UTC"},
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CurrentPeriod() error = %v", err)
	}
	traffic := TrafficSnapshotEvent{
		AuthorizationID: "123e4567-e89b-42d3-a456-426614174000", Period: period,
		Revision: 3, UploadBytes: 10, DownloadBytes: 20,
	}
	raw, _ := json.Marshal(traffic)
	if decoded, err := DecodeTrafficSnapshotEvent(raw); err != nil || decoded.Revision != 3 {
		t.Fatalf("DecodeTrafficSnapshotEvent() = %+v, %v", decoded, err)
	}
	access := AccessEvent{
		SourceEventID: "access-1", PluginID: "io.relayward.test", ServiceID: "main",
		AuthorizationID: traffic.AuthorizationID, SourceIP: "2001:db8::1", Destination: "example.com",
		DestinationPort: 443, Network: "tcp", Protocol: "tls", Action: AccessActionAccepted,
	}
	raw, _ = json.Marshal(access)
	if decoded, err := DecodeAccessEvent(raw); err != nil || decoded.SourceEventID != access.SourceEventID {
		t.Fatalf("DecodeAccessEvent() = %+v, %v", decoded, err)
	}
	access.SourceIP = "2001:0db8::1"
	if err := ValidateAccessEvent(access); err == nil {
		t.Fatal("ValidateAccessEvent() accepted non-canonical IP")
	}
}

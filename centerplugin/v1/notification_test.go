package centerpluginv1

import "testing"

func TestDecodeNotificationRequest(t *testing.T) {
	raw := []byte(`{"severity":"warning","subject":"Node policy","body":"Quota exceeded\nReview the authorization.","dedup_key":"quota:authorization-1"}`)
	value, err := DecodeNotificationRequest(raw)
	if err != nil {
		t.Fatalf("DecodeNotificationRequest() error = %v", err)
	}
	if value.Severity != NotificationWarning || value.DedupKey != "quota:authorization-1" {
		t.Fatalf("DecodeNotificationRequest() = %+v", value)
	}

	for _, invalid := range [][]byte{
		[]byte(`{"severity":"debug","subject":"Node","body":"Message"}`),
		[]byte(`{"severity":"info","subject":"Node","body":"Message","channel":"telegram"}`),
		[]byte(`{"severity":"info","subject":"Node","body":"Message"} {}`),
	} {
		if _, err := DecodeNotificationRequest(invalid); err == nil {
			t.Fatalf("DecodeNotificationRequest(%s) succeeded", invalid)
		}
	}
}

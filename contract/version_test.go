package contract

import (
	"strings"
	"testing"
)

func TestValidateSemanticVersion(t *testing.T) {
	for _, value := range []string{"0.1.0", "1.2.3-alpha.1", "1.2.3+build.7"} {
		if err := ValidateSemanticVersion(value); err != nil {
			t.Fatalf("ValidateSemanticVersion(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"", "v1.2.3", "1.2", "1.2.3-01"} {
		if err := ValidateSemanticVersion(value); err == nil {
			t.Fatalf("ValidateSemanticVersion(%q) succeeded", value)
		}
	}
}

func TestSupports(t *testing.T) {
	if !Supports([]uint32{1, 2}, 2) {
		t.Fatal("expected version 2 to be supported")
	}
	if Supports([]uint32{1, 2}, 3) {
		t.Fatal("did not expect version 3 to be supported")
	}
}

func TestValidatePluginID(t *testing.T) {
	for _, value := range []string{"relayward", "io.relayward.test", "relayward-plugin"} {
		if err := ValidatePluginID(value); err != nil {
			t.Fatalf("ValidatePluginID(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"", "Relayward", ".relayward", "relayward_", strings.Repeat("a", 129)} {
		if err := ValidatePluginID(value); err == nil {
			t.Fatalf("ValidatePluginID(%q) succeeded", value)
		}
	}
}

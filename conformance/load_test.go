package conformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadContractPluginManifest(t *testing.T) {
	path := filepath.Join("..", "fixtures", "contract-plugin", "manifest.json")
	value, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if value.ID != "io.relayward.contract-test" {
		t.Fatalf("manifest id = %q", value.ID)
	}
}

func TestLoadManifestRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxContractFileSize+1)), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := LoadManifest(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("LoadManifest() error = %v, want size error", err)
	}
}

func TestLoadEnvelopeFixture(t *testing.T) {
	path := filepath.Join("..", "fixtures", "envelopes", "system-hello.json")
	value, err := LoadEnvelope(path)
	if err != nil {
		t.Fatalf("LoadEnvelope() error = %v", err)
	}
	if value.Type != "system.hello" {
		t.Fatalf("envelope type = %q", value.Type)
	}
}

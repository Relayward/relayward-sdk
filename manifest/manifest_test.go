package manifest

import (
	"strings"
	"testing"

	"github.com/Relayward/relayward-sdk/contract"
)

const validManifest = `{
  "api_version": "relayward.plugin/v1",
  "id": "io.relayward.contract-test",
  "name": "Relayward contract test plugin",
  "version": "0.1.0",
  "kind": "runtime",
  "requires": {"control_api": 1, "agent_api": 1, "ui_api": 1},
  "permissions": [{"name": "core.nodes.read", "reason": "Exercise permission validation."}],
  "artifacts": [
    {"role": "center", "file": "contract-plugin-center-linux-amd64", "size": 1000, "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "os": "linux", "arch": "amd64"},
    {"role": "node", "file": "contract-plugin-node-linux-amd64", "size": 2000, "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "os": "linux", "arch": "amd64"},
    {"role": "ui", "file": "contract-plugin-ui.tar.gz", "size": 3000, "sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
  ]
}`

func TestDecodeAndCompatibility(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	supported := contract.SupportedAPIs{
		Control: []uint32{contract.ControlAPIMajor},
		Agent:   []uint32{contract.AgentAPIMajor},
		UI:      []uint32{contract.UIAPIMajor},
	}
	if err := CheckCompatibility(value, supported); err != nil {
		t.Fatalf("CheckCompatibility() error = %v", err)
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	input := strings.Replace(validManifest, `"name": "Relayward contract test plugin",`, `"name": "Relayward contract test plugin", "unexpected": true,`, 1)
	if _, err := Decode(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Decode() error = %v, want unknown field", err)
	}
}

func TestValidateRejectsTraversal(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	value.Artifacts[0].File = "../plugin"
	if err := Validate(value); err == nil || !strings.Contains(err.Error(), "release asset file name") {
		t.Fatalf("Validate() error = %v, want file name error", err)
	}
}

func TestValidateRejectsFeatureNodeArtifact(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	value.Kind = KindFeature
	if err := Validate(value); err == nil || !strings.Contains(err.Error(), "feature plugins") {
		t.Fatalf("Validate() error = %v, want feature plugin error", err)
	}
}

func TestCompatibilityRejectsUnsupportedAPI(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if err := CheckCompatibility(value, contract.SupportedAPIs{Control: []uint32{2}}); err == nil {
		t.Fatal("CheckCompatibility() error = nil, want unsupported API error")
	}
}

func TestValidateRejectsInvalidSemanticVersion(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	value.Version = "1.0.0-01"
	if err := Validate(value); err == nil || !strings.Contains(err.Error(), "semantic version") {
		t.Fatalf("Validate() error = %v, want semantic version error", err)
	}
}

func TestValidateRejectsInvalidArtifactSize(t *testing.T) {
	value, err := Decode(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	value.Artifacts[0].Size = MaximumExecutableArtifactBytes + 1
	if err := Validate(value); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("Validate() error = %v, want size error", err)
	}
}

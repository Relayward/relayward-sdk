package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Relayward/relayward-sdk/contract"
	"github.com/Relayward/relayward-sdk/manifest"
)

func TestVerifyPluginReleaseChecksActualAssets(t *testing.T) {
	directory := t.TempDir()
	center := []byte("center")
	digest := sha256.Sum256(center)
	value := manifest.Manifest{
		APIVersion: contract.ManifestAPIVersion, ID: "io.relayward.test", Name: "Test",
		Version: "1.2.3", Kind: manifest.KindFeature,
		Requires:    manifest.Requirements{ControlAPI: contract.ControlAPIMajor},
		Permissions: []manifest.Permission{},
		Artifacts: []manifest.Artifact{{
			Role: manifest.ArtifactCenter, File: "center", Size: int64(len(center)),
			SHA256: hex.EncodeToString(digest[:]), OS: "linux", Arch: "amd64",
		}},
	}
	raw, _ := json.Marshal(value)
	if err := os.WriteFile(filepath.Join(directory, PluginReleaseManifest), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "center"), center, 0o700); err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyPluginRelease(directory)
	if err != nil || verified.ID != value.ID {
		t.Fatalf("VerifyPluginRelease() = %+v, %v", verified, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "center"), []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPluginRelease(directory); err == nil {
		t.Fatal("VerifyPluginRelease() accepted a tampered artifact")
	}
}

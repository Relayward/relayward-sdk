package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	centerpluginv1 "github.com/Relayward/relayward-sdk/centerplugin/v1"
	"github.com/Relayward/relayward-sdk/contract"
	"github.com/Relayward/relayward-sdk/manifest"
)

func main() {
	flags := flag.NewFlagSet("relayward-contract-release", flag.ExitOnError)
	directory := flags.String("dist", "dist", "release artifact directory")
	version := flags.String("version", "", "semantic plugin version")
	flags.Parse(os.Args[1:])
	if flags.NArg() != 0 {
		fatal("unexpected positional argument")
	}
	if err := contract.ValidateSemanticVersion(*version); err != nil {
		fatal("invalid version: %v", err)
	}
	agentAPI, uiAPI := uint32(contract.AgentAPIMajor), uint32(contract.UIAPIMajor)
	value := manifest.Manifest{
		APIVersion: contract.ManifestAPIVersion,
		ID:         "io.relayward.contract-test", Name: "Relayward contract test plugin", Version: *version,
		Kind:     manifest.KindRuntime,
		Requires: manifest.Requirements{ControlAPI: contract.ControlAPIMajor, AgentAPI: &agentAPI, UIAPI: &uiAPI},
		Permissions: []manifest.Permission{{
			Name: centerpluginv1.PermissionNodesRead, Reason: "Exercise permission-gated node state access.",
		}},
	}
	for _, item := range []struct {
		role manifest.ArtifactRole
		name string
		os   string
		arch string
	}{
		{role: manifest.ArtifactCenter, name: "contract-plugin-center-linux-amd64", os: "linux", arch: "amd64"},
		{role: manifest.ArtifactNode, name: "contract-plugin-node-linux-amd64", os: "linux", arch: "amd64"},
		{role: manifest.ArtifactUI, name: "contract-plugin-ui.tar.gz"},
	} {
		artifact, err := describeArtifact(filepath.Join(*directory, item.name), item.role, item.name, item.os, item.arch)
		if err != nil {
			fatal("%v", err)
		}
		value.Artifacts = append(value.Artifacts, artifact)
	}
	if err := manifest.Validate(value); err != nil {
		fatal("generated manifest is invalid: %v", err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal("encode manifest: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(*directory, "relayward-plugin.json"), raw, 0o644); err != nil {
		fatal("write manifest: %v", err)
	}
}

func describeArtifact(path string, role manifest.ArtifactRole, name, osName, arch string) (manifest.Artifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifest.Artifact{}, fmt.Errorf("read %s artifact: %w", role, err)
	}
	digest := sha256.Sum256(raw)
	return manifest.Artifact{
		Role: role, File: name, Size: int64(len(raw)), SHA256: hex.EncodeToString(digest[:]), OS: osName, Arch: arch,
	}, nil
}

func fatal(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}

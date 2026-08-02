package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(testManifestJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"manifest", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "manifest valid") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

const testManifestJSON = `{
  "api_version":"relayward.plugin/v1",
  "id":"io.relayward.test",
  "name":"Test",
  "version":"1.2.3",
  "kind":"feature",
  "requires":{"control_api":1},
  "permissions":[],
  "artifacts":[{
    "role":"center","file":"center","size":1,
    "sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "os":"linux","arch":"amd64"
  }]
}`

func TestRunAgentFixtures(t *testing.T) {
	tests := []struct {
		command string
		path    string
	}{
		{command: "agent-register", path: filepath.Join("..", "..", "fixtures", "agent", "register-request.json")},
		{command: "agent-envelope", path: filepath.Join("..", "..", "fixtures", "agent", "hello.json")},
		{command: "agent-event-batch", path: filepath.Join("..", "..", "fixtures", "agent", "event-batch.json")},
		{command: "agent-event-ack", path: filepath.Join("..", "..", "fixtures", "agent", "event-batch-ack.json")},
		{command: "agent-plugin-reconcile", path: filepath.Join("..", "..", "fixtures", "agent", "plugin-reconcile-command.json")},
		{command: "agent-plugin-status", path: filepath.Join("..", "..", "fixtures", "agent", "plugin-status.json")},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run([]string{test.command, test.path}, &stdout, &stderr); code != 0 {
				t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.command+" valid") {
				t.Fatalf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestRunCenterPluginFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "fixtures", "center-plugin")
	tests := []struct {
		command string
		file    string
	}{
		{command: "center-plugin-info", file: "info.json"},
		{command: "center-plugin-activation", file: "activation.json"},
		{command: "center-plugin-status", file: "status.json"},
		{command: "center-plugin-ui", file: "ui-request.json"},
		{command: "center-plugin-nodes", file: "nodes.json"},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run([]string{test.command, filepath.Join(root, test.file)}, &stdout, &stderr); code != 0 {
				t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"unknown", "file"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

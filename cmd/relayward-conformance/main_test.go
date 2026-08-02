package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := filepath.Join("..", "..", "fixtures", "contract-plugin", "manifest.json")

	if code := run([]string{"manifest", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "manifest valid") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"unknown", "file"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

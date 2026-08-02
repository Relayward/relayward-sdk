// Package conformance loads contract artifacts using the same strict limits as Relayward hosts.
package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Relayward/relayward-sdk/manifest"
	"github.com/Relayward/relayward-sdk/protocol"
)

const maxContractFileSize = 1 << 20

func LoadManifest(path string) (manifest.Manifest, error) {
	file, err := openContractFile(path)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()

	return manifest.Decode(file)
}

func LoadEnvelope(path string) (protocol.Envelope, error) {
	file, err := openContractFile(path)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("open envelope: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var value protocol.Envelope
	if err := decoder.Decode(&value); err != nil {
		return protocol.Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return protocol.Envelope{}, fmt.Errorf("decode envelope: trailing JSON value")
		}
		return protocol.Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if err := protocol.ValidateEnvelope(value); err != nil {
		return protocol.Envelope{}, err
	}
	return value, nil
}

func openContractFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("contract path is not a regular file")
	}
	if info.Size() > maxContractFileSize {
		file.Close()
		return nil, fmt.Errorf("contract file exceeds %d bytes", maxContractFileSize)
	}
	return file, nil
}

// Package conformance loads contract artifacts using the same strict limits as Relayward hosts.
package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
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

func LoadAgentRegisterRequest(path string) (agentv1.RegisterRequest, error) {
	file, err := openContractFile(path)
	if err != nil {
		return agentv1.RegisterRequest{}, fmt.Errorf("open Agent registration request: %w", err)
	}
	defer file.Close()

	return agentv1.DecodeRegisterRequest(file)
}

func LoadAgentEventBatch(path string) (agentv1.EventBatch, error) {
	file, err := openContractFile(path)
	if err != nil {
		return agentv1.EventBatch{}, fmt.Errorf("open Agent event batch: %w", err)
	}
	defer file.Close()
	return agentv1.DecodeEventBatch(file)
}

func LoadAgentEventBatchAck(path string) (agentv1.EventBatchAck, error) {
	file, err := openContractFile(path)
	if err != nil {
		return agentv1.EventBatchAck{}, fmt.Errorf("open Agent event batch acknowledgement: %w", err)
	}
	defer file.Close()
	return agentv1.DecodeEventBatchAck(file)
}

func LoadAgentPluginReconcileCommand(path string) (agentv1.Command, error) {
	file, err := openContractFile(path)
	if err != nil {
		return agentv1.Command{}, fmt.Errorf("open plugin reconcile command: %w", err)
	}
	defer file.Close()
	var value agentv1.Command
	if err := decodeContractJSON(file, &value); err != nil {
		return agentv1.Command{}, fmt.Errorf("decode plugin reconcile command: %w", err)
	}
	if _, err := agentv1.DecodePluginReconcileCommand(value); err != nil {
		return agentv1.Command{}, err
	}
	return value, nil
}

func LoadAgentPluginStatus(path string) (agentv1.PluginStatusEvent, error) {
	file, err := openContractFile(path)
	if err != nil {
		return agentv1.PluginStatusEvent{}, fmt.Errorf("open plugin status event: %w", err)
	}
	defer file.Close()
	var value agentv1.PluginStatusEvent
	if err := decodeContractJSON(file, &value); err != nil {
		return agentv1.PluginStatusEvent{}, fmt.Errorf("decode plugin status event: %w", err)
	}
	if err := agentv1.ValidatePluginStatusEvent(value); err != nil {
		return agentv1.PluginStatusEvent{}, err
	}
	return value, nil
}

func LoadEnvelope(path string) (protocol.Envelope, error) {
	file, err := openContractFile(path)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("open envelope: %w", err)
	}
	defer file.Close()

	var value protocol.Envelope
	if err := decodeContractJSON(file, &value); err != nil {
		return protocol.Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if err := protocol.ValidateEnvelope(value); err != nil {
		return protocol.Envelope{}, err
	}
	return value, nil
}

func decodeContractJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func LoadAgentEnvelope(path string) (protocol.Envelope, error) {
	value, err := LoadEnvelope(path)
	if err != nil {
		return protocol.Envelope{}, err
	}
	if err := agentv1.ValidateEnvelope(value); err != nil {
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

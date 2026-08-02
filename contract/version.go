// Package contract defines the common version identifiers used by Relayward contracts.
package contract

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	ManifestAPIVersion   = "relayward.plugin/v1"
	ControlAPIVersion    = "relayward.control/v1"
	AgentAPIVersion      = "relayward.agent/v1"
	NodePluginAPIVersion = "relayward.node-plugin/v1"

	ControlAPIMajor uint32 = 1
	AgentAPIMajor   uint32 = 1
	UIAPIMajor      uint32 = 1
)

var (
	pluginIDPattern        = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
	semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

func ValidatePluginID(value string) error {
	if len(value) > 128 || !pluginIDPattern.MatchString(value) {
		return fmt.Errorf("must be a lowercase dotted identifier of at most 128 characters")
	}
	return nil
}

func ValidateSemanticVersion(value string) error {
	if len(value) > 128 || !semanticVersionPattern.MatchString(value) {
		return fmt.Errorf("must be a semantic version without a leading v")
	}
	coreAndPreRelease := strings.SplitN(value, "+", 2)[0]
	preReleaseStart := strings.IndexByte(coreAndPreRelease, '-')
	if preReleaseStart < 0 {
		return nil
	}
	for _, identifier := range strings.Split(coreAndPreRelease[preReleaseStart+1:], ".") {
		if len(identifier) > 1 && identifier[0] == '0' && allDigits(identifier) {
			return fmt.Errorf("must be a semantic version without pre-release numeric leading zeros")
		}
	}
	return nil
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func IsMessageAPIVersion(value string) bool {
	switch value {
	case ControlAPIVersion, AgentAPIVersion:
		return true
	default:
		return false
	}
}

// SupportedAPIs lists every contract major a host can serve concurrently.
// A plugin is compatible only when each API major it requires appears here.
type SupportedAPIs struct {
	Control []uint32
	Agent   []uint32
	UI      []uint32
}

func Supports(versions []uint32, required uint32) bool {
	for _, version := range versions {
		if version == required {
			return true
		}
	}
	return false
}

// Package contract defines the common version identifiers used by Relayward contracts.
package contract

const (
	ManifestAPIVersion = "relayward.plugin/v1"
	ControlAPIVersion  = "relayward.control/v1"
	AgentAPIVersion    = "relayward.agent/v1"

	ControlAPIMajor uint32 = 1
	AgentAPIMajor   uint32 = 1
	UIAPIMajor      uint32 = 1
)

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

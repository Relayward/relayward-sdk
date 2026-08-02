// Package manifest defines and validates Relayward plugin release manifests.
package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Relayward/relayward-sdk/contract"
)

type Kind string

const (
	KindRuntime Kind = "runtime"
	KindFeature Kind = "feature"
)

type ArtifactRole string

const (
	ArtifactCenter ArtifactRole = "center"
	ArtifactNode   ArtifactRole = "node"
	ArtifactUI     ArtifactRole = "ui"
)

type Requirements struct {
	ControlAPI uint32  `json:"control_api"`
	AgentAPI   *uint32 `json:"agent_api,omitempty"`
	UIAPI      *uint32 `json:"ui_api,omitempty"`
}

type Permission struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Artifact struct {
	Role   ArtifactRole `json:"role"`
	File   string       `json:"file"`
	SHA256 string       `json:"sha256"`
	OS     string       `json:"os,omitempty"`
	Arch   string       `json:"arch,omitempty"`
}

type Manifest struct {
	APIVersion  string       `json:"api_version"`
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Kind        Kind         `json:"kind"`
	Requires    Requirements `json:"requires"`
	Permissions []Permission `json:"permissions"`
	Artifacts   []Artifact   `json:"artifacts"`
}

var (
	idPattern         = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
	permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_-]*)+$`)
	semverPattern     = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	sha256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func Decode(reader io.Reader) (Manifest, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var value Manifest
	if err := decoder.Decode(&value); err != nil {
		return Manifest{}, fmt.Errorf("decode plugin manifest: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if err := Validate(value); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode plugin manifest: trailing JSON value")
		}
		return fmt.Errorf("decode plugin manifest: %w", err)
	}
	return nil
}

func Validate(value Manifest) error {
	if value.APIVersion != contract.ManifestAPIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	if !idPattern.MatchString(value.ID) || len(value.ID) > 128 {
		return fmt.Errorf("id: must be a lowercase dotted identifier of at most 128 characters")
	}
	if name := strings.TrimSpace(value.Name); name == "" || len(name) > 80 {
		return fmt.Errorf("name: must contain 1 to 80 characters")
	}
	if !validSemVer(value.Version) {
		return fmt.Errorf("version: must be a semantic version without a leading v")
	}
	if value.Kind != KindRuntime && value.Kind != KindFeature {
		return fmt.Errorf("kind: unsupported value %q", value.Kind)
	}
	if value.Requires.ControlAPI == 0 {
		return fmt.Errorf("requires.control_api: must be greater than zero")
	}
	if err := validatePermissions(value.Permissions); err != nil {
		return err
	}
	return validateArtifacts(value)
}

func validSemVer(value string) bool {
	if !semverPattern.MatchString(value) {
		return false
	}
	coreAndPreRelease := strings.SplitN(value, "+", 2)[0]
	preReleaseStart := strings.IndexByte(coreAndPreRelease, '-')
	if preReleaseStart < 0 {
		return true
	}
	preRelease := coreAndPreRelease[preReleaseStart+1:]
	for _, identifier := range strings.Split(preRelease, ".") {
		if len(identifier) > 1 && identifier[0] == '0' && allDigits(identifier) {
			return false
		}
	}
	return true
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validatePermissions(permissions []Permission) error {
	seen := make(map[string]struct{}, len(permissions))
	for index, permission := range permissions {
		if !permissionPattern.MatchString(permission.Name) || len(permission.Name) > 128 {
			return fmt.Errorf("permissions[%d].name: invalid permission identifier", index)
		}
		if reason := strings.TrimSpace(permission.Reason); reason == "" || len(reason) > 200 {
			return fmt.Errorf("permissions[%d].reason: must contain 1 to 200 characters", index)
		}
		if _, exists := seen[permission.Name]; exists {
			return fmt.Errorf("permissions[%d].name: duplicate permission %q", index, permission.Name)
		}
		seen[permission.Name] = struct{}{}
	}
	return nil
}

func validateArtifacts(value Manifest) error {
	seen := make(map[ArtifactRole]struct{}, len(value.Artifacts))
	for index, artifact := range value.Artifacts {
		if artifact.Role != ArtifactCenter && artifact.Role != ArtifactNode && artifact.Role != ArtifactUI {
			return fmt.Errorf("artifacts[%d].role: unsupported value %q", index, artifact.Role)
		}
		if _, exists := seen[artifact.Role]; exists {
			return fmt.Errorf("artifacts[%d].role: duplicate role %q", index, artifact.Role)
		}
		seen[artifact.Role] = struct{}{}
		if artifact.File == "" || artifact.File == "." || artifact.File == ".." || strings.ContainsAny(artifact.File, `/\\`) {
			return fmt.Errorf("artifacts[%d].file: must be a release asset file name", index)
		}
		if !sha256Pattern.MatchString(artifact.SHA256) {
			return fmt.Errorf("artifacts[%d].sha256: must be 64 lowercase hexadecimal characters", index)
		}
		switch artifact.Role {
		case ArtifactCenter, ArtifactNode:
			if artifact.OS != "linux" || artifact.Arch != "amd64" {
				return fmt.Errorf("artifacts[%d]: executable target must be linux/amd64", index)
			}
		case ArtifactUI:
			if artifact.OS != "" || artifact.Arch != "" {
				return fmt.Errorf("artifacts[%d]: UI artifact must not declare os or arch", index)
			}
		}
	}

	if _, exists := seen[ArtifactCenter]; !exists {
		return fmt.Errorf("artifacts: center artifact is required")
	}
	_, hasNode := seen[ArtifactNode]
	_, hasUI := seen[ArtifactUI]
	if value.Kind == KindFeature && hasNode {
		return fmt.Errorf("artifacts: feature plugins cannot contain a node artifact")
	}
	if hasNode != (value.Requires.AgentAPI != nil) {
		return fmt.Errorf("requires.agent_api: must be present exactly when a node artifact is declared")
	}
	if hasUI != (value.Requires.UIAPI != nil) {
		return fmt.Errorf("requires.ui_api: must be present exactly when a UI artifact is declared")
	}
	if value.Requires.AgentAPI != nil && *value.Requires.AgentAPI == 0 {
		return fmt.Errorf("requires.agent_api: must be greater than zero")
	}
	if value.Requires.UIAPI != nil && *value.Requires.UIAPI == 0 {
		return fmt.Errorf("requires.ui_api: must be greater than zero")
	}
	return nil
}

func CheckCompatibility(value Manifest, supported contract.SupportedAPIs) error {
	if !contract.Supports(supported.Control, value.Requires.ControlAPI) {
		return fmt.Errorf("control API v%d is not supported", value.Requires.ControlAPI)
	}
	if value.Requires.AgentAPI != nil && !contract.Supports(supported.Agent, *value.Requires.AgentAPI) {
		return fmt.Errorf("agent API v%d is not supported", *value.Requires.AgentAPI)
	}
	if value.Requires.UIAPI != nil && !contract.Supports(supported.UI, *value.Requires.UIAPI) {
		return fmt.Errorf("UI API v%d is not supported", *value.Requires.UIAPI)
	}
	return nil
}

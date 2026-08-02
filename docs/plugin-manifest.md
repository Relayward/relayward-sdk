# Plugin Release Manifest v1

Every installable Relayward plugin release includes one JSON manifest validated against `relayward.plugin/v1`. The manifest describes identity, compatibility, requested permissions, and immutable release assets. Repository metadata and download URLs come from the GitHub Release API and are not trusted from manifest fields.

## Identity

- `api_version` is exactly `relayward.plugin/v1`.
- `id` is a stable lowercase dotted identifier. It does not change when the display name or repository changes.
- `name` is display text between 1 and 80 characters.
- `version` is a semantic version without the Git tag's leading `v`.
- `kind` is `runtime` or `feature`. Feature plugins run only at the center and cannot declare a node artifact.

## Compatibility

`requires.control_api` declares the exact control API major required by every plugin. `requires.agent_api` is present exactly when the release contains a node component. `requires.ui_api` is present exactly when it contains a UI component.

A host can support multiple majors during migration. Installation is rejected unless every required major appears in the relevant host's advertised set.

## Permissions

Each permission contains a stable dotted `name` and a human-readable `reason`. Duplicate declarations are invalid. The SDK validates permission syntax; the installing host separately rejects names it does not implement and asks the administrator to approve all recognized permissions before execution.

Permissions describe approved SDK operations, not direct database, unrestricted process, or arbitrary filesystem access.

## Artifacts

Each role appears at most once:

- `center`: required Linux AMD64 static executable.
- `node`: optional Linux AMD64 static executable for runtime plugins.
- `ui`: optional browser asset archive without an OS or architecture.

`file` is a single GitHub Release asset name, never a path or URL. `sha256` contains 64 lowercase hexadecimal characters. Relayward resolves the named asset from the installed GitHub repository and verifies its digest before activation.

The fixture in `fixtures/contract-plugin/manifest.json` demonstrates the complete shape. Its checksums are deliberately fake and the fixture is not an installable release.

## Validation

Unknown fields and trailing JSON values are rejected for v1. A manifest is limited to 1 MiB by Relayward hosts and conformance tooling.

```bash
relayward-conformance manifest relayward-plugin.json
```

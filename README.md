# Relayward SDK

Relayward SDK owns the versioned contracts shared by the Relayward control plane, Agent, plugins, and sandboxed plugin pages.

The initial contract foundation provides:

- a strict, versioned plugin release manifest;
- explicit permission declarations and host API compatibility checks;
- a generic control-message envelope and standard problem codes;
- the Linux AMD64 Agent registration and heartbeat control contract;
- Go validation helpers and a conformance command;
- a TypeScript package for shared UI-facing protocol types;
- fixtures used by producer and consumer CI.

Proxy protocols and unfinished business RPCs are intentionally absent. They are added only with a working producer, consumer, and conformance fixture.

## Local Checks

```bash
go test ./...
go vet ./...
go run ./cmd/relayward-conformance manifest fixtures/contract-plugin/manifest.json
go run ./cmd/relayward-conformance agent-register fixtures/agent/register-request.json
go run ./cmd/relayward-conformance agent-envelope fixtures/agent/hello.json

cd ui
npm ci
npm run typecheck
npm test
npm run build
```

Versioning and compatibility rules are documented in `docs/versioning.md`; the release manifest is specified in `docs/plugin-manifest.md`.

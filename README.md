# Relayward SDK

Relayward SDK owns the versioned contracts shared by the Relayward control plane, Agent, plugins, and sandboxed plugin pages.

The initial contract foundation provides:

- a strict, versioned plugin release manifest;
- the center-plugin lifecycle, permission-gated Host RPC, and sandbox UI bridge contracts;
- idempotent plugin event publication and channel-neutral notification request contracts;
- the node-plugin Unix-socket gRPC lifecycle and configuration API;
- deterministic authorization periods, policy reconciliation, traffic, activity, service-control, and dynamic-blocking contracts;
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
npm --prefix ui ci
npm --prefix ui run typecheck
npm --prefix ui test
npm --prefix ui run build
./scripts/build-contract-plugin.sh 0.1.0 /tmp/relayward-contract-plugin
go run ./cmd/relayward-conformance plugin-release /tmp/relayward-contract-plugin
go run ./cmd/relayward-conformance agent-register fixtures/agent/register-request.json
go run ./cmd/relayward-conformance agent-envelope fixtures/agent/hello.json
go run ./cmd/relayward-conformance agent-envelope fixtures/agent/command.json
go run ./cmd/relayward-conformance agent-envelope fixtures/agent/command-result.json
go run ./cmd/relayward-conformance agent-envelope fixtures/agent/command-result-ack.json
go run ./cmd/relayward-conformance agent-event-batch fixtures/agent/event-batch.json
go run ./cmd/relayward-conformance agent-event-ack fixtures/agent/event-batch-ack.json
go run ./cmd/relayward-conformance agent-policy-reconcile fixtures/agent/policy-reconcile-command.json
go run ./cmd/relayward-conformance agent-traffic-snapshot fixtures/agent/traffic-snapshot.json
go run ./cmd/relayward-conformance agent-access-event fixtures/agent/access-event.json
go run ./cmd/relayward-conformance node-plugin-info fixtures/node-plugin/info.json
go run ./cmd/relayward-conformance center-plugin-services fixtures/center-plugin/services.json
go run ./cmd/relayward-conformance center-plugin-subscription fixtures/center-plugin/subscription.json
go run ./cmd/relayward-conformance center-plugin-published-events fixtures/center-plugin/published-events.json

```

Versioning and compatibility rules are documented in `docs/versioning.md`; the release manifest is specified in `docs/plugin-manifest.md`.

## Releases

Semantic `vMAJOR.MINOR.PATCH` tags publish the Go module and a GitHub Release. A release contains:

- the static Linux AMD64 `relayward-conformance` command;
- a versioned `@relayward/ui-sdk` package archive;
- deterministic shared conformance fixtures;
- the installable contract-test plugin manifest, center and node binaries, and UI archive;
- `SHA256SUMS` for every published asset.

The UI SDK is distributed through GitHub Releases rather than an npm registry. The contract-test plugin is for conformance and integration testing only; it is not a production proxy runtime.

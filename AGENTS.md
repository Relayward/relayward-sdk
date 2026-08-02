# Relayward SDK AGENTS.md

## Project Role

This repository owns the versioned contracts shared by the Relayward control plane, Agent, runtime plugins, feature plugins, and sandboxed plugin pages. It also owns Go and TypeScript SDK helpers, conformance fixtures, and contract-test tooling.

Business storage, control-plane services, Agent runtime behavior, and production plugins do not belong in this repository.

## Contract Governance

- Define a public contract only when its producer and consumer behavior is understood.
- Prefer small, explicit, versioned messages over generic maps or untyped payloads.
- Specify identity, ordering, idempotency, compatibility, error, timeout, and retry semantics alongside each contract.
- Treat backward-incompatible changes as new contract versions. Do not preserve retired xui-stack wire or data compatibility.
- Keep proxy-core-specific fields inside runtime-plugin-owned payloads; standard kernel telemetry remains core-neutral.

## Security

- Treat manifests, RPC requests, events, subscription fragments, and plugin UI messages as untrusted input.
- Do not expose credentials, tokens, raw private configuration, or unrestricted filesystem and process access through SDK contracts.
- Keep plugin permissions explicit and narrowly scoped.

## Engineering Conventions

- Keep Go and TypeScript representations behaviorally aligned where they model the same contract.
- Add shared fixtures and conformance tests for each frozen cross-process contract.
- Avoid framework dependencies in core contract packages unless they are required by the chosen transport.
- Do not add placeholder business protocols merely to make the SDK look complete.

## Validation

Run the checks relevant to the change:

- `go test ./...`
- `go vet ./...`
- `npm ci`, `npm run typecheck`, `npm test`, and `npm run build` from `ui/`

Contract changes must also be validated in every affected producer and consumer repository.

# Contract Versioning

Relayward applications and contract APIs use separate versions.

Application and plugin releases use semantic versions without a leading `v` inside manifests. Git tags add the conventional `v` prefix. Contract APIs use stable identifiers such as `relayward.plugin/v1` and `relayward.node-plugin/v1`, with explicit positive integer majors in compatibility declarations.

## Compatibility

- A host advertises every contract major it supports concurrently.
- A plugin is compatible only when every major in its `requires` object is advertised by the relevant host.
- Adding optional fields or new message types within a major is backward-compatible only when older consumers can safely ignore them.
- Removing or renaming fields, changing field meaning, or changing required behavior creates a new contract major.
- A new major does not implicitly replace the previous major. Removal requires an explicit deprecation window and evidence that installed consumers no longer need it.
- Unknown manifest fields are rejected. A manifest that needs new fields must declare a manifest API version understood by the host.

## Idempotency

Every state-changing command must carry an idempotency key scoped to its authenticated sender and command type. A receiver persists the request digest and terminal result before acknowledging the command. Repeating the same key and request returns the stored result; reusing the key with a different request returns `conflict`.

Transport retries do not imply command failure. Senders retain unacknowledged commands, and receivers must make duplicate delivery safe before a concrete command contract is frozen.

## Errors

Contract errors use the standard problem codes from the Go and TypeScript protocol packages. `retryable` describes whether repeating the unchanged request may succeed. Error messages are diagnostic text and must not be used for program flow or contain credentials and private configuration.

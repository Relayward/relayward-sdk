# Agent Control API v1

`relayward.agent/v1` defines registration and the authenticated control session between one Relayward center and one Relayward Agent. It is independent from plugin host APIs and carries no proxy-core-specific fields.

## Identity And Registration

An administrator first creates a node and a short-lived, single-use registration token. The Agent sends `RegisterRequest` to the center over HTTPS. The center atomically consumes the token, replaces any previous node credential, and returns a new random node credential exactly once.

Registration is restricted to Linux AMD64. A successful response binds the immutable node ID to the credential. The Agent stores the node ID, display name, and credential in an owner-only identity file; the center URL remains in the separate Agent configuration. Registration tokens and node credentials are separate secret classes and are never interchangeable.

Registration requests and responses use strict JSON. Unknown fields, unsupported API versions, unsupported platforms, malformed secrets, and unsorted or duplicate capabilities are rejected. HTTP error responses use `protocol.Problem` and never echo either secret.

## Control Session

The Agent initiates a WSS connection and authenticates with its node ID and credential. The Agent sends `agent.hello` first. The center validates that its payload matches the authenticated identity, fences any older session for the same node, and returns `center.hello` with a fresh session ID and heartbeat interval.

Only one session ID is active for a node at a time. A newer authenticated session supersedes the older connection. Messages from a superseded session cannot update node state.

After the handshake, the Agent sends `agent.heartbeat` at the negotiated interval. The center persists the latest accepted heartbeat before returning `center.heartbeat_ack`. The acknowledgement carries the originating envelope ID, allowing the Agent to reject delayed or mismatched replies. Missing three negotiated intervals is treated as a failed session and causes reconnect with bounded exponential backoff and jitter.

## Ordering And Retry

Control messages use the common envelope and its unique 128-bit message ID. Heartbeats are ordered within one WSS session but are not replayed after disconnection; the next accepted heartbeat replaces the previous liveness snapshot.

Registration can be retried only while the one-time token remains unused. A client that loses a successful registration response must obtain a new registration token because the original token has already been consumed.

## Durable Commands

The center persists a command before delivery. A heartbeat acknowledgement may carry one nested `center.command` envelope whose `idempotency_key` is the stable command ID. The command contains a typed kind, issuance and expiry times, and a bounded JSON payload. The command ID and SHA-256 digest of the complete command remain stable across every redelivery; transport envelope IDs do not.

The Agent validates and durably records the command ID and request digest before execution. Repeating the same ID and request returns the stored terminal result. Reusing an ID with a different digest is a protocol conflict. If the Agent stops after recording a request but before recording its result, it may invoke the handler again after restart; every command handler must therefore use the stable command ID and implement its own recoverable, idempotent state transition.

The Agent persists a terminal `agent.command_result` before sending it. The center validates the command ownership and request digest, persists the terminal result idempotently, and only then returns `center.command_result_ack`. An acknowledgement matches both the result envelope ID and the command digest. Until that acknowledgement is durably recorded locally, the Agent resends the stored result after reconnect. A repeated identical result is acknowledged again; a different result for a terminal command is a conflict.

Commands are delivered in creation order, one active command per node. Expired commands are not dispatched or started. Heartbeats continue while a command executes, and each handler receives a context limited to ten minutes. Command failure is terminal for that command ID even when its structured problem says a newly issued command may be retryable.

### Agent self-update

An `agent.update` command carries one semantic version without a leading `v`. The Agent resolves that version only from the fixed official Relayward Agent GitHub repository, verifies the bounded release manifest, exact artifact size and SHA-256, installed launcher compatibility, and the candidate's reported build version before switching an immutable version link.

The old process does not complete the command when it requests its supervised restart. The candidate must establish an authenticated control session and receive a heartbeat acknowledgement before the pending update is atomically confirmed. It then completes the original durable command with an `activated` output. A candidate that exits before confirmation is rolled back by the stable launcher; the restored version completes the same command as failed. Re-delivery uses the durable pending, confirmed, or failed update state instead of downloading or switching twice.

## Security And Limits

- Production registration uses HTTPS and control sessions use WSS.
- Registration bodies and control envelopes are limited to 1 MiB.
- Command payloads are limited to 512 KiB and result output to 256 KiB.
- Secrets must never appear in message envelopes, informational logs, errors, metrics, or audit metadata.
- The center stores only credential hashes.
- Agent state directories use owner-only permissions and identity files use mode `0600`.
- `protocol.error` contains a validated `protocol.Problem`; diagnostic text cannot be used for program flow.

Fixtures under `fixtures/agent/` are conformance inputs for the center and Agent implementations.

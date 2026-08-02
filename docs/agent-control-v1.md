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

State-changing commands are added to this API only together with durable idempotency behavior in both center and Agent. Their absence from the initial message set is intentional.

## Security And Limits

- Production registration uses HTTPS and control sessions use WSS.
- Registration bodies and control envelopes are limited to 1 MiB.
- Secrets must never appear in message envelopes, informational logs, errors, metrics, or audit metadata.
- The center stores only credential hashes.
- Agent state directories use owner-only permissions and identity files use mode `0600`.
- `protocol.error` contains a validated `protocol.Problem`; diagnostic text cannot be used for program flow.

Fixtures under `fixtures/agent/` are conformance inputs for the center and Agent implementations.

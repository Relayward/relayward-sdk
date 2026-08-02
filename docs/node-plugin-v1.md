# Node Plugin API v1

`relayward.node-plugin/v1` is the local gRPC contract between one Relayward Agent and one node plugin. It runs only over an Agent-owned Unix socket and does not expose a TCP listener.

## Process Contract

The Agent starts the immutable node artifact without command-line arguments and supplies these environment variables:

- `RELAYWARD_NODE_PLUGIN_SOCKET`: absolute path of the Unix socket the plugin must create.
- `RELAYWARD_NODE_PLUGIN_DATA_DIR`: persistent, plugin-specific data directory.
- `RELAYWARD_NODE_PLUGIN_ID`: expected manifest plugin ID.

The Agent creates private parent directories, removes only its own stale socket, and verifies the socket and gRPC identity before sending configuration. The plugin must not become externally active before `ApplyConfiguration` succeeds. Standard output and standard error are diagnostic streams and must not contain configuration, credentials, source addresses, or access events.

## Identity And Readiness

`GetInfo` reports the exact API version, immutable plugin ID, and semantic plugin version. The Agent rejects an artifact whose identity or version differs from the desired release. A process is ready only after `GetInfo` succeeds within the startup deadline.

## Configuration

Configuration is an opaque JSON object bounded by the Agent command limit. It can contain private runtime material, so the Agent stores it in owner-only state and never logs or places it in status events. Every request carries a positive monotonic generation and the SHA-256 of compact JSON.

The Agent calls `ValidateConfiguration` before `ApplyConfiguration`. Both responses must echo the exact generation and digest. Validation must not change externally observable plugin state. Apply must be atomic from the plugin's perspective: success means the complete generation is active; a gRPC error means the prior generation remains active or the process fails closed.

## Health

`GetStatus` reports the applied generation, configuration digest, health, and a bounded diagnostic message. `HEALTH_HEALTHY` is valid only after a configuration has been applied. Messages must not contain private configuration or credentials and are not used for program flow.

The Agent waits for the applied generation to become healthy before acknowledging a reconcile command. Failed upgrades or configuration changes restore the last successfully persisted version and configuration. Process crashes after activation are restarted with bounded backoff; terminal state changes are emitted as durable `plugin.status` events.

## Reconciliation

The center sends one full `plugin.reconcile` desired state rather than incremental install/start/configure commands. `running` and `stopped` include the exact version, bounded GitHub download URL, byte size, SHA-256, and complete configuration. `absent` carries none of those fields. Generation is monotonic per node and plugin.

The Agent persists a validated desired state before execution. Replaying the same command is idempotent through the Agent command store. A successful result reports the achieved generation, state, version, and configuration digest; a failed result leaves the last successful state available. Signed download URLs are transient delivery data and must not appear in audit metadata or events.

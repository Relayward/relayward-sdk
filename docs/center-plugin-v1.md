# Center Plugin API v1

`relayward.center-plugin/v1` is the local gRPC contract between the Relayward center and one installed center plugin. It is never exposed on TCP.

## Process contract

The center starts the immutable Linux AMD64 center artifact without arguments. The plugin creates the socket named by `RELAYWARD_CENTER_PLUGIN_SOCKET`; the center creates a separate Host RPC socket named by `RELAYWARD_CENTER_HOST_SOCKET`. The center also supplies an owner-only persistent data directory and the expected plugin ID.

The plugin must report its exact API version, plugin ID, and semantic release version before activation. The center sends the complete sorted set of administrator-approved permissions. The plugin must echo that set and become healthy before the release is committed as active.

## Host permissions

Host RPC authorization is bound to the plugin process, its expected plugin ID, and its approved manifest permissions. Version 1 defines:

- `core.nodes.read`, which permits `ListNodes`. The response exposes node ID, display name, enabled state, and current connection state; it does not expose node credentials, addresses, Agent hostnames, or event data.
- `core.events.read`, which permits a feature plugin to consume the standard event stream. Event access includes source IP and destination data when present, so it must be declared with a specific reason and approved explicitly. Runtime plugins are never registered as event consumers even if they declare the permission.
- `core.services.write`, which permits `ReplaceServices`. A runtime plugin atomically replaces only its own service catalog for one node. The Host supplies the plugin identity from the supervised process, so a plugin cannot create services for another plugin. An empty list removes the plugin's catalog for that node. Service IDs and capabilities are sorted, unique, bounded values. Each service supplies a SHA-256 digest of all plugin-owned inputs that can change its subscription output; the digest reveals no configuration and changes whenever those inputs change.

A missing permission is returned as gRPC `PermissionDenied`. Registering services for a node where the plugin has no node instance is rejected.

New Host RPCs require a concrete producer, consumer, permission, validation rules, and conformance test. Plugins never receive direct database access.

## Feature event delivery

Each active feature plugin approved for `core.events.read` has its own durable center-side cursor and failure state. The center sends bounded batches through `ConsumeEvents` in ingest order. The envelope exposes a stable event ID, opaque cursor, node ID, kind, observed and received times, and the validated JSON payload; it does not expose Agent stream credentials or transport state.

Consumption is at least once. A plugin must persist any state derived from the complete batch before returning `through_cursor`, and must deduplicate replays by event ID. The response can acknowledge only the final cursor, so partial success is not representable. An RPC error, invalid acknowledgement, timeout, crash, or unhealthy process leaves the cursor unchanged and records a retryable failure for that plugin without delaying Agent acknowledgement or any other consumer.

Replay is bounded by the center's hot-event retention. A plugin unavailable beyond that window resumes from the next retained event instead of preventing cleanup and allowing the event database to grow without bound. Long-term normalized access archives are independent of this live-consumer stream.

The same namespaced event interface is the extension point for later structured notification requests and notification-channel feature plugins. Version 1 deliberately does not define or implement Telegram, Uptime Kuma, or another channel-specific API.

## UI RPC

Sandboxed plugin pages call their own center plugin through the parent UI bridge. `InvokeUI` accepts a bounded plugin-defined method identifier and a JSON object, and returns bounded valid JSON. Browser credentials, administrator sessions, arbitrary center URLs, filesystem paths, and process controls are not part of this contract.

## Subscription rendering

`RenderSubscription` is called only for an active authorization and the enabled services bound to that plugin on one node. The request contains opaque node and authorization IDs, the node's public address, and the sorted bound service IDs and display names. It does not contain a subscription token, user contact details, traffic history, or another plugin's services.

The plugin returns exactly one contribution per requested service, in the same order. A contribution contains a display name and one or more bounded fragments: absolute share URIs, JSON objects representing Mihomo proxies, and JSON objects representing sing-box outbounds. Plugins may omit unsupported formats for a service, but each service must contribute at least one fragment. The center validates and canonicalizes every fragment, removes exact duplicates, and owns the final Base64, YAML, and JSON documents.

The operation must be deterministic and free of side effects. The center applies a bounded deadline. A complete result is cached against a digest of the authorization, node, bindings, service catalog, plugin versions, and request inputs; cached content is never reused after those inputs change.

## Health and recovery

The center polls `GetStatus` with bounded deadlines. Startup or activation failure leaves the previous active release unchanged. A later crash or repeated unhealthy result triggers bounded restart attempts; an upgrade that cannot become healthy restores the retained previous version.

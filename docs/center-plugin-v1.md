# Center Plugin API v1

`relayward.center-plugin/v1` is the local gRPC contract between the Relayward center and one installed center plugin. It is never exposed on TCP.

## Process contract

The center starts the immutable Linux AMD64 center artifact without arguments. The plugin creates the socket named by `RELAYWARD_CENTER_PLUGIN_SOCKET`; the center creates a separate Host RPC socket named by `RELAYWARD_CENTER_HOST_SOCKET`. The center also supplies an owner-only persistent data directory and the expected plugin ID.

The plugin must report its exact API version, plugin ID, and semantic release version before activation. The center sends the complete sorted set of administrator-approved permissions. The plugin must echo that set and become healthy before the release is committed as active.

## Host permissions

Host RPC authorization is bound to the plugin process and its approved manifest permissions. Version 1 implements only `core.nodes.read`, which permits `ListNodes`. The response exposes node ID, display name, enabled state, and current connection state; it does not expose node credentials, addresses, Agent hostnames, or event data. A missing permission is returned as gRPC `PermissionDenied`.

New Host RPCs require a concrete producer, consumer, permission, validation rules, and conformance test. Plugins never receive direct database access.

## UI RPC

Sandboxed plugin pages call their own center plugin through the parent UI bridge. `InvokeUI` accepts a bounded plugin-defined method identifier and a JSON object, and returns bounded valid JSON. Browser credentials, administrator sessions, arbitrary center URLs, filesystem paths, and process controls are not part of this contract.

## Health and recovery

The center polls `GetStatus` with bounded deadlines. Startup or activation failure leaves the previous active release unchanged. A later crash or repeated unhealthy result triggers bounded restart attempts; an upgrade that cannot become healthy restores the retained previous version.

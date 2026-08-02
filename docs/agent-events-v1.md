# Agent Event Delivery v1

Relayward Agents deliver durable events to the center through a separate authenticated HTTPS endpoint. Event delivery does not share the WebSocket control path, so a large upload or retry cannot delay heartbeats or command acknowledgements.

## Identity And Ordering

An Agent owns one random 128-bit stream ID for the lifetime of its local event queue. Sequence numbers start at one, increase without gaps, and survive process restarts. Deleting the queue creates a new stream rather than reusing sequence numbers from the previous stream.

Each event has a deterministic SHA-256 ID over the API version, node ID, stream ID, sequence, kind, UTC observation time, and compact JSON payload. The batch repeats the authenticated node ID so SDK validation can verify every event ID. The center rejects a batch whose node ID does not match the URL and bearer credential.

## Delivery And Acknowledgement

The Agent sends the oldest contiguous pending events in a gzip-compressed JSON batch. A batch contains at most 500 events, each payload is at most 256 KiB, the compressed request is at most 1 MiB, and expanded JSON is at most 4 MiB.

The center writes the complete contiguous range and its stream cursor in one event-database transaction. Only after that transaction commits does it return the highest contiguous sequence. Replaying identical events is accepted and acknowledged again. Reusing a node, stream, and sequence with different content, reusing an event ID for different content, or sending a gap is a conflict.

The Agent removes events only after durably applying a matching acknowledgement. A missing, malformed, or out-of-range acknowledgement leaves the batch on disk for retry. The queue has explicit byte and event-count limits; producers receive a capacity error instead of silent success when either limit is reached.

## Event Semantics

Event transport is core-neutral. Kinds are namespaced identifiers and payloads are bounded JSON. The first standard payloads are:

- `traffic.snapshot`: the Agent's absolute upload and download ledger for one authorization and deterministic period, with a monotonic revision. The center replaces an older revision rather than summing deliveries.
- `access.observed`: a normalized runtime access decision with the plugin telemetry stream ID, a source event ID stable inside that stream, authorization and service identity, and only the applicable source, destination, port, network, protocol, and action fields. Consumers deduplicate by node, plugin, source stream, and source event ID so retries collapse without confusing a later plugin installation that starts a new stream.
- `policy.status`: the locally enforced authorization state, absolute period totals, active and blocked IP counts, and enforcement reason.
- `plugin.status`: node-plugin lifecycle state and the capabilities reported by a healthy running process.

The center first persists and acknowledges the transport event. Independent consumers normalize traffic and access payloads using their own durable cursors; consumer failure does not delay Agent acknowledgement. Access payloads never contain proxy credentials, subscription tokens, UUID credentials, or complete client configuration.

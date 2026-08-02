# Contract Test Plugin Fixture

This non-production runtime plugin exercises center and node process lifecycle, permission-gated Host RPC, structured event publication, configuration rollback, health reporting, and the sandbox UI bridge.

Build an installable GitHub Release-shaped directory with:

```sh
npm --prefix ui run build
./scripts/build-contract-plugin.sh 0.1.0 /tmp/relayward-contract-plugin
go run ./cmd/relayward-conformance plugin-release /tmp/relayward-contract-plugin
```

The generated directory contains accurate artifact sizes and SHA-256 digests. It is a CI fixture and does not provide proxy capability.

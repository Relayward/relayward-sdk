import { describe, expect, it } from "vitest";

import {
  createRelaywardUIClient,
  MANIFEST_API_VERSION,
  RelaywardUIError,
  UI_API_MAJOR,
  UI_BRIDGE_API_VERSION,
  type PluginUIRequest,
  type UITransport,
} from "./index";

describe("contract versions", () => {
  it("exports the frozen v1 identifiers", () => {
    expect(MANIFEST_API_VERSION).toBe("relayward.plugin/v1");
    expect(UI_API_MAJOR).toBe(1);
  });
});

class MemoryTransport implements UITransport {
  sent: PluginUIRequest[] = [];
  listener: ((message: unknown) => void) | undefined;

  send(message: PluginUIRequest) {
    this.sent.push(message);
  }

  subscribe(listener: (message: unknown) => void) {
    this.listener = listener;
    return () => { this.listener = undefined; };
  }

  respond(result: unknown, ok = true) {
    const request = this.sent.at(-1);
    if (!request) throw new Error("request is missing");
    this.listener?.({
      api_version: UI_BRIDGE_API_VERSION,
      direction: "host-to-plugin",
      id: request.id,
      ok,
      ...(ok ? { result } : { problem: result }),
    });
  }
}

describe("plugin UI bridge", () => {
  it("correlates context and plugin RPC responses", async () => {
    const transport = new MemoryTransport();
    const client = createRelaywardUIClient(transport);

    const context = client.context();
    transport.respond({ plugin_id: "io.relayward.test", theme: "light" });
    await expect(context).resolves.toEqual({ plugin_id: "io.relayward.test", theme: "light" });

    const rpc = client.rpc<{ count: number }>("nodes.summary", { enabled: true });
    expect(transport.sent.at(-1)?.payload).toEqual({ method: "nodes.summary", parameters: { enabled: true } });
    transport.respond({ count: 2 });
    await expect(rpc).resolves.toEqual({ count: 2 });
    client.dispose();
  });

  it("surfaces bounded host problems and rejects invalid methods", async () => {
    const transport = new MemoryTransport();
    const client = createRelaywardUIClient(transport);
    const confirmation = client.confirm({ title: "Remove", message: "Remove this item?", destructive: true });
    transport.respond({ code: "permission_denied", message: "Denied.", retryable: false }, false);
    await expect(confirmation).rejects.toBeInstanceOf(RelaywardUIError);
    await expect(client.rpc("../admin", {})).rejects.toThrow("Invalid plugin UI RPC method");
    client.dispose();
  });
});

import { browserUITransport, createRelaywardUIClient } from "./relayward-ui-sdk.js";

const client = createRelaywardUIClient(browserUITransport());
const identity = document.querySelector("#identity");
const nodeCount = document.querySelector("#node-count");
const error = document.querySelector("#error");

async function refresh() {
  error.textContent = "";
  try {
    const context = await client.context();
    const summary = await client.rpc("nodes.summary", {});
    identity.textContent = context.plugin_id;
    nodeCount.textContent = String(summary.count);
  } catch (cause) {
    error.textContent = cause instanceof Error ? cause.message : "Plugin request failed.";
  }
}

document.querySelector("#refresh").addEventListener("click", refresh);
document.querySelector("#navigate").addEventListener("click", () => client.navigate("nodes"));
window.addEventListener("pagehide", () => client.dispose(), { once: true });
void refresh();

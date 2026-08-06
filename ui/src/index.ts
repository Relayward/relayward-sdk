export const MANIFEST_API_VERSION = "relayward.plugin/v1" as const;
export const UI_API_MAJOR = 1 as const;
export const UI_BRIDGE_API_VERSION = "relayward.plugin-ui/v1" as const;

export type ErrorCode =
  | "invalid_argument"
  | "unauthenticated"
  | "permission_denied"
  | "not_found"
  | "conflict"
  | "unsupported"
  | "unavailable"
  | "internal";

export interface FieldViolation {
  field: string;
  description: string;
}

export interface Problem {
  code: ErrorCode;
  message: string;
  retryable: boolean;
  violations?: FieldViolation[];
}

export type Theme = "light" | "dark";
export type Locale = "zh-CN" | "en";
export type NavigationTarget = "plugins" | "nodes" | "users" | "authorizations" | "audit";

export interface UIContext {
  plugin_id: string;
  theme: Theme;
  locale: Locale;
}

export interface ConfirmOptions {
  title: string;
  message: string;
  confirm_label?: string;
  destructive?: boolean;
}

export interface UITransport {
  send(message: PluginUIRequest): void;
  subscribe(listener: (message: unknown) => void): () => void;
}

export interface RelaywardUIClient {
  context(): Promise<UIContext>;
  rpc<T>(method: string, parameters: Record<string, unknown>): Promise<T>;
  navigate(target: NavigationTarget): Promise<void>;
  confirm(options: ConfirmOptions): Promise<boolean>;
  dispose(): void;
}

type BridgeMethod = "context" | "rpc" | "navigate" | "confirm";

export interface PluginUIRequest {
  api_version: typeof UI_BRIDGE_API_VERSION;
  direction: "plugin-to-host";
  id: string;
  method: BridgeMethod;
  payload: unknown;
}

export interface PluginUIResponse {
  api_version: typeof UI_BRIDGE_API_VERSION;
  direction: "host-to-plugin";
  id: string;
  ok: boolean;
  result?: unknown;
  problem?: Problem;
}

interface PendingRequest {
  resolve: (value: unknown) => void;
  reject: (reason: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

const uiMethodPattern = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/;
const maximumUIJSONBytes = 512 << 10;

export function createRelaywardUIClient(transport: UITransport, timeoutMilliseconds = 15_000): RelaywardUIClient {
  if (!Number.isSafeInteger(timeoutMilliseconds) || timeoutMilliseconds < 1 || timeoutMilliseconds > 60_000) {
    throw new Error("timeoutMilliseconds must be between 1 and 60000");
  }
  let sequence = 0;
  let disposed = false;
  const pending = new Map<string, PendingRequest>();
  const unsubscribe = transport.subscribe((candidate) => {
    if (!isPluginUIResponse(candidate)) return;
    const request = pending.get(candidate.id);
    if (!request) return;
    pending.delete(candidate.id);
    clearTimeout(request.timer);
    if (candidate.ok) {
      request.resolve(candidate.result);
      return;
    }
    request.reject(new RelaywardUIError(candidate.problem ?? internalProblem()));
  });

  function call(method: BridgeMethod, payload: unknown): Promise<unknown> {
    if (disposed) return Promise.reject(new Error("Relayward UI client is disposed"));
    const normalizedPayload = normalizeJSON(payload);
    const id = `ui-${Date.now().toString(36)}-${(++sequence).toString(36)}`;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id);
        reject(new Error("Relayward UI host request timed out"));
      }, timeoutMilliseconds);
      pending.set(id, { resolve, reject, timer });
      try {
        transport.send({
          api_version: UI_BRIDGE_API_VERSION,
          direction: "plugin-to-host",
          id,
          method,
          payload: normalizedPayload,
        });
      } catch (cause) {
        clearTimeout(timer);
        pending.delete(id);
        reject(cause instanceof Error ? cause : new Error("Relayward UI host request failed"));
      }
    });
  }

  return {
    async context() {
      const value = await call("context", {});
      if (!isUIContext(value)) throw new Error("Relayward UI host returned an invalid context");
      return value;
    },
    async rpc<T>(method: string, parameters: Record<string, unknown>) {
      if (method.length > 128 || !uiMethodPattern.test(method)) throw new Error("Invalid plugin UI RPC method");
      return await call("rpc", { method, parameters }) as T;
    },
    async navigate(target: NavigationTarget) {
      if (!(["plugins", "nodes", "users", "authorizations", "audit"] as string[]).includes(target)) {
        throw new Error("Invalid Relayward navigation target");
      }
      await call("navigate", { target });
    },
    async confirm(options: ConfirmOptions) {
      const value = await call("confirm", options);
      if (typeof value !== "boolean") throw new Error("Relayward UI host returned an invalid confirmation");
      return value;
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribe();
      for (const request of pending.values()) {
        clearTimeout(request.timer);
        request.reject(new Error("Relayward UI client is disposed"));
      }
      pending.clear();
    },
  };
}

export function browserUITransport(): UITransport {
  if (typeof window === "undefined" || window.parent === window) {
    throw new Error("Relayward UI SDK must run inside a plugin iframe");
  }
  const parent = window.parent;
  return {
    send(message) {
      parent.postMessage(message, "*");
    },
    subscribe(listener) {
      const receive = (event: MessageEvent<unknown>) => {
        if (event.source === parent) listener(event.data);
      };
      window.addEventListener("message", receive);
      return () => window.removeEventListener("message", receive);
    },
  };
}

export class RelaywardUIError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.message);
    this.name = "RelaywardUIError";
    this.problem = problem;
  }
}

function normalizeJSON(value: unknown): unknown {
  let encoded: string | undefined;
  try {
    encoded = JSON.stringify(value);
  } catch {
    throw new Error("Relayward UI request must be JSON serializable");
  }
  if (encoded === undefined || encoded.length > maximumUIJSONBytes) {
    throw new Error(`Relayward UI request must contain at most ${maximumUIJSONBytes} bytes of JSON`);
  }
  return JSON.parse(encoded) as unknown;
}

function isPluginUIResponse(value: unknown): value is PluginUIResponse {
  if (!isRecord(value) || value.api_version !== UI_BRIDGE_API_VERSION || value.direction !== "host-to-plugin" ||
      typeof value.id !== "string" || value.id.length === 0 || typeof value.ok !== "boolean") return false;
  if (value.ok) return value.problem === undefined;
  return value.result === undefined && (value.problem === undefined || isProblem(value.problem));
}

function isProblem(value: unknown): value is Problem {
  if (!isRecord(value) || typeof value.code !== "string" || typeof value.message !== "string" || typeof value.retryable !== "boolean") return false;
  return ["invalid_argument", "unauthenticated", "permission_denied", "not_found", "conflict", "unsupported", "unavailable", "internal"].includes(value.code);
}

function isUIContext(value: unknown): value is UIContext {
  return isRecord(value) && typeof value.plugin_id === "string" && value.plugin_id.length > 0 &&
    (value.theme === "light" || value.theme === "dark") && (value.locale === "zh-CN" || value.locale === "en");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function internalProblem(): Problem {
  return { code: "internal", message: "Relayward UI host request failed.", retryable: false };
}

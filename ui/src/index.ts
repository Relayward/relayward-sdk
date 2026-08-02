export const MANIFEST_API_VERSION = "relayward.plugin/v1" as const;
export const UI_API_MAJOR = 1 as const;

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

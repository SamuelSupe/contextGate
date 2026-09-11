export const errorLabels: Record<string, string> = {
  consent_expired:
    "Authorization request expired. Start again from your client.",
  client_not_found: "Client not found. Register the client and try again.",
  unauthorized: "Your session or credential is no longer valid. Sign in again.",
  invalid_credentials: "Incorrect password. Please try again.",
  csrf_failed: "Session verification failed. Reload the page and try again.",
  cancelled: "Query cancelled or authorization changed.",
  conflict: "This configuration changed. Reload it before editing.",
  select_at_least_one_source: "Select at least one data source.",
};
export interface Diagnostic {
  code: string;
  message: string;
  request_id?: string;
  native_code?: string;
}
export function diagnostic(error: Diagnostic) {
  return [
    errorLabels[error.code] || error.message,
    error.native_code && `Database code: ${error.native_code}.`,
    error.request_id && `Request ID: ${error.request_id}`,
  ]
    .filter(Boolean)
    .join(" ");
}
export class APIError extends Error {
  constructor(public detail: Diagnostic) {
    super(diagnostic(detail));
  }
}
let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(path, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf,
      ...options.headers,
    },
  });
  const body = await response.json();
  if (!response.ok) {
    const e = body.error;
    throw new APIError(
      typeof e === "object" && e
        ? e
        : {
            code: String(e || "request_failed"),
            message:
              errorLabels[e] ||
              body.error_description ||
              `Request failed (${response.status}).`,
          },
    );
  }
  return body as T;
}
export const payload = (value: unknown) => JSON.stringify(value);
export function message(error: unknown) {
  if (error instanceof DOMException && error.name === "AbortError")
    return "Query cancelled.";
  return error instanceof Error
    ? error.message
    : "Operation failed. Please try again.";
}
export function date(value?: string) {
  return value
    ? new Date(value).toLocaleString("en-GB", { hour12: false })
    : "Never";
}

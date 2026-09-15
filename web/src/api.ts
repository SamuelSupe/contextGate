import { getLocale, t, translate } from "./i18n";

export const errorLabels: Record<string, string> = {
  consent_expired:
    "Authorization request expired. Start again from your client.",
  client_not_found: "Client not found. Register the client and try again.",
  unauthorized: "Your session or credential is no longer valid. Sign in again.",
  invalid_credentials: "Incorrect username or password. Please try again.",
  password_change_required: "Change your temporary password before continuing.",
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
export function diagnostic(error: Diagnostic, localized = true) {
  const text = localized
    ? t
    : (value: string, args?: Record<string, string>) =>
        translate("en", value, args);
  return [
    text(errorLabels[error.code] || error.message),
    error.native_code &&
      text("Database code: {code}.", { code: error.native_code }),
    error.request_id && text("Request ID: {id}", { id: error.request_id }),
  ]
    .filter(Boolean)
    .join(" ");
}
export class APIError extends Error {
  constructor(public detail: Diagnostic) {
    super(diagnostic(detail, false));
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
    if (
      response.status === 401 &&
      !["/api/login", "/api/setup"].includes(path)
    ) {
      window.dispatchEvent(new Event("contextgate:session-expired"));
    }
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
    ? new Date(value).toLocaleString(
        getLocale() === "zh-CN" ? "zh-CN" : "en-GB",
        { hour12: false },
      )
    : t("Never");
}

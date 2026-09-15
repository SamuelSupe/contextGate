import { t } from "./i18n";
import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  X,
  Database,
  LoaderCircle,
  CheckCircle2,
  AlertCircle,
  ShieldCheck,
  ShieldAlert,
  Copy,
} from "lucide-react";
import { date, diagnostic } from "./api";
import type { Probe } from "./types";

export function Button({
  children,
  busy = false,
  primary = false,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  busy?: boolean;
  primary?: boolean;
}) {
  return (
    <button
      type="button"
      {...props}
      disabled={props.disabled || busy}
      className={`${primary ? "primary" : ""} ${props.className || ""}`}
    >
      {busy ? <LoaderCircle size={15} className="spin" /> : null}
      {children}
    </button>
  );
}
export function Field({
  label,
  required = false,
  children,
  hint,
}: {
  label: string;
  required?: boolean;
  children: ReactNode;
  hint?: string;
}) {
  return (
    <label className="field">
      <span>
        {label}
        {required ? <b className="required">*</b> : null}
      </span>
      {children}
      {hint ? <small>{hint}</small> : null}
    </label>
  );
}
export function ErrorNote({ error }: { error: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (error) ref.current?.scrollIntoView({ block: "nearest" });
  }, [error]);
  return error ? (
    <div ref={ref} className="notice error" role="alert">
      <AlertCircle size={17} />
      <span>{t(error)}</span>
    </div>
  ) : null;
}
export function Empty({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <Database size={30} strokeWidth={1.3} />
      <h3>{title}</h3>
      {description ? <p>{description}</p> : null}
      {action}
    </div>
  );
}
export function Loading() {
  return (
    <div className="loading" role="status">
      <LoaderCircle className="spin" size={20} />
      {t("Loading…")}
    </div>
  );
}
export function Protection({
  probe,
  detail = false,
}: {
  probe?: Probe;
  detail?: boolean;
}) {
  if (!probe)
    return (
      <span className="status muted">
        <AlertCircle size={15} />
        {t("Not verified")}
      </span>
    );
  if (!probe.connected && !detail)
    return (
      <span className="status amber">
        <ShieldAlert size={15} />
        {t("Unverified")}
      </span>
    );
  if (!probe.connected)
    return (
      <div className="notice warning">
        <span>
          <strong>{t("Connection check failed")}</strong>
          {detail && probe.error ? <p>{diagnostic(probe.error)}</p> : null}
          <small className="block">
            {t("Checked ")}
            {date(probe.checked_at)}
            {t(" · Read-only protection is unverified")}
          </small>
        </span>
      </div>
    );
  const verified = ["engine_enforced", "verified"].includes(
    probe.permission_status,
  );
  const isolated = probe.permission_status === "api_isolated";
  if (detail)
    return (
      <div className={`notice ${verified ? "success" : "warning"}`}>
        <span>
          {verified ? <ShieldCheck size={20} /> : <ShieldAlert size={20} />}
        </span>
        <div>
          <strong>
            {verified
              ? probe.permission_status === "engine_enforced"
                ? t("Engine read-only protection verified")
                : t("Account read permissions verified")
              : isolated
                ? t("Query API isolation")
                : probe.protection === "declared_read_api"
                  ? t("Administrator-declared read-only operation")
                  : t("Account permissions unverified")}
          </strong>
          <p>
            {verified
              ? probe.permission_status === "engine_enforced"
                ? t(
                    "The engine enforces read-only execution. This does not verify all permissions held by the database account.",
                  )
                : t("The reported database grants provide read-only access.")
              : isolated
                ? t(
                    "Only query APIs are exposed. The database token itself has administrator permissions.",
                  )
                : probe.protection === "declared_read_api"
                  ? t(
                      "ContextGate restricts the request, but cannot prove an arbitrary API is read-only. This declaration is not a verified upstream permission.",
                    )
                  : t(
                      "Query operations are restricted. Verify account grants in the database.",
                    )}
          </p>
          <details>
            <summary>{t("View verification evidence")}</summary>
            <ul>
              {probe.evidence.map((item) => (
                <li key={item}>{t(item)}</li>
              ))}
            </ul>
            <small>
              {probe.server_version
                ? t(
                    probe.protection === "declared_read_api"
                      ? "API contract {server_version} · "
                      : "Database version {server_version} · ",
                    {
                      server_version: probe.server_version,
                    },
                  )
                : ""}
              {date(probe.checked_at)}
            </small>
          </details>
        </div>
      </div>
    );
  return (
    <span className={`status ${verified ? "green" : "amber"}`}>
      {verified ? <CheckCircle2 size={15} /> : <ShieldAlert size={15} />}{" "}
      {verified
        ? probe.permission_status === "engine_enforced"
          ? t("Engine enforced")
          : t("Account verified")
        : isolated
          ? t("API isolation")
          : probe.protection === "declared_read_api"
            ? t("Declared read-only")
            : t("Permissions unverified")}
    </span>
  );
}
export function Drawer({
  title,
  subtitle,
  children,
  footer,
  onClose,
  wide = false,
  returnFocus,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  wide?: boolean;
  returnFocus?: HTMLElement | null;
}) {
  const ref = useRef<HTMLElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useEffect(() => {
    const previous =
      returnFocus || (document.activeElement as HTMLElement | null);
    const pane = ref.current;
    const first = pane?.querySelector<HTMLElement>(
      "input,button,select,textarea",
    );
    first?.focus();
    const handle = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeRef.current();
      if (e.key === "Tab" && pane) {
        const items = [
          ...pane.querySelectorAll<HTMLElement>(
            "button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),a[href],summary",
          ),
        ].filter((x) => x.offsetParent !== null);
        if (items.length === 0) return;
        const first = items[0],
          last = items[items.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", handle);
    return () => {
      document.removeEventListener("keydown", handle);
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return (
    <>
      <div className="drawer-scrim" onClick={onClose} />
      <aside
        ref={ref}
        className={`drawer ${wide ? "wide" : ""}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header>
          <div>
            <h2>{title}</h2>
            {subtitle ? <p>{subtitle}</p> : null}
          </div>
          <button
            className="icon-button"
            aria-label={t("Close panel")}
            onClick={onClose}
          >
            <X size={20} />
          </button>
        </header>
        <div className="drawer-body">{children}</div>
        {footer ? <footer>{footer}</footer> : null}
      </aside>
    </>
  );
}
export function CopyButton({
  text,
  onCopied,
}: {
  text: string;
  onCopied?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    setCopied(false);
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [text]);
  return (
    <Button
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setCopied(true);
          if (timer.current) clearTimeout(timer.current);
          timer.current = setTimeout(() => setCopied(false), 2000);
          onCopied?.();
        } catch {
          window.prompt(t("Copy content"), text);
        }
      }}
    >
      {copied ? <CheckCircle2 size={14} /> : <Copy size={14} />}
      <span aria-live="polite">{copied ? t("Copied") : t("Copy")}</span>
    </Button>
  );
}

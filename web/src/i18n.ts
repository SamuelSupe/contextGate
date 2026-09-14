import { useSyncExternalStore } from "react";
import zhCN from "./locales/zh-CN.json" with { type: "json" };

export type Locale = "en" | "zh-CN";
export const localeStorageKey = "mcpdbhub.locale.v1";
const listeners = new Set<() => void>();

export function validLocale(value: unknown): Locale {
  return value === "zh-CN" ? "zh-CN" : "en";
}

function savedLocale(): Locale {
  try {
    return validLocale(localStorage.getItem(localeStorageKey));
  } catch {
    return "en";
  }
}

let locale = savedLocale();
export const getLocale = () => locale;

function updateLocale(next: Locale) {
  if (locale === next) return;
  locale = next;
  listeners.forEach((listener) => listener());
}

export function setLocale(next: Locale) {
  updateLocale(validLocale(next));
  try {
    localStorage.setItem(localeStorageKey, locale);
  } catch {
    // Private browsing can deny storage; switching still works for this page.
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

if (typeof window !== "undefined") {
  window.addEventListener("storage", (event) => {
    if (event.key === localeStorageKey || event.key === null)
      updateLocale(savedLocale());
  });
}

export const useLocale = () =>
  useSyncExternalStore(subscribe, getLocale, () => "en" as Locale);

type Values = Record<string, string | number | boolean | null | undefined>;
export function translate(
  language: Locale,
  text: string,
  values: Values = {},
): string {
  const translated =
    language === "zh-CN" && Object.hasOwn(zhCN, text.trim())
      ? (zhCN as Record<string, string>)[text.trim()]
      : undefined;
  const format =
    translated === undefined
      ? text
      : (text.match(/^\s*/)?.[0] || "") +
        translated +
        (text.match(/\s*$/)?.[0] || "");
  // Only explicitly supplied placeholders are replaced. Values are never
  // translated or interpreted as markup, JSON, query text, or other placeholders.
  return format.replace(/\{(\w+)\}/g, (match, key: string) =>
    Object.hasOwn(values, key) ? String(values[key]) : match,
  );
}

export const t = (text: string, values?: Values) =>
  translate(locale, text, values);

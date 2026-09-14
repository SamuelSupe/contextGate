import assert from "node:assert/strict";
import test from "node:test";
import zhCN from "../src/locales/zh-CN.json" with { type: "json" };
import {
  getLocale,
  localeStorageKey,
  setLocale,
  t,
  translate,
  validLocale,
} from "../src/i18n.ts";
import { parameterJSON, parameterValues } from "../src/template-parameters.ts";

test("translated messages preserve interpolation contracts and opaque business values", () => {
  const placeholders = (text: string) =>
    [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
  for (const [english, chinese] of Object.entries(zhCN)) {
    assert.deepEqual(placeholders(chinese), placeholders(english), english);
  }
  const name = "Settings <script> {name} 客户";
  assert.equal(translate("zh-CN", "Connect {name}", { name }), `连接 ${name}`);
  assert.equal(translate("en", "Connect {name}", { name }), `Connect ${name}`);
  assert.equal(translate("zh-CN", "toString"), "toString");
  assert.equal(
    translate("zh-CN", "Unknown server diagnostic"),
    "Unknown server diagnostic",
  );
  assert.equal(translate("zh-CN", "Connect {name}"), "连接 {name}");
});

test("language preference survives reload input, tolerates unavailable storage and leaves query values intact", () => {
  const previous = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  const values = new Map<string, string>();
  try {
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: {
        setItem: (key: string, value: string) => values.set(key, value),
      },
    });
    setLocale("zh-CN");
    assert.equal(t("Settings"), "设置");
    assert.equal(validLocale(values.get(localeStorageKey)), "zh-CN");
    const query =
      '{"name":"Settings","id":9007199254740993,"amount":0.123456789012345678901}';
    assert.deepEqual(
      parameterValues(parameterJSON(parameterValues(query))),
      parameterValues(query),
    );
    setLocale("en");
    assert.equal(t("Settings"), "Settings");
    assert.equal(validLocale(values.get(localeStorageKey)), "en");
    assert.equal(validLocale("unsupported"), "en");
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      get() {
        throw new Error("storage unavailable");
      },
    });
    assert.doesNotThrow(() => setLocale("zh-CN"));
    assert.equal(getLocale(), "zh-CN");
  } finally {
    if (previous) Object.defineProperty(globalThis, "localStorage", previous);
    else Reflect.deleteProperty(globalThis, "localStorage");
    setLocale("en");
  }
});

/**
 * No source file calls the browser's own `confirm` or `prompt`.
 * jetstore_maintenance_02 `I-39`, 2026-10-02.
 *
 * **The embedded browser pane the app is tested in never shows either.**
 * Measured: `window.confirm(...)` returns `false` in about a millisecond with
 * nothing on screen, so every confirmed action and every prompt was a button
 * that silently did nothing there. Eleven call sites now ask the app's own
 * dialog (`shell/prompts.tsx`). A twelfth written natively would pass every
 * other test — screen tests drive the app's dialog, and one that stubbed
 * `window.confirm` would answer whatever it was told — so the rule is asserted
 * here, as a text search over the sources, exactly as strong as it looks: the
 * `dialogStyles.test.ts` precedent.
 */

import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = fileURLToPath(new URL("./", import.meta.url));

function sources(dir: string, found: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = `${dir}${entry.name}`;
    if (entry.isDirectory()) sources(`${path}/`, found);
    else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) found.push(path);
  }
  return found;
}

/** Strips `//` and block comments, so prose that names `window.prompt` is not a call. */
const code = (text: string) => text.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/.*$/gm, "");

/**
 * A call of the browser's own: `window.confirm(`, `window.prompt(`,
 * `globalThis.…`, or a bare `confirm(`/`prompt(` — but not a method call such
 * as `prompts.confirm(` or `host.confirm(`, which is this file's API and the
 * action grammar's, and not a declaration such as `confirm(message: string)`.
 */
const NATIVE = /(?:\b(?:window|globalThis|self)\s*\.\s*(?:confirm|prompt)\s*\(|(?<![.\w])(?:confirm|prompt)\s*\((?![^)]*:))/;

describe("the browser's own prompts", () => {
  it("are called from no source file under src/", () => {
    const sites = sources(here)
      .filter((path) => NATIVE.test(code(readFileSync(path, "utf8"))))
      .map((path) => path.slice(here.length));
    expect(sites).toEqual([]);
  });

  it("would be found if one were written", () => {
    // The guard's own negative control: a pattern that matched nothing would
    // pass the case above for ever.
    expect(NATIVE.test("const ok = window.confirm(message);")).toBe(true);
    expect(NATIVE.test("const id = window.prompt(request.prompt);")).toBe(true);
    expect(NATIVE.test("if (!confirm('sure?')) return;")).toBe(true);
    expect(NATIVE.test("const agreed = await prompts.confirm(message);")).toBe(false);
    expect(NATIVE.test("return (await host.confirm(step.message)) ? a : b;")).toBe(false);
    expect(NATIVE.test("  confirm(message: string): Promise<boolean>;")).toBe(false);
  });
});

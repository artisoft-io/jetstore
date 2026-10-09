/**
 * Every dialog in the app, against the stylesheet rule that sizes it.
 * jetstore_maintenance_02 `AB.1` (D08), 2026-10-01.
 *
 * **Before that date nothing styled a `<dialog>`** (assessment F38), so every
 * dialog was as wide as its content and the failure viewer was a twenty-column
 * text box. The fix is one `.uf-dialog` rule in `styles.css`, and Michel's answer
 * to Q-7 is that it covers *every* dialog. That claim is only as good as the
 * premise under it — that every dialog carries the class — so this file asserts
 * the premise rather than trusting it:
 *
 *  1. exactly one source file renders a `<dialog>` element, and it is
 *     `userflow/FormDialog.tsx`, where `ModalDialog` sets the class;
 *  2. that class has a rule, with a width, a viewport bound, and a phone
 *     breakpoint that reaches it.
 *
 * The first is a text search, exactly as strong as it looks — the
 * `buttonStyles.test.ts` precedent. A JSX `<dialog` in a second file fails it by
 * name; a `<dialog>` written in backticks in a comment does not, which is how
 * the corpus mentions the element in prose.
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

/** A `<dialog` that opens an element: not preceded by a backtick, which is prose. */
const DIALOG_ELEMENT = /(^|[^`])<dialog(\s|>|$)/m;

describe("dialogs and the rule that sizes them", () => {
  it("renders a <dialog> element in one file only, the one that sets .uf-dialog", () => {
    const sites = sources(here)
      .filter((path) => DIALOG_ELEMENT.test(readFileSync(path, "utf8")))
      .map((path) => path.slice(here.length));
    expect(sites).toEqual(["userflow/FormDialog.tsx"]);
    expect(readFileSync(`${here}userflow/FormDialog.tsx`, "utf8")).toMatch(/className="uf-dialog"/);
  });

  it("gives .uf-dialog a width, a viewport bound and a phone breakpoint", () => {
    const css = readFileSync(`${here}styles.css`, "utf8");
    const rule = /\n\.uf-dialog \{([^}]*)\}/.exec(css)?.[1] ?? "";
    expect(rule).toMatch(/\bwidth:/);
    expect(rule).toMatch(/max-width:/);
    expect(rule).toMatch(/max-height:/);
    const phone = /@media \(max-width: \d+px\) \{\s*\.uf-dialog \{([^}]*)\}/.exec(css)?.[1] ?? "";
    expect(phone).toMatch(/width: 100%/);
    expect(phone).toMatch(/height: 100%/);
  });
});

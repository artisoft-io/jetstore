/**
 * Every button style a document declares, against the stylesheet that has to
 * draw it. Task **D.11**, from the report on the flow navigation buttons.
 *
 * **`buttonFidelity.test.ts` says what it does not cover and this is the half it
 * names.** That file compares *declarations* — the document says `primary`, the
 * Dart said `ufPrimary`, 147 of 147 agree — and stops there. Nothing then asked
 * whether `primary` reaches a rule. It did not: `FormRenderer` emitted
 * `btn--primary` where the stylesheet defines `.btn-primary`, so **537 buttons
 * across the 14 installed flow documents drew as the base button** with their
 * declared style inert, and the two corpora agreed with each other the whole
 * time.
 *
 * **D.4 fixed the same defect in the table action bar and did not find this
 * one**, because it was reported rather than measured — *Start Pipeline does not
 * look primary* names one button, and the fix that follows a report is the size
 * of the report. The comment D.4 left enumerated the call sites it had checked,
 * which is I-281's shape and is why the count above is eleven times the 49 it
 * counted.
 *
 * So this file asserts the join the two sides never had:
 *
 *  1. every style any installed document declares has a rule, or is on the short
 *     list of styles that deliberately have none;
 *  2. no source file emits the `btn--` form at all.
 *
 * The second is the blunt one and is the one that would have caught both sites.
 * It is a text search over the sources, which is exactly as strong as it looks —
 * it cannot see a class name built some other way — but the failure it guards
 * is a typo in a template literal, and that is the shape a typo takes.
 */

import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = fileURLToPath(new URL("./", import.meta.url));
const assets = `${here}../../jets/workspace_assets/`;

/**
 * Styles with no rule, and why each is intentional.
 *
 * `secondary` **is** the base button — `.btn` alone — and the stylesheet says so
 * where the rule would otherwise go. Listing it here rather than giving it an
 * empty rule keeps the stylesheet honest about which classes do something.
 */
const NO_RULE_NEEDED = new Set(["secondary"]);

function declaredStyles(): Map<string, string[]> {
  const found = new Map<string, string[]>();
  const note = (style: unknown, where: string) => {
    if (typeof style !== "string") return;
    found.set(style, [...(found.get(style) ?? []), where]);
  };
  const walk = (node: unknown, where: string): void => {
    if (Array.isArray(node)) return node.forEach((n) => walk(n, where));
    if (node === null || typeof node !== "object") return;
    const record = node as Record<string, unknown>;
    if ("style" in record && ("label" in record || "action" in record)) note(record["style"], where);
    for (const value of Object.values(record)) walk(value, where);
  };
  for (const dir of [`${assets}user_flows/`, `${assets}table_configs/`]) {
    for (const file of readdirSync(dir).filter((f) => f.endsWith(".json"))) {
      walk(JSON.parse(readFileSync(dir + file, "utf8")), file);
    }
  }
  for (const file of readdirSync(`${here}screens/documents/`).filter((f) => f.endsWith(".json"))) {
    walk(JSON.parse(readFileSync(`${here}screens/documents/${file}`, "utf8")), file);
  }
  return found;
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = `${dir}${entry.name}`;
    if (entry.isDirectory()) return sourceFiles(`${path}/`);
    // **Tests are excluded, and this file is why**: its own `it` title contains
    // the literal it searches for, so the first run flagged the check itself.
    // Nothing under a test renders production UI, so the exclusion costs the
    // check nothing and removes the one false positive it can generate.
    if (/\.test\.tsx?$/.test(entry.name)) return [];
    return /\.tsx?$/.test(entry.name) ? [path] : [];
  });
}

describe("declared button styles against the stylesheet", () => {
  it("finds the styles the installed documents actually declare", () => {
    // An equality, so that a new style brings somebody back to this file rather
    // than being drawn as the base button and nobody noticing for a phase.
    expect([...declaredStyles().keys()].sort()).toEqual(["danger", "primary", "secondary"]);
  });

  it("gives every declared style a rule, or a reason for having none", () => {
    const css = readFileSync(`${here}styles.css`, "utf8");
    const unstyled: string[] = [];
    for (const [style, where] of declaredStyles()) {
      if (NO_RULE_NEEDED.has(style)) continue;
      if (!new RegExp(`\\.btn-${style}\\b`).test(css)) {
        unstyled.push(`${style} (declared in ${[...new Set(where)].slice(0, 3).join(", ")})`);
      }
    }
    expect(unstyled).toEqual([]);
  });

  it("emits no `btn--` class anywhere in the sources", () => {
    // The regression guard, and the one that would have caught both sites: D.4
    // corrected `datatable/ActionBar.tsx` and left `userflow/FormRenderer.tsx`
    // and `screens/InferServerAdmin.tsx` emitting it.
    const offenders: string[] = [];
    for (const path of sourceFiles(here)) {
      for (const [i, line] of readFileSync(path, "utf8").split("\n").entries()) {
        // **Comment lines are skipped, and the first run is why.** This file and
        // `FormRenderer.tsx` both discuss `btn--` in prose to say what went
        // wrong, and a search that cannot tell an explanation from an emission
        // flags the explanation — which is the failure mode that gets a test
        // deleted rather than fixed. A commented-out class name emits nothing,
        // so nothing is lost by not looking at it.
        const code = line.trim();
        if (code.startsWith("//") || code.startsWith("*") || code.startsWith("/*")) continue;
        // **A bare search for the token, and the first attempt was cleverer and
        // did not work.** It required a quote immediately before `btn--`, which
        // the actual defect does not have — the line reads
        // `` `btn btn--${style}` ``, with the class the typo is in sitting second
        // in the list. Reintroducing the bug and watching this test still pass is
        // what found that; a guard nobody has seen fail is a guess.
        if (line.includes("btn--")) offenders.push(`${path.slice(here.length)}:${i + 1}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});

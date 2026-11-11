/**
 * No table document is committed in two places.
 *
 * A table a flow draws is installed into a workspace by `install_workspace_assets`
 * and read from there; a table a screen draws is imported into this bundle. The
 * split is by consumer, and for one day it had an exception: `pipelineExecStatusTable`
 * is drawn by `homeFiltersUF` from the workspace *and* by the Home screen, so X.5
 * committed it in both places and this file asserted the two copies agreed.
 *
 * **A guard on two copies is not a fix, and this is what replaced it.** Home now
 * reads the installed document at mount (`Home.tsx`, `WORKSPACE_TABLE`), so there
 * is one copy again. What is asserted here is the invariant rather than the
 * exception: two copies kept in step by nothing is the failure this repository
 * keeps finding — I-14's two names for one thing, C.0a's fixture that survived a
 * regeneration because the checksum was a guard on the side that computed it.
 *
 * If a second screen ever needs a flow's table, the answer this file exists to
 * push you towards is Home's: read it from the workspace. The alternative is a
 * copy, and a copy needs an argument rather than a `cp`.
 */

import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const bundled = fileURLToPath(new URL("./tables/", import.meta.url));
const installed = fileURLToPath(
  new URL("../../../jets/workspace_assets/table_configs/", import.meta.url),
);

const keysIn = (dir: string): Set<string> =>
  new Set(
    readdirSync(dir)
      .filter((f) => f.endsWith(".tc.json"))
      .map((f) => f.slice(0, -".tc.json".length)),
  );

describe("the two table corpora", () => {
  it("share no document", () => {
    const inBoth = [...keysIn(bundled)].filter((k) => keysIn(installed).has(k)).sort();
    expect(inBoth).toEqual([]);
  });

  it("are both non-empty, so an empty directory cannot pass the check above", () => {
    expect(keysIn(bundled).size).toBeGreaterThan(0);
    expect(keysIn(installed).size).toBeGreaterThan(0);
  });
  it("show no two columns of one table under the same heading", () => {
    // Process Errors showed two `Domain Key` columns, the run's object type and
    // the row's key value, and a reader could not tell which was which.
    const repeated: string[] = [];
    for (const dir of [bundled, installed]) {
      for (const key of keysIn(dir)) {
        const doc = JSON.parse(readFileSync(`${dir}${key}.tc.json`, "utf8")) as {
          columns?: { label?: string; isHidden?: boolean }[];
        };
        const seen = new Set<string>();
        for (const column of doc.columns ?? []) {
          if (column.isHidden || column.label === undefined) continue;
          if (seen.has(column.label)) repeated.push(`${key}: ${column.label}`);
          seen.add(column.label);
        }
      }
    }
    expect(repeated).toEqual([]);
  });
});

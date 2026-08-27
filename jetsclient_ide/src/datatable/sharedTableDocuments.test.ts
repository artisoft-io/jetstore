/**
 * The table documents committed in two places, and whether they agree.
 *
 * A table a flow draws is installed into a workspace by `install_workspace_assets`
 * and read from there by `FlowStore.loadTables`; a table a screen draws is imported
 * into this bundle. The split is by consumer, so a table with both kinds of consumer
 * is committed twice — once under `jets/workspace_assets/table_configs/` and once
 * here.
 *
 * **Two copies kept in step by nothing is the failure this repository keeps
 * finding** — I-14's two names for one thing, C.0a's fixture that stayed stale
 * through a regeneration because the checksum was a guard on the side that computed
 * it. `table.test.ts` writes both copies from one emitter, which makes them agree at
 * the moment of regeneration and says nothing about a hand edit to one of them.
 * This supplies the missing operand.
 *
 * **The list is named rather than derived, and that is the point of it.** Sharing is
 * a decision: it means a table is drawn by a flow *and* by a screen, and the honest
 * resolution is usually to make the screen read it from the workspace so that one
 * copy is left. A second entry appearing here should be a change somebody argued
 * for, not a file that turned up.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const bundled = fileURLToPath(new URL("./tables/", import.meta.url));
const installed = fileURLToPath(
  new URL("../../../jets/workspace_assets/table_configs/", import.meta.url),
);

/**
 * Drawn by `homeFiltersUF` from the workspace and by the Home screen from the
 * bundle — F.5 and C.6, which is where the sharing was first recorded.
 */
const SHARED = ["pipelineExecStatusTable"] as const;

describe("a table document committed in both places", () => {
  for (const key of SHARED) {
    it(`${key} is byte-identical in the bundle and in the workspace assets`, () => {
      expect(readFileSync(`${installed}${key}.tc.json`, "utf8")).toBe(
        readFileSync(`${bundled}${key}.tc.json`, "utf8"),
      );
    });
  }

  it("is one document, and a second is a decision rather than a discovery", async () => {
    const { readdirSync } = await import("node:fs");
    const keysIn = (dir: string) =>
      new Set(
        readdirSync(dir)
          .filter((f) => f.endsWith(".tc.json"))
          .map((f) => f.slice(0, -".tc.json".length)),
      );
    const inBoth = [...keysIn(bundled)].filter((k) => keysIn(installed).has(k)).sort();
    expect(inBoth).toEqual([...SHARED].sort());
  });
});

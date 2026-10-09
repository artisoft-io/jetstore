/**
 * Every flow on disk is a consistent set. `jetstore_maintenance_02` task AG.1
 * (`R12`, that project's `I-9`).
 *
 * ## Why this walks the directory rather than naming the flows
 *
 * Until 2026-10-01 this check lived in `proofFlows.test.ts` as an `it.each` over
 * **ten** flows named by hand, against fourteen `.uf.json` on disk: `mapFileUF`
 * and the three flows `cpipes-contract` projects were never in it. That was
 * tolerable while every document was either emitted from the legacy fixture or
 * projected by a tool, because each had an ancestor that had once worked. **It
 * stops being tolerable when documents are hand-authored or written by a
 * model** — the direction `ui_refresh`'s `I-299` was given on 2026-10-01 — since a
 * new flow added to the directory and not to the list would be checked against
 * its schema (Go, on save) and never against its siblings. The Go corpus tests
 * already glob `user_flows/`; this is the TypeScript half doing the same.
 *
 * ## What "consistent" means here
 *
 * The same checks `FlowStore.load` runs between documents, in the same order,
 * minus the two that need a build rather than the files:
 *
 * 1. the three sibling documents exist and parse against their schemas;
 * 2. `validateDocumentSet` — forms, actions, end-state buttons, item sources;
 * 3. every table the forms name exists in `table_configs/` and parses;
 * 4. `validateTableActions` — a table's `doAction` and `showDialog` resolve in
 *    *this flow's* action and form documents.
 *
 * Escape resolution against the production registry is the validator's (AG.2),
 * not this test's.
 */

import { existsSync, readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { TableConfigDocumentSchema, type TableConfigDocument } from "../datatable/table";
import { validateDocumentSet, validateTableActions } from "./documentSet";
import { FormDocumentSchema, type FormDocument } from "./form";
import { UserFlowSchema, type UserFlow } from "./schema";
import { tableKeysOf } from "./store";

const assetsDir = fileURLToPath(new URL("../../../jets/workspace_assets/", import.meta.url));

/** One line per problem, so a failure names every offence at once. */
function checkFlowOnDisk(root: string, key: string): string[] {
  const problems: string[] = [];
  const read = <T>(path: string, schema: { safeParse(v: unknown): { success: boolean; data?: unknown; error?: unknown } }): T | undefined => {
    const full = `${root}${path}`;
    if (!existsSync(full)) {
      problems.push(`${path}: missing`);
      return undefined;
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(readFileSync(full, "utf8"));
    } catch (error) {
      problems.push(`${path}: not valid JSON: ${(error as Error).message}`);
      return undefined;
    }
    const result = schema.safeParse(parsed);
    if (!result.success) {
      problems.push(`${path}: does not parse: ${String(result.error)}`);
      return undefined;
    }
    return result.data as T;
  };

  const flow = read<UserFlow>(`user_flows/${key}.uf.json`, UserFlowSchema);
  const actions = read<ActionDocument>(`user_flows/${key}.ua.json`, ActionDocumentSchema);
  const forms = read<FormDocument>(`user_flows/${key}.form.json`, FormDocumentSchema);
  if (flow === undefined || actions === undefined || forms === undefined) return problems;

  for (const f of validateDocumentSet({ flow, actions, forms })) {
    problems.push(`${f.document}${f.path}: ${f.message}`);
  }

  const tables: Record<string, TableConfigDocument> = {};
  for (const tableKey of tableKeysOf(forms)) {
    const table = read<TableConfigDocument>(`table_configs/${tableKey}.tc.json`, TableConfigDocumentSchema);
    if (table !== undefined) tables[tableKey] = table;
  }
  for (const f of validateTableActions(actions, forms, tables)) {
    problems.push(`${f.document}${f.path}: ${f.message}`);
  }
  return problems;
}

/** Every flow key under `<root>user_flows/`, read off the directory. */
const flowKeysOnDisk = (root: string): string[] =>
  readdirSync(`${root}user_flows/`)
    .filter((f) => f.endsWith(".uf.json"))
    .map((f) => f.slice(0, -".uf.json".length))
    .sort();

describe("every shipping flow is a consistent set", () => {
  const keys = flowKeysOnDisk(assetsDir);

  it("finds the flows on disk rather than from a list", () => {
    // A floor rather than an equality: the directory is allowed to grow and
    // shrink (track AD of this phase retires one flow), and an exact count would
    // make this a test of the news. What must not happen is the walk finding
    // nothing — a moved directory would turn every case below into a vacuous pass.
    expect(keys.length).toBeGreaterThan(0);
  });

  it.each(keys)("%s", (key) => {
    expect(checkFlowOnDisk(assetsDir, key)).toEqual([]);
  });
});

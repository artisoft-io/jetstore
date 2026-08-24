/**
 * Tests for the table configuration document. Task I.3.
 *
 * Four things, and they are not the same thing.
 *
 * 1. **The emitted JSON Schema matches the committed artifact**, which is what
 *    Go embeds and enforces at save time. Regenerate with `UPDATE_SCHEMA=1`.
 * 2. **All 37 shipping configurations translate and validate.** The rule S.1
 *    set: a real config that fails means the schema is wrong, not the config.
 * 3. **The translation loses nothing.** Round-tripping every configuration back
 *    through `fromDocument` must reproduce it exactly. A forward-only check
 *    cannot see a dropped field — every document would validate and the
 *    behaviour would be quietly gone.
 * 4. **The schema rejects what it claims to reject.** The rules this document
 *    adds over the Dart are asserted here; the negative suite proper carries the
 *    cases both languages run.
 */

import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import screenCorpus from "../screens/fixtures/screen_configs.json";
import {
  TableConfigDocumentSchema,
  actionNamesOf,
  emitJsonSchema,
  escapeNamesOf,
  tablePath,
  type TableConfigDocument,
} from "./table";
import { fromDocument, toDocument, toDocuments } from "./tableTranslate";
import type { TableConfig } from "./types";

const artifactPath = fileURLToPath(new URL("./table.schema.json", import.meta.url));
const tablesDir = fileURLToPath(new URL("./tables/", import.meta.url));

const tables = (corpus as { tables: Record<string, unknown> }).tables as Record<string, TableConfig>;

/**
 * The one non-flow configuration this directory carries, and why it is here.
 *
 * **F18, arriving as a file.** `pipelineExecStatusTable` is registered on the
 * non-flow side and *rendered* by `homeFiltersUF`'s last state, so F.5 cannot
 * author that flow without it — `FlowStore.load` reads a document for every table
 * a form names (`userflow/store.ts`, `loadTables`) and refuses the set when one is
 * missing. It is translated out of `screens/fixtures/screen_configs.json` rather
 * than hand-written, for the same reason the 37 are: a document written by hand is
 * a reading of the Dart and a document translated from the corpus is a
 * measurement of it.
 *
 * **The other 27 non-flow tables stay out of this directory until a screen needs
 * them**, which is C.6's decision to make and not this task's. What F.5 owes track
 * C is the *shape* — the schema fields this one needed and the escape names it
 * introduced — and that is what the assertions below pin down.
 */
const screenTables = (screenCorpus as { tables: Record<string, unknown> }).tables as Record<
  string,
  TableConfig
>;
const NON_FLOW_KEYS = ["pipelineExecStatusTable"] as const;

const flowDocuments = toDocuments(tables);
const documents: Record<string, TableConfigDocument> = {
  ...flowDocuments,
  ...Object.fromEntries(NON_FLOW_KEYS.map((key) => [key, toDocument(screenTables[key]!)])),
};
const configOf = (key: string): TableConfig => tables[key] ?? screenTables[key]!;
const documentOf = (key: string) => `${JSON.stringify(documents[key], null, 2)}\n`;

describe("the emitted JSON Schema", () => {
  it("matches the committed artifact", () => {
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") writeFileSync(artifactPath, emitted);
    expect(readFileSync(artifactPath, "utf8")).toBe(emitted);
  });

  it("has the 38 translated configurations committed beside it, for the Go check", () => {
    // `jets/userflow/table_schema_test.go` reads this directory and the emitted
    // schema and asserts the same documents pass the Go validator that enforces
    // them at save time — two languages against one artifact rather than two
    // readings of it.
    if (process.env.UPDATE_SCHEMA === "1") {
      rmSync(tablesDir, { recursive: true, force: true });
      mkdirSync(tablesDir, { recursive: true });
      for (const key of Object.keys(documents)) writeFileSync(`${tablesDir}${key}.tc.json`, documentOf(key));
    }
    const onDisk = readdirSync(tablesDir).filter((f) => f.endsWith(".tc.json")).sort();
    expect(onDisk).toEqual(Object.keys(documents).sort().map((k) => `${k}.tc.json`));
    for (const key of Object.keys(documents)) {
      expect(readFileSync(`${tablesDir}${key}.tc.json`, "utf8")).toBe(documentOf(key));
    }
  });

  it("is draft 2020-12, which is what santhosh-tekuri/jsonschema/v6 reads", () => {
    expect((emitJsonSchema() as Record<string, unknown>)["$schema"]).toBe(
      "https://json-schema.org/draft/2020-12/schema",
    );
  });

  it("closes every object", () => {
    // The flow schema has one deliberate exception, `states`, whose property
    // names are the state keys. This document has none: every object in it has a
    // fixed key set, so an unclosed one would be a hole rather than a design.
    // The two open maps it does carry — `navigationParams` and
    // `stateFormNavigationParams` — are `additionalProperties: {…}` with a
    // `propertyNames` constraint, which is a schema, not `true`.
    const open: string[] = [];
    const walk = (node: unknown, path: string): void => {
      if (node === null || typeof node !== "object") return;
      const obj = node as Record<string, unknown>;
      if (obj["type"] === "object") {
        const extra = obj["additionalProperties"];
        if (extra !== false && typeof extra !== "object") open.push(path);
      }
      for (const [key, value] of Object.entries(obj)) walk(value, `${path}.${key}`);
    };
    walk(emitJsonSchema(), "$");
    expect(open).toEqual([]);
  });
});

describe("the 37 shipping configurations", () => {
  it("all translate, and the one non-flow table F.5 needed makes 38", () => {
    expect(Object.keys(flowDocuments).length).toBe(37);
    expect(Object.keys(documents).length).toBe(38);
  });

  it("all validate against the schema", () => {
    const failures: string[] = [];
    for (const [key, doc] of Object.entries(documents)) {
      const parsed = TableConfigDocumentSchema.safeParse(doc);
      if (!parsed.success) failures.push(`${key}: ${parsed.error.issues.map((i) => i.message).join("; ")}`);
    }
    expect(failures).toEqual([]);
  });

  it("round-trip back to the corpus with nothing lost", () => {
    // The check the schema's cuts have to survive. Each cut in `table.ts` was
    // made because no configuration sets the field; if one does, this fails
    // rather than the document quietly meaning less than the Dart.
    for (const key of Object.keys(documents)) {
      expect({ [key]: fromDocument(key, documents[key]!) }).toEqual({ [key]: configOf(key) });
    }
  });

  it("split nine static and 28 query, as the corpus does", () => {
    // Derived from the documents; checked against a count taken from the corpus
    // a different way — `apiPath` being empty is how the Dart says "static", and
    // the discriminant is this schema's invention.
    const byKind = Object.values(flowDocuments).reduce<Record<string, number>>(
      (acc, d) => ({ ...acc, [d.source]: (acc[d.source] ?? 0) + 1 }),
      {},
    );
    const fromCorpus = Object.values(tables).filter((t) => t.apiPath === "").length;
    expect(byKind).toEqual({ static: 9, query: 28 });
    expect(fromCorpus).toBe(9);
  });

  it("carry the corpus's 275 columns, 25 actions and 49 where clauses", () => {
    const count = (f: (d: TableConfigDocument) => number) =>
      Object.values(flowDocuments).reduce((n, d) => n + f(d), 0);
    expect({
      columns: count((d) => d.columns.length),
      actions: count((d) => (d.source === "query" ? (d.actions ?? []).length : 0)),
      where: count((d) => (d.source === "query" ? (d.where ?? []).length : 0)),
    }).toEqual({ columns: 275, actions: 25, where: 49 });
  });

  it("name two escapes between them, not six", () => {
    // Six closures in the corpus — three `cellFilter`, three `isEnabledFnc` —
    // and reading them showed two bodies written three times each. Naming them
    // is what collapses the duplication; this asserts the collapse rather than
    // leaving it in a comment.
    const names = new Set(Object.values(flowDocuments).flatMap((d) => escapeNamesOf(d)));
    expect([...names].sort()).toEqual(["fileKeyLabel", "hasDataRegistryFilters"]);
  });

  it("name eleven action-document entries", () => {
    const names = new Set(Object.values(flowDocuments).flatMap((d) => actionNamesOf(d)));
    expect(names.size).toBe(11);
  });

  it("put every table configuration under table_configs/", () => {
    expect(tablePath("lfSourceConfigTable")).toBe("table_configs/lfSourceConfigTable.tc.json");
  });
});

/**
 * The decisions C.6 inherits. Task F.5.
 *
 * **F.5 and C.6 both want `pipelineExecStatusTable` and F.5 ran first**, so these
 * are set here rather than read from track C (plan §3, F18). They are asserted
 * rather than described because a decision recorded only in prose is a decision
 * the next task can contradict without anything failing.
 */
describe("pipelineExecStatusTable, the table track C and track F share", () => {
  const doc = documents["pipelineExecStatusTable"]!;

  it("is a query table with two from clauses and the source_period join", () => {
    expect(doc.source).toBe("query");
    if (doc.source !== "query") return;
    expect(doc.from).toEqual([
      { schema: "jetsapi", table: "pipeline_execution_status" },
      { schema: "jetsapi", table: "source_period" },
    ]);
    expect(doc.where).toEqual([{ column: "source_period_key", joinWith: "source_period.key" }]);
  });

  it("keeps the six action-bar buttons in order and the five row buttons apart", () => {
    if (doc.source !== "query") return;
    expect((doc.actions ?? []).map((a) => [a.key, a.action])).toEqual([
      ["startPipeline", "showScreen"],
      ["refreshTable", "refreshTable"],
      ["setHomeFilters", "showScreen"],
      ["setSessionIdFilters", "setSessionIdFilter"],
      ["setRequestIdFilters", "setRequestIdFilter"],
      ["clearHomeFilters", "clearHomeFilters"],
    ]);
    expect((doc.secondRowActions ?? []).map((a) => a.key)).toEqual([
      "viewStatusDetails",
      "viewProcessErrors",
      "viewFailureDetails",
      "viewExecStatsDetails",
      "resubmitPipeline",
    ]);
  });

  it("names the three closures apart, which the blanket mapping did not", () => {
    // The F.5 correction. `translateAction` sent every `hasIsEnabledFnc` to
    // `hasDataRegistryFilters`, which is true of the 37 and false of all three
    // here — `clearHomeFilters` reads `homeFilters` and the two prompt buttons
    // are `(state) => true`.
    if (doc.source !== "query") return;
    expect((doc.actions ?? []).filter((a) => a.isEnabled).map((a) => [a.key, a.isEnabled])).toEqual([
      ["setSessionIdFilters", "alwaysEnabled"],
      ["setRequestIdFilters", "alwaysEnabled"],
      ["clearHomeFilters", "hasHomeFilters"],
    ]);
    expect(escapeNamesOf(doc)).toEqual(["alwaysEnabled", "fileKeyLabel", "hasHomeFilters"]);
  });

  it("carries the second row's cross-document references, so I-88's check sees them", () => {
    // `resubmitPipeline` is the flow's only table-run action and
    // `showFailureDetailsDialog` its only table-opened form, and **both are on the
    // second row**. Before F.5 `actionNamesOf` and `validateTableActions` walked
    // the first row alone, so this table would have reported no references at all.
    expect(actionNamesOf(doc)).toEqual(["resubmitPipeline"]);
    if (doc.source !== "query") return;
    expect((doc.secondRowActions ?? []).find((a) => a.action === "showDialog")?.configForm).toBe(
      "showFailureDetailsDialog",
    );
  });

  it("authors the one calculated column and drops nothing else", () => {
    const calculated = doc.columns.filter((c) => c.calculatedAs);
    expect(calculated.map((c) => [c.name, c.calculatedAs])).toEqual([
      ["run_duration", "AGE(last_update, start_time)"],
    ]);
    expect(doc.columns.filter((c) => c.cellFilter).map((c) => c.name)).toEqual([
      "main_input_file_key",
    ]);
  });

  it("has no fromConfigRowActions, and that is a decision rather than an omission", () => {
    // `AppConfig.getConfigurableActionConfig()` builds them from `BUTTON_CFG_JSON`,
    // a `String.fromEnvironment` constant (`jetsclient/lib/button_config.dart`,
    // `buttonsConfigJson`). They are per-deployment buttons, not table
    // configuration, so they must not become authorable content — a workspace file
    // naming one would be a second way to configure a deployment. The corpus
    // records the list empty because the corpus was generated without the variable
    // set, which is the same reason it cannot be measured from here (**I-102**).
    expect(configOf("pipelineExecStatusTable").fromConfigRowActions).toEqual([]);
    expect(Object.keys(doc)).not.toContain("fromConfigRowActions");
  });
});

describe("the schema rejects", () => {
  const base = () => structuredClone(documents["lfSourceConfigTable"]!) as Record<string, unknown>;
  const staticBase = () => structuredClone(documents["input_format"]!) as Record<string, unknown>;
  const rejects = (doc: unknown) => expect(TableConfigDocumentSchema.safeParse(doc).success).toBe(false);

  it("a document with no source", () => {
    const doc = base();
    delete doc["source"];
    rejects(doc);
  });

  it("a static table that names a schema", () => {
    rejects({ ...staticBase(), from: [{ schema: "jetsapi", table: "client_registry" }] });
  });

  it("a query table that carries rows", () => {
    rejects({ ...base(), rows: [["a"]] });
  });

  it("a query table with no from clause", () => {
    rejects({ ...base(), from: [] });
  });

  it("a table with no columns", () => {
    rejects({ ...base(), columns: [] });
  });

  it("an unknown action type", () => {
    // The enum is the point: `actionType` in the Dart is a bare string, and the
    // seven here are the seven the corpus uses.
    rejects({ ...base(), actions: [{ key: "k", label: "L", action: "exec_ddl", style: "primary" }] });
  });

  it("an apiAction, which this document deliberately has no field for", () => {
    // The security cut. `apiAction` reaches `DataTableAction.Action` and
    // dispatches over `jets/apiserver/api_tables.go:42`, so a table that could
    // name one could name `exec_ddl`. `additionalProperties: false` is what
    // makes the absence enforceable rather than merely undocumented.
    rejects({ ...base(), apiAction: "exec_ddl" });
  });

  it("an unknown property anywhere", () => {
    rejects({ ...base(), defaultToAllRows: true });
  });

  it("a negative keyColumnIdx", () => {
    rejects({ ...base(), formStateBinding: { keyColumnIdx: -1 } });
  });

  it("a schemaVersion other than 1", () => {
    rejects({ ...base(), schemaVersion: 2 });
  });
});

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
 *
 * **C.4 is the third, and it is the one that widened the schema rather than the
 * list.** `queryToolResultSetTable` is the only configuration in either corpus
 * that asks the server for a *statement* rather than for a structure, and
 * translating it cost three schema changes that no earlier reading of the corpus
 * predicted: a query table may declare no `columns` and no `sortColumn` (four of
 * the non-flow 25 do, and the server describes the result instead), a where clause
 * may name no column (one clause of fifty, and it is a gate rather than a
 * predicate), and `requestColumnDef` comes back. **The seam held for the third
 * time and the schema moved for the second**, which is the honest summary: adding
 * the key is cheap and the document is only as cheap as the corpus is uniform.
 *
 * **C.2 is the first screen to widen this list, and it did exactly what F.5 said
 * widening it would be.** `workspaceRegistryTable` is the `/workspaces` screen's
 * only table; adding the key is the whole of adding the document, and what it
 * cost was two schema fields — `apiAction` and `enableWhen` — both of which
 * `table.ts` had already named as fields the non-flow corpus sets. The list is a
 * seam and it held.
 *
 * ## Two more, and the seam behaved as advertised. Task C.7
 *
 * `pipelineExecDetailsTable` and `cpipesExecDetailsTable` are the tables of
 * `/executionStatusDetails/:session_id` and `/executionStatsDetails/:session_id`
 * — one screen behind two routes (see `screens/TableScreen.tsx`). I-102 decision 1
 * said widening this list is the whole of adding a table, and for the second of
 * the two it was: one entry, and the document appeared. For the first it was one
 * entry plus two schema fields, because `error_message` sets `maxLines` and
 * `columnWidth` and the translation refused both.
 *
 * **That is the seam working rather than failing.** `toDocument` throws by name
 * on anything the schema dropped, so the cost of adding a table is bounded by
 * what it actually sets and is discovered at once rather than as a document that
 * silently means less than the Dart.
 *
 * **What "widening the list is the whole of adding a table" understates is the
 * counters, and there are three.** This file's *40*, `jets/userflow`'s
 * `TestShippingTablesValidate` and `jets/datatable`'s
 * `TestShippingTablesPassTheSaveCheck` each hard-code the directory's size, and
 * all three fail on the next table. **That is deliberate and should stay**: the
 * two sides emit and validate independently, so a literal on each is the only
 * assertion that catches a document emitted and not committed, or committed and
 * not emitted. But it makes the true cost *one entry, three counters, and
 * whatever the configuration sets that the schema dropped* — worth knowing
 * before the remaining 25 rather than after the first of them.
 */
const screenTables = (screenCorpus as { tables: Record<string, unknown> }).tables as Record<
  string,
  TableConfig
>;
const NON_FLOW_KEYS = [
  "pipelineExecStatusTable",
  "workspaceRegistryTable",
  "pipelineExecDetailsTable",
  "cpipesExecDetailsTable",
  "queryToolResultSetTable",
  "inputLoaderStatusTable",
  "inputTable",
  "inputFileViewerTable",
  "processErrorsTable",
  "inputRecordsFromProcessErrorTable",
  "reteSessionRdfTypeTable",
  "userTable",
  "userRolesTable",
  "ruleConfigv2Table",
  "wsDomainClassTable",
  "wsDataPropertyTable",
  "wsDomainTableTable",
  "wsJetRulesTable",
  "wsRuleTermsTable",
  "wsMainSupportFilesTable",
] as const;

/**
 * The two documents in this directory that are **not** translations. Task C.9.
 *
 * `reteSessionEntityKeyTable` and `reteSessionEntityDetailsTable` name a
 * `modelStateHandler`, which the screen corpus records as
 * `hasModelStateHandler: true` and cannot record further: the handler is a Dart
 * closure (`jetsclient/lib/modules/rete_session/model_handlers.dart`) and no
 * corpus contains a closure. So `toDocument` refuses them, deliberately, and
 * these two are written by hand.
 *
 * **They are the boundary of I-102 decision 1 rather than an exception to it.**
 * The decision says a non-flow table is *translated* because "a hand-written
 * document is a reading of the Dart where this is a measurement of it". That
 * holds wherever the corpus carries what the document needs; it stops holding
 * where the configuration's meaning is in code. Naming the boundary is more
 * useful than either restating the rule or quietly departing from it, because
 * the next reader's question is *which of my tables can I translate* and the
 * answer is now checkable: whichever ones `toDocument` does not refuse.
 *
 * **And the hand-authoring is put back under measurement**, which is the part
 * worth copying. `fromDocument` of each is asserted equal to the corpus
 * configuration in every field the corpus can express — see the last case in this
 * file — so what is trusted to a human here is exactly one field per document and
 * every other field is compared against the Dart.
 */
/**
 * **Two tasks reached this idea from opposite sides and they stay two lists — C.9
 * and C.3a, reconciled when they merged.** C.9's two name a Dart closure the
 * corpus records as a boolean, so `toDocument` refuses them and keeps refusing
 * them. C.3a's two have **no Dart original at all**, because the `lookups` view
 * was never built in Flutter, so there is nothing for `toDocument` to be given.
 *
 * **The consequence is shared and the premise is not**, which is why this is two
 * constants rather than one: neither kind can be a *measurement* of the Dart — the
 * claim I-102 decision 1 actually makes about a translated document — but only
 * C.9's kind can be checked field-by-field against a corpus entry, and only
 * C.3a's needs `jets/workspace_schema.sql` instead. Collapsing them would put both
 * checks on a list where half the members fail each.
 *
 * **The checkable rule they share is `toDocument`'s: a non-flow table is
 * translatable iff `toDocument` does not refuse it.** The two sessions invented
 * the names `HAND_AUTHORED_KEYS` and `AUTHORED_KEYS` independently, hours apart,
 * for lists in the same file; that they mean different things is luck rather than
 * design, and the distinction above is the one to read them by.
 */
const HAND_AUTHORED_KEYS = ["reteSessionEntityKeyTable", "reteSessionEntityDetailsTable"] as const;

/** C.3a's two: no Dart original, checked against `jets/workspace_schema.sql`. */
const AUTHORED_KEYS = ["wsLookupTableTable", "wsLookupColumnTable"] as const;

const flowDocuments = toDocuments(tables);
const translated: Record<string, TableConfigDocument> = {
  ...flowDocuments,
  ...Object.fromEntries(NON_FLOW_KEYS.map((key) => [key, toDocument(screenTables[key]!)])),
};
const readAuthored = (keys: readonly string[]): Record<string, TableConfigDocument> =>
  Object.fromEntries(
    keys.map((key) => [
      key,
      JSON.parse(readFileSync(`${tablesDir}${key}.tc.json`, "utf8")) as TableConfigDocument,
    ]),
  );
const handAuthored = readAuthored(HAND_AUTHORED_KEYS);
const authoredDocuments = readAuthored(AUTHORED_KEYS);
const documents: Record<string, TableConfigDocument> = {
  ...translated,
  ...handAuthored,
  ...authoredDocuments,
};
const configOf = (key: string): TableConfig => tables[key] ?? screenTables[key]!;
const documentOf = (key: string) => `${JSON.stringify(documents[key], null, 2)}\n`;

describe("the emitted JSON Schema", () => {
  it("matches the committed artifact", () => {
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") writeFileSync(artifactPath, emitted);
    expect(readFileSync(artifactPath, "utf8")).toBe(emitted);
  });

  it("has the 61 configurations committed beside it, for the Go check", () => {
    // `jets/userflow/table_schema_test.go` reads this directory and the emitted
    // schema and asserts the same documents pass the Go validator that enforces
    // them at save time — two languages against one artifact rather than two
    // readings of it.
    if (process.env.UPDATE_SCHEMA === "1") {
      mkdirSync(tablesDir, { recursive: true });
      // **Removing what this emitter no longer owns rather than wiping the
      // directory, as of C.9.** The wipe was right while every document was
      // emitted here; two are now hand-authored and committed beside the rest, and
      // a wipe would delete them on the next regeneration — silently, since the
      // very next line would rewrite everything else and the suite would go green
      // with two files gone.
      for (const file of readdirSync(tablesDir).filter((f) => f.endsWith(".tc.json"))) {
        if (!(file.slice(0, -".tc.json".length) in documents)) rmSync(`${tablesDir}${file}`);
      }
      for (const key of Object.keys(translated)) writeFileSync(`${tablesDir}${key}.tc.json`, documentOf(key));
    }
    const onDisk = readdirSync(tablesDir).filter((f) => f.endsWith(".tc.json")).sort();
    expect(onDisk).toEqual(Object.keys(documents).sort().map((k) => `${k}.tc.json`));
    for (const key of Object.keys(translated)) {
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
  it("all translate, and the twenty-four non-flow tables so far make 61", () => {
    // 37 + F.5's one + C.2's one + C.4's one + C.7's two + C.6's three + C.9's
    // five + C.13's two + C.10's one + C.3's six + C.3a's two, of which four are authored rather than translated. The three counts are asserted separately because
    // "how many documents are there" and "how many were measured rather than
    // written" are different questions and only the second can regress quietly.
    expect(Object.keys(flowDocuments).length).toBe(37);
    expect(Object.keys(translated).length).toBe(57);
    expect(Object.keys(handAuthored).length).toBe(2);
    expect(Object.keys(documents).length).toBe(61);
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
    for (const key of Object.keys({ ...translated, ...handAuthored })) {
      // **`modelSource` is dropped before comparing, and it is the one field in
      // `TableConfig` with no corpus counterpart.** Task C.9. The Dart spells a
      // form-state table's row source as two fields — `modelStateFormKey` and a
      // function pointer — and the document spells it as one construct; the
      // restore sets *both* the corpus field and this one, so the corpus half is
      // still compared and this half is what the corpus cannot hold. It is an
      // addition rather than a loss, which is what this case is guarding against.
      const { modelSource, ...restored } = fromDocument(key, documents[key]!);
      void modelSource;
      expect({ [key]: restored }).toEqual({ [key]: configOf(key) });
    }
  });

  /**
   * The two hand-authored documents, checked against the Dart. Task C.9.
   *
   * **This is what keeps `HAND_AUTHORED_KEYS` from being an escape hatch.** The
   * case above proves the *translated* documents lose nothing by restoring them
   * and comparing to the corpus; these two were never translated, so the same
   * comparison is the only thing standing between "authored from the Dart" and
   * "authored from memory". Everything the corpus can express is compared;
   * `modelSource` is not, for the reason above, and `hasModelStateHandler` is not,
   * because it is the one bit the corpus holds *instead of* the handler.
   */
  it("hand-authors the two handler-backed tables faithfully, field by field", () => {
    for (const key of HAND_AUTHORED_KEYS) {
      const { modelSource, hasModelStateHandler, ...restored } = fromDocument(key, documents[key]!);
      expect({ key, hasModelStateHandler }).toEqual({ key, hasModelStateHandler: true });
      expect(modelSource).toEqual({
        from: "map",
        key: expect.stringMatching(/^rete_session\./) as unknown as string,
        indexBy: expect.any(String) as unknown as string,
      });
      const { hasModelStateHandler: corpusFlag, ...corpus } = screenTables[key]!;
      expect({ key, corpusFlag }).toEqual({ key, corpusFlag: true });
      expect({ [key]: restored }).toEqual({ [key]: corpus });
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

/**
 * The two fields `workspaceRegistryTable` brought back. Task C.2.
 *
 * Asserted here for F.5's reason and one more of its own: **`enableWhen` is the
 * first field in this document whose spelling differs from the corpus's in a way
 * a round trip alone cannot check.** The round trip proves index → name → index
 * is exact; it cannot prove the *name* is the right one, because it reads the
 * same `columns` array in both directions. So the column name is pinned by hand.
 */
describe("workspaceRegistryTable, the /workspaces screen's table", () => {
  const doc = documents["workspaceRegistryTable"]!;
  const config = configOf("workspaceRegistryTable");

  it("accounts for every apiAction either corpus holds, which is what makes the enum closed", () => {
    // **The membership argument, asserted rather than described.** `table.ts`
    // carried these counts in a comment for about four hours; C.0a's fixture
    // regeneration then took `read` from 17 to 14 by removing three dead
    // configurations. The argument for a three-member enum is unaffected by that
    // and the sentence would have been wrong anyway, which is the case for putting
    // a count where something re-derives it.
    //
    // A fifth value appearing here is not a test to relax: it is a table asking
    // the apiserver for an authority no authored document has been allowed to
    // name, and the enum is the decision about whether it may.
    const byAction = Object.values(screenTables).reduce<Record<string, number>>(
      (acc, t) => ({ ...acc, [t.apiAction]: (acc[t.apiAction] ?? 0) + 1 }),
      {},
    );
    // The message is on the assertion rather than only in the comment above,
    // because "expected 4, got 5" teaches nobody and this is not a number to
    // update. A fifth key is a table asking the apiserver for an authority no
    // authored document has been allowed to name; whether it may is a decision,
    // and `ApiActionSchema` is where it is taken.
    expect(
      Object.keys(byAction).sort(),
      "a value here that ApiActionSchema does not carry is a table reaching an authority no authored document may name — widen the enum deliberately or not at all",
    ).toEqual(["preview_file", "raw_query_tool", "read", "workspace_read"]);
    // The three exceptions are counted and `read` is not: the enum is about which
    // authorities exist, and how many tables take the default is not a fact it
    // rests on — it is the fact C.0a moved.
    expect(byAction["workspace_read"]).toBe(9);
    expect(byAction["preview_file"]).toBe(1);
    expect(byAction["raw_query_tool"]).toBe(1);
    // All 37 flow tables read, which is why I.3 could answer S.7 with no field.
    expect([...new Set(Object.values(tables).map((t) => t.apiAction))]).toEqual(["read"]);
  });

  it("names workspace_read, which the 37 flow tables had no way to say", () => {
    if (doc.source !== "query") return;
    expect(doc.apiAction).toBe("workspace_read");
    // The round trip's own claim, stated separately because it is the one that
    // would have failed before C.2: `fromDocument` restored a constant `read`.
    expect(fromDocument("workspaceRegistryTable", doc).apiAction).toBe("workspace_read");
  });

  it("leaves apiAction off a table that reads, so the 37 are untouched", () => {
    const flow = flowDocuments["lfSourceConfigTable"]!;
    expect(Object.keys(flow)).not.toContain("apiAction");
    expect(fromDocument("lfSourceConfigTable", flow).apiAction).toBe("read");
  });

  it("gates eight buttons on the selected row's status, by column name", () => {
    if (doc.source !== "query") return;
    const gated = [...(doc.actions ?? []), ...(doc.secondRowActions ?? [])].filter(
      (a) => a.enableWhen !== undefined,
    );
    expect(gated.map((a) => a.key)).toEqual([
      "openWorkspace",
      "exportWorkspaceClientConfig",
      "loadWorkspaceConfig",
      "deleteWorkspace",
      "compileWorkspace",
      "commitWorkspace",
      "pushOnlyWorkspace",
      "pullWorkspace",
    ]);
    // Every criterion in either corpus tests this one column. The Dart says `6`;
    // that this is the column at index 6 is what the translation had to know.
    const columns = new Set(gated.flatMap((a) => a.enableWhen!.flat().map((c) => c.column)));
    expect([...columns]).toEqual(["status"]);
    expect(doc.columns[6]?.name).toBe("status");
  });

  it("keeps the disjunction of conjunctions rather than flattening it", () => {
    if (doc.source !== "query") return;
    // Two of the eight use the inner list, and a flattened `enableWhen` would
    // have turned "neither removed nor in progress" into "either", which is the
    // difference between refusing a mid-compile export and permitting one.
    const exportAction = (doc.actions ?? []).find((a) => a.key === "exportWorkspaceClientConfig");
    expect(exportAction?.enableWhen).toEqual([
      [
        { column: "status", is: "doesNotContain", value: "removed" },
        { column: "status", is: "doesNotContain", value: "in progress" },
      ],
    ]);
    const commit = (doc.secondRowActions ?? []).find((a) => a.key === "commitWorkspace");
    expect(commit?.enableWhen).toEqual([
      [{ column: "status", is: "contains", value: "modified" }],
    ]);
  });

  it("is the second table with a second action row, and the corpus has no third", () => {
    if (doc.source !== "query") return;
    expect((doc.secondRowActions ?? []).map((a) => a.key)).toEqual([
      "compileWorkspace",
      "commitWorkspace",
      "pushOnlyWorkspace",
      "pullWorkspace",
      "doGitStatus",
      "doGitCommand",
      "viewGitLogWorkspace",
      "refreshTable",
    ]);
    const withSecondRow = Object.entries(screenTables)
      .filter(([, t]) => t.secondRowActions.length > 0)
      .map(([key]) => key);
    expect(withSecondRow.sort()).toEqual(["pipelineExecStatusTable", "workspaceRegistryTable"]);
  });

  it("names no escape, so this screen needs no predicate registered", () => {
    // All eight gates are row criteria, which are data; `hasIsEnabledFnc` is
    // false on every action of this table. The two mechanisms look alike and are
    // not the same one — see `tableTranslate.ts`.
    expect(escapeNamesOf(doc)).toEqual([]);
    expect([...config.actions, ...config.secondRowActions].some((a) => a.hasIsEnabledFnc)).toBe(false);
  });

  it("names three action-document entries and five dialog forms on the second row", () => {
    expect(actionNamesOf(doc)).toEqual(["compileWorkspace", "deleteWorkspace", "openWorkspace"]);
    if (doc.source !== "query") return;
    expect(
      (doc.secondRowActions ?? []).filter((a) => a.action === "showDialog").map((a) => a.configForm),
    ).toEqual([
      "commitWorkspaceDialog",
      "pushOnlyWorkspaceDialog",
      "doGitStatusWorkspaceDialog",
      "doGitCommandWorkspaceDialog",
      "viewGitLogWorkspaceDialog",
    ]);
  });
});

/**
 * The two execution-detail tables. Task C.7.
 *
 * They are asserted together because the interesting claim is about the *pair*:
 * two routes, one screen, and the only differences between the documents are the
 * ones a screen is parameterised by.
 */
describe("the two execution-detail tables, which are one screen behind two routes", () => {
  const status = documents["pipelineExecDetailsTable"]!;
  const stats = documents["cpipesExecDetailsTable"]!;

  it("both filter on session_id out of the route, and on nothing else", () => {
    // The mechanism of both screens. `formStateKey` is what `makeWhereClause`
    // looks up in `routeParams` when there is no form state — which is what a
    // `ScreenOne` table has (`datatable/query.ts`, `firstRouteParam`).
    for (const doc of [status, stats]) {
      if (doc.source !== "query") throw new Error("both are query tables");
      expect(doc.where).toEqual([{ column: "session_id", formStateKey: "session_id" }]);
    }
  });

  it("neither offers an action or a dialog, on either row", () => {
    // What makes the screen a table and nothing else: no action bar, so no
    // `actionDispatch`, no escape registry and no dialog host (I-68).
    for (const doc of [status, stats]) {
      if (doc.source !== "query") throw new Error("both are query tables");
      expect(doc.actions).toBeUndefined();
      expect(doc.secondRowActions).toBeUndefined();
      expect(escapeNamesOf(doc)).toEqual([]);
      expect(actionNamesOf(doc)).toEqual([]);
    }
  });

  it("differ only in the four things a screen is parameterised by", () => {
    // Read the failure of this as "the pair stopped being one screen", which is
    // the claim `TableScreen` rests on.
    if (status.source !== "query" || stats.source !== "query") throw new Error("query");
    expect(status.from).toEqual([{ schema: "jetsapi", table: "pipeline_execution_details" }]);
    expect(stats.from).toEqual([{ schema: "jetsapi", table: "cpipes_execution_status_details" }]);
    expect([status.sortColumn, status.sortAscending]).toEqual(["shard_id", true]);
    expect([stats.sortColumn, stats.sortAscending]).toEqual(["total_input_files_size_mb", undefined]);
    expect([status.columns.length, stats.columns.length]).toEqual([12, 7]);
    expect([status.rowsPerPage, stats.rowsPerPage]).toEqual([10, 10]);
  });

  it("carries the second and last calculatedAs in the corpus, which stays a fragment", () => {
    // **I-105 said C.6 would bring this in and it is C.7's**: the home screen's
    // three tables are `inputLoaderStatusTable`, `inputRegistryTable` and
    // `pipelineExecStatusTable`, and `pipelineExecDetailsTable` is named by
    // `/executionStatusDetails/:session_id` and by nothing else
    // (`screens/fixtures/screen_reachability.json`).
    //
    // Two sites, one expression, and the decision is to leave it authored — see
    // the `calculatedAs` note in `table.ts` for why the server is what settles it.
    const sites = Object.entries(documents).flatMap(([key, doc]) =>
      doc.columns.filter((c) => c.calculatedAs).map((c) => [key, c.name, c.calculatedAs]),
    );
    expect(sites).toEqual([
      ["pipelineExecStatusTable", "run_duration", "AGE(last_update, start_time)"],
      ["pipelineExecDetailsTable", "run_duration", "AGE(last_update, start_time)"],
    ]);
  });

  it("brings back maxLines and columnWidth, which the prediction did not name", () => {
    // The whole of what the 38th and 39th tables cost the schema. `error_message`
    // is a stack trace in a cell, and it is the only column in either document
    // that sets either field.
    const clamped = status.columns.filter((c) => c.maxLines !== undefined || c.columnWidth !== undefined);
    expect(clamped.map((c) => [c.name, c.maxLines, c.columnWidth])).toEqual([
      ["error_message", 3, 600],
    ]);
    expect(stats.columns.some((c) => c.maxLines ?? c.columnWidth)).toBe(false);
  });

  it("is what the non-flow corpus sets that the flow corpus does not — the whole list", () => {
    // **F66.** `table.ts` predicted nine fields track C would restore and the
    // list is short by four; this derives it rather than restating it, so a
    // fifth one cannot hide the way these two did. Fields already in the schema
    // are excluded: what is left is what a screen still has to pay for.
    const inSchema = new Set(["calculatedAs", "secondRowActions", "maxLines", "columnWidth"]);
    const owed = new Set<string>();
    for (const config of Object.values(screenTables)) {
      if (config.withClauses.length > 0) owed.add("withClauses");
      if (config.secondRowActions.length > 0) owed.add("secondRowActions");
      if (config.modelStateFormKey) owed.add("modelStateFormKey");
      if (config.hasModelStateHandler) owed.add("hasModelStateHandler");
      if (config.requestColumnDef) owed.add("requestColumnDef");
      if (config.sortColumnTableName) owed.add("sortColumnTableName");
      if (config.dataRowMinHeight !== undefined) owed.add("dataRowMinHeight");
      if (config.dataRowMaxHeight !== undefined) owed.add("dataRowMaxHeight");
      if (config.apiAction !== "read") owed.add("apiAction");
      for (const column of config.columns) {
        if (column.calculatedAs) owed.add("calculatedAs");
        if (column.maxLines) owed.add("maxLines");
        if (column.columnWidth) owed.add("columnWidth");
      }
      const walk = (where: (typeof config.whereClauses)[number]): void => {
        if (where.lookupColumnInFormState) owed.add("lookupColumnInFormState");
        if (where.like) owed.add("like");
        if (where.ge ?? where.le) owed.add("ge/le");
        if (where.orWith) walk(where.orWith);
      };
      for (const where of config.whereClauses) walk(where);
      for (const action of [...config.actions, ...config.secondRowActions]) {
        if (action.stateGroup !== 0) owed.add("stateGroup");
        if (action.hasActionDelegate) owed.add("actionDelegate");
        if (action.actionEnableCriterias?.length) owed.add("actionEnableCriterias");
      }
    }
    expect([...owed].filter((f) => !inSchema.has(f)).sort()).toEqual([
      "actionEnableCriterias",
      "apiAction",
      "dataRowMaxHeight",
      "dataRowMinHeight",
      "hasModelStateHandler",
      "like",
      "lookupColumnInFormState",
      "modelStateFormKey",
      "requestColumnDef",
      "sortColumnTableName",
      "withClauses",
    ]);
  });
});

describe("queryToolResultSetTable, the /queryTool screen's result table", () => {
  const doc = documents["queryToolResultSetTable"]!;
  const config = configOf("queryToolResultSetTable");

  it("declares no columns, and it is not alone in that", () => {
    if (doc.source !== "query") return;
    expect(doc.columns).toEqual([]);
    expect(doc.sortColumn).toBeUndefined();
    // **Four of the non-flow corpus, not one.** The other three are C.9's, C.11's
    // and C.12's, and they get their columns by the other mechanism — a `read`
    // request that names none is already the request for a `columnDef`
    // (`jets/datatable/data_table_action.go`, `DoReadAction`). Asserted here
    // rather than described in `table.ts`, for the reason C.2's enum count is.
    const columnless = Object.entries(screenTables)
      .filter(([, t]) => t.columns.length === 0)
      .map(([key]) => key)
      .sort();
    expect(columnless).toEqual([
      "inputFileViewerTable",
      "inputRecordsFromProcessErrorTable",
      "inputTable",
      "queryToolResultSetTable",
    ]);
    // The same four, arrived at from the other field. Two independently derived
    // lists agreeing is the check a single measurement cannot give itself.
    const unsorted = Object.entries(screenTables)
      .filter(([, t]) => t.sortColumnName === "")
      .map(([key]) => key)
      .sort();
    expect(unsorted).toEqual(columnless);
    // And none of the 37 flow tables is like either, which is why the schema
    // could require both until now.
    expect(Object.values(tables).filter((t) => t.columns.length === 0)).toEqual([]);
  });

  it("is the only configuration that asks the server to describe the result", () => {
    if (doc.source !== "query") return;
    expect(doc.requestColumnDef).toBe(true);
    expect(doc.apiAction).toBe("raw_query_tool");
    const asking = Object.entries({ ...tables, ...screenTables })
      .filter(([, t]) => t.requestColumnDef)
      .map(([key]) => key);
    expect(asking).toEqual(["queryToolResultSetTable"]);
  });

  it("carries a where clause that names no column, which is a gate", () => {
    if (doc.source !== "query") return;
    expect(doc.where).toEqual([{ formStateKey: "query.ready" }]);
    // One clause of the two corpora, asserted so that a second one is a decision
    // rather than an accident: a column-less clause contributes nothing to a
    // request and only blocks the table (`datatable/binding.ts`).
    const gates = Object.entries({ ...tables, ...screenTables }).flatMap(([key, t]) =>
      t.whereClauses.filter((w) => w.column === "").map(() => key),
    );
    expect(gates).toEqual(["queryToolResultSetTable"]);
  });

  it("round-trips the three sentinels the Dart spells as empty strings", () => {
    // The round trip covers this with the other 39, and it is asserted again on
    // its own because this document is the one where *absence* carries three
    // separate meanings — no columns, no sort column, no filtered column — and a
    // restore that got any of them wrong would still produce a valid document.
    const restored = fromDocument("queryToolResultSetTable", doc);
    expect(restored.columns).toEqual([]);
    expect(restored.sortColumnName).toBe("");
    expect(restored.whereClauses[0]!.column).toBe("");
    expect(restored.requestColumnDef).toBe(true);
    expect(restored).toEqual(config);
  });
});

/**
 * The two authored documents. Task C.3a.
 *
 * **A hand-transcribed document that is wrong is still well formed**, which is
 * C.2b's finding on a dialog whose *Close* button is keyed `dialogCancel` and
 * styled `dialogOk`. These two have no Dart to be wrong about — the `lookups`
 * view was never built — so the risk is not transcription but *invention*: a
 * column that is not a column, a join that does not join. The schema cannot see
 * either, because both are identifiers.
 *
 * So what is asserted here is the pair against **`jets/workspace_schema.sql`**,
 * which is the only thing that knows what `workspace.db` holds. That is the same
 * move the project makes everywhere else — check the claim against the other end
 * rather than against a reading of your own.
 */
describe("the two lookup tables, which are authored rather than translated", () => {
  const columnsOf = (key: string) =>
    authoredDocuments[key]!.columns.map((c) => `${c.table ?? ""}.${c.name}`);

  it("select only columns workspace_schema.sql declares", () => {
    // `lookup_tables` and `lookup_columns` from `jets/workspace_schema.sql`, and
    // `workspace_control.source_file_name`, which every other compiled view joins
    // to the same way.
    const schema: Record<string, string[]> = {
      lookup_tables: [
        "key", "name", "table_name", "csv_file", "lookup_key", "lookup_resources", "source_file_key",
      ],
      lookup_columns: ["lookup_table_key", "name", "type", "as_array"],
      workspace_control: ["key", "source_file_name", "is_main"],
    };
    for (const key of AUTHORED_KEYS) {
      for (const qualified of columnsOf(key)) {
        const [table, column] = qualified.split(".");
        expect(schema[table!], `${key}: ${qualified}`).toBeDefined();
        expect(schema[table!], `${key}: ${qualified}`).toContain(column);
      }
    }
  });

  it("join on the foreign keys that schema declares, and qualify every column", () => {
    const doc = authoredDocuments["wsLookupTableTable"]!;
    expect(doc.source).toBe("query");
    if (doc.source !== "query") return;
    expect(doc.where).toEqual([{ column: "source_file_key", joinWith: "workspace_control.key" }]);

    const columns = authoredDocuments["wsLookupColumnTable"]!;
    expect(columns.source).toBe("query");
    if (columns.source !== "query") return;
    expect(columns.where).toEqual([
      { column: "lookup_table_key", table: "lookup_columns", joinWith: "lookup_tables.key" },
    ]);
    // Every column names its table. Both documents have a two-table `FROM` and
    // `wsLookupColumnTable` selects two columns called `name`, so an unqualified
    // one is ambiguous to the server rather than merely untidy.
    for (const key of AUTHORED_KEYS) {
      for (const c of authoredDocuments[key]!.columns) expect(c.table, key).toBeTruthy();
    }
  });

  it("read the compiled workspace, like the six they sit beside", () => {
    for (const key of AUTHORED_KEYS) {
      const doc = authoredDocuments[key]!;
      expect(doc.source).toBe("query");
      if (doc.source !== "query") return;
      expect(doc.apiAction).toBe("workspace_read");
      for (const from of doc.from) expect(from.schema).toBe("$SCHEMA");
      // The sort has to name its table for the same reason the columns do.
      expect(doc.sortColumnTable).toBeTruthy();
    }
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

  it("a static table with no columns", () => {
    // **This case read "a table with no columns" and used a query table until
    // C.4**, which is the whole of what changed: a query table may declare none
    // and the server describes the result, and a static table's rows are compiled
    // in with nothing to describe them.
    rejects({ ...staticBase(), columns: [] });
  });

  it("a static table with no sortColumn", () => {
    const doc = staticBase();
    delete doc["sortColumn"];
    rejects(doc);
  });

  it("a requestColumnDef of false, rather than absent", () => {
    // `z.literal(true)`: the field is a request, and "do not request" is said by
    // omission. Two spellings for one meaning is I-14 again, and it is the same
    // shape `apiAction`'s absent `read` takes one field over.
    rejects({ ...base(), requestColumnDef: false });
  });

  it("a requestColumnDef on a static table, which sends nothing", () => {
    rejects({ ...staticBase(), requestColumnDef: true });
  });

  it("an unknown action type", () => {
    // The enum is the point: `actionType` in the Dart is a bare string, and the
    // seven here are the seven the corpus uses.
    rejects({ ...base(), actions: [{ key: "k", label: "L", action: "exec_ddl", style: "primary" }] });
  });

  it("an apiAction outside the three-member enum", () => {
    // **The security cut, narrowed rather than relaxed — C.2.** This case read
    // "an apiAction, which this document deliberately has no field for" until the
    // first screen needed `workspace_read`. The property it protects is unchanged:
    // `apiAction` reaches `DataTableAction.Action` and dispatches over the whole
    // switch in `jets/apiserver/api_tables.go` (`DoDataTableAction`), so what must
    // stay impossible is an authored table naming `exec_ddl`. A closed enum of
    // three is a different object from a free string, and
    // `additionalProperties: false` no longer carries this constraint on its own.
    rejects({ ...base(), apiAction: "exec_ddl" });
    rejects({ ...base(), apiAction: "insert_rows" });
    // Not a member either: `read` is said by omission, and two spellings for one
    // meaning is what the three members exist to avoid.
    rejects({ ...base(), apiAction: "read" });
  });

  it("an apiAction on a static table, which sends nothing", () => {
    rejects({ ...staticBase(), apiAction: "workspace_read" });
  });

  const gated = (enableWhen: unknown) => ({
    ...base(),
    actions: [{ key: "k", label: "L", action: "doAction", style: "primary", enableWhen }],
  });

  it("an enableWhen naming an unknown comparison", () => {
    rejects(gated([[{ column: "status", is: "startsWith", value: "x" }]]));
  });

  it("an enableWhen with an empty conjunction, or none at all", () => {
    // An empty inner list is vacuously true and would *enable* the button, which
    // is the opposite of what writing a gate means; an empty outer list disables
    // it forever. Neither is a thing a document should be able to say by
    // accident, and `.min(1)` on both is why it cannot.
    rejects(gated([[]]));
    rejects(gated([]));
  });

  it("an enableWhen criterion with no value", () => {
    // Nullable in the Dart, where it means "a gate that never opens" on
    // `contains` and "test for an empty cell" on `equals`. No configuration wants
    // either, and both deserve their own spelling if one ever does.
    rejects(gated([[{ column: "status", is: "contains", value: "" }]]));
    rejects(gated([[{ column: "status", is: "contains" }]]));
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

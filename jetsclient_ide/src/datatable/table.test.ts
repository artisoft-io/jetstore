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
import {
  TableConfigDocumentSchema,
  actionNamesOf,
  emitJsonSchema,
  escapeNamesOf,
  tablePath,
  type TableConfigDocument,
} from "./table";
import { fromDocument, toDocuments } from "./tableTranslate";
import type { TableConfig } from "./types";

const artifactPath = fileURLToPath(new URL("./table.schema.json", import.meta.url));
const tablesDir = fileURLToPath(new URL("./tables/", import.meta.url));

const tables = (corpus as { tables: Record<string, unknown> }).tables as Record<string, TableConfig>;
const documents = toDocuments(tables);
const documentOf = (key: string) => `${JSON.stringify(documents[key], null, 2)}\n`;

describe("the emitted JSON Schema", () => {
  it("matches the committed artifact", () => {
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") writeFileSync(artifactPath, emitted);
    expect(readFileSync(artifactPath, "utf8")).toBe(emitted);
  });

  it("has the 37 translated configurations committed beside it, for the Go check", () => {
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
  it("all translate", () => {
    expect(Object.keys(documents).length).toBe(37);
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
    for (const [key, config] of Object.entries(tables)) {
      expect({ [key]: fromDocument(key, documents[key]!) }).toEqual({ [key]: config });
    }
  });

  it("split nine static and 28 query, as the corpus does", () => {
    // Derived from the documents; checked against a count taken from the corpus
    // a different way — `apiPath` being empty is how the Dart says "static", and
    // the discriminant is this schema's invention.
    const byKind = Object.values(documents).reduce<Record<string, number>>(
      (acc, d) => ({ ...acc, [d.source]: (acc[d.source] ?? 0) + 1 }),
      {},
    );
    const fromCorpus = Object.values(tables).filter((t) => t.apiPath === "").length;
    expect(byKind).toEqual({ static: 9, query: 28 });
    expect(fromCorpus).toBe(9);
  });

  it("carry the corpus's 275 columns, 25 actions and 49 where clauses", () => {
    const count = (f: (d: TableConfigDocument) => number) =>
      Object.values(documents).reduce((n, d) => n + f(d), 0);
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
    const names = new Set(Object.values(documents).flatMap((d) => escapeNamesOf(d)));
    expect([...names].sort()).toEqual(["fileKeyLabel", "hasDataRegistryFilters"]);
  });

  it("name eleven action-document entries", () => {
    const names = new Set(Object.values(documents).flatMap((d) => actionNamesOf(d)));
    expect(names.size).toBe(11);
  });

  it("put every table configuration under table_configs/", () => {
    expect(tablePath("lfSourceConfigTable")).toBe("table_configs/lfSourceConfigTable.tc.json");
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

/**
 * Tests for the `/dataTable` query builder (task A.4a).
 *
 * Three layers, and the third is the one that makes this a port rather than a
 * rewrite:
 *
 *  1. **Branch tests** over `makeWhereClause` and `makeQuery`, one per arm of the
 *     Dart original, including the arms no user flow exercises.
 *  2. **Corpus tests** over all 37 real table configurations, generated from the
 *     running Flutter app by `jetsclient/test/table_config_corpus_test.dart`.
 *     They assert properties that must hold for every table rather than one
 *     payload each, which is what catches a branch that only ever runs on
 *     configurations nobody wrote a fixture for.
 *  3. **Goldens** for `load_files`, the flow Phase 2 proves the widget on, with
 *     the payload written out in full.
 *
 * **What the goldens are, honestly.** They are the payload read out of the Dart
 * by hand, not captured from a running Dart process. `_makeQuery` is private to
 * a Flutter `FormFieldState` and cannot be driven headlessly, so there is no
 * mechanical capture available; what *is* mechanical is the input — the table
 * configurations themselves come from the app rather than from transcription,
 * which removes the larger of the two error sources. Recorded in the tracking
 * documents rather than left as a footnote here.
 */

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { makeQuery, makeWhereClause } from "./query";
import type {
  FormStateReader,
  FormStateValue,
  QueryContext,
  TableConfig,
  WhereClause,
} from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;

function formState(
  values: Record<string, FormStateValue>,
  isDialog = false,
): FormStateReader {
  return {
    isDialog,
    getValue: (_group: number, key: string) => values[key],
  };
}

/** A context with everything off, so each test turns on only what it is about. */
function context(config: TableConfig, over: Partial<QueryContext> = {}): QueryContext {
  return {
    config,
    indexOffset: 0,
    rowsPerPage: config.rowsPerPage,
    sortColumnName: config.sortColumnName,
    sortColumnTableName: config.sortColumnTableName,
    sortAscending: config.sortAscending,
    ...over,
  };
}

function whereClause(over: Partial<WhereClause> = {}): WhereClause {
  return {
    column: "col",
    defaultValue: [],
    lookupColumnInFormState: false,
    ...over,
  };
}

const minimalConfig: TableConfig = {
  ...tables["lfSourceConfigTable"]!,
  whereClauses: [],
};

describe("makeWhereClause", () => {
  const state = () => ({ addWhereClauseOnClient: true });

  it("takes a static default value when there is no form", () => {
    const wc = whereClause({ defaultValue: ["file"] });
    expect(makeWhereClause(wc, context(minimalConfig), state())).toEqual({
      table: "",
      column: "col",
      values: ["file"],
    });
  });

  it("emits a joinWith clause when there is no default", () => {
    const wc = whereClause({ joinWith: "source_period.key" });
    expect(makeWhereClause(wc, context(minimalConfig), state())).toEqual({
      table: "",
      column: "col",
      joinWith: "source_period.key",
    });
  });

  it("prefers the default value over joinWith, as the Dart does", () => {
    const wc = whereClause({ defaultValue: ["a"], joinWith: "t.k" });
    expect(makeWhereClause(wc, context(minimalConfig), state())).toEqual({
      table: "",
      column: "col",
      values: ["a"],
    });
  });

  it("reads a single value out of form state", () => {
    const wc = whereClause({ formStateKey: "table_name" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ table_name: "staging" }),
    });
    expect(makeWhereClause(wc, ctx, state())).toEqual({
      table: "",
      column: "col",
      values: ["staging"],
    });
  });

  it("reads a list out of form state without wrapping it", () => {
    const wc = whereClause({ formStateKey: "keys" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ keys: ["a", "b"] }),
    });
    expect(makeWhereClause(wc, ctx, state())).toEqual({
      table: "",
      column: "col",
      values: ["a", "b"],
    });
  });

  it("drops the clause when the form-state list is empty", () => {
    // Load-bearing: a cleared multi-selection widens the query rather than
    // producing `IN ()`, which would return nothing.
    const wc = whereClause({ formStateKey: "keys" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ keys: [] }),
    });
    expect(makeWhereClause(wc, ctx, state())).toBeNull();
  });

  it("falls back to the default value when form state holds nothing", () => {
    const wc = whereClause({ formStateKey: "missing", defaultValue: ["d"] });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({}, true),
    });
    expect(makeWhereClause(wc, ctx, state())).toEqual({
      table: "",
      column: "col",
      values: ["d"],
    });
  });

  it("falls back to a route parameter when the form is not a dialog", () => {
    const wc = whereClause({ formStateKey: "session_id", table: "t" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({}),
      routeParams: { session_id: "s1" },
    });
    expect(makeWhereClause(wc, ctx, state())).toEqual({
      table: "t",
      column: "col",
      values: ["s1"],
    });
  });

  it("does not fall back to a route parameter inside a dialog", () => {
    // A dialog inherits its parent's form state, so the route belongs to the
    // parent screen and reading it here would filter by the wrong thing.
    const wc = whereClause({ formStateKey: "session_id" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({}, true),
      routeParams: { session_id: "s1" },
    });
    expect(makeWhereClause(wc, ctx, state())).toBeNull();
  });

  it("drops the clause when a predicate is not satisfied", () => {
    const wc = whereClause({
      defaultValue: ["x"],
      predicate: { formStateKey: "mode", expectedValue: "edit" },
    });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ mode: "view" }),
    });
    expect(makeWhereClause(wc, ctx, state())).toBeNull();
  });

  it("keeps the clause when the predicate is satisfied", () => {
    const wc = whereClause({
      defaultValue: ["x"],
      predicate: { formStateKey: "mode", expectedValue: "edit" },
    });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ mode: "edit" }),
    });
    expect(makeWhereClause(wc, ctx, state())).toEqual({
      table: "",
      column: "col",
      values: ["x"],
    });
  });

  it.each([
    ["like", { like: "%abc%" }],
    ["ge", { ge: "2026-01-01" }],
    ["le", { le: "2026-12-31" }],
  ])("emits a %s clause", (_name, extra) => {
    const wc = whereClause({ ...extra, defaultValue: ["ignored"] });
    expect(makeWhereClause(wc, context(minimalConfig), state())).toEqual({
      table: "",
      column: "col",
      ...extra,
    });
  });

  it("clears the implicit client filter when a clause names the client column", () => {
    const s = state();
    makeWhereClause(whereClause({ column: "client", defaultValue: ["c"] }), context(minimalConfig), s);
    expect(s.addWhereClauseOnClient).toBe(false);
  });

  it("clears the implicit client filter from a predicate on client", () => {
    const s = state();
    const wc = whereClause({
      defaultValue: ["x"],
      predicate: { formStateKey: "client", expectedValue: "c" },
    });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ client: "c" }),
    });
    makeWhereClause(wc, ctx, s);
    expect(s.addWhereClauseOnClient).toBe(false);
  });

  it("throws when lookupColumnInFormState finds no string", () => {
    // The Dart asserts, and asserts are stripped in release builds, so this is a
    // deliberate divergence: a wrong column name would otherwise reach the server.
    const wc = whereClause({ lookupColumnInFormState: true, column: "col_key" });
    const ctx = context(minimalConfig, {
      formField: { group: 0, key: "t" },
      formState: formState({ col_key: ["not", "a", "string"] }),
    });
    expect(() => makeWhereClause(wc, ctx, state())).toThrow(/not a string/);
  });
});

describe("makeQuery", () => {
  it("emits the columns, sort and paging the config and state declare", () => {
    const config = tables["lfSourceConfigTable"]!;
    const msg = makeQuery(context(config, { indexOffset: 40, rowsPerPage: 20 }));

    expect(msg.action).toBe("read");
    expect(msg.offset).toBe(40);
    expect(msg.limit).toBe(20);
    expect(msg.sortColumn).toBe("last_update");
    expect(msg.sortAscending).toBe(false);
    // Hidden columns are still selected — hiding is a rendering concern.
    expect(msg.columns).toHaveLength(config.columns.length);
    expect(msg.columns[0]).toEqual({ table: "", column: "key", calculatedAs: "" });
  });

  it("adds the implicit client filter when a client is selected and selected", () => {
    const config = tables["lfFileKeyStagingTable"]!;
    const msg = makeQuery(context(config, { selectedClient: "acme" }));
    expect(msg.whereClauses).toContainEqual({
      table: "input_registry",
      column: "client",
      values: ["acme"],
    });
  });

  it("omits the implicit client filter when no client is selected", () => {
    const config = tables["lfFileKeyStagingTable"]!;
    const msg = makeQuery(context(config));
    expect(msg.whereClauses ?? []).not.toContainEqual(
      expect.objectContaining({ column: "client", values: expect.anything() }),
    );
  });

  it("excludes the 'Any' client on the input registry table specifically", () => {
    const config = tables["inputRegistryTable"]!;
    const msg = makeQuery(
      context(config, { formField: { group: 0, key: "inputRegistryTable" } }),
    );
    expect(msg.whereClauses).toContainEqual(
      expect.objectContaining({ column: "client", not_in_values: ["Any"] }),
    );
  });

  it("applies home filters only to the pipeline execution status table", () => {
    const homeFilters = [whereClause({ column: "session_id", defaultValue: ["s1"] })];
    const config = tables["lfSourceConfigTable"]!;

    const applied = makeQuery(
      context(config, {
        homeFilters,
        formField: { group: 0, key: "pipelineExecStatusTable" },
      }),
    );
    expect(applied.whereClauses).toContainEqual({
      table: "",
      column: "session_id",
      values: ["s1"],
    });

    const notApplied = makeQuery(
      context(config, { homeFilters, formField: { group: 0, key: "somethingElse" } }),
    );
    expect(notApplied.whereClauses ?? []).not.toContainEqual(
      expect.objectContaining({ column: "session_id" }),
    );
  });

  it.each([
    "inputRegistryTable",
    "main_input_registry_key",
    "merged_input_registry_keys",
  ])("applies data registry filters to %s", (key) => {
    const config = tables["lfSourceConfigTable"]!;
    const msg = makeQuery(
      context(config, {
        dataRegistryFilters: [whereClause({ column: "year", defaultValue: ["2026"] })],
        formField: { group: 0, key },
      }),
    );
    expect(msg.whereClauses).toContainEqual({
      table: "",
      column: "year",
      values: ["2026"],
    });
  });

  it("resolves an empty from-clause table name from form state", () => {
    const config: TableConfig = {
      ...tables["lfSourceConfigTable"]!,
      fromClauses: [{ schemaName: "public", tableName: "", asTableName: "" }],
    };
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "t" },
        formState: formState({ table_name: "resolved" }),
      }),
    );
    expect(msg.fromClauses).toEqual([{ schema: "public", table: "resolved" }]);
  });

  it("falls back to the route for an empty from-clause table name", () => {
    const config: TableConfig = {
      ...tables["lfSourceConfigTable"]!,
      fromClauses: [{ schemaName: "public", tableName: "", asTableName: "alias" }],
    };
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "t" },
        formState: formState({}),
        routeParams: { table_name: "from_route" },
      }),
    );
    expect(msg.fromClauses).toEqual([
      { schema: "public", table: "from_route", asTable: "alias" },
    ]);
  });

  it("substitutes state variables into a WITH clause and unquotes NULL", () => {
    // No user flow uses withClauses; this covers the branch rather than a flow.
    const config: TableConfig = {
      ...tables["lfSourceConfigTable"]!,
      withClauses: [
        {
          withName: "w",
          asStatement: "select * from t where a = '{a}' and b = '{b}'",
          stateVariables: ["a", "b"],
        },
      ],
    };
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "t" },
        formState: formState({ a: "x" }),
      }),
    );
    expect(msg.withClauses).toEqual([
      { name: "w", stmt: "select * from t where a = 'x' and b = NULL" },
    ]);
  });

  it("carries the workspace name from form state, then from the route", () => {
    const config = tables["lfSourceConfigTable"]!;
    expect(
      makeQuery(
        context(config, {
          formField: { group: 0, key: "t" },
          formState: formState({ workspace_name: "ws1" }),
        }),
      ).workspaceName,
    ).toBe("ws1");

    expect(
      makeQuery(
        context(config, {
          formField: { group: 0, key: "t" },
          formState: formState({}),
          routeParams: { workspace_name: ["ws2"] },
        }),
      ).workspaceName,
    ).toBe("ws2");

    expect(makeQuery(context(config)).workspaceName).toBeUndefined();
  });

  it("attaches an orWith clause to its parent", () => {
    const config: TableConfig = {
      ...tables["lfSourceConfigTable"]!,
      whereClauses: [
        whereClause({
          column: "a",
          defaultValue: ["1"],
          orWith: whereClause({ column: "b", defaultValue: ["2"] }),
        }),
      ],
    };
    const msg = makeQuery(context(config));
    expect(msg.whereClauses).toEqual([
      {
        table: "",
        column: "a",
        values: ["1"],
        orWith: { table: "", column: "b", values: ["2"] },
      },
    ]);
  });
});

describe("the whole corpus", () => {
  const keys = Object.keys(tables);
  const querying = keys.filter((k) => tables[k]!.apiPath === "/dataTable");

  it("is the 37 user flow tables, 28 of which query", () => {
    expect(keys).toHaveLength(37);
    expect(querying).toHaveLength(28);
  });

  it.each(querying)("builds a well-formed payload for %s", (key) => {
    const config = tables[key]!;
    const msg = makeQuery(
      context(config, { formField: { group: 0, key }, formState: formState({}) }),
    );

    // Every field the Go struct requires is present and of the right shape.
    expect(msg.action).toBe("read");
    expect(Array.isArray(msg.fromClauses)).toBe(true);
    expect(msg.fromClauses.length).toBe(config.fromClauses.length);
    expect(Array.isArray(msg.withClauses)).toBe(true);
    expect(msg.columns).toHaveLength(config.columns.length);
    expect(typeof msg.offset).toBe("number");
    expect(typeof msg.limit).toBe("number");
    expect(typeof msg.sortAscending).toBe("boolean");
    // No clause may be emitted with an undefined column: that is the shape that
    // reaches the server as a quoted empty identifier and fails at the database.
    for (const wc of msg.whereClauses ?? []) {
      expect(typeof wc.column).toBe("string");
      expect(wc.column.length).toBeGreaterThan(0);
    }
  });

  it("emits no where clauses for a table whose filters all read empty state", () => {
    // With nothing in form state and no route, a table filtered entirely on form
    // state contributes nothing — the *reason* A.4c has to block the request
    // rather than send it, since this payload would return every row.
    const config = tables["lfFileKeyStagingTable"]!;
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "lfFileKeyStagingTable" },
        formState: formState({}),
      }),
    );
    expect(msg.whereClauses).toEqual([
      // The join and the static default survive; `table_name` does not.
      { table: "", column: "source_period_key", joinWith: "source_period.key" },
      { table: "", column: "source_type", values: ["file"] },
    ]);
  });
});

describe("load_files goldens", () => {
  it("table 1 — the source config selector", () => {
    const config = tables["lfSourceConfigTable"]!;
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "lfSourceConfigTable" },
        formState: formState({}),
        selectedClient: "acme",
      }),
    );

    expect(msg).toEqual({
      action: "read",
      withClauses: [],
      fromClauses: [{ schema: "jetsapi", table: "source_config" }],
      whereClauses: [
        { table: "source_config", column: "client", values: ["acme"] },
      ],
      offset: 0,
      limit: 20,
      columns: [
        { table: "", column: "key", calculatedAs: "" },
        { table: "", column: "client", calculatedAs: "" },
        { table: "", column: "org", calculatedAs: "" },
        { table: "", column: "object_type", calculatedAs: "" },
        { table: "", column: "table_name", calculatedAs: "" },
        { table: "", column: "input_format", calculatedAs: "" },
        { table: "", column: "last_update", calculatedAs: "" },
      ],
      sortColumn: "last_update",
      sortColumnTable: "",
      sortAscending: false,
    });
  });

  it("table 2 — the file selector, gated by table 1's selection", () => {
    const config = tables["lfFileKeyStagingTable"]!;
    const msg = makeQuery(
      context(config, {
        formField: { group: 0, key: "lfFileKeyStagingTable" },
        // What table 1 publishes on selection: its `table_name` column.
        formState: formState({ table_name: "staging_claims" }),
        selectedClient: "acme",
        indexOffset: 20,
      }),
    );

    expect(msg.fromClauses).toEqual([
      { schema: "jetsapi", table: "input_registry" },
      { schema: "jetsapi", table: "source_period" },
    ]);
    expect(msg.whereClauses).toEqual([
      { table: "", column: "table_name", values: ["staging_claims"] },
      { table: "", column: "source_period_key", joinWith: "source_period.key" },
      { table: "", column: "source_type", values: ["file"] },
      { table: "input_registry", column: "client", values: ["acme"] },
    ]);
    expect(msg.offset).toBe(20);
    expect(msg.limit).toBe(20);
    expect(msg.columns).toHaveLength(13);
    expect(msg.sortColumn).toBe("last_update");
  });
});

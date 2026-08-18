/**
 * Builds the `/dataTable` request body from a table configuration and the state
 * around it. Task A.4a of the UI port's Phase 2.
 *
 * **This is a port, not a redesign.** It reproduces
 * `jetsclient/lib/components/data_table_source.dart` — `_addOrWith` (`:294`),
 * `_makeWhereClause` (`:301`) and `_makeQuery` (`:428`) — branch for branch,
 * including the parts that look accidental, because the Go server on the other
 * end (`jets/datatable/data_table_action.go:61`) has been reading exactly this
 * shape for years and a "tidier" payload is a different payload. Where the Dart
 * does something surprising, the comment says so rather than fixing it.
 *
 * It is deliberately free of React, the DOM and any transport: the whole point
 * of splitting A.4a out of the widget is that this half has a checkable contract
 * and can be tested without either.
 */

import type {
  DataTableAction,
  QueryContext,
  WhereClause,
  WhereClausePayload,
} from "./types";

/**
 * Form-state and table keys the builder special-cases.
 *
 * Every one of these is a hard-coded key inside otherwise generic code, and that
 * is the assessment's §3.2 defect rather than a design: three of the four make
 * the builder behave differently for one named table. They are gathered here so
 * the port carries them visibly, and so Phase 3 can move them into the flow
 * configuration where they belong. Values from `jetsclient/lib/utils/constants.dart`.
 */
export const KEYS = {
  client: "client", // FSK.client
  tableName: "table_name", // FSK.tableName
  workspaceName: "workspace_name", // FSK.wsName
  pipelineExecStatusTable: "pipelineExecStatusTable", // DTKeys
  inputRegistryTable: "inputRegistryTable", // DTKeys
  mainInputRegistryKey: "main_input_registry_key", // FSK
  mergedInputRegistryKeys: "merged_input_registry_keys", // FSK
} as const;

/** Mutable bookkeeping that `_makeQuery` and `_makeWhereClause` share. */
interface BuildState {
  addWhereClauseOnClient: boolean;
}

function firstRouteParam(
  ctx: QueryContext,
  key: string,
): string | undefined {
  const value = ctx.routeParams?.[key];
  if (value === undefined || value === null) return undefined;
  return Array.isArray(value) ? value[0] : value;
}

/**
 * `_makeWhereClause` (`data_table_source.dart:301`).
 *
 * Returns `null` when the clause contributes nothing — an unsatisfied predicate,
 * or a form-state key with no value and no default. A null here is how a table
 * ends up querying without a filter it nominally has; the *blocking* behaviour
 * that stops the query outright lives in A.4c, not in this function.
 */
export function makeWhereClause(
  wc: WhereClause,
  ctx: QueryContext,
  state: BuildState,
): WhereClausePayload | null {
  const field = ctx.formField;
  const table = wc.table ?? "";

  // The column name may itself come from form state.
  let columnName = wc.column;
  if (wc.lookupColumnInFormState && field) {
    const v = ctx.formState?.getValue(field.group, wc.column);
    // The Dart asserts `v is String`. Asserts are stripped in release builds, so
    // a non-string there produces a malformed column name rather than an error;
    // this throws instead, because a silently wrong column name is worse than a
    // failed request. No user flow exercises this path (0 of 51 clauses).
    if (typeof v !== "string") {
      throw new Error(
        `lookupColumnInFormState: form state key "${wc.column}" is not a string`,
      );
    }
    columnName = v;
  }

  // A clause on `client` replaces the implicit client filter added at the end.
  if (wc.column === KEYS.client) {
    state.addWhereClauseOnClient = false;
  }

  // Value from a navigation parameter — only when there is no form state, or the
  // form is not a dialog and holds nothing under the key. Dialogs are excluded
  // because they inherit a parent form state and the route is the parent's.
  const configGroup = field ? field.group : 0;
  if (wc.formStateKey != null) {
    const fs = ctx.formState;
    if (!fs || (!fs.isDialog && fs.getValue(configGroup, wc.formStateKey) == null)) {
      const value = firstRouteParam(ctx, wc.formStateKey);
      if (value != null) {
        return { table, column: columnName, values: [value] };
      }
    }
  }

  // A predicate gates the whole clause: unsatisfied, the clause disappears.
  if (field && wc.predicate) {
    const value = ctx.formState?.getValue(field.group, wc.predicate.formStateKey);
    if (wc.predicate.formStateKey === KEYS.client) {
      state.addWhereClauseOnClient = false;
    }
    // Deliberately `!==` against a possibly-undefined expectedValue, matching
    // Dart's `!=` on a nullable String.
    if ((value ?? null) !== (wc.predicate.expectedValue ?? null)) {
      return null;
    }
  }

  if (wc.like != null) return { table, column: columnName, like: wc.like };
  if (wc.ge != null) return { table, column: columnName, ge: wc.ge };
  if (wc.le != null) return { table, column: columnName, le: wc.le };

  if (!field || wc.formStateKey == null) {
    // No form to read from: the clause is whatever it declares statically.
    if (wc.defaultValue.length > 0) {
      return { table, column: columnName, values: wc.defaultValue };
    }
    if (wc.joinWith != null) {
      return { table, column: columnName, joinWith: wc.joinWith };
    }
  } else {
    const values = ctx.formState?.getValue(field.group, wc.formStateKey);
    if (values != null) {
      if (typeof values === "string") {
        return { table, column: columnName, values: [values] };
      }
      if (!Array.isArray(values)) {
        throw new Error(
          `form state key "${wc.formStateKey}" holds neither a string nor a list`,
        );
      }
      // An empty list falls through to null: the clause is dropped rather than
      // becoming `IN ()`. This is what makes a cleared selection widen the query
      // instead of emptying it, and it is load-bearing.
      if (values.length > 0) {
        return { table, column: columnName, values };
      }
    } else if (wc.defaultValue.length > 0) {
      return { table, column: columnName, values: wc.defaultValue };
    }
  }

  return null;
}

/** `_addOrWith` (`data_table_source.dart:294`). */
function addOrWith(
  wc: WhereClause,
  data: WhereClausePayload | null,
  ctx: QueryContext,
  state: BuildState,
): WhereClausePayload | null {
  if (wc.orWith && data) {
    data.orWith = makeWhereClause(wc.orWith, ctx, state);
  }
  return data;
}

/**
 * `_makeQuery` (`data_table_source.dart:428`).
 *
 * Key insertion order follows the Dart, so a serialised payload compares equal
 * as text and not merely as a structure.
 */
export function makeQuery(ctx: QueryContext): DataTableAction {
  const config = ctx.config;
  const field = ctx.formField;
  const columns = config.columns;
  const state: BuildState = { addWhereClauseOnClient: true };

  if (ctx.selectedClient == null) {
    state.addWhereClauseOnClient = false;
  }

  let hasClientColumn = false;
  const selectColumns = columns.map((c) => ({
    table: c.table ?? "",
    column: c.name,
    calculatedAs: c.calculatedAs ?? "",
  }));
  for (const col of columns) {
    if (col.name === KEYS.client) hasClientColumn = true;
  }
  if (!hasClientColumn) {
    state.addWhereClauseOnClient = false;
  }

  const msg: DataTableAction = {
    action: config.apiAction,
    withClauses: [],
    fromClauses: [],
    offset: 0,
    limit: 0,
    columns: [],
    sortColumn: "",
    sortColumnTable: "",
    sortAscending: false,
  };

  // WITH clauses, with `{stateVariable}` substitution.
  msg.withClauses = config.withClauses.map((wc) => {
    let stmt = wc.asStatement;
    for (const k of wc.stateVariables) {
      const v = field ? ctx.formState?.getValue(field.group, k) : undefined;
      // The Dart uses RegExp('{$k}') and interpolates the value with `?? 'NULL'`,
      // then strips the quotes off 'NULL' — so an absent variable becomes a bare
      // SQL NULL. Reproduced, quirk included.
      stmt = stmt.split(`{${k}}`).join(typeof v === "string" ? v : "NULL");
    }
    stmt = stmt.split("'NULL'").join("NULL");
    return { name: wc.withName, stmt };
  });

  // FROM clauses. An empty tableName is resolved from form state or the route.
  msg.fromClauses = config.fromClauses.map((fc) => {
    let table = fc.tableName;
    if (table === "") {
      const fromState = field
        ? ctx.formState?.getValue(field.group, KEYS.tableName)
        : undefined;
      const v =
        typeof fromState === "string"
          ? fromState
          : firstRouteParam(ctx, KEYS.tableName);
      if (v != null) {
        table = v;
      }
      // The Dart prints "Error: Don't have a table_name!" and carries on with an
      // empty table, producing a query the server rejects. Left as-is: turning it
      // into a throw is a behaviour change, and A.4b is where the user-visible
      // error belongs.
    }
    return fc.asTableName !== ""
      ? { schema: fc.schemaName, table, asTable: fc.asTableName }
      : { schema: fc.schemaName, table };
  });

  if (config.distinctOnClauses.length > 0) {
    msg.distinctOnClauses = config.distinctOnClauses;
  }

  const whereClauses: WhereClausePayload[] = [];
  for (const wc of config.whereClauses) {
    const value = addOrWith(wc, makeWhereClause(wc, ctx, state), ctx, state);
    if (value != null) whereClauses.push(value);
  }

  // Two blocks of filters that apply to named tables only. See KEYS above: this
  // is generic code that knows four table keys by name.
  if (field && field.key === KEYS.pipelineExecStatusTable && ctx.homeFilters) {
    for (const wc of ctx.homeFilters) {
      const value = addOrWith(wc, makeWhereClause(wc, ctx, state), ctx, state);
      if (value != null) whereClauses.push(value);
    }
  }
  if (field && ctx.dataRegistryFilters) {
    if (
      field.key === KEYS.inputRegistryTable ||
      field.key === KEYS.mainInputRegistryKey ||
      field.key === KEYS.mergedInputRegistryKeys
    ) {
      for (const wc of ctx.dataRegistryFilters) {
        const value = addOrWith(wc, makeWhereClause(wc, ctx, state), ctx, state);
        if (value != null) whereClauses.push(value);
      }
    }
  }

  // The implicit client filter, added last so an explicit clause on `client`
  // (which cleared the flag above) wins.
  //
  // The Dart indexes `fromClauses[0]` unguarded and would throw on a table with
  // none. All 28 querying tables in the corpus declare at least one — the nine
  // that declare none are the static ones, which never reach this function — so
  // the fallback below is unreachable rather than a behaviour change.
  const firstFromTable = config.fromClauses[0]?.tableName ?? "";
  if (state.addWhereClauseOnClient) {
    whereClauses.push({
      table: firstFromTable,
      column: KEYS.client,
      values: [ctx.selectedClient as string],
    });
  } else if (hasClientColumn) {
    if (field && field.key === KEYS.inputRegistryTable) {
      whereClauses.push({
        table: firstFromTable,
        column: KEYS.client,
        not_in_values: ["Any"],
      });
    }
  }

  if (whereClauses.length > 0) {
    msg.whereClauses = whereClauses;
  }

  msg.offset = ctx.indexOffset;
  msg.limit = ctx.rowsPerPage;

  if (columns.length > 0) {
    msg.columns = selectColumns;
    msg.sortColumn = ctx.sortColumnName;
    msg.sortColumnTable = ctx.sortColumnTableName;
  } else if (ctx.columnNameMaps && ctx.columnNameMaps.length > 0) {
    msg.columns = ctx.columnNameMaps;
    msg.sortColumn = ctx.sortColumnName;
    msg.sortColumnTable = ctx.sortColumnTableName;
  } else {
    msg.columns = [];
    msg.sortColumn = "";
    msg.sortColumnTable = "";
  }

  msg.sortAscending = ctx.sortAscending;

  const fromState = field
    ? ctx.formState?.getValue(field.group, KEYS.workspaceName)
    : undefined;
  const workspaceName =
    fromState != null ? fromState : ctx.routeParams?.[KEYS.workspaceName];
  if (workspaceName != null) {
    msg.workspaceName = Array.isArray(workspaceName)
      ? (workspaceName[0] as string)
      : workspaceName;
  }

  return msg;
}

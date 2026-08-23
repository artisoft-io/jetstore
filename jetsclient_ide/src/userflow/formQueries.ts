/**
 * A form's named queries: substitution, readiness, and one request. Task I.2b.
 *
 * This is the half of I-11 that is not a widget. `Dropdown.tsx` was written in
 * A.3 to take `items` as a prop "so a static list and a query result reach it the
 * same way", and I.2a authored the static list; what was still missing was the
 * thing that turns a declaration into a result. Both of `fmMappingFormUF`'s exotic
 * fields want it and so do five of the eleven dropdowns (F21, I-60).
 *
 * ## One request, not one per field
 *
 * `raw_query_map` takes a map of name to statement and answers with a map of name
 * to rows (`jets/datatable/data_table_action.go`, `ExecRawQueryMap`), which is
 * what `components/form.dart` uses and what the field-level path in
 * `components/dropdown_form_field.dart` does *not* — that one issues a `raw_query`
 * per widget. Collapsing to the map shape is the port's choice and it is the
 * cheaper one: `fmMappingFormUF` declares three queries and would otherwise be
 * three round trips before the first row can be drawn.
 *
 * ## Substitution, and the one place this refuses to copy the Dart
 *
 * `form.dart` builds its statement with
 * `item.value.replaceAll(RegExp('{$stateKey}'), value)` and
 * `dropdown_form_field.dart` does the same, so a form-state value reaches the
 * database as SQL. Every one of the four substitution sites in the corpus lands
 * **inside a single-quoted literal** — `WHERE client = '{client}'`,
 * `WHERE table_name = '{table_name}'`, `AND table_name = '{table_name}'`,
 * `WHERE md.object_type = '{object_type}'` — so doubling an embedded quote is
 * both correct and sufficient there, and it is what `quoteLiteral` does.
 *
 * **It is a divergence and it is stated rather than hidden.** A future site that
 * substitutes an *identifier* rather than a literal would be silently wrong under
 * this rule where it was silently injectable under the Dart's. That is the better
 * of the two failures and it is still a failure, which is why the count is in this
 * comment: four of four today, and the fifth needs a second look rather than a
 * copy of the fourth. Recorded as I-72.
 *
 * ## A query with a missing parameter does not run
 *
 * The Dart's two paths agree on this and say it differently: `form.dart` bails out
 * of the whole batch on the first null, and `dropdown_form_field.dart` clears the
 * field's items and returns. Here it is per query — a form whose second query is
 * waiting on a choice still gets its first — which is a behaviour change only for
 * a form that has both kinds, and `fmMappingFormUF` is the only form with more
 * than one query at all.
 */

import type { JetsRow } from "../datatable/types";
import type { FormState } from "../datatable/formState";
import type { Form, FormQuery } from "./form";

/** What `raw_query_map` answers with, once the envelope is off. */
export interface QueryMapResult {
  result_map?: Record<string, unknown>;
}

/** Posts an arbitrary `/dataTable` action. `ApiClient.dataTable`, narrowed. */
export type QueryPoster = (payload: Record<string, unknown>) => Promise<QueryMapResult>;

/**
 * Doubles single quotes, the standard SQL string-literal escape.
 *
 * Postgres reads `''` inside a literal as one quote whenever
 * `standard_conforming_strings` is on, which has been the default since 9.1 and
 * is what every other statement in this repository already assumes. A backslash
 * is left alone for the same reason: under that setting it is an ordinary
 * character in a literal.
 */
export function quoteLiteral(value: string): string {
  return value.replaceAll("'", "''");
}

/** A scalar out of form state: the value, or the first of a selection. */
function scalar(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw)) return raw.length > 0 ? scalar(raw[0]) : null;
  return null;
}

/**
 * The statement to run, or null when a parameter is not yet available.
 *
 * Parameters are read from group 0, matching `form.dart`'s `getValue(0, stateKey)`
 * — see `FormQuerySchema.params`.
 *
 * **A list-valued parameter takes its first element**, which is what
 * `dropdown_form_field.dart` does (`valueStr += value[0]`) and what
 * `form.dart` *means* to do: its list branch substitutes into `query`, a local
 * initialised to `""` rather than to the statement, so a list-valued predicate
 * there yields an empty statement. Following the intent rather than the defect,
 * and noting it because the two files disagree and only one of them is right.
 */
export function resolveQuery(query: FormQuery, formState: FormState): string | null {
  let sql = query.sql;
  for (const param of query.params ?? []) {
    const value = scalar(formState.getValue(0, param));
    if (value === null || value === "") return null;
    sql = sql.replaceAll(`{${param}}`, quoteLiteral(value));
  }
  return sql;
}

export interface QueryPlan {
  /** Query name to resolved statement, for the queries that can run now. */
  ready: Record<string, string>;
  /** Names whose parameters are not all present yet. */
  waiting: string[];
  /**
   * A value that changes exactly when the resolved statements do.
   *
   * The re-run trigger. It replaces the Dart's two separate guards —
   * `predicatePreviousValue`, which skips a re-query when the predicate is
   * unchanged, and `isKeyUpdated`, which skips one when the notification came
   * from the widget itself. Both are answers to "did the *input* change", and a
   * signature over the resolved statements answers it once for every query.
   */
  signature: string;
}

export function planQueries(form: Form, formState: FormState): QueryPlan {
  const ready: Record<string, string> = {};
  const waiting: string[] = [];
  for (const [name, query] of Object.entries(form.queries ?? {})) {
    const sql = resolveQuery(query, formState);
    if (sql === null) waiting.push(name);
    else ready[name] = sql;
  }
  return {
    ready,
    waiting: waiting.sort(),
    signature: JSON.stringify(Object.entries(ready).sort()),
  };
}

/** Rows keyed by query name, or an empty map when there is nothing to run. */
export async function runQueries(
  plan: QueryPlan,
  post: QueryPoster,
): Promise<Record<string, JetsRow[]>> {
  const names = Object.keys(plan.ready);
  if (names.length === 0) return {};
  const body = await post({ action: "raw_query_map", query_map: plan.ready });
  const raw = body.result_map ?? {};
  const rows: Record<string, JetsRow[]> = {};
  for (const name of names) {
    // A name the server answered nothing for is an empty result rather than a
    // missing one: `ExecRawQueryMap` fills every key it was given or fails the
    // whole request, so the only way here is a shape this client did not expect.
    rows[name] = Array.isArray(raw[name]) ? (raw[name] as JetsRow[]) : [];
  }
  return rows;
}

/**
 * The items a `dropdown` or `typeahead` field offers, from column 0.
 *
 * Column 0 is the value and the label both, as both Dart paths do —
 * `DropdownItemConfig(label: e[0]!, value: e[0]!)` in `setDropdownItems` and in
 * `form.dart`'s cache builder. A null in column 0 is dropped rather than offered
 * as an unpickable blank.
 */
export function itemsFromRows(rows: JetsRow[] | undefined): { value: string; label: string }[] {
  if (rows === undefined) return [];
  const items: { value: string; label: string }[] = [];
  for (const row of rows) {
    const value = row[0];
    if (typeof value === "string" && value !== "") items.push({ value, label: value });
  }
  return items;
}

/**
 * Suggestions for a typeahead, ordered and filtered as the Dart's are.
 *
 * A port of `suggestionsCallback` (`components/typeahead_form_field.dart`), whose
 * two branches are genuinely different rules rather than one rule with a special
 * case:
 *
 *  - **Empty pattern** — every suggestion, with the ones resembling
 *    `priorityTarget` floated to the front. The target is lowercased, its `:`
 *    turned into `_`, and split on `_`; a suggestion containing any part is
 *    priority. Order within each half is the query's.
 *  - **Non-empty pattern** — a substring match with spaces removed from both
 *    sides and both lowercased (`doesMatch`), and **no priority ordering**. The
 *    Dart does not apply it there either; once the user is typing, their text is
 *    the better signal.
 *
 * **One divergence, in the empty-pattern branch.** The Dart does not discard
 * empty parts, so a target ending in `_` or `:` splits to a part that every
 * suggestion contains and the whole list becomes priority — which is the same
 * order as no priority at all, silently. Discarding them keeps the feature
 * working on such a target. No data property in the corpus ends in a separator,
 * so this changes no observed behaviour; it is written down because it changes
 * the rule rather than the outcome.
 */
export function suggestionsFor(
  items: readonly string[],
  pattern: string,
  priorityTarget: string | null,
): string[] {
  if (pattern !== "") {
    const needle = pattern.toLowerCase().split(" ").join("");
    return items.filter((item) => item.toLowerCase().split(" ").join("").includes(needle));
  }
  if (priorityTarget === null || priorityTarget === "") return [...items];
  const parts = priorityTarget.toLowerCase().replaceAll(":", "_").split("_").filter((p) => p !== "");
  if (parts.length === 0) return [...items];
  const priority: string[] = [];
  const rest: string[] = [];
  for (const item of items) {
    const lower = item.toLowerCase();
    (parts.some((p) => lower.includes(p)) ? priority : rest).push(item);
  }
  return [...priority, ...rest];
}

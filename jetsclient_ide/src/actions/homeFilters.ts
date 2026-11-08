/**
 * `homeFiltersUF`'s escapes, and the store they write. Task F.5.
 *
 * **This is the first action escape the port has, and it earned the name.** F.1
 * expected `mapFileUF` to need one and the grammar swallowed both of its save
 * buttons instead (I-74), so `productionRegistry.actions` has been `{}` since it
 * was written. `updateHomeFilters` is different in kind: it does not post, set or
 * navigate — it **compiles a filter into `WhereClause` objects** that a *different*
 * screen's query builder will splice into its `WHERE`, including `LIKE` patterns
 * and `now() - interval '…'` arithmetic
 * (`jetsclient/lib/modules/user_flows/home_filters/form_action_helpers.dart`,
 * `updateHomeFilters`). A grammar that could say that would be a query language,
 * and the flow that wanted it would still be the only caller.
 *
 * **So the answer to "could a primitive remove this?" is no, and the question was
 * asked.** I-74 is the direction this project travels — F.1 *removed* an escape by
 * adding two primitives — and the test that decides it is whether the primitive
 * would be justified beyond the one arm. `rows: "everyGroup"` and `require` were;
 * a where-clause builder is not.
 *
 * ## Where the filters live
 *
 * `JetsRouterDelegate` in the Flutter app holds three fields — `homeFilters`,
 * `dataRegistryFilters` and `homeFiltersState`
 * (`jetsclient/lib/routes/jets_router_delegate.dart`, `homeFilters`) — and the
 * query builder reads the first two by table key
 * (`datatable/query.ts`, `makeQuery`, which already takes them on `QueryContext`).
 * This module is that state, and it is module-level for the same reason the
 * registry is: there is one build, so there is one set of filters, and a
 * per-screen copy would make "is the home table filtered?" depend on who asked.
 *
 * ## One divergence, stated rather than smoothed over
 *
 * **The Dart accumulates the filter values in a validator and this reads them out
 * of form state.** `homeFiltersFormValidator` writes each field into
 * `homeFiltersState` as a side effect of validating it, and `updateHomeFilters`
 * then reads that map. Both maps hold the same values at the moment the escape
 * runs — the flow keeps one `JetsFormState` across its five states
 * (`screens/user_flow_screen.dart`), the escape runs immediately after
 * `formKey.currentState!.validate()` in the same arm, and the keys are the same —
 * so the accumulation buys nothing here that the form state does not already have.
 *
 * What it *does* buy is persistence between runs, since the router outlives the
 * screen and the form state does not. That is what `seedFromHomeFilters` restores,
 * so this module keeps a saved snapshot too — written by the escape rather than by
 * the validator, which is one writer instead of two.
 *
 * The validator therefore keeps only its **validation**, and only where the rule
 * is a relation between siblings; see `homeFiltersFormValidator` below.
 *
 * ## A third list, with no Dart original
 *
 * **`sessionRegistryFilters`, for the Session Registry tab** (`session_registry`
 * Phase 2, JM-081, 2026-11-08), set by that tab's *Set Filters* dialog and *Set
 * Session Id* prompt and read by `makeQuery` for `sessionRegistryTable` alone.
 * It is a list of its own rather than a share of the home filters because the
 * view it filters, `jetsapi.session_registry_view`, has none of
 * `pipeline_execution_status`'s columns that the home filters name — and the
 * archive tab, which does have them, shares the home filters instead, with
 * their table rewritten (`datatable/query.ts`, `FILTERS_BY_FIELD`).
 *
 * **It refuses what the server would splice.** `ge` and `le` reach the SQL as
 * written (`jets/datatable/data_table_action.go`, `visitWhereClause`), which is
 * I-103 for the home filters' time bounds. This list builds its bounds only from
 * input that has been checked to be digits, a date, or a count of units, so a
 * typed quote is a refusal rather than part of a query.
 */

import type { FormState } from "../datatable/formState";
import type { WhereClause } from "../datatable/types";
import type { EscapeContext } from "./escapes";

/** The keys `homeFiltersState` carries between runs, in the Dart's order. */
const SAVED_KEYS = [
  "hfProcessTableUF",
  "process_name",
  "hfStatusTableUF",
  "status",
  "hfFileKeyFilterTypeTableUF",
  "hfFileKeyMatchType",
  "hfFileKeySubstring",
  "hfStartTime",
  "hfStartOffset",
  "hfEndTime",
  "hfEndOffset",
] as const;

/** The two tables the Dart filters, by name. */
const EXEC_STATUS = "pipeline_execution_status";
const INPUT_REGISTRY = "input_registry";

/** The Session Registry tab's view. */
const SESSION_REGISTRY = "session_registry_view";

interface FilterStore {
  homeFilters: WhereClause[] | null;
  dataRegistryFilters: WhereClause[] | null;
  saved: Record<string, string | string[]>;
  sessionRegistryFilters: WhereClause[] | null;
  /** The Session Registry dialog's answers, which seed its next open. */
  sessionRegistrySaved: Record<string, string>;
}

const store: FilterStore = {
  homeFilters: null,
  dataRegistryFilters: null,
  saved: {},
  sessionRegistryFilters: null,
  sessionRegistrySaved: {},
};

/** The home filters as they stand, for a `QueryContext`. */
export const currentHomeFilters = (): WhereClause[] | null => store.homeFilters;
/** The data-registry filters as they stand, for a `QueryContext`. */
export const currentDataRegistryFilters = (): WhereClause[] | null => store.dataRegistryFilters;
/** The saved filter values, which `seedFromHomeFilters` reads. */
export const savedHomeFilterState = (): Readonly<Record<string, string | string[]>> => store.saved;

/** The Session Registry tab's filters as they stand, for a `QueryContext`. */
export const currentSessionRegistryFilters = (): WhereClause[] | null => store.sessionRegistryFilters;
/** The Session Registry dialog's last answers, which the screen seeds it from. */
export const savedSessionRegistryFilterState = (): Readonly<Record<string, string>> =>
  store.sessionRegistrySaved;

/** Empties every list, as a fresh page load would. Tests, and nothing else. */
export function resetHomeFilters(): void {
  store.homeFilters = null;
  store.dataRegistryFilters = null;
  store.saved = {};
  store.sessionRegistryFilters = null;
  store.sessionRegistrySaved = {};
}

const where = (parts: Omit<WhereClause, "defaultValue" | "lookupColumnInFormState"> &
  Partial<Pick<WhereClause, "defaultValue">>): WhereClause => ({
  defaultValue: [],
  lookupColumnInFormState: false,
  ...parts,
});

/** `unpack` — a scalar, or the first element of a selection. */
function unpack(value: unknown): string | null {
  if (typeof value === "string") return value;
  if (Array.isArray(value) && value.length > 0 && typeof value[0] === "string") return value[0];
  return null;
}

/**
 * `unpackToList` — a list, or a decoded Postgres `{a,b}` literal.
 *
 * The same function `interpret.ts` implements for `fromKeyList`; it is repeated
 * here rather than exported from there because that one is the *grammar's*
 * conversion and this is a Dart helper an escape body calls. They agree, and the
 * test drives both against the same values.
 */
function unpackToList(value: unknown): string[] | null {
  if (value === null || value === undefined) return null;
  if (Array.isArray(value)) return value.filter((v): v is string => typeof v === "string");
  if (typeof value !== "string") return null;
  if (value === "{}") return [];
  if (value.startsWith("{")) return value.slice(1, -1).split(",");
  return [value];
}

/**
 * `now()`/`timestamp '…'` plus an optional interval, as the Dart writes it.
 *
 * **The string goes into `ge`/`le` and reaches the server as a SQL fragment**, not
 * as a bound parameter — `makeWhereClause` copies it through
 * (`datatable/query.ts`) and the Go builder splices it. That is the Flutter app's
 * behaviour and this port reproduces it rather than improving it, because a
 * different shape here would make the two apps disagree about a live table while
 * both are shipping. Recorded as **I-103**.
 */
function timeBound(at: string | null, offset: string | null): string | null {
  const hasAt = at !== null && at !== "";
  const hasOffset = offset !== null && offset !== "";
  if (!hasAt && !hasOffset) return null;
  let value = hasAt ? `timestamp '${at}'` : "now()";
  if (hasOffset) value += `-interval '${offset}'`;
  return value;
}

/**
 * Compiles the flow's answers into the two filter lists. The action escape.
 *
 * Step for step with the Dart: process names, statuses, the file-key match, the
 * start bound, the end bound. **The order matters and is asserted**, because the
 * lists are sent as an array and a server-side `WHERE` built from them in a
 * different order is a different query text — which is the trap F.3 found in a
 * coverage document (I-90).
 */
export function updateHomeFilters(context: EscapeContext): Promise<string | null> {
  const { formState, group } = context;
  const read = (key: string): unknown => formState.getValue(group, key);

  const homeFilters: WhereClause[] = [];
  const dataRegistryFilters: WhereClause[] = [];

  const processNames = unpackToList(read("process_name"));
  if (processNames !== null) {
    homeFilters.push(where({ table: EXEC_STATUS, column: "process_name", defaultValue: processNames }));
  }

  const statuses = unpackToList(read("status"));
  if (statuses !== null) {
    homeFilters.push(where({ table: EXEC_STATUS, column: "status", defaultValue: statuses }));
  }

  const matchType = unpack(read("hfFileKeyMatchType"));
  const substring = unpack(read("hfFileKeySubstring"));
  if (matchType !== null && substring !== null && substring !== "") {
    // `None` is spelled as the empty value column of the option table, so it
    // falls through the switch exactly as the Dart's `default:` does.
    const patterns: Record<string, string | null> = {
      equals_value: null,
      starts_with: `${substring}%`,
      ends_with: `%${substring}`,
      contains: `%${substring}%`,
    };
    if (matchType in patterns) {
      const like = patterns[matchType];
      const clause = (table: string, column: string): WhereClause =>
        like === null
          ? where({ table, column, defaultValue: [substring] })
          : where({ table, column, like });
      homeFilters.push(clause(EXEC_STATUS, "main_input_file_key"));
      dataRegistryFilters.push(clause(INPUT_REGISTRY, "file_key"));
    }
  }

  const start = timeBound(unpack(read("hfStartTime")), unpack(read("hfStartOffset")));
  if (start !== null) {
    homeFilters.push(where({ table: EXEC_STATUS, column: "start_time", ge: start }));
    dataRegistryFilters.push(where({ table: INPUT_REGISTRY, column: "last_update", ge: start }));
  }

  const end = timeBound(unpack(read("hfEndTime")), unpack(read("hfEndOffset")));
  if (end !== null) {
    homeFilters.push(where({ table: EXEC_STATUS, column: "start_time", le: end }));
    dataRegistryFilters.push(where({ table: INPUT_REGISTRY, column: "last_update", le: end }));
  }

  store.homeFilters = homeFilters;
  store.dataRegistryFilters = dataRegistryFilters;
  store.saved = snapshot(formState, group);
  // The screen recomputes the query context from this store, so it has to be told
  // something changed; nothing in form state did.
  formState.notifyListeners();
  return Promise.resolve(null);
}

/** What survives the flow: the answers, minus the ones left blank. */
function snapshot(formState: FormState, group: number): Record<string, string | string[]> {
  const saved: Record<string, string | string[]> = {};
  for (const key of SAVED_KEYS) {
    const value = formState.getValue(group, key);
    // The Dart *removes* the key when the value is null, so an absent key and a
    // null one are the same thing and the snapshot spells it the one way.
    if (value === null || value === undefined) continue;
    if (Array.isArray(value) && value.length === 0) continue;
    saved[key] = value as string | string[];
  }
  const email = formState.getValue(group, "user_email");
  if (typeof email === "string" && email !== "") saved["user_email"] = email;
  return saved;
}

/**
 * The `Clear Filters` button. Also an action escape.
 *
 * `data_table.dart`'s `clearHomeFilters` arm sets all three to empty and refreshes
 * — **empty rather than null**, which is why `hasHomeFilters` tests emptiness and
 * not nullity. The Dart also clears the table's own selection; that is the
 * widget's and `thenRefresh` on the request is what asks for it.
 */
export function clearHomeFilters(context: EscapeContext): Promise<string | null> {
  store.homeFilters = [];
  store.dataRegistryFilters = [];
  store.saved = {};
  context.formState.notifyListeners();
  return Promise.resolve(null);
}

/**
 * Replaces the filters with one on `session_id` or `request_id`.
 *
 * The two Dart arms are the same code with a column swapped, except that the
 * data-registry half joins differently: `session_id` joins
 * `pipeline_execution_status.input_session_id` and `request_id` joins
 * `pipeline_execution_status.input_request_id`
 * (`jetsclient/lib/components/data_table.dart`, `setSessionIdFilter`).
 *
 * **Not an escape**, because no document names it — the action *type* is the
 * reference, and `actionDispatch` turns it into a `promptFilter` request the host
 * serves. An escape name would be a second way to reach the same behaviour.
 */
export function setIdFilter(column: "session_id" | "request_id", input: string): void {
  const values = input.split(",").map((part) => part.trim());
  const joinWith =
    column === "session_id"
      ? "pipeline_execution_status.input_session_id"
      : "pipeline_execution_status.input_request_id";
  store.homeFilters = [where({ table: EXEC_STATUS, column, defaultValue: values })];
  store.dataRegistryFilters = [
    where({ table: INPUT_REGISTRY, column, joinWith }),
    where({ table: EXEC_STATUS, column, defaultValue: values }),
  ];
}

/**
 * Seeds the form from the filters the last run saved. The initializer escape.
 *
 * `formStateInitializer` on the flow document, and the only one in the corpus
 * (`userflow/schema.ts`). The Dart copies `homeFiltersState` into group 0 key by
 * key; so does this, from the snapshot the escape above writes.
 */
export function seedFromHomeFilters(context: EscapeContext): void {
  for (const [key, value] of Object.entries(store.saved)) {
    context.formState.setValue(context.group, key, value);
  }
}

/** `homeFilters != null && homeFilters.isNotEmpty` — the `Clear Filters` gate. */
export const hasHomeFilters = (_formState: FormState, _group: number): boolean =>
  store.homeFilters !== null && store.homeFilters.length > 0;

/**
 * `(state) => true`.
 *
 * A name for a closure that does nothing, and it exists so the corpus translation
 * stays lossless — see the note in `datatable/tableTranslate.ts`. Two sites, both
 * on `pipelineExecStatusTable`.
 */
export const alwaysEnabled = (_formState: FormState, _group: number): boolean => true;

/**
 * The one cross-field rule of the flow. The validator escape.
 *
 * **`homeFiltersFormValidator` is 70 lines in the Dart and about two rules here**,
 * for the reason I-73 gave about `mappingFormValidator`: most of a Dart validator
 * is not validation. Six of its eight cases `return null` unconditionally after a
 * side effect, and the side effects are the accumulation this module's escape does
 * instead. What is left:
 *
 * - `hfFileKeyFilterTypeTableUF` must be chosen — expressible as `required`, and
 *   it is, on the field. The Dart's extra `hfFileKeyMatchType != null` conjunct is
 *   not a second condition: that key is the option table's second column and every
 *   row of the five has a non-null value there, `None`'s being the empty string
 *   (`home_filters/data_table_config.dart`, `hfFileKeyFilterTypeTableUF`).
 * - `hfFileKeySubstring` is required **iff** a filter type other than `None` is
 *   chosen. That is a relation between two fields, so it is here.
 *
 * The message strings are the Dart's.
 */
export function homeFiltersFormValidator(
  context: EscapeContext,
  key: string,
  value: unknown,
): string | null {
  if (key !== "hfFileKeySubstring") return null;
  // The Dart reads the *key column* here, not the value column: `None`'s key is
  // the literal `None` and its value is empty, so testing the value would make
  // "no filter" indistinguishable from "nothing selected" and the branch would
  // never fire.
  const filterType = unpack(context.formState.getValue(context.group, "hfFileKeyFilterTypeTableUF"));
  if (filterType === null || filterType === "None") return null;
  const substring = unpack(value);
  return substring === null || substring === "" ? "Enter a file key fragment" : null;
}

// ---------------------------------------------------------------------------
// The Session Registry tab's filters. `session_registry` Phase 2, 2026-11-08.
// ---------------------------------------------------------------------------

/**
 * The dialog's keys, in the order its fields are drawn and its clauses built.
 *
 * Prefixed `sr` as `homeFiltersUF`'s are `hf`, because the dialog's form state is
 * a group of the screen's and a bare `source_type` there would be read by any
 * table whose where clause names it.
 */
export const SESSION_REGISTRY_FILTER_KEYS = [
  "srSourceType",
  "srMonthPeriodFrom",
  "srMonthPeriodTo",
  "srUpdatedFrom",
  "srUpdatedTo",
] as const;

/** The most digits a `session_num` holds: the view's own bound, a bigint's 18. */
const SESSION_NUM = /^[0-9]{1,18}$/;
/** A month period, as the view's `month_period` column holds it. */
const DIGITS = /^[0-9]{1,9}$/;
/** A source type: a word, as `session_registry.source_type` holds one. */
const WORD = /^[A-Za-z0-9_]{1,40}$/;
/** A session id in a comma list: what the server may put between quotes. */
const SESSION_ID = /^[A-Za-z0-9_.:-]{1,80}$/;
/** A date, or a date and time, as `timestamp '…'` reads it. */
const TIMESTAMP = /^[0-9]{4}-[0-9]{2}-[0-9]{2}([ T][0-9]{2}:[0-9]{2}(:[0-9]{2})?Z?)?$/;
/** A count of units, as `interval '…'` reads it: `3 days`, `12 hours`. */
const OFFSET = /^[0-9]{1,4} (minute|hour|day|week|month|year)s?$/;

/**
 * An instant typed into the dialog, as the SQL fragment `ge`/`le` carries.
 *
 * A date or timestamp is `timestamp '<it>'`; a count of units is that long
 * before now, `now()-interval '<it>'`, the home filters' offset in one field
 * rather than two. Null for anything else, which the validator refuses — this is
 * the check that keeps a typed quote out of the query.
 */
function instant(value: string): string | null {
  if (TIMESTAMP.test(value)) return `timestamp '${value.replace("T", " ").replace(/Z$/, "")}'`;
  if (OFFSET.test(value)) return `now()-interval '${value}'`;
  return null;
}

/** The dialog's value for a key, trimmed; empty when unset. */
function answer(formState: FormState, group: number, key: string): string {
  return (unpack(formState.getValue(group, key)) ?? "").trim();
}

/**
 * The Session Registry dialog's field rule. The validator escape.
 *
 * One rule per kind of field: a source type is a word, a month period is
 * digits, an instant is a date or a count of units. Blank is always allowed — a
 * blank field is no filter — and the two ends of a range are checked apart, so
 * an open range is a range.
 */
export function sessionRegistryFiltersValidator(
  _context: EscapeContext,
  key: string,
  value: unknown,
): string | null {
  const text = (unpack(value) ?? "").trim();
  if (text === "") return null;
  switch (key) {
    case "srSourceType":
      return WORD.test(text) ? null : "A source type is one word, such as file or domain_table";
    case "srMonthPeriodFrom":
    case "srMonthPeriodTo":
      return DIGITS.test(text) ? null : "A month period is digits only";
    case "srUpdatedFrom":
    case "srUpdatedTo":
      return instant(text) !== null
        ? null
        : "Enter a date (2026-11-08), a date and time (2026-11-08T15:00:00Z), or a duration (3 days)";
    default:
      return null;
  }
}

/**
 * Compiles the dialog's answers into `sessionRegistryFilters`. The action escape.
 *
 * **It replaces the list**, the session-id filter included, as `updateHomeFilters`
 * replaces the home filters: the tab has one set of filters and the last thing
 * the user asked for is it. The answers are checked again here, although
 * `validate` ran first, because a filter that reached the store unchecked would
 * reach the server unchecked; a refusal stops the action with its message.
 */
export function updateSessionRegistryFilters(context: EscapeContext): Promise<string | null> {
  const { formState, group } = context;
  const filters: WhereClause[] = [];
  const saved: Record<string, string> = {};
  for (const key of SESSION_REGISTRY_FILTER_KEYS) {
    const text = answer(formState, group, key);
    if (text === "") continue;
    const refusal = sessionRegistryFiltersValidator(context, key, text);
    if (refusal !== null) return Promise.resolve(refusal);
    saved[key] = text;
    switch (key) {
      case "srSourceType":
        filters.push(where({ table: SESSION_REGISTRY, column: "source_type", defaultValue: [text] }));
        break;
      case "srMonthPeriodFrom":
        filters.push(where({ table: SESSION_REGISTRY, column: "month_period", ge: text }));
        break;
      case "srMonthPeriodTo":
        filters.push(where({ table: SESSION_REGISTRY, column: "month_period", le: text }));
        break;
      case "srUpdatedFrom":
        filters.push(where({ table: SESSION_REGISTRY, column: "last_update", ge: instant(text)! }));
        break;
      case "srUpdatedTo":
        filters.push(where({ table: SESSION_REGISTRY, column: "last_update", le: instant(text)! }));
        break;
    }
  }
  store.sessionRegistryFilters = filters;
  store.sessionRegistrySaved = saved;
  formState.notifyListeners();
  return Promise.resolve(null);
}

/** The Session Registry tab's *Clear Filters*. An action escape. */
export function clearSessionRegistryFilters(context: EscapeContext): Promise<string | null> {
  store.sessionRegistryFilters = [];
  store.sessionRegistrySaved = {};
  context.formState.notifyListeners();
  return Promise.resolve(null);
}

/** Whether the Session Registry tab has a filter to clear. */
export const hasSessionRegistryFilters = (_formState: FormState, _group: number): boolean =>
  store.sessionRegistryFilters !== null && store.sessionRegistryFilters.length > 0;

/**
 * The Session Registry tab's *Set Session Id*: a comma list, or a range. `Q-6`.
 *
 * - **`a, b, c`** filters `session_id` on those values, as `setIdFilter` does for
 *   the other tabs. The server puts each between quotes as written, so an id is
 *   refused unless it is letters, digits and `_ . : -`.
 * - **`from..to`**, either end empty, becomes `ge` and `le` clauses on the view's
 *   numeric `session_num`, which is what makes `1000..` mean a number and not a
 *   string that sorts after `1000`. **Each bound must be digits only, at most 18**
 *   — the server splices `ge` and `le` as written, so this check is the whole of
 *   what keeps the range out of the SQL — and a range with neither end, or with
 *   its ends reversed, is refused rather than read as everything or nothing.
 *
 * Returns the refusal, or null once the filter is set. **It replaces the tab's
 * filters**, as `setIdFilter` replaces the home filters.
 */
export function setSessionRegistryIdFilter(input: string): string | null {
  const text = input.trim();
  const range = text.indexOf("..");
  if (range >= 0) {
    const from = text.slice(0, range).trim();
    const to = text.slice(range + 2).trim();
    if (from === "" && to === "") return "Give at least one end of the range, as 1000.. or ..2000";
    for (const bound of [from, to]) {
      if (bound !== "" && !SESSION_NUM.test(bound)) {
        return `A range bound is digits only, at most 18 of them: "${bound}" is not`;
      }
    }
    if (from !== "" && to !== "" && BigInt(from) > BigInt(to)) {
      return `The range ${from}..${to} is empty: its first end is after its second`;
    }
    const filters: WhereClause[] = [];
    if (from !== "") filters.push(where({ table: SESSION_REGISTRY, column: "session_num", ge: from }));
    if (to !== "") filters.push(where({ table: SESSION_REGISTRY, column: "session_num", le: to }));
    store.sessionRegistryFilters = filters;
    store.sessionRegistrySaved = {};
    return null;
  }
  const values = text.split(",").map((part) => part.trim()).filter((part) => part !== "");
  if (values.length === 0) return "Enter session ids separated by commas, or a range such as 1000..2000";
  const refused = values.find((value) => !SESSION_ID.test(value));
  if (refused !== undefined) {
    return `"${refused}" is not a session id: letters, digits and _ . : - only`;
  }
  store.sessionRegistryFilters = [where({ table: SESSION_REGISTRY, column: "session_id", defaultValue: values })];
  store.sessionRegistrySaved = {};
  return null;
}

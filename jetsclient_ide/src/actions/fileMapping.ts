/**
 * The `file_mapping/` directory's escapes — two of `mapFileUF`'s and two of
 * `fileMappingUF`'s. Tasks F.1 and F.8.
 *
 * **The header said "`mapFileUF`'s two escapes" for seven tasks, and the module
 * is named for the *directory*.** `file_mapping/` defines two flows (I-61), and
 * both put a body here: F.1's pair are the worksheet's, F.8's pair belong to the
 * flow above it. They share a directory, a table and nothing else — which is
 * exactly the split I-61 was written about, arriving in this file.
 *
 * The file mapping worksheet is the corpus's hardest form and the one Phase 2
 * deliberately kept out of the proof flows: one state, zero declared fields, a
 * row builder, two exotic field types and a 222-line validator (I-61). What is
 * here is the part of it that does not become data — and the measurement of what
 * that turned out to be is the point of this comment.
 *
 * ## The action half became data after all, and no escape was needed
 *
 * `actions/coverage/fileMappingUF.ua.json` had `mapperOk` and `mapperDraft`
 * reaching a `saveProcessMapping` escape, from I-22's coverage pass. Writing the
 * runtime document showed the grammar can express both, given two additions that
 * are justified beyond this flow: `rows: "everyGroup"` (the Dart's
 * `getInternalState()`, which is what a repeating form posts) and a `require`
 * step (sixteen arms across the delegates open with the same missing-key guard).
 * **So the escape count for this flow is two, not three**, and §4's risk —
 * *"an eighth escape is an acceptable outcome rather than a failure"* — is
 * discharged at seven.
 *
 * ## What could not become data, and why each is the right shape
 *
 * **`seedMappingRow`** is `inputFieldRowBuilder`'s writing half
 * (`jetsclient/lib/modules/user_flows/file_mapping/form_config.dart`, inside
 * `inputFieldRowBuilder`). The layout it also returns *is* data — `rows` in the
 * form document, drawn once per group — but the seeding is a three-term coalesce
 * whose last term is a membership test against a second query, and a binding
 * vocabulary for that would have one consumer.
 *
 * **`mappingFormValidator`** is the 222 lines. Every branch of it this form
 * reaches is a relation between sibling values, which the `required`/`json` rules
 * cannot state and which four bespoke rule kinds would only state for this form.
 *
 * ## The 222 lines are not 222 lines, and the plan should know it
 *
 * `mappingFormValidator` has **one** consumer — `fmMappingFormUF`, the grep is
 * one line — and a form validator is only ever called with its own form's field
 * keys. That form declares five: `input_column`, `function_name`, `argument`,
 * `default_value`, `error_message` — the Dart's `FSK` *values*, which is what a
 * form-state key is. The
 * Dart's switch has **twelve** cases; the other seven name `client`, `org`,
 * `object_type`, `source_type`, `entity_rdf_type`, `lookback_periods` and
 * `table_name`, and **no form in `jetsclient` declares a field with any of those
 * keys** — they are the "Add/Update Process Input" dialog's, and that dialog is
 * gone.
 *
 * The 46-line block above the switch is dead for the same kind of reason: it is
 * guarded on `objectTypeRegistryCache`, which is filled only by
 * `configure_files`'s object-type dropdown (`returnedModelCacheKey`), on a
 * different flow's form. `mapFileUF` runs standalone from a route, so the cache
 * is never there and the block never runs.
 *
 * **So the live surface is five cases of twelve, about 99 lines of 222.** That is
 * I-39 and I-63's shape a third time — dead configuration found only by asking a
 * reachability question — and it is why the port below is shorter than the
 * original rather than a translation of it. Recorded as I-73.
 */

import type { JetsRow } from "../datatable/types";
import type {
  ActionEscape,
  EscapeContext,
  RowInitializerEscape,
  ValidatorEscape,
} from "./escapes";

/**
 * The query names this pair reads out of form state.
 *
 * **They are the document's names, and that coupling is deliberate.** An escape
 * and the document that names it are a pair — `FlowStore` refuses the set if
 * either half is missing — so a constant here that must match `mapFileUF.form.json`
 * is one fact in two files rather than a hidden dependency. The alternative is a
 * parameterised escape, which is a configuration language for one caller.
 */
const INPUT_COLUMNS = "inputColumns";
const MAPPING_FUNCTIONS = "mappingFunctions";

/**
 * The Dart's key, and the reason it is spelled out here.
 *
 * `FSK.isRequiredFlag` is **`"flag.is_required"`**, not `is_required_flag`
 * (`jetsclient/lib/utils/constants.dart`, `isRequiredFlag`). The
 * constant-name-versus-value trap has caught this project five times now —
 * `flowActions.test.ts` extracts `constants.dart` at test time for exactly this
 * reason — and it is worth a named constant because nothing else in this file
 * reads it and a typo would simply make every property look optional.
 */
const IS_REQUIRED = "flag.is_required";

/** Column 0 of a query's rows, as a set. */
function firstColumn(context: EscapeContext, query: string): string[] {
  return (context.formState.queryRows(query) ?? [])
    .map((row) => row[0])
    .filter((value): value is string => typeof value === "string");
}

/** A scalar out of form state: the value, or the first of a selection. */
function scalar(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw)) return raw.length > 0 ? scalar(raw[0]) : null;
  return null;
}

const isEmpty = (raw: unknown): boolean => {
  const value = scalar(raw);
  return value === null || value === "";
};

/**
 * Seeds one worksheet row. The writing half of `inputFieldRowBuilder`.
 *
 * `inputFieldsQuery` selects eight columns:
 *
 * | # | Column | Becomes |
 * |---|---|---|
 * | 0 | `md.data_property` | `data_property`, and the row's label |
 * | 1 | `md.is_required` | `flag.is_required`, and the `*` on the label |
 * | 2 | `pm.input_column` | `input_column`, first choice |
 * | 3 | `pm.function_name` | `function_name` |
 * | 4 | `pm.argument` | `argument` |
 * | 5 | `pm.default_value` | `default_value` |
 * | 6 | `pm.error_message` | `error_message` |
 * | 7 | `md.default_column_value` | `input_column`, second choice |
 *
 * **The Dart reads columns 2 to 6 from `savedState` and 0, 1 and 7 from the row,
 * and the two are the same list.** `savedStateQuery` and `inputFieldsQuery` are
 * both `"inputFieldsQuery"` (`form_config.dart`, the `FormConfig` for
 * `FormKeys.fmMappingFormUF`), so `savedState?[index][n]` *is* `inputFieldRow[n]`.
 * The closure reads as a join of two results and is a column map over one, which
 * is what makes this port a table rather than an argument.
 *
 * **`index` is the group**, not `context.group`: the escape is called once per
 * row and each row is its own validation group. `context.group` is the form's,
 * which is 0 for every flow in the corpus.
 */
export const seedMappingRow: RowInitializerEscape = (context, row: JetsRow, index) => {
  const { formState } = context;
  const dataProperty = row[0] ?? "";
  const isRequired = row[1] === "1";

  // Set only when required, as the Dart does — the validator asks whether the
  // key is "1", so an explicit "0" and an absent key mean the same thing and
  // writing one of them would be writing a value nothing reads.
  if (isRequired) formState.setValue(index, IS_REQUIRED, "1");

  formState.setValue(index, "data_property", dataProperty);
  // The label the row is headed with. Built here rather than in the document
  // because it is a value with a rule in it; the document reads it by key.
  formState.setValue(index, "data_property_label", `${dataProperty}${isRequired ? "*" : ""}`);

  // `saved ?? md.default_column_value ?? (the data property, if the staging
  // table happens to have a column of that name)`. The third term is why this is
  // an escape: it is a membership test against another query's result.
  const columns = firstColumn(context, INPUT_COLUMNS);
  const fallback = row[7] ?? (columns.includes(dataProperty) ? dataProperty : null);
  formState.setValue(index, "input_column", row[2] ?? fallback);

  formState.setValue(index, "function_name", row[3]);
  formState.setValue(index, "argument", row[4]);
  formState.setValue(index, "default_value", row[5]);
  formState.setValue(index, "error_message", row[6]);
};

/**
 * The worksheet's validator. Five cases, ported branch for branch.
 *
 * The Dart's side effects are deliberately not reproduced.
 * `markFormKeyAsValid` / `markFormKeyAsInvalid` maintain a set that
 * `JetsFormState.isFormValid()` then reads (`jets_form_state.dart`,
 * `isFormValid`); here `isFormValid` re-runs the validator over every field of
 * every group, which is the same answer without a second copy of it to keep in
 * step. The token refresh at the top of the Dart's function belongs to
 * `ApiClient`, which does it on every request.
 */
export const mappingFormValidator: ValidatorEscape = (context, key, value) => {
  const { formState, group } = context;
  const text = scalar(value);
  const at = (sibling: string): string | null => scalar(formState.getValue(group, sibling));

  switch (key) {
    case "input_column": {
      if (text !== null && text !== "") {
        // A typed column that the staging table does not have. The control
        // accepts it — the Dart's typeahead writes through on every keystroke —
        // so this is where the user is told.
        return firstColumn(context, INPUT_COLUMNS).includes(text)
          ? null
          : "Input Column is not valid.";
      }
      if (at(IS_REQUIRED) !== "1") return null;
      // Required, and unmapped: a default or an error message stands in for it.
      if (!isEmpty(at("default_value"))) return null;
      if (!isEmpty(at("error_message"))) return null;
      return "Input Column must be selected or either a default or an error message must be provided.";
    }

    case "function_name":
      return null;

    case "argument": {
      const functionName = at("function_name");
      if (functionName === null || functionName === "") {
        return text !== null && text !== ""
          ? "Remove the argument when no function is selected"
          : null;
      }
      if (text !== null && text !== "") return null;
      // `mappingFunctionsQuery` selects `function_name, is_argument_required`, so
      // whether the argument is required is column 1 of the row naming it.
      const row = (formState.queryRows(MAPPING_FUNCTIONS) ?? []).find((r) => r[0] === functionName);
      // Absent rather than asserting: the Dart asserts, and Dart strips asserts
      // in release (I-47), so its production behaviour on a missing cache is to
      // throw inside `firstWhere`. Treating it as "not required" keeps the form
      // usable while the query is in flight.
      if (row === undefined) return null;
      return row[1] === "1" ? "Cleansing function argument is required" : null;
    }

    case "default_value": {
      const errorMessage = at("error_message");
      const bothGiven = !isEmpty(text) && !isEmpty(errorMessage);
      return bothGiven ? "Cannot specify both a default value and an error message" : null;
    }

    case "error_message":
      return null;

    default:
      // The Dart prints "Oops … has no validator configured for form field" and
      // returns null. Returning null is the same; the print is not reproduced,
      // because a form whose fields and validator are authored together cannot
      // reach here without `validateDocumentSet` having been bypassed.
      return null;
  }
};

/**
 * The nine columns `downloadMapping` selects, in the Dart's order.
 *
 * They are the CSV's header row *and* the request's `columns` list, which is why
 * they are one array rather than two — the Dart writes the header as a literal
 * string and builds the column list beside it
 * (`file_mapping/form_action_helpers.dart`, `downloadMapping`), and the two
 * agreeing is what makes the file readable by the intake dialog that reads it
 * back.
 */
const MAPPING_COLUMNS = [
  "client",
  "org",
  "object_type",
  "data_property",
  "input_column",
  "function_name",
  "argument",
  "default_value",
  "error_message",
] as const;

/**
 * Every cell quoted, except a null, which is written as nothing at all.
 *
 * The Dart writes `'"$column"'` per non-null cell and nothing for a null, with
 * commas between (`downloadMapping`). It does **not** escape an embedded quote,
 * and neither does this: a mapping value containing `"` produces the same
 * malformed row in both apps, which is a divergence not worth introducing — see
 * I-123.
 */
function csvRow(cells: readonly (string | null)[]): string {
  return cells.map((cell) => (cell === null ? "" : `"${cell}"`)).join(",");
}

/**
 * `downloadMapping` — the flow's table button. Task F.8.
 *
 * **The one escape of the corpus that is an escape for the reason the mechanism
 * was designed for.** It issues a `read` whose shape no `post` step can express —
 * two from clauses joined on `table_name`, three value filters and nine named
 * columns — turns up to a thousand rows into a CSV, and hands the browser a file.
 * The grammar has a `query` step and it is the wrong one: that resolves a
 * *registered* statement and returns the first row by column name.
 *
 * **The three `unpack`s are the Dart's and they matter.** `client`, `org` and
 * `object_type` are published by `fmInputSourceMappingUF`'s `formStateBinding`,
 * so a selected row puts each in a one-element list; the request's `values`
 * arrays would then hold a list inside a list and match nothing.
 *
 * **Every failure path returns null, which is the Dart's and reads wrong until
 * you check it.** A 401 returns null silently — the api client has already
 * logged the user out — and any other failure shows a message and returns null
 * rather than the message. Returning it would put an error banner on a screen
 * that already has a snackbar, and `runAction` would stop an action that has
 * nothing left to do.
 */
export const downloadMapping: ActionEscape = async (context, host) => {
  const { formState, group } = context;
  const read = (key: string): string | null => scalar(formState.getValue(group, key));
  const client = read("client");
  const org = read("org");
  const objectType = read("object_type");

  const rows = await host.read({
    endpoint: "/dataTable",
    body: {
      action: "read",
      fromClauses: [
        { schema: "jetsapi", table: "source_config" },
        { schema: "jetsapi", table: "process_mapping" },
      ],
      whereClauses: [
        { table: "source_config", column: "client", values: [client] },
        { table: "source_config", column: "org", values: [org] },
        { table: "source_config", column: "object_type", values: [objectType] },
        { table: "source_config", column: "table_name", joinWith: "process_mapping.table_name" },
      ],
      offset: 0,
      limit: 1000,
      columns: MAPPING_COLUMNS.map((column) => ({ column })),
      sortColumn: "data_property",
      sortAscending: true,
    },
  });

  if (rows === null) {
    host.notify("error", "Unknown Error reading data from table");
    return null;
  }

  const lines = [csvRow(MAPPING_COLUMNS), ...rows.map(csvRow), ""];
  host.download("mapping.csv", lines.join("\n"));
  return null;
};

/**
 * `loadRawRows` — the Save button of `loadRawRowsDialog`. Task F.8.
 *
 * **This one is an escape on a security ground rather than an expressive one,
 * and the distinction is the whole of I-121.** The arm is four lines: write
 * `user_email` into form state and post the state as one row
 * (`file_mapping/form_action_helpers.dart`, `loadRawRows`). A `set` step and a
 * `post` with `transport: "insertRows"` say exactly that — I-74's question, asked
 * and answered *yes*. What refuses it is S.7's allowlist: `insert_raw_rows`
 * parses the pasted text and runs `DELETE FROM jetsapi.process_mapping` in a
 * pre-processing hook **before** `InsertRows` calls `VerifyUserPermission`
 * (`jets/datatable/data_table_action.go`, `InsertRawRows`). Adding it to
 * `ServerActionSchema` would let an authored document delete a client's mappings
 * on behalf of a user holding no `client_config` — the confused-deputy shape
 * `schema.ts` names as the reason the allowlist exists.
 *
 * **The body reproduces `postInsertRows`** (`modules/actions/delegate_helpers.dart`,
 * `postInsertRows`), which is what the `insertRows` transport already does in the
 * interpreter: 401 is silent, 200 closes and refreshes, 409 is a duplicate, and
 * anything else records the server's message under `serverError` and closes
 * anyway. That duplication is the price of the paragraph above and is stated
 * rather than hidden.
 */
export const loadRawRows: ActionEscape = async (context, host) => {
  const { formState, group } = context;
  formState.setValue(group, "user_email", host.userEmail() as never);

  const result = await host.post({
    endpoint: "/dataTable",
    body: {
      action: "insert_raw_rows",
      fromClauses: [{ table: "raw_rows/process_mapping" }],
      data: [formState.snapshot(group)],
    },
  });

  if (result.statusCode === 200) {
    host.close();
    formState.requestRefresh();
    return null;
  }
  // The api client turns a 401 into a sign-out and this never sees one; the Dart
  // returns null there and so does the branch that would.
  const message =
    result.statusCode === 409
      ? "Duplicate record. Please verify."
      : (result.error ?? "Something went wrong. Please try again.");
  formState.setValue(group, "serverError", message as never);
  host.notify("error", message);
  host.close();
  return message;
};

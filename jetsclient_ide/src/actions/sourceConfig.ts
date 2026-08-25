/**
 * `sourceConfigUF`'s escapes. Task F.7, the last of the nine flows.
 *
 * ## Two transcribed escapes, two answers, and neither is F.8's
 *
 * I-121 narrowed I-74's rule to two questions — *can the grammar say it* and *may
 * an authored document be allowed to* — and F.8 answered them for `downloadMapping`
 * (no, and so it stays a body) and `loadRawRows` (yes, and it stays a body anyway,
 * because S.7's allowlist declines the target). This flow's two transcribed escapes
 * land on a third answer each.
 *
 * **`loadSourceConfigWithFileTypeInference` is gone, and most of it became steps.**
 * F.2 predicted it: `scSelectSourceConfigUF`
 * (`jetsclient/lib/modules/user_flows/configure_files/form_action_delegates.dart`,
 * `configureFilesFormActions`) is "six conditional `set`s in a row", and the `when`
 * guard F.2 added for one arm of `workspacePullUF` expresses all of them —
 * the twelve `unpack` assignments, the two-way `is_part_files` mapping and the
 * five-way backward-compatibility inference are twenty-one `set` steps in
 * `flows/sourceConfigUF.ua.json` and no code. **What is left is one thing the
 * grammar has no value form for**: `jsonDecode` of a string held in form state,
 * to lift one field out of it. That is `readXlsxSheetOption` below, and it is
 * four lines rather than the whole arm.
 *
 * **`saveSourceConfigForFileType` stays whole, and the reason is a gap in the
 * payload grammar rather than in the vocabulary.** `addSourceConfigOk` copies the
 * form state, projects the copy differently for each of eight file types, and posts
 * the copy. `RowsSchema`'s `wholeState` has exactly that projection — `normalise`
 * and `omit` — and **neither carries a `when`**, because `guard` is a property of a
 * *step*. So the grammar's two ways to say this arm are:
 *
 *  - one `post` per (file type × single-or-multi-part × insert-or-update) triple,
 *    which is twenty-eight guarded posts for one button; or
 *  - `set` steps on the live form state followed by one `post`, which is **not**
 *    what the Dart does — it mutates a copy, and the difference shows the moment
 *    the save fails: the user is left on the summary screen with `input_format`
 *    rewritten and the columns it does not use already nulled, so pressing the
 *    button a second time posts a different record.
 *
 * Adding `when` to `normalise` and `omit` would fix it and has one consumer in the
 * corpus, which is the test I-62 states and this fails. So it is a body.
 */

import type { FormState } from "../datatable/formState";
import type { ActionEscape, ValidatorEscape } from "./escapes";

/** `unpack` (`delegate_helpers.dart:10`): a scalar, or the first of a list. */
function scalar(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (typeof raw === "number") return String(raw);
  if (Array.isArray(raw)) return raw.length > 0 ? scalar(raw[0]) : null;
  return null;
}

/**
 * Lifts `currentSheet` out of the file's format-options blob.
 *
 * `state[scInputFormatDataJson]` holds a JSON object written by
 * `scEditXlsxOptionsUF` — `{"currentSheet": "…"}` — and an existing record's copy
 * of it comes back from the database as a string. The Dart decodes it and reads the
 * one key so that the edit path re-opens on the sheet the record names.
 *
 * **The `when` guard on the `escape` step is the Dart's `if`, and it reads the
 * post-inference `input_format` where the Dart reads the pre-inference local.**
 * They agree: the inference above it writes `csv`, `headerless_csv`, `fixed_width`
 * and the two `*_with_schema_provider` values, and reads `''`, `headerless_csv` and
 * `fixed_width` — so it neither produces nor consumes an xlsx value, and the key is
 * unchanged between the two readings whenever this guard is true.
 *
 * **The message is the Dart's, mislabelled, and is reproduced rather than fixed.**
 * `"Input column names is not a valid json"` is a copy-paste from the arm above it;
 * what failed to parse is the spreadsheet options. See I-133.
 */
export const readXlsxSheetOption: ActionEscape = async (context) => {
  const { formState, group } = context;
  const options = scalar(formState.getValue(group, "input_format_data_json"));
  if (options === null || options === "") return null;
  try {
    const parsed: unknown = JSON.parse(options);
    const sheet =
      parsed !== null && typeof parsed === "object"
        ? scalar((parsed as Record<string, unknown>)["currentSheet"])
        : null;
    formState.setValue(group, "currentSheet", sheet);
  } catch (error) {
    return `Input column names is not a valid json: ${(error as Error).message}`;
  }
  return null;
};

/**
 * The columns each file type does **not** use, and the ones it blanks.
 *
 * A transcription of the switch in `addSourceConfigOk`, one entry per `case`
 * label. `clear` is the Dart's `= null` and `blank` is its `= ''`; the two are
 * different values on the wire and the same value in Postgres for
 * `input_format_data_json`, which is why they are kept apart here rather than
 * merged — the Dart draws the distinction and this is a port.
 *
 * `as` is the two `*_with_schema_provider` variants, which are an authoring
 * convenience rather than a stored value: the record says `headerless_csv` or
 * `fixed_width`, and `scSelectSourceConfigUF`'s inference is what turns it back
 * into the variant when the record is re-opened.
 */
const FILE_TYPE_PROJECTION: Record<
  string,
  { clear: readonly string[]; blank: readonly string[]; as?: string }
> = {
  xlsx: { clear: ["input_columns_json", "input_columns_positions_csv"], blank: [] },
  headerless_xlsx: { clear: ["input_columns_positions_csv"], blank: [] },
  csv: {
    clear: ["input_columns_json", "input_columns_positions_csv"],
    blank: ["input_format_data_json"],
  },
  parquet: {
    clear: ["input_columns_json", "input_columns_positions_csv"],
    blank: ["input_format_data_json"],
  },
  headerless_csv: {
    clear: ["input_columns_positions_csv"],
    blank: ["input_format_data_json"],
  },
  parquet_select: {
    clear: ["input_columns_positions_csv"],
    blank: ["input_format_data_json"],
  },
  headerless_csv_with_schema_provider: {
    clear: ["input_columns_json", "input_columns_positions_csv"],
    blank: ["input_format_data_json"],
    as: "headerless_csv",
  },
  fixed_width: {
    clear: ["input_columns_json"],
    blank: ["input_format_data_json"],
  },
  fixed_width_with_schema_provider: {
    clear: ["input_columns_positions_csv", "input_columns_json"],
    blank: ["input_format_data_json"],
    as: "fixed_width",
  },
};

/** The Dart's `is_part_files` mapping, and it is a number rather than a string. */
const PART_FILES: Record<string, number> = {
  scSingleFileOption: 0,
  scMultiPartFileOption: 1,
};

/**
 * Saves the file configuration, projecting the payload by its file type.
 *
 * **The target is chosen by whether a record was selected**, which is I-115's shape
 * one flow later: `key` is set only on the edit path, where `scSelectSourceConfigUF`
 * unpacked it out of the table's selection, so `null` means *add*.
 *
 * **Two divergences from `postSimpleAction`, both stated.** The Dart shows a
 * snackbar on success and this does not — the flow ends on this button and the
 * screen changes underneath it. And on 409 the Dart shows *two* alert dialogs,
 * because the branch that reports the conflict has no `return` after it; that one is
 * reproduced, as a notification each, and recorded in I-133 rather than quietly
 * repaired.
 */
export const saveSourceConfigForFileType: ActionEscape = async (context, host) => {
  const { formState, group } = context;
  const row: Record<string, unknown> = { ...formState.snapshot(group) };

  const fileType = scalar(row["input_format"]);
  const projection = fileType === null ? undefined : FILE_TYPE_PROJECTION[fileType];
  if (projection === undefined) return "error";
  for (const key of projection.clear) row[key] = null;
  for (const key of projection.blank) row[key] = "";
  if (projection.as !== undefined) row["input_format"] = projection.as;

  const partFiles = PART_FILES[scalar(row["scSingleOrMultiPartFileOption"]) ?? ""];
  if (partFiles === undefined) return "error";
  row["is_part_files"] = partFiles;

  const table = row["key"] == null ? "source_config" : "update/source_config";
  const result = await host.post({
    endpoint: "/dataTable",
    body: {
      action: "insert_rows",
      fromClauses: [{ table }],
      data: [row],
    },
  });

  if (result.statusCode === 200) {
    formState.requestRefresh();
    return null;
  }
  if (result.statusCode === 409) host.notify("error", "Record already exist, please verify.");
  host.notify("error", "Server error, please try again.");
  return "Error while saving file configuration";
};

/**
 * The three rules of `configureFilesFormValidator` that are not field properties.
 *
 * The other nine cases are `required` and `json` rules on the fields themselves
 * (`../userflow/forms/sourceConfigUF.form.json`), which is the split F.1 established
 * and `validateForm` enforces by running the rules first and this second.
 *
 * **`client` is here rather than a `required` rule because the Dart's test is
 * `characters.length > 1`**, not *non-empty* — a one-character client name is
 * rejected. Reproduced, because a validator that accepts a value the Flutter app
 * refuses would let a user create a record the other app cannot edit.
 *
 * **`org`'s rule is not here, and it is the one this port cannot reproduce.** The
 * Dart refuses `null` and accepts `''`, which are the two items of that dropdown —
 * a valueless prompt and an explicit *No Organization*. `DropdownItemSchema.value`
 * is a required string, so both would be `""`; see I-131.
 */
export const sourceConfigFormValidator: ValidatorEscape = (context, key, value) => {
  const { formState } = context;
  const v = scalar(value);
  switch (key) {
    case "client":
      return v !== null && [...v].length > 1 ? null : "Client name must be selected.";

    case "input_columns_json": {
      // Nullable unless the file has no header row of its own to read.
      if (!NEEDS_INPUT_COLUMNS.has(fileTypeOf(formState) ?? "")) return null;
      if (v === null || v === "") return "Input column names must be provided";
      try {
        JSON.parse(v);
      } catch (error) {
        return `Input column names is not a valid json: ${(error as Error).message}`;
      }
      return null;
    }

    case "input_columns_positions_csv":
      if (fileTypeOf(formState) !== "fixed_width") return null;
      return v === null || v === ""
        ? "Input columns names and positions must be provided using csv"
        : null;

    default:
      return null;
  }
};

/** The file types whose columns the user has to supply. */
const NEEDS_INPUT_COLUMNS = new Set(["headerless_csv", "headerless_xlsx", "parquet_select"]);

/** Read from group 0, as the Dart's `formState.getValue(0, …)` does. */
function fileTypeOf(formState: FormState): string | null {
  return scalar(formState.getValue(0, "input_format"));
}

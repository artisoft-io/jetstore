/**
 * `mapFileUF`'s two escapes. Task F.1.
 *
 * These are the parts of the file mapping worksheet that did **not** become
 * data, so they are the parts a document cannot be read to check. Every case
 * names the Dart branch it is a port of — `inputFieldRowBuilder` for the seeder
 * and `mappingFormValidator`'s switch for the validator — because that is the
 * only thing standing between "the code runs" and "the code is faithful": a
 * delegate is a function body, so no corpus can be generated for it
 * (`sizing_action_grammar.md` §2).
 */

import { describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import type { JetsRow } from "../datatable/types";
import { mappingFormValidator, seedMappingRow } from "./fileMapping";
import type { EscapeContext } from "./escapes";

/** The eight columns of `inputFieldsQuery`, in order. */
const row = (
  dataProperty: string,
  isRequired: string,
  saved: (string | null)[] = [null, null, null, null, null],
): JetsRow => [
  dataProperty,
  isRequired,
  saved[0] ?? null, // pm.input_column
  saved[1] ?? null, // pm.function_name
  saved[2] ?? null, // pm.argument
  saved[3] ?? null, // pm.default_value
  saved[4] ?? null, // pm.error_message
  null, // md.default_column_value
];

function context(formState: FormState, group = 0): EscapeContext {
  return { formState, group, flowKey: "mapFileUF" };
}

function withColumns(columns: string[], functions: [string, string][] = []): FormState {
  const formState = new FormState();
  formState.setQueryRows(
    "inputColumns",
    columns.map((c) => [c]),
  );
  formState.setQueryRows(
    "mappingFunctions",
    functions.map(([name, required]) => [name, required]),
  );
  return formState;
}

describe("seedMappingRow", () => {
  it("writes each column into its key, in the row's own group", () => {
    const formState = withColumns([]);
    formState.resizeFormState(3);
    seedMappingRow(
      context(formState),
      row("member:dob", "0", ["dob", "to_date", "%Y", "1900-01-01", "bad date"]),
      2,
    );
    expect(formState.snapshot(2)).toEqual({
      data_property: "member:dob",
      data_property_label: "member:dob",
      input_column: "dob",
      function_name: "to_date",
      argument: "%Y",
      default_value: "1900-01-01",
      error_message: "bad date",
    });
  });

  it("marks a required property and puts the asterisk on its label", () => {
    const formState = withColumns([]);
    seedMappingRow(context(formState), row("claim:id", "1"), 0);
    expect(formState.getValue(0, "flag.is_required")).toBe("1");
    expect(formState.getValue(0, "data_property_label")).toBe("claim:id*");
  });

  it("writes no flag at all when the property is optional", () => {
    // The Dart sets the key only when required, and the validator asks whether it
    // is "1" — so an explicit "0" would be a value nothing reads.
    const formState = withColumns([]);
    seedMappingRow(context(formState), row("claim:id", "0"), 0);
    expect(formState.getValue(0, "flag.is_required")).toBeUndefined();
  });

  it("defaults the input column to the data property when the table has one", () => {
    // The third term of the coalesce, and the reason this is an escape: a
    // membership test against another query's result.
    const formState = withColumns(["member_id", "claim:id"]);
    seedMappingRow(context(formState), row("claim:id", "0"), 0);
    expect(formState.getValue(0, "input_column")).toBe("claim:id");
  });

  it("leaves the input column unset when the table has no such column", () => {
    const formState = withColumns(["member_id"]);
    seedMappingRow(context(formState), row("claim:id", "0"), 0);
    expect(formState.getValue(0, "input_column")).toBeUndefined();
  });

  it("prefers the saved mapping over both defaults", () => {
    const formState = withColumns(["claim:id", "clm_id"]);
    seedMappingRow(context(formState), row("claim:id", "0", ["clm_id"]), 0);
    expect(formState.getValue(0, "input_column")).toBe("clm_id");
  });

  it("prefers the model's default column over the data property", () => {
    const formState = withColumns(["claim:id", "declared_default"]);
    const withColumnDefault = row("claim:id", "0");
    withColumnDefault[7] = "declared_default";
    seedMappingRow(context(formState), withColumnDefault, 0);
    expect(formState.getValue(0, "input_column")).toBe("declared_default");
  });

  it("seeds the group it is given, not the context's", () => {
    // `context.group` is the *form's* group, 0 for every flow in the corpus; the
    // row's group is the index. Getting this backwards would write every row into
    // group 0 and show one row n times.
    const formState = withColumns([]);
    seedMappingRow(context(formState, 0), row("a", "0"), 0);
    formState.resizeFormState(3);
    seedMappingRow(context(formState, 0), row("b", "0"), 1);
    seedMappingRow(context(formState, 0), row("c", "0"), 2);
    expect([0, 1, 2].map((g) => formState.getValue(g, "data_property"))).toEqual(["a", "b", "c"]);
  });
});

describe("mappingFormValidator — input_column", () => {
  it("accepts a column the staging table has", () => {
    const formState = withColumns(["member_id"]);
    expect(mappingFormValidator(context(formState), "input_column", "member_id")).toBeNull();
  });

  it("refuses a column it does not, which is what the typeahead cannot do", () => {
    const formState = withColumns(["member_id"]);
    expect(mappingFormValidator(context(formState), "input_column", "membre_id")).toBe(
      "Input Column is not valid.",
    );
  });

  it("accepts an empty mapping for an optional property", () => {
    const formState = withColumns(["member_id"]);
    expect(mappingFormValidator(context(formState), "input_column", null)).toBeNull();
  });

  it("refuses an empty mapping for a required property", () => {
    const formState = withColumns(["member_id"]);
    formState.setValue(0, "flag.is_required", "1");
    expect(mappingFormValidator(context(formState), "input_column", "")).toBe(
      "Input Column must be selected or either a default or an error message must be provided.",
    );
  });

  it("accepts a required property with a default value instead", () => {
    const formState = withColumns(["member_id"]);
    formState.setValue(0, "flag.is_required", "1");
    formState.setValue(0, "default_value", "unknown");
    expect(mappingFormValidator(context(formState), "input_column", "")).toBeNull();
  });

  it("accepts a required property with an error message instead", () => {
    const formState = withColumns(["member_id"]);
    formState.setValue(0, "flag.is_required", "1");
    formState.setValue(0, "error_message", "member id is missing");
    expect(mappingFormValidator(context(formState), "input_column", "")).toBeNull();
  });
});

describe("mappingFormValidator — the cleansing function and its argument", () => {
  const functions: [string, string][] = [
    ["to_upper", "0"],
    ["to_date", "1"],
  ];

  it("never rejects the function itself", () => {
    expect(mappingFormValidator(context(withColumns([], functions)), "function_name", "x")).toBeNull();
  });

  it("refuses an argument with no function to take it", () => {
    const formState = withColumns([], functions);
    expect(mappingFormValidator(context(formState), "argument", "%Y")).toBe(
      "Remove the argument when no function is selected",
    );
  });

  it("accepts no argument when no function is selected", () => {
    const formState = withColumns([], functions);
    expect(mappingFormValidator(context(formState), "argument", null)).toBeNull();
  });

  it("requires the argument the registry says is required", () => {
    const formState = withColumns([], functions);
    formState.setValue(0, "function_name", "to_date");
    expect(mappingFormValidator(context(formState), "argument", "")).toBe(
      "Cleansing function argument is required",
    );
    expect(mappingFormValidator(context(formState), "argument", "%Y")).toBeNull();
  });

  it("does not require one the registry says is optional", () => {
    const formState = withColumns([], functions);
    formState.setValue(0, "function_name", "to_upper");
    expect(mappingFormValidator(context(formState), "argument", "")).toBeNull();
  });

  it("passes while the registry query is still in flight", () => {
    // The Dart asserts and then calls `firstWhere`, which throws in release
    // (I-47). Treating an absent row as "not required" keeps the form usable
    // rather than blank.
    const formState = new FormState();
    formState.setValue(0, "function_name", "to_date");
    expect(mappingFormValidator(context(formState), "argument", "")).toBeNull();
  });
});

describe("mappingFormValidator — default value and error message", () => {
  it("refuses both at once", () => {
    const formState = withColumns([]);
    formState.setValue(0, "error_message", "missing");
    expect(mappingFormValidator(context(formState), "default_value", "unknown")).toBe(
      "Cannot specify both a default value and an error message",
    );
  });

  it("accepts either alone, and neither", () => {
    const formState = withColumns([]);
    expect(mappingFormValidator(context(formState), "default_value", "unknown")).toBeNull();
    expect(mappingFormValidator(context(formState), "default_value", null)).toBeNull();
    formState.setValue(0, "error_message", "missing");
    expect(mappingFormValidator(context(formState), "default_value", null)).toBeNull();
  });

  it("never rejects the error message itself", () => {
    expect(mappingFormValidator(context(withColumns([])), "error_message", "x")).toBeNull();
  });
});

describe("the seven cases that are not ported", () => {
  it("passes a key no branch names, as the Dart does", () => {
    // `client`, `org`, `object_type`, `source_type`, `entity_rdf_type`,
    // `lookback_periods` and `table_name` are the "Add/Update Process Input"
    // dialog's, and **no form in `jetsclient` declares a field with any of those
    // keys** — a form validator is only ever called with its own form's keys, and
    // `mappingFormValidator` has exactly one consumer. Asserting the fall-through
    // is what keeps the omission a decision rather than a hole (I-73).
    const formState = withColumns([]);
    for (const key of ["client", "org", "object_type", "source_type", "entity_rdf_type"]) {
      expect(mappingFormValidator(context(formState), key, "")).toBeNull();
    }
  });
});

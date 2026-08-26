/**
 * The form document's field kinds. Task I.2a.
 *
 * The negative suite (S.6) covers what the schema must *reject*; this covers
 * what the `dropdown` variant must accept and what the rest of the form
 * machinery must then do with it. Both halves are needed: a variant that parses
 * but that `valueFieldsOf` does not return is a field no rule can reach, and
 * nothing in the schema would say so.
 */

import { describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import {
  FieldSchema,
  FormDocumentSchema,
  FormSchema,
  fieldsOf,
  itemSourcesOf,
  valueFieldsOf,
  type Form,
} from "./form";
import { isFormValid, validateForm } from "./validateForm";

/** The shape the agentic project's template projection emits (I-43). */
const variantChoice = {
  field: "dropdown",
  key: "columnAggregationType",
  label: "Aggregation",
  items: [
    { value: "", label: "Select an aggregation" },
    { value: "sum", label: "Sum" },
    { value: "count", label: "Count" },
  ],
} as const;

/** One of ours: `pcAutomationUF`'s `source_period_type`, corpus-shaped. */
const periodType = {
  field: "dropdown",
  key: "source_period_type",
  label: "Execution frequency",
  defaultItemPos: 1,
  items: [
    { value: "", label: "Select execution frequency" },
    { value: "monthly", label: "Monthly" },
    { value: "weekly", label: "Weekly" },
    { value: "daily", label: "Daily" },
  ],
} as const;

describe("the dropdown field kind", () => {
  it("accepts a closed list of literals, with no query anywhere in it", () => {
    const parsed = FieldSchema.parse(variantChoice);
    expect(parsed).toEqual(variantChoice);
  });

  it("accepts the two options our own flows use and the requester's does not", () => {
    // 4 of the eleven `FormDropdownFieldConfig` instances set `isReadOnly` and 2
    // set `defaultItemPos` (`datatable/fixtures/form_fields.json`). They are
    // optional because their consumer will not send them, not absent because it
    // will not (I-43).
    expect(FieldSchema.parse(periodType)).toEqual(periodType);
    expect(FieldSchema.parse({ ...variantChoice, isReadOnly: true })).toMatchObject({
      isReadOnly: true,
    });
  });

  it("takes an item list the widget can be handed unchanged", () => {
    // `DropdownItem` at `widgets/Dropdown.tsx:33` is `{value, label}`; the field
    // is the prop. If these ever diverge, a renderer has to map between them.
    const parsed = FieldSchema.parse(variantChoice);
    expect(parsed).toHaveProperty("items");
    const items = (parsed as { items: { value: string; label: string }[] }).items;
    for (const item of items) {
      expect(Object.keys(item).sort()).toEqual(["label", "value"]);
    }
  });

  it("allows an empty value but never an empty label", () => {
    // The prompt item — nine of the eleven open with a "Select a…" entry, whose
    // Dart value is null (`models/form_config.dart:326`). An unlabelled choice
    // is unpickable, so `label` keeps its `min(1)`.
    expect(FieldSchema.safeParse(variantChoice).success).toBe(true);
    expect(
      FieldSchema.safeParse({
        ...variantChoice,
        items: [{ value: "sum", label: "" }],
      }).success,
    ).toBe(false);
  });

  it("rejects an empty list, a missing list, and an item without a value", () => {
    const { items: _items, ...noItems } = variantChoice;
    expect(FieldSchema.safeParse(noItems).success).toBe(false);
    expect(FieldSchema.safeParse({ ...variantChoice, items: [] }).success).toBe(false);
    expect(
      FieldSchema.safeParse({ ...variantChoice, items: [{ label: "Sum" }] }).success,
    ).toBe(false);
  });

  it("rejects I-11's query options, which are not this field's business", () => {
    // The named query is form-level machinery and is I.2b. A document that
    // spelled it here would be accepted and ignored, which is the failure the
    // closed objects exist to prevent.
    for (const extra of [
      { dropdownItemsQuery: "clientQuery" },
      { returnedModelCacheKey: "cache.client" },
      { stateKeyPredicates: ["client"] },
    ]) {
      expect(FieldSchema.safeParse({ ...variantChoice, ...extra }).success).toBe(false);
    }
  });

  it("rejects a negative defaultItemPos and accepts one past the end", () => {
    // Bounded below and not above, on purpose: an index into a sibling key is a
    // cross-field rule and `form.ts` may only use constructs that emit. Out of
    // range degrades — `items[pos]?.value` is `undefined` and nothing is seeded.
    expect(FieldSchema.safeParse({ ...variantChoice, defaultItemPos: -1 }).success).toBe(false);
    expect(FieldSchema.safeParse({ ...variantChoice, defaultItemPos: 1.5 }).success).toBe(false);
    expect(FieldSchema.safeParse({ ...variantChoice, defaultItemPos: 99 }).success).toBe(true);
  });
});

describe("a dropdown inside a form", () => {
  const form: Form = FormSchema.parse({
    title: "Aggregation",
    rows: [
      [
        {
          ...variantChoice,
          rules: [{ rule: "required", message: "Please choose an aggregation." }],
        },
      ],
      [{ field: "label", text: "The variant decides which holes follow." }, { field: "spacer" }],
    ],
    actions: [{ action: "ufNext", label: "Next", enableOnlyWhenFormValid: true }],
  });

  it("is a value field, so rules reach it", () => {
    expect(fieldsOf(form)).toHaveLength(3);
    expect(valueFieldsOf(form).map((f) => f.key)).toEqual(["columnAggregationType"]);
  });

  it("fails `required` on the prompt item, which is what makes the prompt work", () => {
    // The prompt item's value is `""`, and `required` already means "not null
    // and not empty" — so "the author must choose" needs no rule of its own.
    const formState = new FormState();
    formState.setValue(0, "columnAggregationType", "");
    expect(validateForm(form, formState, 0)).toEqual([
      { key: "columnAggregationType", message: "Please choose an aggregation.", group: 0 },
    ]);

    formState.setValue(0, "columnAggregationType", "sum");
    expect(isFormValid(form, formState, 0)).toBe(true);
  });

  it("fails `required` when the field was never touched at all", () => {
    expect(isFormValid(form, new FormState(), 0)).toBe(false);
  });
});

/**
 * **`extendsKey`, the third rule and the first that reads a second field.** Task
 * C.3b.
 *
 * The Dart it ports is `workspaceIDEFormValidator`'s `wsDbSourceFileName` arm
 * (`jetsclient/lib/modules/workspace_ide/screen_delegates.dart`,
 * `workspaceIDEFormValidator`), which has four outcomes and this rule reproduces
 * three of them: the prefix key is unset, the value does not extend the prefix,
 * and the value extends it. The fourth — the Dart's separate *"must be entered"*
 * message for a value shorter than the prefix — is the same failure as the
 * second, and the Dart's own two messages differ only in wording.
 */
describe("the extendsKey rule", () => {
  const form: Form = FormSchema.parse({
    title: "Add Workspace File",
    rows: [
      [
        {
          field: "text",
          key: "source_file_name",
          label: "File Name",
          rules: [
            {
              rule: "extendsKey",
              key: "workspace.section",
              message: "File name must be entered, preserving the directory prefix.",
            },
          ],
        },
      ],
    ],
    actions: [{ action: "ok", label: "Add File" }],
  });
  const message = "File name must be entered, preserving the directory prefix.";

  const check = (prefix: string | undefined, value: string | undefined) => {
    const formState = new FormState();
    if (prefix !== undefined) formState.setValue(0, "workspace.section", prefix);
    if (value !== undefined) formState.setValue(0, "source_file_name", value);
    return validateForm(form, formState, 0);
  };

  it("passes a value that extends the prefix", () => {
    expect(check("data_model/", "data_model/claim.jr")).toEqual([]);
  });

  it("fails a value that leaves the section", () => {
    expect(check("data_model/", "jet_rules/claim.jr")).toEqual([
      { key: "source_file_name", message, group: 0 },
    ]);
  });

  it("fails the bare prefix, which is a directory rather than a file", () => {
    // **The half `startsWith` alone would accept**, and the reason this is one
    // rule rather than two: the Dart guards its `startsWith` with
    // `value.characters.length > wsSection.characters.length`.
    expect(check("data_model/", "data_model/")).toEqual([
      { key: "source_file_name", message, group: 0 },
    ]);
  });

  it("fails an empty value and an untouched field alike", () => {
    expect(check("data_model/", "")).toEqual([{ key: "source_file_name", message, group: 0 }]);
    expect(check("data_model/", undefined)).toEqual([
      { key: "source_file_name", message, group: 0 },
    ]);
  });

  it("names the key when the prefix itself is missing, rather than passing", () => {
    // A document defect, not a user error. Treating an absent prefix as `""`
    // would degrade this rule into `required` — `"anything"` extends `""` — and
    // the form would look validated. The Dart says *"Invalid configuration"*
    // here; this says which key is empty, which is the same information and more
    // of it.
    const errors = check(undefined, "somewhere/new.jr");
    expect(errors).toHaveLength(1);
    expect(errors[0]!.message).toBe(`${message} (no prefix: "workspace.section" is not set)`);
    expect(check("", "somewhere/new.jr")[0]!.message).toBe(errors[0]!.message);
  });
});

describe("query-backed item sources", () => {
  /** `fmMappingFormUF`, reduced to the two fields that take items from a query. */
  const mapping: Form = FormSchema.parse({
    title: "File Mapping Worksheet",
    queries: {
      inputColumns: {
        sql: "SELECT column_name FROM information_schema.columns WHERE table_name = '{table_name}'",
        params: ["table_name"],
      },
      mappingFunctions: { sql: "SELECT function_name FROM jetsapi.mapping_function_registry" },
    },
    rows: [
      [
        {
          field: "typeahead",
          key: "input_column",
          label: "Input Column",
          hint: "Input File Column Name",
          itemsFrom: "inputColumns",
          priorityKey: "data_property",
          maxLength: 120,
          rules: [{ rule: "required", message: "Input Column must be selected." }],
        },
        {
          field: "dropdown",
          key: "function_name",
          label: "Cleansing function",
          items: [{ value: "", label: "Select cleansing function" }],
          itemsFrom: "mappingFunctions",
        },
      ],
    ],
    actions: [{ action: "mapperOk", label: "Save", enableOnlyWhenFormValid: true }],
  });

  it("makes a typeahead a value field, so rules reach it", () => {
    expect(valueFieldsOf(mapping).map((f) => f.key)).toEqual(["input_column", "function_name"]);
  });

  it("validates a typeahead like any other value field", () => {
    const formState = new FormState();
    expect(validateForm(mapping, formState, 0)).toEqual([
      // The group travels with the error: a repeating form has one error list
      // across n groups, and the renderer has to know which row to mark (F.1).
      { key: "input_column", message: "Input Column must be selected.", group: 0 },
    ]);
    formState.setValue(0, "input_column", "member_id");
    expect(isFormValid(mapping, formState, 0)).toBe(true);
  });

  it("names every item source with the field that names it", () => {
    expect(itemSourcesOf(mapping)).toEqual([
      { key: "input_column", query: "inputColumns" },
      { key: "function_name", query: "mappingFunctions" },
    ]);
  });

  it("reports nothing for a form whose dropdown carries only literals", () => {
    // I.2a's variant is untouched by I.2b: `itemsFrom` is optional on a dropdown
    // and absent is the static case.
    const literal: Form = FormSchema.parse({
      rows: [[variantChoice]],
      actions: [{ action: "ufNext", label: "Next" }],
    });
    expect(itemSourcesOf(literal)).toEqual([]);
  });

  it("refuses a typeahead with no item source", () => {
    // A typeahead with nothing to suggest is a text field, and a document that
    // means a text field should say `text`.
    const { itemsFrom, ...rest } = {
      field: "typeahead",
      key: "input_column",
      label: "Input Column",
      itemsFrom: "inputColumns",
    };
    void itemsFrom;
    expect(FieldSchema.safeParse(rest).success).toBe(false);
  });

  it("refuses the Dart's own spelling of the same idea", () => {
    // `typeaheadMenuItemCacheKey` names a form-state cache key and
    // `dropdownItemsQuery` carries SQL on the field; the document has one
    // construct where the Dart has three, so all three are invented fields here.
    for (const invented of [
      { field: "typeahead", key: "c", label: "C", itemsFrom: "q", typeaheadMenuItemCacheKey: "k" },
      { field: "typeahead", key: "c", label: "C", itemsFrom: "q", priorityTargetKey: "d" },
      { field: "dropdown", key: "c", label: "C", items: [{ value: "a", label: "A" }], dropdownItemsQuery: "SELECT 1" },
    ]) {
      expect(FieldSchema.safeParse(invented).success).toBe(false);
    }
  });

  it("requires a statement and refuses an empty params list", () => {
    const base = { schemaVersion: 1, forms: { f: { rows: [[{ field: "spacer" }]], actions: [{ action: "ufCompleted", label: "Done" }] } } };
    const withQueries = (queries: unknown) => ({
      ...base,
      forms: { f: { ...base.forms.f, queries } },
    });
    expect(FormDocumentSchema.safeParse(withQueries({ q: { params: ["a"] } })).success).toBe(false);
    expect(FormDocumentSchema.safeParse(withQueries({ q: { sql: "SELECT 1", params: [] } })).success).toBe(false);
    expect(FormDocumentSchema.safeParse(withQueries({ q: { sql: "SELECT 1" } })).success).toBe(true);
  });
});

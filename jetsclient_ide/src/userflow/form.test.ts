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
import { FieldSchema, FormSchema, fieldsOf, valueFieldsOf, type Form } from "./form";
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
      { key: "columnAggregationType", message: "Please choose an aggregation." },
    ]);

    formState.setValue(0, "columnAggregationType", "sum");
    expect(isFormValid(form, formState, 0)).toBe(true);
  });

  it("fails `required` when the field was never touched at all", () => {
    expect(isFormValid(form, new FormState(), 0)).toBe(false);
  });
});

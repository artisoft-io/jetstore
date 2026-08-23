/**
 * The form document. Task S.5, and the answer to I-15.
 *
 * ## The decision, made where I-15 said to make it
 *
 * I-15 recorded that S.1's `formConfig` is a *reference* to a compiled screen,
 * so Phase 2 would make the flow graph authorable and leave the forms compiled —
 * half of R2. It recommended deciding at S.5, with the proof flows in hand,
 * rather than widening Phase 2 speculatively.
 *
 * **With the proof flows in hand, the form belongs in Phase 2, and the evidence
 * is how little it turns out to be.** The three forms the two flows use:
 *
 * | Form | Contents |
 * |---|---|
 * | `rfkSubmitSchemaEvent` | 2 text inputs, 1 label, 3 spacers, 2 actions |
 * | `lfSelectSourceConfigUF` | 1 data table, the 3 standard actions |
 * | `lfSelectFileKeysUF` | 1 data table, 3 actions (one relabelled) |
 *
 * Four field kinds and two validation rules. The assessment's §4.1 caveat feared
 * "a JSON representation of `FormConfig` and its eight field subclasses"; what a
 * flow actually needs is the three widgets track A already built, plus the two
 * things that are not widgets — a label and a spacer.
 *
 * ## What this deliberately does not cover
 *
 * **Validators beyond `required` and `json`.** Those two cover both proof flows
 * exactly: `registerFileKeyFormValidator` checks two fields are non-empty and
 * that one parses as JSON; `loadFilesFormValidator` checks two tables have a
 * selection. `file_mapping`'s 222-line validator is unchanged as an escape, and
 * a form needing more than these two names one.
 *
 * **Dropdown item *queries*** — I-11, still open, and now I.2b. Neither proof
 * flow uses a dropdown, which is why S.5 could be reached without settling it.
 * The *static* half arrived later and is here — see `dropdown` below.
 *
 * ## Why a sibling document rather than fields on the state
 *
 * The corpus says 46 states have 46 distinct forms, one to one, which argues the
 * form belongs *inside* the state. It is a sibling anyway, `<key>.form.json`,
 * for two reasons that outweigh that today: the actions document already
 * established the pattern for exactly this shape of thing
 * (`sizing_action_grammar.md` R5), and inlining would rewrite all eleven
 * `.uf.json` documents and the coverage fixture to prove a point about two of
 * them. **Inline is the better end state and this is the migration** — recorded
 * rather than lost.
 */

import { z } from "zod";

import { Identifier } from "./schema";

/**
 * A validation rule. Two, because two is what the corpus needs.
 *
 * `required` is "not null and not empty", which is what both Dart validators
 * mean by it — a text field holding `""` and a table with no selection both
 * fail.
 */
export const RuleSchema = z
  .union([
    z.strictObject({ rule: z.literal("required"), message: z.string().min(1) }),
    z.strictObject({ rule: z.literal("json"), message: z.string().min(1) }),
  ])
  .meta({ id: "Rule", description: "A field-level validation rule" });

/**
 * One choice offered by a `dropdown`.
 *
 * Mirrors `DropdownItem` (`widgets/Dropdown.tsx:33`) exactly, because the field
 * is the widget's prop: a document that parses is a list the widget takes.
 *
 * **`value` may be empty, and that is the prompt item.** The Dart's
 * `DropdownItemConfig.value` is nullable (`models/form_config.dart:326`) and
 * **nine of the eleven** shipping dropdowns open with a "Select a…" entry
 * declared with a label and no value at all —
 * `DropdownItemConfig(label: 'Select a Client')`,
 * `user_flows/pipeline_config/form_config.dart:40`. It stands for *nothing
 * chosen*. Here that is `""`, which is the same thing `validateForm`'s
 * `required` rule already treats as empty — so a prompt item and a `required`
 * rule compose into "the author must choose" without a rule of their own.
 * `label` is not optional in the same way: an unlabelled choice is unpickable.
 */
export const DropdownItemSchema = z
  .strictObject({
    value: z.string(),
    label: z.string().min(1),
  })
  .meta({ id: "DropdownItem", description: "One choice offered by a dropdown" });

/**
 * One field.
 *
 * `text`, `dataTable` and `dropdown` are widgets; `label` and `spacer` are not,
 * and they are here because a form is a layout as well as a set of inputs — the
 * 55 `PaddingConfig` and 12 `TextFieldConfig` instances across the flows (I-12)
 * are the second largest thing in a form after the inputs themselves.
 */
export const FieldSchema = z
  .union([
    z.strictObject({
      field: z.literal("text"),
      key: Identifier,
      label: z.string().min(1),
      hint: z.string().optional(),
      maxLines: z.number().int().positive().optional(),
      maxLength: z.number().int().positive().optional(),
      autofocus: z.boolean().optional(),
      rules: z.array(RuleSchema).optional(),
    }),
    z.strictObject({
      field: z.literal("dataTable"),
      key: Identifier,
      /** The table configuration key; A.4's widget resolves it. */
      table: Identifier,
      rules: z.array(RuleSchema).optional(),
    }),
    /**
     * A closed list, chosen from literals. Task I.2a, requested by the agentic
     * project (I-43).
     *
     * **The widget is not the gap and never was.** A.3 built `Dropdown.tsx` to
     * take `items` as a prop precisely so "a static list and a query result
     * reach it the same way" — so what was missing was a way to *author* the
     * static list, which is this variant and nothing else. I-11's machinery —
     * the named query, `returnedModelCacheKey`, `stateKeyPredicates` — is
     * untouched, because it is form-level and this is a field-level literal.
     *
     * `defaultItemPos` and `isReadOnly` are here for our own flows rather than
     * for the requester's: the form-field corpus puts `isReadOnly` on 4 of the
     * eleven `FormDropdownFieldConfig` instances and `defaultItemPos` on 2
     * (`datatable/fixtures/form_fields.json`). The projection that asked for
     * this variant will use neither, which settles what is on *its* critical
     * path and not what the schema owes.
     *
     * **`defaultItemPos` is bounded below and not above**, deliberately. An
     * index past the end is a cross-field rule, and this file may only use
     * constructs that emit (`schema.ts`); a `.refine()` would be enforced in the
     * browser and silently absent in Go, which is the worst of the three
     * options. Out of range degrades rather than throws — `items[pos]?.value` at
     * `Dropdown.tsx:65` is `undefined` and the field simply seeds nothing.
     */
    z.strictObject({
      field: z.literal("dropdown"),
      key: Identifier,
      label: z.string().min(1),
      items: z.array(DropdownItemSchema).min(1),
      /** Index into `items`, selected when the form state holds nothing. */
      defaultItemPos: z.number().int().nonnegative().optional(),
      isReadOnly: z.boolean().optional(),
      rules: z.array(RuleSchema).optional(),
    }),
    z.strictObject({ field: z.literal("label"), text: z.string().min(1) }),
    z.strictObject({ field: z.literal("spacer") }),
  ])
  .meta({ id: "Field", description: "One field of a form" });

/**
 * A form's action button.
 *
 * `action` is the name dispatched — either one of the six the flow engine
 * handles itself (`ufNext`, `ufPrevious`, `ufCancel`, `ufCompleted`,
 * `ufContinueLater`, `ufStartFlow`) or a name in the flow's action document.
 *
 * `enableOnlyWhenFormValid` is form-wide, because the Dart says so in as many
 * words at `models/form_config.dart:535`. Reading it per field would be a
 * behaviour change wearing a tidy-up.
 */
export const FormActionSchema = z
  .strictObject({
    action: Identifier,
    label: z.string().min(1),
    style: z.enum(["primary", "secondary", "danger"]).optional(),
    capability: Identifier.optional(),
    enableOnlyWhenFormValid: z.boolean().optional(),
  })
  .meta({ id: "FormAction" });

export const FormSchema = z
  .strictObject({
    title: z.string().min(1).optional(),
    /** Rows, rendered in order; a row is a horizontal group. */
    rows: z.array(z.array(FieldSchema).min(1)).min(1),
    actions: z.array(FormActionSchema).min(1),
  })
  .meta({ id: "Form", description: "One screen of a user flow" });

export const FormDocumentSchema = z
  .strictObject({
    schemaVersion: z.literal(1),
    forms: z.record(Identifier, FormSchema),
  })
  .meta({
    id: "FormDocument",
    title: "JetStore UserFlow form document",
    description: "The forms of one user flow, authored as data",
  });

export type Rule = z.infer<typeof RuleSchema>;
export type DropdownItem = z.infer<typeof DropdownItemSchema>;
export type Field = z.infer<typeof FieldSchema>;
export type FormAction = z.infer<typeof FormActionSchema>;
export type Form = z.infer<typeof FormSchema>;
export type FormDocument = z.infer<typeof FormDocumentSchema>;

export function emitJsonSchema(): unknown {
  return z.toJSONSchema(FormDocumentSchema, { io: "input" });
}

/** Every field in a form, flattened out of its rows. */
export function fieldsOf(form: Form): Field[] {
  return form.rows.flat();
}

/** A field that holds a value, and can therefore carry rules. */
export type ValueField = Extract<
  Field,
  { field: "text" } | { field: "dataTable" } | { field: "dropdown" }
>;

/** Fields that hold a value and can therefore be validated. */
export function valueFieldsOf(form: Form): ValueField[] {
  return fieldsOf(form).filter(
    (f): f is ValueField =>
      f.field === "text" || f.field === "dataTable" || f.field === "dropdown",
  );
}

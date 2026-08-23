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
 * A named query the form runs to fill its item sources. Task I.2b.
 *
 * ## The query is named on the form, and in the Dart that is true of one of the
 * two mechanisms rather than of both
 *
 * I-11 and I-43 both state it as a fact about `jetsclient`, and **half of it is
 * a fact about the port instead** (I-70). The Dart has two item-source paths and
 * they disagree about where the SQL lives:
 *
 * | Dart | Where the SQL is | How it runs |
 * |---|---|---|
 * | `FormDropdownFieldConfig.dropdownItemsQuery` | **on the field** (`models/form_config.dart:348`) | `raw_query`, from the widget (`components/dropdown_form_field.dart`, `queryDropdownItems`) |
 * | `FormConfig.dropdownItemsQueries` / `typeaheadItemsQueries` | **on the form** (`models/form_config.dart:129`–`:130`) | `raw_query_map`, once, from the form (`components/form.dart`, `queryInputFieldItems`) |
 *
 * Five of the eleven dropdowns take the first path and two fields of
 * `fmMappingFormUF` take the second. **This schema has one**: a query is declared
 * here, on the form, and a field names it. That collapses two Dart classes and
 * two request shapes into one construct, which is I-60's observation — *"the two
 * exotic types are one widget and one item source"* — carried into the document.
 *
 * ## `params` is `stateKeyPredicates`, and it is both of them
 *
 * The Dart spells the same idea twice: `FormConfig.stateKeyPredicates` substitutes
 * form-state values into every query in the map, and
 * `FormDropdownFieldConfig.stateKeyPredicates` substitutes into that one field's
 * query *and* re-runs it when the value changes. Here a query declares the keys
 * it reads and both behaviours follow from that: a query whose parameters are not
 * all present does not run, and one whose parameter changes runs again
 * (`formQueries.ts`).
 *
 * ## What is deliberately not carried across
 *
 * `returnedModelCacheKey` (2 of 11) named a second place to find the rows. Here
 * the query's own name addresses them — `FormState.queryRows` — so a second key
 * would be a second name for one thing, which is the cut I-52 made thirteen times
 * over on the table document.
 *
 * `whereStateContains` is declared by the Dart and set by **no** field in any
 * flow (`widgets/Dropdown.tsx`, from the form-field corpus), so it is absent for
 * the reason `apiAction` is absent from `.tc.json`: a construct with no use is a
 * surface with no test.
 */
export const FormQuerySchema = z
  .strictObject({
    /**
     * The statement, run by `raw_query_map` against the deployment's database.
     *
     * **Raw SQL in an authored document is a capability question and it was
     * asked** (I-71). Saving a workspace file requires `workspace_ide`
     * (`jets/datatable/workspace_data_table_action.go`, `SaveWorkspaceFileContent`),
     * and `workspace_ide` *is* `CapabilityQueryTool`
     * (`jets/datatable/data_table_action.go`, `CapabilityQueryTool`) — the
     * capability the free-SQL query tool already requires. So an author who can
     * write this file can already run the same statement through
     * `raw_query_tool`, and the document grants them nothing new. That is the
     * opposite conclusion to I.3a's on `apiAction`, and for a stated reason: that
     * field reached a *write* switch, and this one reaches the read path the
     * author is already trusted with.
     */
    sql: z.string().min(1),
    /**
     * Form-state keys substituted into `sql` as `{key}`, read from group 0.
     *
     * Group 0 rather than the field's group, matching `form.dart`'s
     * `getValue(0, stateKey)`: a query is the form's and a repeating row's group
     * is not a thing it can see.
     */
    params: z.array(Identifier).min(1).optional(),
  })
  .meta({ id: "FormQuery", description: "A named query filling a form's item sources" });

/**
 * One field.
 *
 * `text`, `dataTable`, `dropdown` and `typeahead` are widgets; `label` and
 * `spacer` are not, and they are here because a form is a layout as well as a set
 * of inputs — the 55 `PaddingConfig` and 12 `TextFieldConfig` instances across the
 * flows (I-12) are the second largest thing in a form after the inputs themselves.
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
      /**
       * A `queries` entry whose rows are appended to `items`. Task I.2b.
       *
       * **Appended, not substituted**, because that is what the Dart does with
       * both of its paths: `setDropdownItems` starts from `_config.items` and adds
       * the model, and `form.dart`'s cache builder puts a "Select…" entry in front
       * of the rows. So the literal list keeps carrying the prompt item — which is
       * why `items` stays required with a minimum of one — and the query supplies
       * the choices.
       *
       * Column 0 of each row is the value **and** the label, as both Dart paths
       * do (`DropdownItemConfig(label: e[0]!, value: e[0]!)`). A query selecting a
       * second column is not thereby offering it: `process_name, key` is read for
       * its second column by an action, through the rows rather than through the
       * dropdown.
       */
      itemsFrom: Identifier.optional(),
      /** Index into `items`, selected when the form state holds nothing. */
      defaultItemPos: z.number().int().nonnegative().optional(),
      isReadOnly: z.boolean().optional(),
      rules: z.array(RuleSchema).optional(),
    }),
    /**
     * A text box with suggestions. Task I.2b, and the one widget F.0b found
     * missing (F21, I-60).
     *
     * **It is not a dropdown and the difference is the point.** The value is
     * whatever the user typed — the Dart's `onChanged` writes it through on every
     * keystroke (`components/typeahead_form_field.dart`, `JetsTypeaheadFormField`)
     * — and the suggestion list only helps them type it. `mapFileUF`'s validator
     * is what rejects a value that is not a column of the staging table, which is
     * why membership is a rule rather than a constraint of the control.
     *
     * `itemsFrom` is required here and optional on `dropdown`: a typeahead with no
     * suggestions is a text field, and a document that means a text field should
     * say `text`.
     */
    z.strictObject({
      field: z.literal("typeahead"),
      key: Identifier,
      label: z.string().min(1),
      hint: z.string().optional(),
      /** The `queries` entry whose column 0 supplies the suggestions. */
      itemsFrom: Identifier,
      /**
       * A sibling key whose value floats the suggestions that resemble it.
       *
       * `priorityTargetKey` (`models/form_config.dart:442`). The Dart splits the
       * target on `:` and `_`, lowercases, and puts every suggestion containing
       * any part first — so mapping the `member:dob` data property offers the
       * columns with `member` or `dob` in them before the other four hundred. It
       * is ordering only: nothing is hidden, which is what makes it safe to leave
       * unset.
       */
      priorityKey: Identifier.optional(),
      maxLength: z.number().int().positive().optional(),
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
    /**
     * Named queries this form runs when it loads. Task I.2b.
     *
     * A field names one with `itemsFrom`; nothing else reads them today, and the
     * rows are addressable by query name from an escape, which is where the two
     * `metadataQueries` caches of `fmMappingFormUF` end up.
     */
    queries: z.record(Identifier, FormQuerySchema).optional(),
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
export type FormQuery = z.infer<typeof FormQuerySchema>;
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
  { field: "text" } | { field: "dataTable" } | { field: "dropdown" } | { field: "typeahead" }
>;

const VALUE_KINDS = new Set(["text", "dataTable", "dropdown", "typeahead"]);

/** Fields that hold a value and can therefore be validated. */
export function valueFieldsOf(form: Form): ValueField[] {
  return fieldsOf(form).filter((f): f is ValueField => VALUE_KINDS.has(f.field));
}

/**
 * Every `queries` entry a field names, with the field that names it.
 *
 * **The check this exists for is the one no schema can state**: `itemsFrom` is an
 * `Identifier`, and whether that identifier is a key of the *same form's*
 * `queries` is a relation between two properties of one object. Zod could say it
 * with `.refine()` and `z.toJSONSchema` would drop it, so Go would not enforce it
 * — the split I.2a called the worst of the three (`defaultItemPos`). So it is a
 * document-set check instead (`documentSet.ts`), where both languages can reach
 * it and a generator can run it without a browser.
 */
export function itemSourcesOf(form: Form): { key: string; query: string }[] {
  const sources: { key: string; query: string }[] = [];
  for (const field of fieldsOf(form)) {
    if (field.field === "typeahead") sources.push({ key: field.key, query: field.itemsFrom });
    if (field.field === "dropdown" && field.itemsFrom !== undefined) {
      sources.push({ key: field.key, query: field.itemsFrom });
    }
  }
  return sources;
}

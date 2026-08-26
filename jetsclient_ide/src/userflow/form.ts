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
 * ## What S.5 deliberately did not cover, and what has arrived since
 *
 * **Validators beyond `required` and `json`.** Those two cover both proof flows
 * exactly: `registerFileKeyFormValidator` checks two fields are non-empty and
 * that one parses as JSON; `loadFilesFormValidator` checks two tables have a
 * selection. S.5 said a form needing more than these two would name an escape,
 * and **F.1 is the first that does** — `validator` on `FormSchema`, carrying
 * `mappingFormValidator`.
 *
 * **Dropdown item *queries*** — I-11. Neither proof flow uses a dropdown, which
 * is why S.5 could be reached without settling it. The *static* half arrived with
 * I.2a and the *query* half with I.2b, which is `queries` and `itemsFrom` below.
 *
 * **A form drawn once per row of a query.** Not foreseen here at all; one form of
 * the fifty has it, and it is `repeat` below (F.1).
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
    /**
     * The inverse gate. One site: `mapperDraft`
     * (`file_mapping/form_config.dart`, `enableOnlyWhenFormNotValid: true`).
     *
     * **It is a real behaviour rather than a stylistic mirror of the flag above.**
     * "Save as Draft" is offered *because* the worksheet does not validate — a
     * half-finished mapping is worth keeping — so the two buttons are never
     * enabled at the same time, and the Save button is the one the user should
     * reach for once it lights up. `form_button.dart` reads them as two
     * independent branches of one condition.
     */
    enableOnlyWhenFormNotValid: z.boolean().optional(),
  })
  .meta({ id: "FormAction" });

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
      /**
       * The field is shown and not edited. Task F.2, and the first half of I-62.
       *
       * **23 of the 46 `FormInputFieldConfig` instances set it**
       * (`datatable/fixtures/form_fields.json`), and `TextInput.tsx` has taken
       * the prop since A.3 — so the gap was the authoring layer alone, which is
       * what I-62 said and why it is one line here.
       *
       * **The count read *20 of the 36* until F.6 and both halves were wrong**
       * (I-111). The corpus has held 46 since it was generated and
       * `TextInput.tsx`'s own option table reads 23 of 46, three files away — so
       * two documents in this repository disagreed about one generated number,
       * and the one nobody re-derived is the one that drifted. 20 is the count
       * over the forms a *state* names, whose denominator is 38 rather than 36.
       *
       * **F.1 declined it and F.2 takes it, for the reason I-62 asked for: a
       * consumer.** `mapFileUF`'s five text inputs set neither this nor
       * `defaultValue`, so building it there would have been building for
       * nobody. All **eight** text fields of F.2's five forms set it, and they
       * hold `workspace_name` and `workspace_uri` — the workspace the flow is
       * about, seeded from the route and posted back as `workspaceName`. An
       * editable copy of that is not a cosmetic divergence: it is a form on
       * which the user can retarget the pull.
       */
      isReadOnly: z.boolean().optional(),
      /**
       * A named predicate deciding whether the field is read-only. Task C.2b.
       *
       * **`isReadOnlyEval`, which is a Dart closure and therefore a name here** —
       * the move `cellFilter` and a table action's `isEnabled` already make. The
       * corpus can only report `hasIsReadOnlyEval: true`, so which predicate a
       * field wants is a fact only the Dart source has, and I-103's rule applies:
       * the mapping is data keyed by form and field, not a constant.
       *
       * **Four sites in either corpus and all four are on `/workspaces`'s
       * dialogs** — `addWorkspace`'s name, uri and branch, and
       * `doGitStatusWorkspaceDialog`'s command. Zero in the flows, which is why
       * no task before C.2b met it. **They are two bodies, not four**: the name
       * and branch fields share *is this the deployment's active workspace*, and
       * the uri and command fields share *is a workspace uri configured*.
       *
       * It resolves through `predicates`, the namespace a table action's
       * `isEnabled` already uses, rather than through a namespace of its own: the
       * signature is the same `(formState, group) => boolean` and a second
       * namespace holding functions of the same type would be a distinction
       * nothing draws.
       *
       * **It composes with `isReadOnly` as an or**, and no field sets both. A
       * name that does not resolve leaves the field **read-only**, which is the
       * safe direction and the one `actionBarModel.ts` takes for the same reason:
       * a missing predicate must not silently open something that was closed.
       */
      isReadOnlyFrom: Identifier.optional(),
      /**
       * The value the field starts with when form state holds nothing. Task F.6,
       * and the second half of I-62.
       *
       * **Four sites in the 50-form corpus and all four are
       * `pipelineConfigUF`'s** — `rule_config_json` on `pcAutomationUF` and
       * `pcSummaryUF`, both `"[]"`, and `lookback_periods` on the two
       * `pcNewProcessInputDialog` forms, both `"0"`
       * (`datatable/fixtures/form_fields.json`). I-62 left it unbuilt because it
       * had no consumer; this flow is the whole of it.
       *
       * **It is required rather than cosmetic on this flow, which is why it is
       * built here and was not before.** `pipelineConfigFormValidatorUF` refuses
       * a null `rule_config_json` with *"Please provide a value."*
       * (`pipeline_config/form_action_delegates.dart`,
       * `pipelineConfigFormValidatorUF`), and nothing else ever writes that key
       * on the add path. Without the default the user meets a blocked Next on a
       * field they were never told to fill.
       */
      defaultValue: z.string().optional(),
      /**
       * What the widget will let the user type. Task F.6.
       *
       * **Two sites, both `lookback_periods` on this flow's dialogs**, and both
       * `digitsOnly` (`datatable/fixtures/form_fields.json`). `TextInput.tsx` has
       * taken the prop and implemented all four restrictions since A.3; as with
       * `defaultValue` the gap was the authoring layer alone.
       *
       * **The enum is the Dart's `TextRestriction` in full rather than the one
       * member the corpus uses**, because `restrict` already implements four and
       * a schema that admitted one would have to be widened by whoever meets the
       * second — a change to an exported union rather than to a document.
       */
      textRestriction: z.enum(["none", "allLower", "allUpper", "digitsOnly"]).optional(),
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
       * second column is not thereby offering it.
       *
       * **`process_name, key` was cited here as an action reading the second
       * column through the rows, and F.6 authored that site and found it does
       * not.** What reads `cache.process_config` is `ruleConfigv2FormActions`
       * (`jetsclient/lib/modules/actions/config_delegates.dart`), on a screen
       * outside every flow; `pcAddPipelineConfigUF`, the arm in the flow that
       * declares the dropdown, gets the same `key` from its own `query` step
       * against `process_config`. So `returnedModelCacheKey` has two writers and
       * one reader, and the reader is nobody's flow — which is why this schema
       * needs no field for it and why `pipelineConfigUF`'s form document drops it
       * (I-118). The sentence above is still the rule; the example was the wrong
       * one.
       */
      itemsFrom: Identifier.optional(),
      /** Index into `items`, selected when the form state holds nothing. */
      defaultItemPos: z.number().int().nonnegative().optional(),
      isReadOnly: z.boolean().optional(),
      /** As on `text` — no corpus site, and the two kinds share the widget prop. */
      isReadOnlyFrom: Identifier.optional(),
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
    /**
     * A line of text. `TextFieldConfig`, 12 instances across the flows (I-12).
     *
     * **`fromKey` is the repeating-row case and it is why this is a union.** A
     * form with `repeat` draws the same fields once per row, so a label that says
     * which row this is cannot be a literal: `fmMappingFormUF`'s per-row heading
     * is the data property being mapped, with a `*` when it is required
     * (`file_mapping/form_config.dart`, inside `inputFieldRowBuilder`). The seed
     * escape writes that string into the group's form state and the label reads
     * it, which keeps the *computation* in the escape and the *layout* here.
     *
     * Exactly one of the two, because a label with both would have to pick.
     */
    z.strictObject({ field: z.literal("label"), text: z.string().min(1) }),
    z.strictObject({ field: z.literal("label"), fromKey: Identifier }),
    z.strictObject({ field: z.literal("spacer") }),
    /**
     * A button *inside the rows* rather than in the action bar. Task F.2.
     *
     * **The corpus has three and this is not a layout preference in any of
     * them.** `form_fields.json` reports `FormActionConfig` at three sites —
     * `wpLoadConfigUF`'s "Load All Clients Config", and
     * `pcViewMergeProcessInputsUF` and `pcViewInjectedProcessInputsUF`'s two
     * "add" buttons, which are `pipelineConfigUF`'s and therefore F.6's. Each
     * sits beside the table it acts on, and each is an *alternative* to
     * finishing the form: the action bar below still carries Previous / Cancel /
     * Next, and the flow continues if the user ignores the button.
     *
     * **It is `FormActionSchema` extended, not a second button shape**, because
     * the Dart's is one class — `FormActionConfig` appears in `actions` and in
     * `inputFields` alike (`models/form_config.dart`). So `capability`,
     * `enableOnlyWhenFormValid` and its inverse mean here exactly what they mean
     * there, and `buttonsOf` below is what stops the document-set checks from
     * seeing only half the buttons a form offers.
     *
     * **A `button` field holds no value**, so it is not in `ValueField`, carries
     * no `rules` and cannot be validated — the same line `label` and `spacer` are
     * on.
     */
    FormActionSchema.extend({ field: z.literal("button") }),
  ])
  .meta({ id: "Field", description: "One field of a form" });

/**
 * A form drawn once per row of a query. Task F.1.
 *
 * **One form of the fifty has this shape** — `fmMappingFormUF`, the only one the
 * corpus reports with `hasRowBuilder: true` and `fieldCount: 0`
 * (`datatable/fixtures/form_fields.json`). The Dart builds it with a closure:
 * `inputFieldRowBuilder` is called once per row of `inputFieldsQuery` and returns
 * the field configurations for that row, having first written the row's values
 * into validation group *i* (`form.dart`, `queryInputFieldItems`).
 *
 * **The split here is layout from computation, and it is the whole design.** The
 * *layout* is the same for every row, so it is `rows` — declared once, drawn once
 * per group. The *seeding* is not: `input_column`'s default is
 * `saved ?? column 7 ?? (the data property, if it is a column of the staging
 * table)`, a three-term coalesce whose last term is a membership test against a
 * second query. Expressing that would take a binding vocabulary — coalesce,
 * literal mapping, membership — with exactly one consumer, and this project's
 * answer to computation in a document is a named escape rather than a language
 * for it (`plan/phase3_plan.md` §5: *declarative with a named escape*).
 *
 * So `seed` names a `rowInitializers` entry, which is handed the row and the
 * group index and writes the values. The registry refuses an unresolved name at
 * load, as it does for every other escape kind.
 *
 * **`from` names a query rather than carrying one** so that the same result feeds
 * the row count and anything else that reads it — `savedStateQuery` and
 * `inputFieldsQuery` are literally the same query in the Dart, which is why the
 * row builder's `savedState?[index][n]` and `inputFieldRow[n]` are the same
 * value and the seeding is a column map rather than a join.
 */
export const RepeatSchema = z
  .strictObject({
    /** A `queries` entry; one validation group per row it returns. */
    from: Identifier,
    /** A `rowInitializers` escape, called per row before the group is drawn. */
    seed: Identifier,
  })
  .meta({ id: "Repeat", description: "Draw this form once per row of a query" });

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
    /** Draw `rows` once per row of a query. Task F.1 — see `RepeatSchema`. */
    repeat: RepeatSchema.optional(),
    /**
     * A `validators` escape run over every value field, beside the field `rules`.
     * Task F.1, and the first of these to exist.
     *
     * **The rules cover a form whose fields are independent, and this is the one
     * that is not.** `mappingFormValidator` is 222 lines
     * (`file_mapping/form_mapping_validator.dart`, `mappingFormValidator`) and
     * every branch of it that `mapFileUF` reaches is a *relation between sibling
     * values*: an input column must be one of the staging table's columns, an
     * argument is required or forbidden according to the function chosen beside
     * it, a default value and an error message may not both be given. Four
     * bespoke rule kinds would be a rule language written for one form; a named
     * validator is the mechanism `escapes.ts` already declares and I-15 predicted
     * would be wanted.
     *
     * It runs *in addition to* `rules`, not instead of them, so a form may use
     * both — and the errors are merged by key, first one winning per field.
     */
    validator: Identifier.optional(),
    /**
     * Show validation messages as the user types, rather than when an action
     * asks. Task F.1.
     *
     * `AutovalidateMode.always` (`models/form_config.dart`, `autovalidateMode`).
     * **Four live sites in the whole corpus and all four are in
     * `fmMappingFormUF`** — the typeahead, its wrapped input, the function
     * argument and the default value
     * (`file_mapping/form_config.dart:196`, `:206`, `:235`, `:249`; a fifth at
     * `:215` is commented out). The two `onUserInteraction` sites are
     * `pipelineConfigUF` dropdowns and are not this.
     *
     * **Form-level rather than per field, because four of four are one form** and
     * because validity here is already form-wide (`validateForm.ts`). Per-field
     * granularity would be a distinction the corpus does not draw.
     *
     * It is not cosmetic on this form. Save is gated on `enableOnlyWhenFormValid`
     * over *every* row, so without live messages a user with fifty properties to
     * map sees a disabled button and no indication of which row is wrong.
     */
    autovalidate: z.boolean().optional(),
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
export type Repeat = z.infer<typeof RepeatSchema>;
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

/**
 * Every button a form offers — the action bar's and the rows' alike. Task F.2.
 *
 * **This exists because the document-set checks were written when there was only
 * one place to look**, and adding the second place without it would have made
 * both of them quietly incomplete: `checkActions` would stop refusing an inline
 * button naming an undefined action, and `checkEndStateButtons` would stop
 * refusing an end state whose *inline* button advances (I-57). Neither failure
 * shows at authoring time, which is the whole reason that layer exists.
 *
 * Order is the action bar first, then the rows in document order — the bar is
 * where a reader looks first and a finding's list reads better that way. Nothing
 * depends on the order.
 */
export function buttonsOf(form: Form): FormAction[] {
  const inline = fieldsOf(form).filter(
    (f): f is Extract<Field, { field: "button" }> => f.field === "button",
  );
  return [...form.actions, ...inline];
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

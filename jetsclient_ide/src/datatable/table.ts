/**
 * The table configuration document. Task I.3, and the last piece of R2 that
 * Phase 2 left referenced rather than authored.
 *
 * ## What was missing
 *
 * S.5 gave a form a `dataTable` field carrying `table: Identifier` — "the table
 * configuration key; A.4's widget resolves it" (`userflow/form.ts`). Nothing
 * resolved it. The 37 configurations the flows use existed only as
 * `fixtures/table_configs.json`, generated out of the running Flutter app for
 * the query builder's tests, and `types.ts` mirrors the Dart class rather than
 * describing a document anyone could write. So a flow was authorable, its forms
 * were authorable, and the tables those forms render were still Dart.
 *
 * This file is the document that closes that. `table_configs/<key>.tc.json`,
 * validated in Go at save time like the other three.
 *
 * ## One table per document, and no self-key
 *
 * `.uf.json` is one flow; `.ua.json` and `.form.json` are records of many. This
 * follows the flow rather than the other two, and the corpus is why: **a table
 * configuration is shared and a form is not.** The form document could be a
 * record keyed inside its flow's file because 46 states have 46 distinct forms,
 * none shared (I-15). Tables are not like that — `wpClientList` and
 * `wpClientListRO` are each named by *two* flows, `loadConfigUF` and
 * `workspacePullUF`, and `pipelineExecStatusTable` is registered on the non-flow
 * side and rendered by `homeFiltersUF` (F18). A per-flow container would force
 * either a duplicated configuration or an ownership decision the configuration
 * does not support.
 *
 * So the file name is the key, which is S.3's rule for the flow document applied
 * to the same problem: two names for one thing is one name too many (I-14).
 *
 * ## Measured against the 37, not transcribed from the Dart
 *
 * Every field here is set by at least one of the 37 flow configurations, and
 * every field the 37 set is here. That cuts the mirror in `types.ts` down
 * considerably, and each cut is a count rather than a judgement:
 *
 * | Dropped | Why |
 * |---|---|
 * | `apiPath`, `apiAction` | `/dataTable` and `read` on all 37. See the note below — this is the security decision, not a tidy-up. |
 * | `index` on a column | equals the array position on all 275. A field that can only ever be wrong. |
 * | `maxLines`, `columnWidth` | `0` on all 275 columns. |
 * | `calculatedAs` | never set in the flows. |
 * | `defaultToAllRows`, `requestColumnDef` | `false` on all 37. |
 * | `sortColumnTableName` | `""` on all 37. |
 * | `lookupColumnInFormState` | `false` on all 49 where clauses. |
 * | `stateGroup` on an action | `0` on all 25. |
 * | `withClauses`, `distinctOnClauses`, `secondRowActions`, `fromConfigRowActions` | empty on all 37. |
 * | `sqlQuery`, `modelStateFormKey`, ~~`actionEnableCriterias`~~, `dataRowMinHeight`, `dataRowMaxHeight`, `predicate`/`ge`/`le` | never set in the flows. |
 * | `hasActionDelegate`, `hasModelStateHandler` | `false` on all 25 actions and all 37 tables. A table action reaches a delegate through `actionType: doAction` and `actionName`, not through this field. |
 *
 * **Four of those are set by the non-flow configurations and will come back**,
 * and knowing which is the point of having measured both corpora at once —
 * `withClauses` (1 table), `secondRowActions` (2), `modelStateFormKey` (2),
 * `hasModelStateHandler` (2), `actionEnableCriterias` (8 actions), `like` (2
 * clauses), `calculatedAs` (2 columns), `dataRowMin/MaxHeight` (1), and three
 * more `apiAction` values. Track C adds them as a screen needs them, which is
 * the discipline S.5 set for the form document and I.2a followed. **The shape
 * here is chosen to admit them**: every one is an added optional property or an
 * added enum member, none needs a different union.
 *
 * **F.5 brought back three and C.2 brings back two more, and that prediction is
 * worth scoring rather than quietly consuming.** `secondRowActions`,
 * `calculatedAs` and two enum members arrived with `pipelineExecStatusTable`;
 * `apiAction` and `actionEnableCriterias` — spelled `enableWhen` — arrive with
 * `workspaceRegistryTable`. The counts held exactly: the 8 criteria-bearing
 * actions are all on this one table, and the 2 tables with a second action row
 * are these two. **The "added optional property or added enum member" claim held
 * for four of the five**; `enableWhen` needed a new object schema as well, which
 * is not a different union and is more than the sentence promised.
 *
 * What is left outstanding for the remaining 26 tables: `withClauses` (1 table),
 * `modelStateFormKey` and `hasModelStateHandler` (2), `like` (2 clauses),
 * `dataRowMin/MaxHeight` (1).
 *
 * ## `apiAction` is dropped rather than enumerated, and that is the security call
 *
 * A table configuration's `apiAction` goes straight into `DataTableAction.Action`
 * and dispatches over the whole switch in `jets/apiserver/api_tables.go:42` —
 * thirty-odd names including `exec_ddl`, `drop_table`, `insert_rows` and
 * `save_workspace_file_content`. An authored table naming one of those is the
 * confused-deputy problem S.7 described for the action grammar, arriving through
 * a second door: the person writing the file needs only `workspace_ide`.
 *
 * S.7 answered its version with an enum (`actions/schema.ts`, `ServerActionSchema`)
 * because flows genuinely use six actions. **All 37 tables use one**, so the
 * stronger answer is available here: the field does not exist, and a query table
 * issues `read`. Track C needs `workspace_read`, `preview_file` and
 * `raw_query_tool` for nine, one and one of its 28 — at which point this becomes
 * ~~a four-member enum~~ **a three-member enum whose absence means `read`**, with
 * the same reasoning S.7 wrote down, and never a bare identifier.
 *
 * **C.2 is that point, and the enum has three members rather than the four
 * predicted above.** The prediction is left standing and corrected here rather
 * than edited away. The two corpora hold **four** values between them —
 * `read`, `workspace_read`, `preview_file` and `raw_query_tool` — and the first
 * is the default. Admitting it as a member would give one meaning two spellings,
 * which is I-14's rule, and would rewrite all 37 committed documents to say the
 * thing they already say by saying nothing. So `ApiActionSchema` carries the
 * three exceptions and `fromDocument` restores `read` for a document that names
 * none.
 *
 * **The counts are asserted in `table.test.ts` rather than written here**, and
 * that is deliberate: an earlier draft of this paragraph carried them, and they
 * were four hours from being wrong — C.0a's fixture regeneration removes three
 * dead configurations and takes `read` from 17 to 14 without touching either
 * argument this enum rests on. A count in a comment is a claim nobody re-derives;
 * the same count in an assertion fails the moment the corpus moves. This is the
 * project's own "generate the measurement" rule applied to a sentence rather than
 * to a document.
 *
 * **The absence is a sentinel and that is a cost worth naming**, because this
 * file argues the other way about `source`: the Dart tells a static table from a
 * query table by `apiPath` being empty, and `source` replaced that sentinel with
 * a statement.
 *
 * **A sentinel is a problem when absence means something a reader must
 * distinguish, and here nothing distinguishes them.** `source` had to stop being
 * one because a document's kind is what *every* reader branches on, so absence
 * was load-bearing for control flow. `apiAction` is read by one thing at one
 * moment — `makeQuery` setting the request's `action` (`query.ts`) — and the
 * absent case and the `read` case take the same arm. That is the reason the cost
 * is acceptable, and it is written down because the paragraph above states the
 * cost without it and would otherwise read as a known defect.
 *
 * **The direction of the failure is the other half.** `read` is the *least*
 * privileged of the four, so a forgotten field cannot buy an authority: the worst
 * a mistake here produces is a table more restricted than its author meant, which
 * is a bug report rather than an incident. A sentinel standing for the permissive
 * value would not be defensible on any of these grounds.
 *
 * So: the three exceptions are on the page and the rule is off it, which makes a
 * security review of this field a reading rather than a count.
 *
 * ## Two kinds, because the corpus has exactly two
 *
 * Nine of the 37 carry a `staticTableModel` and issue nothing: no `fromClauses`,
 * no `whereClauses`, no actions, three columns, and rows compiled in. The other
 * 28 query and all set `fromClauses`. Splitting them structurally means a static
 * table cannot name a schema and a query table cannot carry rows, which is a
 * property neither the Dart class nor `types.ts` can state.
 *
 * The discriminant `source` is this file's invention, like `field` in the form
 * document. The Dart distinguishes the two by `apiPath` being empty, which is a
 * sentinel rather than a statement.
 *
 * **Only use constructs that emit.** Same rule as `userflow/schema.ts`:
 * `.refine()` is invisible to `z.toJSONSchema`, so it would be enforced in the
 * browser and silently absent in Go. Everything expressible structurally is.
 */

import { z } from "zod";

import type { EscapeReferences } from "../actions/escapes";
import { Identifier } from "../userflow/schema";

/**
 * A column of the table.
 *
 * **No `index`.** All 275 columns in the corpus have `index` equal to their
 * position in the list, which makes it a field that can only ever disagree with
 * the array holding it. This is S.1's reasoning about `nextState` on a nested
 * choice, on a second surface: a value that is always derivable is a value that
 * can be wrong.
 *
 * `table` qualifies the column when the query joins — 75 of 275 set it, and they
 * are the tables with more than one `fromClause` or a `joinWith`.
 *
 * ## Two arms, because a visible column must be labelled and a hidden one need not
 *
 * **22 of 275 columns carry an empty label, and all 22 are hidden.** The converse
 * does not hold — 41 of the 63 hidden columns *are* labelled — so this is not
 * "hidden implies unlabelled", it is "unlabelled implies hidden", holding on
 * 275 of 275. A union states it; a `label: z.string()` allowing the empty string
 * everywhere would let an authored table render a blank column header, which is
 * the failure the corpus happens never to contain.
 *
 * The first arm is any labelled column; the second is a hidden one that may omit
 * the label. A visible column with no label matches neither.
 */
const columnCommon = {
  name: Identifier,
  /** Qualifies `name` when more than one table is in scope. */
  table: Identifier.optional(),
  /** 226 of 275 set one; the empty string is how the Dart says "none". */
  tooltip: z.string().min(1).optional(),
  isNumeric: z.boolean().optional(),
  /**
   * A named entry in the escape registry, rendering the cell.
   *
   * Three columns in the corpus have one and all three are `file_key`
   * (`types.ts`), on `inputRegistryTable`, `main_input_registry_key` and
   * `merged_input_registry_keys`. A closure cannot be data; a name for one
   * can, which is what S.2a built the registry for.
   */
  cellFilter: Identifier.optional(),
  /**
   * A SQL expression the server selects in place of the column. Task F.5.
   *
   * **Never set by a flow's 37 tables and set by exactly one column of the first
   * non-flow table this project authored** — `run_duration` on
   * `pipelineExecStatusTable`, which is `AGE(last_update, start_time)` and has no
   * column behind it (`modules/data_table_config_impl.dart`, `run_duration`). The
   * query builder has always emitted it (`datatable/query.ts`, `makeQuery` sends
   * `calculatedAs` per selected column); what was missing was a way to author it,
   * which is the same one-sided gap I-62 described for `isReadOnly`.
   *
   * **It is a string the server splices into the select list, and that is worth
   * naming rather than leaving to the reader.** `apiAction` was dropped from this
   * document because an authored value reaches a switch in the apiserver (S.7's
   * confused deputy); this one reaches SQL. The two differ in that a table
   * configuration is workspace content edited by a user who already holds
   * `workspace_ide`, and that `calculatedAs` cannot name an action — but it is
   * the widest field in the document and the next person to widen this schema
   * should know it is here. Recorded as **I-104**.
   */
  calculatedAs: z.string().min(1).optional(),
} as const;

export const ColumnSchema = z
  .union([
    z.strictObject({ ...columnCommon, label: z.string().min(1), isHidden: z.boolean().optional() }),
    z.strictObject({ ...columnCommon, label: z.string().min(1).optional(), isHidden: z.literal(true) }),
  ])
  .meta({ id: "Column", description: "One column of a table configuration" });

/**
 * A `FROM` entry.
 *
 * `tableName` may be empty, and that is not sloppiness: `types.ts` records that
 * an empty name means "resolve from form state or route params at query time",
 * which the query builder implements. It is spelled as an explicit absence here
 * so a document cannot say it by accident.
 */
export const FromClauseSchema = z
  .strictObject({
    schema: Identifier,
    /** Absent means "resolve at query time" — the Dart's empty `tableName`. */
    table: Identifier.optional(),
    as: Identifier.optional(),
  })
  .meta({ id: "FromClause" });

/**
 * A `WHERE` entry.
 *
 * Recursive through `orWith`, which two clauses in the corpus use. Zod needs the
 * explicit type annotation for a recursive object, and `z.toJSONSchema` emits it
 * as a `$ref` to the same definition.
 *
 * **`lookupColumnInFormState` is not here**: `false` on all 49. **`predicate`,
 * `ge` and `le` are not here**: never set by a flow. `like` is not here either,
 * and is the one of the four that track C brings back — two of its clauses use
 * it.
 */
export interface WhereClauseDocument {
  column: string;
  table?: string;
  formStateKey?: string;
  defaultValue?: string[];
  joinWith?: string;
  orWith?: WhereClauseDocument;
}

export const WhereClauseSchema: z.ZodType<WhereClauseDocument> = z
  .strictObject({
    column: Identifier,
    table: Identifier.optional(),
    /** 35 of 49 read their value from form state. */
    formStateKey: Identifier.optional(),
    /** Used when the form state holds nothing; 4 of 49 declare one. */
    defaultValue: z.array(z.string()).min(1).optional(),
    /** `source_period.key` and seven others — a qualified column, not a key. */
    joinWith: z.string().min(1).optional(),
    // A getter rather than `z.lazy`: both express the recursion, and `z.lazy`
    // emits an extra anonymous `$defs` entry that indirects to the real one.
    // This artifact is read by people as well as by v6.
    get orWith() {
      return WhereClauseSchema.optional();
    },
  })
  .meta({ id: "WhereClause" });

/**
 * What an action does when pressed.
 *
 * Seven kinds across the 37, and this is the whole list rather than the Dart's:
 * `refreshTable` (1), `toggleCheckboxVisible` (3), `clearHomeFilters` (3),
 * `showScreen` (3), `showDialog` (2), `doActionShowDialog` (3), `doAction` (10).
 *
 * **Nine as of F.5, and the two extra arrived from track F rather than track C.**
 * This comment said *track C's 28 add `setSessionIdFilter` and `setRequestIdFilter`,
 * one each* — true about where they are counted and wrong about who meets them
 * first. Both are on `pipelineExecStatusTable`, the one configuration registered
 * on the non-flow side and rendered by a flow (**F18**), so `homeFiltersUF` could
 * not be authored without them. The prediction was right about the number and
 * wrong about the schedule, which is what F18 is for.
 */
export const ActionTypeSchema = z
  .enum([
    "doAction",
    "doActionShowDialog",
    "showDialog",
    "showScreen",
    "refreshTable",
    "toggleCheckboxVisible",
    "clearHomeFilters",
    "setSessionIdFilter",
    "setRequestIdFilter",
  ])
  .meta({ id: "TableActionType" });

/**
 * The server action a query table issues. Task C.2.
 *
 * **Three members, and the absent fourth is `read`** — see the `apiAction` note
 * in this file's header for why the field exists at all, why it is an enum, and
 * why `read` is not in it.
 *
 * Each member is a `case` of the `/dataTable` dispatch
 * (`jets/apiserver/api_tables.go`, `DoDataTableAction`) and each gates itself:
 * `workspace_read` requires `workspace_ide`
 * (`jets/datatable/workspace_data_table_action.go`, `DoWorkspaceReadAction`),
 * `raw_query_tool` requires `CapabilityQueryTool` and `preview_file` requires
 * `CapabilityReadData`. **The enum is not the authorisation** — it decides what
 * an authored document may *ask for*, and the server decides who may have it.
 * That is S.7's division and this field does not move it.
 *
 * **`raw_query_tool` is the member that shows what the enum is for**, and this
 * paragraph is C.4's, contributed by the task that owns the one table naming it.
 * The other three name a handler that composes its own SQL from the request's
 * structured parts; this one names the handler that executes the request's
 * `query` string verbatim — `ExecRawQuery` passes `dataTableAction.RawQuery`
 * straight to `dbpool.Query` (`jets/datatable/data_table_action.go`, `execQuery`).
 * It is gated accordingly: `raw_query` takes `datatable.CapabilityReadData` and
 * `raw_query_tool` takes `datatable.CapabilityQueryTool`, which is
 * `"workspace_ide"` (`jets/apiserver/api_tables.go`, the `raw_query_tool` case;
 * `jets/datatable/data_table_action.go`, the `CapabilityQueryTool` const), and
 * `TestRawQueryToolIsGatedMoreTightlyThanRawQuery`
 * (`jets/apiserver/read_dispatch_test.go`) refuses to let the two share a case
 * again. **So this is not a tidy-up of a free string; it is the list of
 * authorities an authored table may reach**, and `raw_query_tool` is the one
 * where the authority is "run this SQL". One table has it —
 * `queryToolResultSetTable` — and C.4 is the only screen that will ever author
 * it.
 */
export const ApiActionSchema = z
  .enum(["workspace_read", "preview_file", "raw_query_tool"])
  .meta({ id: "TableApiAction", description: "The server action a query table issues" });

/**
 * One conjunct of a button's row gate. Task C.2.
 *
 * `ActionEnableCriteria` (`jetsclient/lib/models/data_table_config.dart`,
 * `ActionEnableCriteria`), with one change: **the Dart names a column by
 * position and this names it by name.** `columnPos` is an index into the same
 * `columns` array whose `index` field this document already dropped for being a
 * value that can only ever disagree with the array holding it; a criterion
 * pointing at column 6 is that same hazard with a worse failure, because
 * inserting a column above it silently regates the button instead of rendering a
 * blank header. `fromDocument` restores the position by looking the name up, so
 * the round trip is exact.
 *
 * **Four comparison kinds, and the corpus uses two.** All 8 criteria in either
 * corpus are `contains` or `doesNotContain`. `equals` and `notEquals` are
 * admitted anyway, which is `textRestriction`'s argument in `userflow/form.ts`:
 * `isCriteriaMet` implements four, so `availability` ports four, and a
 * two-member enum would have to be widened by whoever meets the third — a change
 * to an exported union rather than to a document.
 *
 * **`value` is required here and nullable in the Dart.** `isCriteriaMet` returns
 * `false` for a null value on `contains`/`doesNotContain` and compares it as null
 * on `equals`/`notEquals`, so a null is either a gate that never opens or a test
 * for an empty cell — neither of which any of the 8 sites wants, and both of
 * which a document should have to say some other way if it ever does.
 */
export const EnableCriterionSchema = z
  .strictObject({
    /** The column whose value is tested, by name. */
    column: Identifier,
    is: z.enum(["equals", "notEquals", "contains", "doesNotContain"]),
    value: z.string().min(1),
  })
  .meta({ id: "EnableCriterion", description: "One test on the selected row" });

/**
 * One button on the table's action bar.
 *
 * **No `stateGroup`**: `0` on all 25. **No `actionDelegate`**: `false` on all 25,
 * because a table action reaches its delegate through `actionName` and the
 * central switch in `config_delegates.dart`, not through a field on the config.
 * That is worth stating rather than leaving as an omission — `types.ts` declares
 * `hasActionDelegate` and a reader would reasonably expect it to be the way out.
 *
 * `capability` is presentation only, as it is everywhere in this app
 * (assessment §3.5): 9 of 25 declare one, `client_config` or `run_pipelines`.
 * The server gates the write it performs, not the button that starts it.
 */
export const TableActionSchema = z
  .strictObject({
    key: Identifier,
    label: z.string().min(1),
    action: ActionTypeSchema,
    style: z.enum(["primary", "secondary", "danger"]),
    /** The action-document entry `doAction` runs. 11 distinct names in the corpus. */
    actionName: Identifier.optional(),
    /** The form a dialog or screen opens. */
    configForm: Identifier.optional(),
    /** The route `showScreen` navigates to; a path template, not an identifier. */
    configScreenPath: z.string().min(1).optional(),
    capability: Identifier.optional(),
    isVisibleWhenCheckboxVisible: z.boolean().optional(),
    isEnabledWhenHavingSelectedRows: z.boolean().optional(),
    isEnabledWhenWhereClauseSatisfied: z.boolean().optional(),
    isEnabledWhenStateHasKeys: z.array(Identifier).min(1).optional(),
    /**
     * Tests on the selected row that must pass. Task C.2.
     *
     * **A disjunction of conjunctions, which is the Dart's own description of
     * it** — *"the outer list is 'or' and the inner list is 'and'"*
     * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
     * The shape is kept rather than flattened because two of the eight sites use
     * the inner list: *Export Client Config* wants a workspace that is neither
     * `removed` nor `in progress`.
     *
     * **Empty on all 37 flow tables and set on 8 actions of one non-flow one**,
     * which is `table.ts`'s own prediction and is now exact: every criteria-
     * bearing action in either corpus is on `workspaceRegistryTable`, and every
     * one of them tests the `status` column.
     *
     * It is not decoration. On that table it is what refuses *Delete* on an
     * active workspace and *Open* on one mid-compile — see **I-181** for what it
     * cost that nothing read it.
     */
    enableWhen: z.array(z.array(EnableCriterionSchema).min(1)).min(1).optional(),
    /** Route parameters, taken literally. */
    navigationParams: z.record(Identifier, z.union([z.string(), z.number()])).optional(),
    /** Route parameters read out of form state, by key. */
    stateFormNavigationParams: z.record(Identifier, Identifier).optional(),
    /**
     * A named registry entry deciding whether the button is enabled.
     *
     * Three actions have one, all of them the `clearFilters` button of a
     * `clearHomeFilters` action — the predicate is over router state rather than
     * over the table, which is exactly why it is not expressible here.
     */
    isEnabled: Identifier.optional(),
  })
  .meta({ id: "TableAction", description: "One button on a table's action bar" });

/**
 * How a selected row lands in form state.
 *
 * `keyColumnIdx` is an index into `columns`; 36 of 37 tables declare one, 29 at
 * column 0 and 7 at column 1. `otherColumns` copies further columns out under
 * their own keys — 22 tables copy none, and one copies fifteen.
 *
 * **Bounded below and not above**, for the reason I.2a's `defaultItemPos` is: an
 * index into a sibling array is a cross-field rule, this file may only use
 * constructs that emit, and a `.refine()` would be enforced in the browser and
 * silently absent in Go.
 */
export const FormStateBindingSchema = z
  .strictObject({
    keyColumnIdx: z.number().int().nonnegative(),
    otherColumns: z
      .array(
        z.strictObject({
          stateKey: Identifier,
          columnIdx: z.number().int().nonnegative(),
        }),
      )
      .optional(),
  })
  .meta({ id: "FormStateBinding" });

/** Fields both kinds carry. Split out so the union states only what differs. */
const commonFields = {
  schemaVersion: z.literal(1),
  /**
   * The heading the table renders above itself.
   *
   * **Optional, because two of the 37 have none** — `pcProcessInputRegistry` and
   * `pcProcessInputRegistry4MI`, both reached only from a `doActionShowDialog`
   * button, where the dialog carries the title and a second one would be
   * duplicated. Absent means "the container titles this", which is a statement;
   * the Dart's empty string is a sentinel for the same thing.
   */
  label: z.string().min(1).optional(),
  columns: z.array(ColumnSchema).min(1),
  sortColumn: Identifier,
  sortAscending: z.boolean().optional(),
  rowsPerPage: z.number().int().positive(),
  isCheckboxVisible: z.boolean().optional(),
  isCheckboxSingleSelect: z.boolean().optional(),
  isReadOnly: z.boolean().optional(),
  showSelectedOnly: z.boolean().optional(),
  noFooter: z.boolean().optional(),
  noCopy2Clipboard: z.boolean().optional(),
  formStateBinding: FormStateBindingSchema.optional(),
} as const;

/**
 * The document. One table configuration.
 *
 * **`source` is the discriminant and the Dart has no equivalent** — it tells the
 * two apart by `apiPath` being the empty string, which is a sentinel standing in
 * for a statement. Nine static, 28 query, and nothing in the corpus is both or
 * neither.
 *
 * `refreshOnKeyUpdateEvent` is on the query arm only: the two tables that
 * declare one are both query tables, and a static table has nothing to refresh.
 */
export const TableConfigDocumentSchema = z
  .discriminatedUnion("source", [
    z.strictObject({
      ...commonFields,
      source: z.literal("query"),
      /**
       * What this table asks the server to do. Absent means `read`. Task C.2.
       *
       * On the query arm only: a static table issues nothing, so a static
       * document naming one would be asking for something it never sends.
       */
      apiAction: ApiActionSchema.optional(),
      from: z.array(FromClauseSchema).min(1),
      where: z.array(WhereClauseSchema).optional(),
      actions: z.array(TableActionSchema).optional(),
      /**
       * A second row of buttons, enabled by a selected row. Task F.5.
       *
       * **Empty on all 37 flow tables and five entries long on the first non-flow
       * one**, which is why it was cut and why it is back: `pipelineExecStatusTable`
       * puts *View Execution Details*, *View Process Errors*, *View Failure
       * Details*, *View Execution Stats* and *Resubmit* here
       * (`modules/data_table_config_impl.dart`, `secondRowActions`).
       *
       * **Same element type as `actions`, deliberately.** The Dart's two lists hold
       * the same class and differ only in where the widget draws them
       * (`components/data_table.dart`, `_actionsRow`), so a second schema would be
       * a distinction the configuration does not make. What differs is convention:
       * four of the five set `isEnabledWhenHavingSelectedRows`, because a row
       * action without a row has nothing to act on.
       *
       * **`validateTableActions` reads both** (`userflow/documentSet.ts`) — I-88's
       * check would otherwise have been blind to the two entries here that name a
       * form and an action, which is every cross-document reference this table has.
       */
      secondRowActions: z.array(TableActionSchema).min(1).optional(),
      /** Re-query when one of these form-state keys changes. Two tables use it. */
      refreshOnKeyUpdateEvent: z.array(Identifier).min(1).optional(),
    }),
    z.strictObject({
      ...commonFields,
      source: z.literal("static"),
      /**
       * The rows, positionally, one entry per column.
       *
       * All nine static tables in the corpus are option lists: three columns —
       * a description, a value and an `option_order` — and two to nine rows.
       * They are the same shape as I.2a's dropdown items rendered as a table,
       * which is an observation about the Flutter UI rather than a licence to
       * change one into the other here.
       */
      rows: z.array(z.array(z.string().nullable())).min(1),
    }),
  ])
  .meta({
    id: "TableConfigDocument",
    title: "JetStore table configuration",
    description: "One data-table configuration, authored as data",
  });

export type Column = z.infer<typeof ColumnSchema>;
export type FromClause = z.infer<typeof FromClauseSchema>;
export type ApiAction = z.infer<typeof ApiActionSchema>;
export type EnableCriterion = z.infer<typeof EnableCriterionSchema>;
export type TableAction = z.infer<typeof TableActionSchema>;
export type FormStateBinding = z.infer<typeof FormStateBindingSchema>;
export type TableConfigDocument = z.infer<typeof TableConfigDocumentSchema>;

export function emitJsonSchema(): unknown {
  return z.toJSONSchema(TableConfigDocumentSchema, { io: "input" });
}

/** The directory a workspace keeps its table configurations in. */
export const TABLE_DIR = "table_configs";

export const tablePath = (key: string): string => `${TABLE_DIR}/${key}.tc.json`;

/**
 * The escape names a table configuration references.
 *
 * Same shape as `escapeReferences` for a flow (`userflow/store.ts`), and for the
 * same reason: the registry is compiled into the bundle, so whether a name
 * resolves is a question only the client can answer. The server validates the
 * document's shape and cannot do this one.
 */
export function escapeNamesOf(table: TableConfigDocument): string[] {
  const names = new Set<string>();
  for (const column of table.columns) if (column.cellFilter) names.add(column.cellFilter);
  if (table.source === "query")
    // Both rows, since F.5 — an `isEnabled` on a second-row button resolves out of
    // the same registry and an unresolved one must fail the load the same way.
    for (const action of [...(table.actions ?? []), ...(table.secondRowActions ?? [])])
      if (action.isEnabled) names.add(action.isEnabled);
  return [...names].sort();
}

/**
 * The same references, carrying which namespace each belongs to. Task I.3b.
 *
 * `escapeNamesOf` flattens both kinds into one sorted list, which is what a
 * *count* wants and not what resolution wants: a `cellFilter` and an `isEnabled`
 * are looked up in different namespaces, and a name registered as one does not
 * satisfy the other. Both functions are kept — the flat one is what the round-trip
 * and corpus tests assert against, and rewriting them to unwrap this would make
 * them less legible for no gain.
 *
 * `at` is a JSON Pointer into the table document, so a load failure names the
 * offending line rather than the file — the same contract `escapeReferences` has
 * for a flow (`userflow/store.ts`).
 */
export function tableEscapeReferences(table: TableConfigDocument): EscapeReferences[] {
  const references: EscapeReferences[] = [];
  table.columns.forEach((column, index) => {
    if (column.cellFilter) {
      references.push({ kind: "cellFilters", name: column.cellFilter, at: `/columns/${index}/cellFilter` });
    }
  });
  if (table.source === "query") {
    (table.actions ?? []).forEach((action, index) => {
      if (action.isEnabled) {
        references.push({ kind: "predicates", name: action.isEnabled, at: `/actions/${index}/isEnabled` });
      }
    });
  }
  return references;
}

/**
 * The action-document entries a table configuration names.
 *
 * A table's `doAction` button runs an entry in the *flow's* action document, so
 * this is a cross-document reference like a state's `stateAction` — and, like
 * that one, it cannot be checked at save time. A `.tc.json` is legitimately
 * saved before the flow that uses it exists, and a table is shared between
 * flows, so there is no single document to check it against.
 */
export function actionNamesOf(table: TableConfigDocument): string[] {
  if (table.source !== "query") return [];
  const names = new Set<string>();
  // Both rows since F.5: `resubmitPipeline` is `pipelineExecStatusTable`'s only
  // `doAction` and it is a second-row button, so a first-row-only walk would have
  // reported this table as naming no action at all.
  for (const action of [...(table.actions ?? []), ...(table.secondRowActions ?? [])])
    if (action.actionName) names.add(action.actionName);
  return [...names].sort();
}

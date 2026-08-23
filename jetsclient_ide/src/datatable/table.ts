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
 * | `sqlQuery`, `modelStateFormKey`, `actionEnableCriterias`, `dataRowMinHeight`, `dataRowMaxHeight`, `predicate`/`ge`/`le` | never set in the flows. |
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
 * a four-member enum with the same reasoning S.7 wrote down, and never a bare
 * identifier.
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
 * Track C's 28 add `setSessionIdFilter` and `setRequestIdFilter`, one each.
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
  ])
  .meta({ id: "TableActionType" });

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
      from: z.array(FromClauseSchema).min(1),
      where: z.array(WhereClauseSchema).optional(),
      actions: z.array(TableActionSchema).optional(),
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
    for (const action of table.actions ?? []) if (action.isEnabled) names.add(action.isEnabled);
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
  for (const action of table.actions ?? []) if (action.actionName) names.add(action.actionName);
  return [...names].sort();
}

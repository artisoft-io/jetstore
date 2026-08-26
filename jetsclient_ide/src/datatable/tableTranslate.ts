/**
 * Corpus `TableConfig` → authored `.tc.json`. Task I.3.
 *
 * The same move S.1 made for the flows (`userflow/translate.ts`): translate the
 * shipping configuration into the document, commit the result, and let Go
 * validate the real corpus rather than a hand-written sample. **A real config
 * that fails means the schema is wrong, not the config** — that rule is why this
 * file exists and is not a convenience.
 *
 * ## The two escape names
 *
 * `types.ts` carries `hasCellFilter` and `hasIsEnabledFnc` because the corpus
 * cannot serialise a Dart closure. Translating needs a name for each, and the
 * corpus's six closures turn out to be **two** bodies written three times each:
 *
 * | Name | What the Dart does | Sites |
 * |---|---|---|
 * | `fileKeyLabel` | shortens a file key for display — `WORKSPACE_FILE_KEY_LABEL_RE` if it matches, otherwise the segment after the last `/` (`start_pipeline/data_table_config.dart:86`) | 3 columns, all `file_key` |
 * | `hasDataRegistryFilters` | `JetsRouterDelegate().dataRegistryFilters` is non-empty (`:371`, `:428`, `:561`) | 3 `clearFilters` actions |
 *
 * **Two more names as of F.5, and the reason is a correction rather than a
 * widening.** `translateAction` mapped *any* `hasIsEnabledFnc` to
 * `hasDataRegistryFilters`, which is true of the 37 flow tables and false of the
 * first non-flow one. `pipelineExecStatusTable` has three closures and none of
 * them is that predicate: `clearHomeFilters` gates on
 * `JetsRouterDelegate().homeFilters` — a different list, and the one this flow
 * writes (`modules/data_table_config_impl.dart`, `clearHomeFilters`) — while
 * `setSessionIdFilters` and `setRequestIdFilters` are written `(state) => true`,
 * which is what `ActionConfig.isEnabled` returns when no closure is set at all
 * (`models/data_table_config.dart`, `isEnabled`).
 *
 * | Name | What the Dart does | Sites |
 * |---|---|---|
 * | `hasHomeFilters` | `JetsRouterDelegate().homeFilters` is non-empty | 1 `clearHomeFilters` action |
 * | `alwaysEnabled` | `(state) => true` | 2 filter-prompt actions |
 *
 * **`alwaysEnabled` is a name for a closure that does nothing, and it is here so
 * the round-trip stays total.** Dropping it would be unobservable — absent and
 * always-true are the same button — but `hasIsEnabledFnc` would then come back
 * `false` and the "translation loses nothing" test would have to make an
 * exception for the one table it most wants to check. A name that says *this
 * closure was read and it was trivial* costs one registry entry.
 *
 * **A fifth name as of C.6, and it is the *cell filter* mapping repeating the
 * mistake F.5 corrected in the one above it.** `translateColumn` sent every
 * `hasCellFilter` to `fileKeyLabel`. Across both corpora there are **six**
 * `cellFilter` sites, not three: the three above, plus `inputLoaderStatusTable`'s
 * `file_key` and `pipelineExecStatusTable`'s `main_input_file_key`, which agree
 * with them — and `inputLoaderStatusTable`'s `error_message`, which does not.
 *
 * | Name | What the Dart does | Sites |
 * |---|---|---|
 * | `errorMessageLabel` | strips `"File contains 0 bad rows,recovered error: "` off a load error (`modules/data_table_config_impl.dart`, the `error_message` column) | 1 column |
 *
 * The count in the first table — *3 columns, all `file_key`* — was a measurement
 * of the flow corpus and is still true of it. What made it dangerous is that the
 * mapping beside it was written as though it were a fact about closures. See
 * `cellFilterEscapeFor`.
 *
 * That the six collapse to two is the argument for naming them rather than
 * porting them: three identical copies of a predicate are three places to fix it.
 * **Registering the two is I.3b's**, and until it happens a document naming them
 * validates and does not load — which is the asymmetry `actions/escapes.ts`
 * describes and the failure mode the agentic project met from the other side.
 *
 * ## What C.2 added, and the one thing it does not translate
 *
 * `apiAction` and `enableWhen`. Both were named in `table.ts`'s table of cuts as
 * fields the non-flow corpus sets, so neither is a discovery — what C.2 supplies
 * is the *shape*, and in one case that is more than a copy: a criterion names its
 * column **by name** here and by position in the Dart, so `translateCriterion`
 * resolves the index forward and `columnPosOf` resolves it back.
 *
 * **`hasIsEnabledFnc` and `enableWhen` are not the same gate and this file now
 * carries both**, which is worth saying because they read alike. `isEnabled` is a
 * closure over *router* state and cannot be data, so it becomes a name; a
 * criterion is a test on the *selected row* and always could have been data. The
 * Dart evaluates the second inside the `isEnabledWhenHavingSelectedRows` branch
 * and the first only after that branch declines
 * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`), so
 * no configuration sets both — `workspaceRegistryTable`'s 8 criteria-bearing
 * actions all have `hasIsEnabledFnc: false`.
 */

import type { TableConfig, ColumnConfig, ActionConfig, WhereClause } from "./types";
import type {
  ApiAction,
  Column,
  EnableCriterion,
  FormStateBinding,
  FromClause,
  TableAction,
  TableConfigDocument,
  WhereClauseDocument,
} from "./table";

/**
 * The server action a document that names none is issuing. Task C.2.
 *
 * One definition, read by both directions of the translation: `toDocument` omits
 * the field when the configuration says this, and `fromDocument` restores it when
 * the document omits it. Two literals would be the same bug the `hasActiveFilters`
 * rename fixed — one value, two places, free to drift.
 */
export const DEFAULT_API_ACTION = "read";

/** The three a document may name; anything else is a translation failure. */
const AUTHORABLE_API_ACTIONS: readonly string[] = ["workspace_read", "preview_file", "raw_query_tool"];

/** The registry name for the file-key display filter. */
export const FILE_KEY_LABEL_ESCAPE = "fileKeyLabel";
/** The registry name for the load-error display filter. Task C.6. */
export const ERROR_MESSAGE_LABEL_ESCAPE = "errorMessageLabel";
/** The registry name for the `clearFilters` enablement predicate. */
export const DATA_REGISTRY_FILTERS_ESCAPE = "hasDataRegistryFilters";
/** The registry name for the home-filter enablement predicate. Task F.5. */
export const HOME_FILTERS_ESCAPE = "hasHomeFilters";
/** The registry name for a `(state) => true` closure. Task F.5. */
export const ALWAYS_ENABLED_ESCAPE = "alwaysEnabled";

/**
 * Which predicate a closure-bearing action names.
 *
 * **Keyed by table and action rather than assumed**, which is the whole of the
 * F.5 correction: the corpus records only *that* a closure exists, so the
 * mapping is knowledge about the Dart and has to be written down somewhere. It
 * was written as a constant, which read as a fact about closures and was a fact
 * about the 37.
 */
/**
 * Which display filter a `cellFilter`-bearing column names. Task C.6.
 *
 * **Keyed by table and column for exactly the reason `isEnabledEscapeFor` below
 * is, and it is the same defect found on a second field.** I-103 corrected the
 * `isEnabled` mapping from a constant to a lookup and said the lesson was that
 * *knowledge the corpus does not carry has to be data, keyed by what it is
 * knowledge about*. The `cellFilter` mapping was left a constant, and it was
 * right for as long as the only corpus was the flows'.
 *
 * Six sites across both corpora. Five are `fileKeyLabel`'s body — three
 * declarations, one of them shared by reference across three configurations. The
 * sixth is `inputLoaderStatusTable`'s `error_message`, which strips a prefix off
 * a load error (`modules/data_table_config_impl.dart`, the `error_message`
 * column) and has nothing to do with file keys; the constant would have rendered
 * a stack trace as `.../` plus its last path segment.
 *
 * A default rather than an exhaustive map, because five of six agree and a table
 * added later is more likely to be the fifth case than the sixth — but a *new*
 * body is now a row here rather than a silent mistranslation, which is the whole
 * of the correction.
 */
function cellFilterEscapeFor(tableKey: string, columnName: string): string {
  if (tableKey === "inputLoaderStatusTable" && columnName === "error_message") {
    return ERROR_MESSAGE_LABEL_ESCAPE;
  }
  return FILE_KEY_LABEL_ESCAPE;
}

function isEnabledEscapeFor(tableKey: string, actionKey: string): string {
  if (tableKey === "pipelineExecStatusTable") {
    return actionKey === "clearHomeFilters" ? HOME_FILTERS_ESCAPE : ALWAYS_ENABLED_ESCAPE;
  }
  return DATA_REGISTRY_FILTERS_ESCAPE;
}

/** Drops a key whose value is the type's own default, so documents stay readable. */
function omitFalse(value: boolean | undefined): true | undefined {
  return value === true ? true : undefined;
}

function translateColumn(tableKey: string, column: ColumnConfig): Column {
  const rest = {
    ...(column.table ? { table: column.table } : {}),
    ...(column.tooltips ? { tooltip: column.tooltips } : {}),
    ...(omitFalse(column.isNumeric) ? { isNumeric: true as const } : {}),
    ...(column.hasCellFilter ? { cellFilter: cellFilterEscapeFor(tableKey, column.name) } : {}),
    ...(column.calculatedAs ? { calculatedAs: column.calculatedAs } : {}),
    // Task C.7. Zero is the Dart's "unset" for both, so the document says it by
    // omission, as it does for every other boolean-ish default here.
    ...(column.maxLines ? { maxLines: column.maxLines } : {}),
    ...(column.columnWidth ? { columnWidth: column.columnWidth } : {}),
  };
  if (column.label) {
    return {
      name: column.name,
      label: column.label,
      ...rest,
      ...(omitFalse(column.isHidden) ? { isHidden: true as const } : {}),
    };
  }
  // The invariant `ColumnSchema`'s second arm rests on: 22 of 275 columns have
  // no label and every one is hidden. A visible unlabelled column would render a
  // blank header, so the translation refuses it rather than emitting a document
  // the schema will reject with a less useful message.
  if (!column.isHidden) {
    throw new Error(`column ${column.name}: a visible column must be labelled — see table.ts`);
  }
  return { name: column.name, ...rest, isHidden: true as const };
}

function translateFrom(from: TableConfig["fromClauses"][number]): FromClause {
  return {
    // Both sentinels omitted rather than emitted as empty strings — see
    // `FromClauseSchema`. An empty `tableName` is "resolve at query time"; an
    // empty `schemaName` is "unqualified", which is what a `WITH` reference is.
    ...(from.schemaName ? { schema: from.schemaName } : {}),
    ...(from.tableName ? { table: from.tableName } : {}),
    ...(from.asTableName ? { as: from.asTableName } : {}),
  };
}

function translateWhere(where: WhereClause): WhereClauseDocument {
  return {
    // Omitted when the Dart's is the empty string — a gate rather than a
    // predicate, and one clause of fifty. See `WhereClauseSchema.column`.
    ...(where.column ? { column: where.column } : {}),
    ...(where.table ? { table: where.table } : {}),
    ...(where.formStateKey ? { formStateKey: where.formStateKey } : {}),
    ...(where.defaultValue.length > 0 ? { defaultValue: where.defaultValue } : {}),
    ...(where.joinWith ? { joinWith: where.joinWith } : {}),
    // `true` or absent, never `false` — see `WhereClauseSchema`. Task C.9.
    ...(where.lookupColumnInFormState ? { lookupColumnInFormState: true as const } : {}),
    ...(where.orWith ? { orWith: translateWhere(where.orWith) } : {}),
  };
}

/**
 * A row gate, with its column index resolved to a column name. Task C.2.
 *
 * `columns` is the table's, because the index is into that array. An index past
 * the end is refused rather than emitted as a dangling name: the Dart's
 * `isCriteriaMet` guards `columnPos < row.length` and returns `false`, so such a
 * criterion is a button that never enables, and a document should not be able to
 * say that by accident.
 */
function translateCriterion(
  columns: ColumnConfig[],
  actionKey: string,
  criterion: NonNullable<ActionConfig["actionEnableCriterias"]>[number][number],
  refuse: (what: string) => never,
): EnableCriterion {
  const column = columns[criterion.columnPos];
  if (column === undefined) {
    refuse(`action ${actionKey}: enableWhen names column ${criterion.columnPos}, past the end`);
  }
  if (typeof criterion.value !== "string" || criterion.value === "") {
    refuse(`action ${actionKey}: enableWhen on ${column!.name} has no value — see table.ts`);
  }
  return {
    column: column!.name,
    is: criterion.criteriaType as EnableCriterion["is"],
    value: criterion.value!,
  };
}

function translateAction(
  tableKey: string,
  action: ActionConfig,
  columns: ColumnConfig[],
  refuse: (what: string) => never,
): TableAction {
  return {
    key: action.key,
    label: action.label,
    action: action.actionType as TableAction["action"],
    style: action.style as TableAction["style"],
    ...(action.actionName ? { actionName: action.actionName } : {}),
    ...(action.configForm ? { configForm: action.configForm } : {}),
    ...(action.configScreenPath ? { configScreenPath: action.configScreenPath } : {}),
    ...(action.capability ? { capability: action.capability } : {}),
    ...(omitFalse(action.isVisibleWhenCheckboxVisible) ? { isVisibleWhenCheckboxVisible: true as const } : {}),
    ...(omitFalse(action.isEnabledWhenHavingSelectedRows) ? { isEnabledWhenHavingSelectedRows: true as const } : {}),
    ...(omitFalse(action.isEnabledWhenWhereClauseSatisfied) ? { isEnabledWhenWhereClauseSatisfied: true as const } : {}),
    ...(action.isEnabledWhenStateHasKeys?.length ? { isEnabledWhenStateHasKeys: action.isEnabledWhenStateHasKeys } : {}),
    ...(action.navigationParams && Object.keys(action.navigationParams).length > 0
      ? { navigationParams: action.navigationParams }
      : {}),
    ...(action.stateFormNavigationParams && Object.keys(action.stateFormNavigationParams).length > 0
      ? { stateFormNavigationParams: action.stateFormNavigationParams }
      : {}),
    ...(action.actionEnableCriterias?.length
      ? {
          enableWhen: action.actionEnableCriterias.map((conjunction) =>
            conjunction.map((c) => translateCriterion(columns, action.key, c, refuse)),
          ),
        }
      : {}),
    ...(action.hasIsEnabledFnc ? { isEnabled: isEnabledEscapeFor(tableKey, action.key) } : {}),
  };
}

function translateBinding(config: TableConfig): { formStateBinding: FormStateBinding } | Record<string, never> {
  const binding = config.formStateConfig;
  if (binding === undefined) return {};
  return {
    formStateBinding: {
      keyColumnIdx: binding.keyColumnIdx,
      ...(binding.otherColumns.length > 0 ? { otherColumns: binding.otherColumns } : {}),
    },
  };
}

/**
 * Translates one configuration.
 *
 * **Throws rather than degrading** on anything the schema deliberately dropped.
 * The dropped fields were dropped because no configuration sets them; a
 * configuration that does set one is a fact about the corpus that changed, and
 * the useful outcome is a failing translation naming it — not a document that
 * silently omits behaviour the Dart has.
 */
export function toDocument(config: TableConfig): TableConfigDocument {
  const refuse = (what: string): never => {
    throw new Error(`${config.key}: ${what} is not in the table document schema — see table.ts`);
  };

  if (config.sqlQuery) refuse("sqlQuery");
  // **`modelStateHandler` is still refused and `modelStateFormKey` is not, and
  // the asymmetry is the boundary of I-102 decision 1. Task C.9.**
  //
  // A `modelStateFormKey` table is translatable because the corpus carries the
  // key. A handler table is not, because the corpus carries
  // `hasModelStateHandler: true` and the handler is a Dart closure that no
  // corpus contains — the same shape as I-154, where a container nobody's
  // traversal walked left a count that no regeneration could repair.
  //
  // So the two tables of `viewReteTriplesDialogV2` that name a handler are the
  // **first hand-authored table documents in this project**, and they are
  // hand-authored under a test rather than on trust: `table.test.ts` asserts
  // `fromDocument` of each equals the corpus configuration in every field the
  // corpus can express. That makes them a measurement of the Dart everywhere
  // except the one field the Dart holds as code.
  if (config.hasModelStateHandler) refuse("modelStateHandler");
  if (config.withClauses.length > 0 && config.withClauses.some((w) => w.withName === "")) {
    refuse("a withClause with no name");
  }
  if (config.distinctOnClauses.length > 0) refuse("distinctOnClauses");
  // `secondRowActions` is in the schema as of F.5; `fromConfigRowActions` is not
  // and should not be — it is built from `BUTTON_CFG_JSON`, a compile-time
  // environment variable (`jetsclient/lib/button_config.dart`,
  // `getConfigurableActionConfig`), so it is deployment configuration rather than
  // table configuration and the corpus records it empty for that reason.
  if (config.fromConfigRowActions.length > 0) refuse("fromConfigRowActions");
  if (config.defaultToAllRows) refuse("defaultToAllRows");
  // `sortColumnTableName`, `dataRowMinHeight` and `dataRowMaxHeight` were all
  // refused here until C.3, and `requestColumnDef` until C.4. One table sets the
  // heights — `wsJetRulesTable`, 64 and 90 — and it is one of the eight the
  // Workspace IDE's compiled views need. They travel as one `rowHeight` object;
  // see `table.ts` for why that is not the call C.7 made on the column pair.
  if ((config.dataRowMinHeight === undefined) !== (config.dataRowMaxHeight === undefined)) {
    refuse("dataRowMinHeight/dataRowMaxHeight: one without the other");
  }
  // `maxLines` and `columnWidth` were refused here until C.7 and are in the
  // schema now: four non-flow columns set both, and `pipelineExecDetailsTable`'s
  // `error_message` is the first one a screen needs. The refusal did its job —
  // it named the field rather than emitting a document quietly missing it.
  const walkWhere = (where: WhereClause): void => {
    // `lookupColumnInFormState` was refused here until C.9 and is in the schema
    // now: one clause in either corpus sets it, and it is the only way
    // `inputRecordsFromProcessErrorTable` can name its key column at all.
    if (where.predicate) refuse(`where ${where.column}: predicate`);
    if (where.like) refuse(`where ${where.column}: like`);
    if (where.ge || where.le) refuse(`where ${where.column}: ge/le`);
    if (where.orWith) walkWhere(where.orWith);
  };
  for (const where of config.whereClauses) walkWhere(where);
  for (const action of [...config.actions, ...config.secondRowActions]) {
    if (action.stateGroup !== 0) refuse(`action ${action.key}: stateGroup`);
    if (action.hasActionDelegate) refuse(`action ${action.key}: actionDelegate`);
  }

  const common = {
    schemaVersion: 1 as const,
    ...(config.label ? { label: config.label } : {}),
    // **Still emitted here rather than per arm, though the schema now defines
    // them per arm.** Where a key is *declared* decides what a document may say;
    // where it is *emitted* decides the byte order of 40 committed files, and
    // moving these into the two returns rewrote every one of them for no change
    // in meaning. `{...common, sortColumn: x}` keeps the position `common` gave
    // it, which is what lets the static arm restate it for the type checker
    // without moving it.
    columns: config.columns.map((column) => translateColumn(config.key, column)),
    // **`sortColumn` is conditional as of C.4 and `sortColumnTable` as of C.3**,
    // and the two reasons are different: four query tables declare no sort column
    // at all because the server describes the result, while `sortColumnTable` is
    // set by eight tables and all eight are the Workspace IDE's compiled views,
    // where the `FROM` has two or three entries and `name` is a column of several.
    ...(config.sortColumnName ? { sortColumn: config.sortColumnName } : {}),
    ...(config.dataRowMinHeight !== undefined && config.dataRowMaxHeight !== undefined
      ? { rowHeight: { min: config.dataRowMinHeight, max: config.dataRowMaxHeight } }
      : {}),
    ...(config.sortColumnTableName ? { sortColumnTable: config.sortColumnTableName } : {}),
    ...(config.sortAscending ? { sortAscending: true as const } : {}),
    rowsPerPage: config.rowsPerPage,
    ...(omitFalse(config.isCheckboxVisible) ? { isCheckboxVisible: true as const } : {}),
    ...(omitFalse(config.isCheckboxSingleSelect) ? { isCheckboxSingleSelect: true as const } : {}),
    ...(omitFalse(config.isReadOnly) ? { isReadOnly: true as const } : {}),
    ...(omitFalse(config.showSelectedOnly) ? { showSelectedOnly: true as const } : {}),
    ...(omitFalse(config.noFooter) ? { noFooter: true as const } : {}),
    ...(omitFalse(config.noCopy2Clipboard) ? { noCopy2Clipboard: true as const } : {}),
    ...translateBinding(config),
  };

  if (config.staticTableModel !== undefined) {
    // A static table sends nothing, so its `apiAction` is unobservable and the
    // corpus records `read` on all nine. It has no home on the static arm and
    // must not silently become one on the query arm.
    if (config.apiAction !== DEFAULT_API_ACTION) refuse(`a static table with apiAction ${config.apiAction}`);
    if (config.fromClauses.length > 0) refuse("a static table with fromClauses");
    if (config.whereClauses.length > 0) refuse("a static table with whereClauses");
    if (config.actions.length > 0) refuse("a static table with actions");
    if (config.secondRowActions.length > 0) refuse("a static table with secondRowActions");
    if (config.refreshOnKeyUpdateEvent.length > 0) refuse("a static table with refreshOnKeyUpdateEvent");
    // Required here and optional on the query arm: a static table's rows are
    // compiled in, so nothing can supply columns it does not declare.
    if (config.columns.length === 0) refuse("a static table with no columns");
    if (!config.sortColumnName) refuse("a static table with no sortColumn");
    return {
      ...common,
      source: "static",
      // Restated so the static arm's required `sortColumn` typechecks; the key
      // keeps `common`'s position, so no committed document moves.
      sortColumn: config.sortColumnName,
      rows: config.staticTableModel,
    };
  }

  if (config.modelStateFormKey) {
    if (config.fromClauses.length > 0) refuse("a form-state table with fromClauses");
    if (config.withClauses.length > 0) refuse("a form-state table with withClauses");
    if (config.secondRowActions.length > 0) refuse("a form-state table with secondRowActions");
    if (config.refreshOnKeyUpdateEvent.length > 0) refuse("a form-state table with refreshOnKeyUpdateEvent");
    if (config.requestColumnDef) refuse("a form-state table with requestColumnDef");
    if (config.apiAction !== DEFAULT_API_ACTION) refuse(`a form-state table with apiAction ${config.apiAction}`);
    if (config.columns.length === 0) refuse("a form-state table with no columns");
    if (!config.sortColumnName) refuse("a form-state table with no sortColumn");
    return {
      ...common,
      source: "formState",
      // Restated for the same reason the static arm restates it: the arm requires
      // it and `common` already put it in position, so no committed byte moves.
      sortColumn: config.sortColumnName,
      model: { from: "key", key: config.modelStateFormKey },
      ...(config.whereClauses.length > 0 ? { where: config.whereClauses.map(translateWhere) } : {}),
      ...(config.actions.length > 0
        ? { actions: config.actions.map((a) => translateAction(config.key, a, config.columns, refuse)) }
        : {}),
    };
  }

  if (config.fromClauses.length === 0) refuse("a query table with no fromClauses");
  // The allowlist, enforced at the translation as well as in the schema. A value
  // outside it is the confused-deputy door `table.ts` describes, and a
  // translation that emitted it would produce a document Go refuses at save time
  // — a failure two layers away from the configuration that caused it.
  if (config.apiAction !== DEFAULT_API_ACTION && !AUTHORABLE_API_ACTIONS.includes(config.apiAction)) {
    refuse(`apiAction ${config.apiAction}`);
  }
  return {
    ...common,
    source: "query",
    // Omitted when it is `read`, which 44 of the two corpora's 65 configurations
    // are; see the header. The 37 flow documents are byte-identical as a result.
    ...(config.apiAction === DEFAULT_API_ACTION ? {} : { apiAction: config.apiAction as ApiAction }),
    ...(config.requestColumnDef ? { requestColumnDef: true as const } : {}),
    ...(config.withClauses.length > 0
      ? {
          with: config.withClauses.map((w) => ({
            name: w.withName,
            sql: w.asStatement,
            ...(w.stateVariables.length > 0 ? { params: w.stateVariables } : {}),
          })),
        }
      : {}),
    from: config.fromClauses.map(translateFrom),
    ...(config.whereClauses.length > 0 ? { where: config.whereClauses.map(translateWhere) } : {}),
    ...(config.actions.length > 0
      ? { actions: config.actions.map((a) => translateAction(config.key, a, config.columns, refuse)) }
      : {}),
    ...(config.secondRowActions.length > 0
      ? {
          secondRowActions: config.secondRowActions.map((a) =>
            translateAction(config.key, a, config.columns, refuse),
          ),
        }
      : {}),
    ...(config.refreshOnKeyUpdateEvent.length > 0
      ? { refreshOnKeyUpdateEvent: config.refreshOnKeyUpdateEvent }
      : {}),
  };
}

/** Translates a whole corpus, keyed as the corpus keys it. */
export function toDocuments(tables: Record<string, TableConfig>): Record<string, TableConfigDocument> {
  const out: Record<string, TableConfigDocument> = {};
  for (const [key, config] of Object.entries(tables)) out[key] = toDocument(config);
  return out;
}

/**
 * The document, back as the configuration the query builder consumes.
 *
 * **This exists to prove the translation loses nothing**, and the test that uses
 * it round-trips all 40: `fromDocument(key, toDocument(c))` must equal `c`. A
 * schema that drops a field it should have kept is invisible to a forward-only
 * translation — every document validates and the behaviour is quietly gone. This
 * is the "check it against something you did not generate the same way" rule
 * applied to a schema rather than to a count.
 *
 * The constants the schema dropped are restored here, which is the honest place
 * for them: they are properties of the *runtime*, not of the document.
 */
export function fromDocument(key: string, doc: TableConfigDocument): TableConfig {
  const columns: ColumnConfig[] = doc.columns.map((column, index) => ({
    index,
    table: column.table,
    name: column.name,
    label: column.label ?? "",
    tooltips: column.tooltip ?? "",
    isNumeric: column.isNumeric ?? false,
    isHidden: column.isHidden ?? false,
    calculatedAs: column.calculatedAs,
    maxLines: column.maxLines ?? 0,
    columnWidth: column.columnWidth ?? 0,
    hasCellFilter: column.cellFilter !== undefined,
  }));

  // The index a criterion's column name stands for. Built once per document
  // rather than per criterion: the lookup is the inverse of `translateCriterion`
  // and it has to agree with the `columns` array the document already ordered.
  const columnPosOf = new Map(doc.columns.map((column, index) => [column.name, index]));

  const restoreAction = (action: TableAction): ActionConfig => ({
    actionType: action.action,
    key: action.key,
    label: action.label,
    style: action.style,
    isVisibleWhenCheckboxVisible: action.isVisibleWhenCheckboxVisible,
    isEnabledWhenHavingSelectedRows: action.isEnabledWhenHavingSelectedRows,
    isEnabledWhenWhereClauseSatisfied: action.isEnabledWhenWhereClauseSatisfied,
    isEnabledWhenStateHasKeys: action.isEnabledWhenStateHasKeys,
    navigationParams: action.navigationParams,
    stateFormNavigationParams: action.stateFormNavigationParams,
    configForm: action.configForm,
    configScreenPath: action.configScreenPath,
    actionName: action.actionName,
    capability: action.capability,
    stateGroup: 0,
    actionEnableCriterias: action.enableWhen?.map((conjunction) =>
      conjunction.map((c) => ({
        // `-1` is unreachable from a translated document — `translateCriterion`
        // refuses a name it could not produce — and is what an authored document
        // naming a column the table does not have would restore to. The Dart
        // treats an out-of-range position as an unmet criterion rather than as an
        // error, so this reproduces "the button never enables" instead of
        // throwing inside a render.
        columnPos: columnPosOf.get(c.column) ?? -1,
        criteriaType: c.is,
        value: c.value,
      })),
    ),
    hasIsEnabledFnc: action.isEnabled !== undefined,
    hasActionDelegate: false,
  });

  // Every arm that has actions, not "the query arm". Task C.9 — a `formState`
  // table declares them too, and `doc.source === "query"` would have restored
  // `reteSessionEntityDetailsTable`'s *Visit Object Entity* button as absent.
  const actions: ActionConfig[] =
    doc.source === "static" ? [] : (doc.actions ?? []).map(restoreAction);
  const secondRowActions: ActionConfig[] =
    doc.source === "query" ? (doc.secondRowActions ?? []).map(restoreAction) : [];

  const restoreWhere = (where: WhereClauseDocument): WhereClause => ({
    table: where.table,
    column: where.column ?? "",
    formStateKey: where.formStateKey,
    defaultValue: where.defaultValue ?? [],
    joinWith: where.joinWith,
    lookupColumnInFormState: where.lookupColumnInFormState === true,
    orWith: where.orWith ? restoreWhere(where.orWith) : undefined,
  });

  return {
    key,
    label: doc.label ?? "",
    // `apiPath` is restored rather than authored — `/dataTable` on all 28 query
    // tables and empty on all nine static ones, which is the sentinel `source`
    // replaced. `apiAction` is authored as of C.2, and a document that names none
    // means `read`; see the note in `table.ts`.
    // **`/dataTable` on every arm but `static`.** The three form-state tables
    // carry `apiPath: '/dataTable'` in the corpus although none of them ever
    // sends anything (`modules/data_table_config_impl.dart`,
    // `reteSessionRdfTypeTable`) — the Dart's sentinel for "static" is the *empty*
    // path and nothing else, so restoring `""` here would have made the round trip
    // disagree with the corpus on a field the corpus does set. Task C.9.
    apiPath: doc.source === "static" ? "" : "/dataTable",
    apiAction: (doc.source === "query" ? doc.apiAction : undefined) ?? DEFAULT_API_ACTION,
    staticTableModel: doc.source === "static" ? doc.rows : undefined,
    // The document's `model` back as the two Dart fields it stands for. A `key`
    // model is `modelStateFormKey`; a `map` model is what the two Dart handlers
    // do, and `hasModelStateHandler` records that the corpus had a closure there.
    // Task C.9 — see `ModelSourceSchema`.
    modelStateFormKey:
      doc.source === "formState" && doc.model.from === "key" ? doc.model.key : undefined,
    modelSource: doc.source === "formState" ? doc.model : undefined,
    isCheckboxVisible: doc.isCheckboxVisible ?? false,
    isCheckboxSingleSelect: doc.isCheckboxSingleSelect ?? false,
    isReadOnly: doc.isReadOnly ?? false,
    showSelectedOnly: doc.showSelectedOnly ?? false,
    actions,
    secondRowActions,
    fromConfigRowActions: [],
    columns,
    defaultToAllRows: false,
    requestColumnDef: doc.source === "query" && doc.requestColumnDef === true,
    withClauses:
      doc.source === "query"
        ? (doc.with ?? []).map((w) => ({
            withName: w.name,
            asStatement: w.sql,
            stateVariables: w.params ?? [],
          }))
        : [],
    fromClauses:
      doc.source === "query"
        ? doc.from.map((from) => ({
            schemaName: from.schema ?? "",
            tableName: from.table ?? "",
            asTableName: from.as ?? "",
          }))
        : [],
    whereClauses: doc.source === "static" ? [] : (doc.where ?? []).map(restoreWhere),
    distinctOnClauses: [],
    refreshOnKeyUpdateEvent: doc.source === "query" ? (doc.refreshOnKeyUpdateEvent ?? []) : [],
    formStateConfig: doc.formStateBinding
      ? {
          keyColumnIdx: doc.formStateBinding.keyColumnIdx,
          otherColumns: doc.formStateBinding.otherColumns ?? [],
        }
      : undefined,
    sortColumnName: doc.sortColumn ?? "",
    sortColumnTableName: doc.sortColumnTable ?? "",
    sortAscending: doc.sortAscending ?? false,
    rowsPerPage: doc.rowsPerPage,
    noFooter: doc.noFooter ?? false,
    noCopy2Clipboard: doc.noCopy2Clipboard,
    dataRowMinHeight: doc.rowHeight?.min,
    dataRowMaxHeight: doc.rowHeight?.max,
    hasModelStateHandler: doc.source === "formState" && doc.model.from === "map",
  };
}

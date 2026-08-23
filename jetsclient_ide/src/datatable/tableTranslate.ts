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
 * That the six collapse to two is the argument for naming them rather than
 * porting them: three identical copies of a predicate are three places to fix it.
 * **Registering the two is I.3b's**, and until it happens a document naming them
 * validates and does not load — which is the asymmetry `actions/escapes.ts`
 * describes and the failure mode the agentic project met from the other side.
 */

import type { TableConfig, ColumnConfig, ActionConfig, WhereClause } from "./types";
import type {
  Column,
  FormStateBinding,
  FromClause,
  TableAction,
  TableConfigDocument,
  WhereClauseDocument,
} from "./table";

/** The registry name for the file-key display filter. */
export const FILE_KEY_LABEL_ESCAPE = "fileKeyLabel";
/** The registry name for the `clearFilters` enablement predicate. */
export const DATA_REGISTRY_FILTERS_ESCAPE = "hasDataRegistryFilters";

/** Drops a key whose value is the type's own default, so documents stay readable. */
function omitFalse(value: boolean | undefined): true | undefined {
  return value === true ? true : undefined;
}

function translateColumn(column: ColumnConfig): Column {
  const rest = {
    ...(column.table ? { table: column.table } : {}),
    ...(column.tooltips ? { tooltip: column.tooltips } : {}),
    ...(omitFalse(column.isNumeric) ? { isNumeric: true as const } : {}),
    ...(column.hasCellFilter ? { cellFilter: FILE_KEY_LABEL_ESCAPE } : {}),
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
    schema: from.schemaName,
    // An empty `tableName` is the Dart's "resolve at query time"; the document
    // says it by omission rather than by a sentinel.
    ...(from.tableName ? { table: from.tableName } : {}),
    ...(from.asTableName ? { as: from.asTableName } : {}),
  };
}

function translateWhere(where: WhereClause): WhereClauseDocument {
  return {
    column: where.column,
    ...(where.table ? { table: where.table } : {}),
    ...(where.formStateKey ? { formStateKey: where.formStateKey } : {}),
    ...(where.defaultValue.length > 0 ? { defaultValue: where.defaultValue } : {}),
    ...(where.joinWith ? { joinWith: where.joinWith } : {}),
    ...(where.orWith ? { orWith: translateWhere(where.orWith) } : {}),
  };
}

function translateAction(action: ActionConfig): TableAction {
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
    ...(action.hasIsEnabledFnc ? { isEnabled: DATA_REGISTRY_FILTERS_ESCAPE } : {}),
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
  if (config.modelStateFormKey) refuse("modelStateFormKey");
  if (config.hasModelStateHandler) refuse("modelStateHandler");
  if (config.withClauses.length > 0) refuse("withClauses");
  if (config.distinctOnClauses.length > 0) refuse("distinctOnClauses");
  if (config.secondRowActions.length > 0) refuse("secondRowActions");
  if (config.fromConfigRowActions.length > 0) refuse("fromConfigRowActions");
  if (config.defaultToAllRows) refuse("defaultToAllRows");
  if (config.requestColumnDef) refuse("requestColumnDef");
  if (config.sortColumnTableName) refuse("sortColumnTableName");
  if (config.dataRowMinHeight !== undefined) refuse("dataRowMinHeight");
  if (config.dataRowMaxHeight !== undefined) refuse("dataRowMaxHeight");
  for (const column of config.columns) {
    if (column.calculatedAs) refuse(`column ${column.name}: calculatedAs`);
    if (column.maxLines) refuse(`column ${column.name}: maxLines`);
    if (column.columnWidth) refuse(`column ${column.name}: columnWidth`);
  }
  const walkWhere = (where: WhereClause): void => {
    if (where.lookupColumnInFormState) refuse(`where ${where.column}: lookupColumnInFormState`);
    if (where.predicate) refuse(`where ${where.column}: predicate`);
    if (where.like) refuse(`where ${where.column}: like`);
    if (where.ge || where.le) refuse(`where ${where.column}: ge/le`);
    if (where.orWith) walkWhere(where.orWith);
  };
  for (const where of config.whereClauses) walkWhere(where);
  for (const action of config.actions) {
    if (action.stateGroup !== 0) refuse(`action ${action.key}: stateGroup`);
    if (action.hasActionDelegate) refuse(`action ${action.key}: actionDelegate`);
    if (action.actionEnableCriterias?.length) refuse(`action ${action.key}: actionEnableCriterias`);
  }

  const common = {
    schemaVersion: 1 as const,
    ...(config.label ? { label: config.label } : {}),
    columns: config.columns.map(translateColumn),
    sortColumn: config.sortColumnName,
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
    if (config.fromClauses.length > 0) refuse("a static table with fromClauses");
    if (config.whereClauses.length > 0) refuse("a static table with whereClauses");
    if (config.actions.length > 0) refuse("a static table with actions");
    if (config.refreshOnKeyUpdateEvent.length > 0) refuse("a static table with refreshOnKeyUpdateEvent");
    return { ...common, source: "static", rows: config.staticTableModel };
  }

  if (config.fromClauses.length === 0) refuse("a query table with no fromClauses");
  return {
    ...common,
    source: "query",
    from: config.fromClauses.map(translateFrom),
    ...(config.whereClauses.length > 0 ? { where: config.whereClauses.map(translateWhere) } : {}),
    ...(config.actions.length > 0 ? { actions: config.actions.map(translateAction) } : {}),
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
 * it round-trips all 37: `fromDocument(key, toDocument(c))` must equal `c`. A
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
    maxLines: 0,
    columnWidth: 0,
    hasCellFilter: column.cellFilter !== undefined,
  }));

  const actions: ActionConfig[] =
    doc.source === "query"
      ? (doc.actions ?? []).map((action) => ({
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
          hasIsEnabledFnc: action.isEnabled !== undefined,
          hasActionDelegate: false,
        }))
      : [];

  const restoreWhere = (where: WhereClauseDocument): WhereClause => ({
    table: where.table,
    column: where.column,
    formStateKey: where.formStateKey,
    defaultValue: where.defaultValue ?? [],
    joinWith: where.joinWith,
    lookupColumnInFormState: false,
    orWith: where.orWith ? restoreWhere(where.orWith) : undefined,
  });

  return {
    key,
    label: doc.label ?? "",
    // Restored, not authored: see the `apiAction` note in `table.ts`.
    apiPath: doc.source === "query" ? "/dataTable" : "",
    apiAction: "read",
    staticTableModel: doc.source === "static" ? doc.rows : undefined,
    isCheckboxVisible: doc.isCheckboxVisible ?? false,
    isCheckboxSingleSelect: doc.isCheckboxSingleSelect ?? false,
    isReadOnly: doc.isReadOnly ?? false,
    showSelectedOnly: doc.showSelectedOnly ?? false,
    actions,
    secondRowActions: [],
    fromConfigRowActions: [],
    columns,
    defaultToAllRows: false,
    requestColumnDef: false,
    withClauses: [],
    fromClauses:
      doc.source === "query"
        ? doc.from.map((from) => ({
            schemaName: from.schema,
            tableName: from.table ?? "",
            asTableName: from.as ?? "",
          }))
        : [],
    whereClauses: doc.source === "query" ? (doc.where ?? []).map(restoreWhere) : [],
    distinctOnClauses: [],
    refreshOnKeyUpdateEvent: doc.source === "query" ? (doc.refreshOnKeyUpdateEvent ?? []) : [],
    formStateConfig: doc.formStateBinding
      ? {
          keyColumnIdx: doc.formStateBinding.keyColumnIdx,
          otherColumns: doc.formStateBinding.otherColumns ?? [],
        }
      : undefined,
    sortColumnName: doc.sortColumn,
    sortColumnTableName: "",
    sortAscending: doc.sortAscending ?? false,
    rowsPerPage: doc.rowsPerPage,
    noFooter: doc.noFooter ?? false,
    noCopy2Clipboard: doc.noCopy2Clipboard,
    hasModelStateHandler: false,
  };
}

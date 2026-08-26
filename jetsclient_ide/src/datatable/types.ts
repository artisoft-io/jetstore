/**
 * The data-table configuration surface, as data.
 *
 * These types mirror `jetsclient/lib/models/data_table_config.dart` and are
 * shaped by `fixtures/table_configs.json`, which is generated from the running
 * Flutter app rather than transcribed by hand. Fields the nine user flows never
 * set are declared anyway — the corpus emits them, and a type that quietly drops
 * a field would let a config change pass unnoticed.
 *
 * Two things are deliberately *not* modelled, because they cannot be:
 * `cellFilter`, `isEnabledFnc`, `modelStateHandler` and `actionDelegate` are
 * Dart closures. The corpus emits `has…` booleans in their place, and each one
 * marks a spot where the React port needs an answer this file cannot carry. That
 * is the assessment's §3.2 — configuration is not data, it embeds closures —
 * reduced to a checklist.
 */

/** A row as the server returns it: positional, nullable, all strings. */
export type JetsRow = (string | null)[];

export interface ColumnConfig {
  index: number;
  table?: string;
  name: string;
  calculatedAs?: string;
  label: string;
  tooltips: string;
  isNumeric: boolean;
  isHidden: boolean;
  maxLines: number;
  columnWidth: number;
  /** A Dart closure in the original. Three columns set one, all `file_key`. */
  hasCellFilter: boolean;
}

export interface FromClause {
  schemaName: string;
  /** Empty means "resolve from form state or route params at query time". */
  tableName: string;
  asTableName: string;
}

export interface WithClause {
  withName: string;
  asStatement: string;
  stateVariables: string[];
}

export interface FormStatePredicate {
  formStateKey: string;
  expectedValue?: string | null;
}

export interface WhereClause {
  table?: string;
  column: string;
  formStateKey?: string;
  defaultValue: string[];
  joinWith?: string;
  predicate?: FormStatePredicate;
  lookupColumnInFormState: boolean;
  like?: string;
  ge?: string;
  le?: string;
  orWith?: WhereClause;
}

export interface DataTableFormStateOtherColumnConfig {
  stateKey: string;
  columnIdx: number;
}

export interface DataTableFormStateConfig {
  keyColumnIdx: number;
  otherColumns: DataTableFormStateOtherColumnConfig[];
}

export interface ActionConfig {
  actionType: string;
  key: string;
  label: string;
  style: string;
  isVisibleWhenCheckboxVisible?: boolean;
  isEnabledWhenHavingSelectedRows?: boolean;
  isEnabledWhenWhereClauseSatisfied?: boolean;
  isEnabledWhenStateHasKeys?: string[];
  navigationParams?: Record<string, string | number>;
  stateFormNavigationParams?: Record<string, string>;
  configForm?: string;
  configScreenPath?: string;
  actionName?: string;
  capability?: string;
  stateGroup: number;
  actionEnableCriterias?: {
    columnPos: number;
    criteriaType: string;
    value?: string | null;
  }[][];
  hasIsEnabledFnc: boolean;
  hasActionDelegate: boolean;
}

export interface TableConfig {
  key: string;
  label: string;
  /** `/dataTable` for the 28 querying tables; empty for the 9 static ones. */
  apiPath: string;
  apiAction: string;
  modelStateFormKey?: string;
  /**
   * Where a form-state table's rows come from, when the document says. Task C.9.
   *
   * **Not in the corpus and not a mirror of a Dart field**, which is why it sits
   * beside `modelStateFormKey` rather than replacing it: the Dart has two fields,
   * a key and a function pointer, and the document has one construct covering
   * both (`table.ts`, `ModelSourceSchema`). `fromDocument` sets both this and
   * `modelStateFormKey` for a `key` model, so a corpus comparison still works on
   * the field the corpus has.
   */
  modelSource?: { from: "key"; key: string } | { from: "map"; key: string; indexBy: string };
  staticTableModel?: JetsRow[];
  isCheckboxVisible: boolean;
  isCheckboxSingleSelect: boolean;
  isReadOnly: boolean;
  showSelectedOnly: boolean;
  actions: ActionConfig[];
  secondRowActions: ActionConfig[];
  fromConfigRowActions: ActionConfig[];
  columns: ColumnConfig[];
  defaultToAllRows: boolean;
  sqlQuery?: { sqlQuery: string; stateVariables: string[] };
  requestColumnDef: boolean;
  withClauses: WithClause[];
  fromClauses: FromClause[];
  whereClauses: WhereClause[];
  distinctOnClauses: string[];
  refreshOnKeyUpdateEvent: string[];
  formStateConfig?: DataTableFormStateConfig;
  sortColumnName: string;
  sortColumnTableName: string;
  sortAscending: boolean;
  rowsPerPage: number;
  noFooter: boolean;
  dataRowMinHeight?: number;
  dataRowMaxHeight?: number;
  noCopy2Clipboard?: boolean;
  hasModelStateHandler: boolean;
}

/**
 * A form-state value. The Dart side stores `dynamic` and the query builder
 * branches on String vs List<String?>; anything else is a bug there and here.
 */
export type FormStateValue = string | (string | null)[] | null | undefined;

/**
 * The reader the query builder needs from the enclosing form.
 *
 * Kept to two members on purpose: A.4a is meant to be callable without a form,
 * a DOM or a React tree, and widening this is how that property gets lost.
 */
export interface FormStateReader {
  getValue(group: number, key: string): FormStateValue;
  /** Dialogs skip the route-param fallback — see `makeWhereClause`. */
  readonly isDialog: boolean;
}

/**
 * Everything outside the table configuration that the payload depends on.
 *
 * In the Flutter app these are reached through `JetsDataTableState` and the
 * `JetsRouterDelegate` singleton. Naming them here is most of what makes the
 * builder testable: `jetsclient/lib/components/data_table_source.dart:428`
 * cannot be called without a widget tree, and this can.
 */
export interface QueryContext {
  config: TableConfig;
  /** Absent when the table is not a form field. Group and key of the field. */
  formField?: { group: number; key: string };
  formState?: FormStateReader;
  /** `JetsRouterDelegate().currentConfiguration?.params`. */
  routeParams?: Record<string, string | string[] | undefined>;
  /** `JetsRouterDelegate().selectedClient`. */
  selectedClient?: string | null;
  /** `JetsRouterDelegate().homeFilters`, applied to one table by key. */
  homeFilters?: WhereClause[] | null;
  /** `JetsRouterDelegate().dataRegistryFilters`, applied to three by key. */
  dataRegistryFilters?: WhereClause[] | null;
  /** Paging and sorting live in widget state, not in the configuration. */
  indexOffset: number;
  rowsPerPage: number;
  sortColumnName: string;
  sortColumnTableName: string;
  sortAscending: boolean;
  /** Set from a server `columnDef` response when the config declares none. */
  columnNameMaps?: { column: string }[];
}

/** One entry of the emitted `whereClauses` array. */
export interface WhereClausePayload {
  table: string;
  column: string;
  values?: (string | null)[];
  not_in_values?: string[];
  joinWith?: string;
  like?: string;
  ge?: string;
  le?: string;
  orWith?: WhereClausePayload | null;
}

/**
 * The `/dataTable` request body.
 *
 * The field names are the Go struct tags of `DataTableAction`
 * (`jets/datatable/data_table_action.go:61`), which is the contract this whole
 * task exists to reproduce exactly.
 */
export interface DataTableAction {
  action: string;
  withClauses: { name: string; stmt: string }[];
  fromClauses: { schema: string; table: string; asTable?: string }[];
  distinctOnClauses?: string[];
  whereClauses?: WhereClausePayload[];
  offset: number;
  limit: number;
  columns: { table: string; column: string; calculatedAs: string }[] | { column: string }[];
  sortColumn: string;
  sortColumnTable: string;
  sortAscending: boolean;
  workspaceName?: string;
}

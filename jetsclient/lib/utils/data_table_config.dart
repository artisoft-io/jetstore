import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/utils/constants.dart';

enum ActionStyle { primary, secondary, danger }

class TableConfig {
  TableConfig(
      {required this.key,
      required this.schemaName,
      required this.tableName,
      this.label = "",
      required this.apiPath,
      required this.isCheckboxVisible,
      required this.isCheckboxSingleSelect,
      required this.actions,
      required this.columns,
      required this.whereClauses,
      this.formStateConfig,
      required this.sortColumnName,
      required this.sortAscending,
      required this.rowsPerPage});
  final String key;
  final String schemaName;
  final String tableName;
  final String label;
  final String apiPath;
  final bool isCheckboxVisible;
  final bool isCheckboxSingleSelect;
  final List<ActionConfig> actions;
  final List<ColumnConfig> columns;
  final List<WhereClause> whereClauses;
  final DataTableFormStateConfig? formStateConfig;
  final String sortColumnName;
  final bool sortAscending;
  final int rowsPerPage;
}

/// enum describing the type of actions that are available to data table
enum DataTableActionType {
  showDialog,
  showScreen,
  toggleCheckboxVisible,
  makeSelectedRowsEditable,
  saveDirtyRows,
  deleteSelectedRows,
  cancelModifications,
}

/// Table Action Configuration
/// case isVisibleWhenCheckboxVisible is null, action always visible
/// case isVisibleWhenCheckboxVisible == false, action visible when table check boxes are NOT visible
/// case isVisibleWhenCheckboxVisible == true, action visible when table check boxes ARE visble
///
/// case isEnabledWhenHavingSelectedRows is null, action always enable when visible
/// case isEnabledWhenHavingSelectedRows == false, action always enabled when table check boxes are visible
/// case isEnabledWhenHavingSelectedRows == true, action enabled when table HAVE selected row(s)
/// [navigationParams] hold param information for navigating to a screen (action type showScreen):
///   - key correspond to the key to provide to navigator's param
///   - value correspond to a column index to take the associated value of the selected row.
///     Note: if the value is a String (rather than an int), then use it as the value to pass to the navigator.
///     (see data table state method [actionDispatcher])
class ActionConfig {
  ActionConfig(
      {required this.actionType,
      required this.key,
      required this.label,
      this.isVisibleWhenCheckboxVisible,
      this.isEnabledWhenHavingSelectedRows,
      this.navigationParams,
      required this.style,
      this.configForm,
      this.configScreenPath,
      this.apiKey});
  final DataTableActionType actionType;
  final String key;
  final String label;
  final bool? isVisibleWhenCheckboxVisible;
  final bool? isEnabledWhenHavingSelectedRows;
  final Map<String, dynamic>? navigationParams;
  final ActionStyle style;
  final String? configForm;
  final String? configScreenPath;
  final String? apiKey;

  /// returns true if action button is visible
  bool isVisible(JetsDataTableState widgetState) {
    if (isVisibleWhenCheckboxVisible != null) {
      return isVisibleWhenCheckboxVisible == widgetState.isTableEditable;
    }
    return true;
  }

  /// returns true if action button is enabled
  bool isEnabled(JetsDataTableState widgetState) {
    if (isEnabledWhenHavingSelectedRows != null) {
      return isEnabledWhenHavingSelectedRows ==
          widgetState.dataSource.hasSelectedRows();
    }
    return true;
  }

  ButtonStyle buttonStyle(ThemeData td) {
    switch (style) {
      case ActionStyle.danger:
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onErrorContainer,
          backgroundColor: td.colorScheme.errorContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

      case ActionStyle.secondary:
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onPrimaryContainer,
          backgroundColor: td.colorScheme.primaryContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

      default: // primary
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onSecondaryContainer,
          backgroundColor: td.colorScheme.secondaryContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));
    }
  }
}

class ColumnConfig {
  ColumnConfig({
    required this.index,
    required this.name,
    required this.label,
    required this.tooltips,
    required this.isNumeric,
    this.isHidden = false,
  });
  final int index;
  final String name;
  final String label;
  final String tooltips;
  final bool isNumeric;
  final bool isHidden;
}

class WhereClause {
  WhereClause({
    required this.column,
    this.formStateKey,
    this.defaultValue = const [],
  });
  final String column;
  final String? formStateKey;
  final List<String> defaultValue;
}

class DataTableFormStateConfig {
  DataTableFormStateConfig(
      {required this.keyColumnIdx, required this.otherColumns});
  final int keyColumnIdx;
  final List<DataTableFormStateOtherColumnConfig> otherColumns;
}

class DataTableFormStateOtherColumnConfig {
  DataTableFormStateOtherColumnConfig({
    required this.stateKey,
    required this.columnIdx,
  });
  final String stateKey;
  final int columnIdx;
}

final Map<String, TableConfig> _tableConfigurations = {
  // Input Loader Status Data Table
  DTKeys.inputLoaderStatusTable: TableConfig(
    key: DTKeys.inputLoaderStatusTable,
    schemaName: 'jetsapi',
    tableName: 'input_loader_status',
    label: 'File Loader Status',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'loadNewFile',
          label: 'Load New File',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.loadFile),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewDomainTable',
          label: 'View Loaded Data',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: domainTableViewerPath,
          navigationParams: {'table': 3}),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addClient',
          label: 'Add Client',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addClient),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        // an example, not really needed...
        stateKey: FSK.tableName,
        columnIdx: 3,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of object in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "load_count",
          label: 'Records Count',
          tooltips: 'Number of records loaded',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "bad_row_count",
          label: 'Bad Records',
          tooltips: 'Number of Bad Records',
          isNumeric: true),
      ColumnConfig(
          index: 7,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the load',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline Job Key',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "node_id",
          label: 'Node ID',
          tooltips: 'Node ID containing there records',
          isNumeric: true),
      ColumnConfig(
          index: 10,
          name: "user_email",
          label: 'User',
          tooltips: 'Who loaded the file',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Pipeline Execution Status Data Table
  DTKeys.pipelineExecStatusTable: TableConfig(
    key: DTKeys.pipelineExecStatusTable,
    schemaName: 'jetsapi',
    tableName: 'pipeline_execution_status',
    label: 'Pipeline Execution Status',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'startPipeline',
          label: 'Start New Pipeline',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: "newPipeline"),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'startE2E',
          label: 'Load & Start Pipeline',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configForm: "newPipeline"),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "pipeline_config_key",
          label: 'Pipeline Config',
          tooltips: 'Pipeline configuration key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 2,
          name: "main_input_registry_key",
          label: 'Main Input Registry',
          tooltips: 'Main input registry key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 3,
          name: "merged_input_registry_keys",
          label: 'Merge-In Input Registry',
          tooltips: 'Merged entities input registry keys',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "client",
          label: 'Client',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Process executed',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the load',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "input_session_id",
          label: 'Input Session',
          tooltips: 'Input session used (overriding input registry)',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline Job Key',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "user_email",
          label: 'User',
          tooltips: 'Who started the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Pipeline Execution Details Data Table
  DTKeys.pipelineExecDetailsTable: TableConfig(
    key: DTKeys.pipelineExecDetailsTable,
    schemaName: 'jetsapi',
    tableName: 'pipeline_execution_details',
    label: 'Pipeline Execution Details',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          column: "pipeline_execution_status_key",
          formStateKey: DTKeys.pipelineExecStatusTable,
          defaultValue: ["NULL"])
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Process executed',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the pipeline shard',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "shard_id",
          label: 'Shard ID',
          tooltips: 'Pipeline shard ID',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "input_records_count",
          label: 'Input Records Count',
          tooltips: 'Number of input records',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "rete_sessions_count",
          label: 'Rete Sessions Count',
          tooltips: 'Number of rete sessions',
          isNumeric: true),
      ColumnConfig(
          index: 7,
          name: "output_records_count",
          label: 'Output Records Count',
          tooltips: 'Number of output records',
          isNumeric: true),
      ColumnConfig(
          index: 8,
          name: "main_input_session_id",
          label: 'Input Session ID',
          tooltips: 'Session ID of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session ID',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "user_email",
          label: 'User',
          tooltips: 'Who started the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'shard_id',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Clients Data Table
  DTKeys.clientsTable: TableConfig(
    key: DTKeys.clientsTable,
    schemaName: 'jetsapi',
    tableName: 'client_registry',
    label: 'Clients',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addClient',
          label: 'Add New Client',
          style: ActionStyle.primary,
          configForm: FormKeys.addClient),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "client",
          label: 'Client Name',
          tooltips: 'Client name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "details",
          label: 'Notes',
          tooltips: '',
          isNumeric: false),
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Source Config Data Table
  DTKeys.sourceConfigsTable: TableConfig(
    key: DTKeys.sourceConfigsTable,
    schemaName: 'jetsapi',
    tableName: 'source_config',
    label: 'File Input Source Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addSource',
          label: 'Add New Source',
          style: ActionStyle.primary,
          configForm: "newPipeline"),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of object in file',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client Name',
          tooltips: 'Client name',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where to load the data',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "grouping_column",
          label: 'Grouping Column',
          tooltips: 'Column to group the rows into a single rete session',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the records was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Domain Table Viewer Data Table
  DTKeys.inputTable: TableConfig(
      key: DTKeys.inputTable,
      schemaName: 'public',
      tableName: '',
      label: 'Input Data Staging',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [],
      actions: [],
      columns: [],
      sortColumnName: '',
      sortAscending: false,
      rowsPerPage: 10),

  // Users Data Table
  DTKeys.usersTable: TableConfig(
    key: DTKeys.usersTable,
    schemaName: 'jetsapi',
    tableName: 'users',
    label: 'User Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "name",
          label: 'Name',
          tooltips: 'User Name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "user_email",
          label: 'Email',
          tooltips: 'User Email',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Last Updated',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  //* DEMO FORM - DEMO MAIN DATA TABLE
  "dataTableDemoMainTableConfig": TableConfig(
    key: "dataTableDemoMainTableConfig",
    schemaName: 'jetsapi',
    tableName: 'process_input',
    label: 'Client Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [WhereClause(column: "client", formStateKey: "client")],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'new',
          label: 'New Row',
          style: ActionStyle.primary,
          configForm: "newPipeline"),
      ActionConfig(
          actionType: DataTableActionType.toggleCheckboxVisible,
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: false),
      ActionConfig(
          actionType: DataTableActionType.saveDirtyRows,
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          actionType: DataTableActionType.deleteSelectedRows,
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          actionType: DataTableActionType.cancelModifications,
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: "dataTableDemoClient",
        columnIdx: 1,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Input Data Table Name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Secondary Key',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source Type can be file or domain_table',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "entity_rdf_type",
          label: 'RDF Type',
          tooltips: 'Entity rdf type',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "grouping_column",
          label: 'Grouping Column',
          tooltips: 'Input record grouping column',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "key_column",
          label: 'Key Column',
          tooltips: 'Input record key column',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "user_email",
          label: 'User',
          tooltips: 'User who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the record was created or last update',
          isNumeric: false),
    ],
    sortColumnName: 'table_name',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  //* DEMO FORM - DEMO SUPPORT DATA TABLE
  "dataTableDemoSupportTableConfig": TableConfig(
    key: "dataTableDemoSupportTableConfig",
    schemaName: 'jetsapi',
    tableName: 'process_mapping',
    label: 'Input Mapping',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "table_name", formStateKey: "dataTableDemoMainTable")
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: "dataProperties",
        columnIdx: 3,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Primary Key',
          tooltips: 'Sequence Primary Key',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Input Data Table Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "input_column",
          label: 'Input Column',
          tooltips: 'Input column',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "data_property",
          label: 'Data Property',
          tooltips: 'Entity data property',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "function_name",
          label: 'Mapping Function',
          tooltips: 'Function applied to input data',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "argument",
          label: 'Argument',
          tooltips: 'Argument for mapping function',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "default_value",
          label: 'Default',
          tooltips:
              'Default value if the mapping function does not yield anything',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "error_message",
          label: 'Error Message',
          tooltips:
              'Alternate to default value, generate an error if no data is available',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "user_email",
          label: 'User',
          tooltips: 'User who created or last updated the record',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the record was created or last update',
          isNumeric: false),
    ],
    sortColumnName: 'input_column',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  //* DEMO ** TABLE ** CODE
  DTKeys.pipelineDemo: TableConfig(
    key: DTKeys.pipelineDemo,
    schemaName: 'jetsapi',
    tableName: 'pipelines',
    label: 'Data Pipeline',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [],
    // no need for formState here since isCheckboxVisible is false
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Process Session ID',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "user_name",
          label: 'Submitted By',
          tooltips: 'Submitted By',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client',
          tooltips: 'Client',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process",
          label: 'Process',
          tooltips: 'Process',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "status",
          label: 'Status',
          tooltips: 'Execution Status',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "submitted_at",
          label: 'Submitted At',
          tooltips: 'Submitted At',
          isNumeric: false),
    ],
    sortColumnName: 'key',
    sortAscending: true,
    rowsPerPage: 10,
  ),
};

TableConfig getTableConfig(String key) {
  var config = _tableConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: table configuration $key not found');
  }
  return config;
}

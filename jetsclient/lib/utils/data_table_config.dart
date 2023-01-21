import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/utils/constants.dart';

/// Data Table Configuration class
/// [refreshOnKeyUpdateEvent] contains list of key that will trigger a table
/// refresh, used when underlying table is updated independently of this table.
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
      this.defaultToAllRows = false,
      required this.whereClauses,
      this.refreshOnKeyUpdateEvent = const[],
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
  final bool defaultToAllRows;
  final List<WhereClause> whereClauses;
  final List<String> refreshOnKeyUpdateEvent;
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
  refreshTable,
  doAction
}

/// Table Action Configuration
/// case isVisibleWhenCheckboxVisible is null, action always visible
/// case isVisibleWhenCheckboxVisible == false, action visible when table check boxes are NOT visible
/// case isVisibleWhenCheckboxVisible == true, action visible when table check boxes ARE visble
///
/// case isEnabledWhenHavingSelectedRows is null, action always enable when visible (unless other conditions exist)
/// case isEnabledWhenHavingSelectedRows == false, action always enabled when table check boxes are visible
/// case isEnabledWhenHavingSelectedRows == true, action enabled when table HAVE selected row(s)
///
/// case isEnabledWhenWhereClauseSatisfied is null, action always enabled when visible (unless other conditions exists)
/// case isEnabledWhenWhereClauseSatisfied == false, action enabled when where clause fails (perhaps to have a 'show all rows' option)
/// case isEnabledWhenWhereClauseSatisfied == true, action enabled when where clause exists and is satisfied
///
/// [navigationParams] hold param information for:
///   - navigating to a screen (action type showScreen) with key corresponding
///     to the key to provide to navigator's param
///   - navigating to a dialog (action type showDialog) with key corresponding
///     to the key to provide dialog form state
///   - key correspond to the key to provide to navigator's param
/// The value associated to the [navigationParam]'s key correspond to a column
/// index to take the associated value of the selected row.
/// Note: if the value is a String (rather than an int), then use it as the value to pass to the navigator.
/// [stateFormNavigationParams] is similar to [navigationParams] with the difference
/// that the value are string corresponding to state form keys.
/// Therefore the navigation params' values are resolved by looking up the value
/// in the state form.
/// (see data table state method [actionDispatcher])
/// actionName is used for DataTableActionType.doAction, corresponding to the action
/// to invoke when the ActionConfig button is pressed
class ActionConfig {
  ActionConfig(
      {required this.actionType,
      required this.key,
      required this.label,
      this.isVisibleWhenCheckboxVisible,
      this.isEnabledWhenHavingSelectedRows,
      this.isEnabledWhenWhereClauseSatisfied,
      this.navigationParams,
      this.stateFormNavigationParams,
      required this.style,
      this.configForm,
      this.configScreenPath,
      this.actionName});
  final DataTableActionType actionType;
  final String key;
  final String label;
  final bool? isVisibleWhenCheckboxVisible;
  final bool? isEnabledWhenHavingSelectedRows;
  final bool? isEnabledWhenWhereClauseSatisfied;
  final Map<String, dynamic>? navigationParams;
  final Map<String, String>? stateFormNavigationParams;
  final ActionStyle style;
  final String? configForm;
  final String? configScreenPath;
  final String? actionName;

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
    if (isEnabledWhenWhereClauseSatisfied != null) {
      return isEnabledWhenWhereClauseSatisfied ==
          widgetState.dataSource.isWhereClauseSatisfiedOrDefaultToAllRows;
    }
    return true;
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
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    // use FSK.key to trigger table refresh when load & Start Pipeline action
    // add a row to input_loader_status table
    refreshOnKeyUpdateEvent: [FSK.key],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.refreshTable,
          key: 'refreshTable',
          label: 'Refresh',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        // an example, not really needed...
        stateKey: FSK.tableName,
        columnIdx: 3,
      ),
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
          name: "user_email",
          label: 'User',
          tooltips: 'Who loaded the file',
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
          label: 'Start Pipeline',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.startPipeline),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'startE2E',
          label: 'Load & Start Pipeline',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.loadAndStartPipeline),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewStatusDetails',
          label: 'View Execution Details',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: executionStatusDetailsPath,
          navigationParams: {'session_id': 10}),
      ActionConfig(
          actionType: DataTableActionType.refreshTable,
          key: 'refreshTable',
          label: 'Refresh',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "pipeline_config_key",
          label: 'Pipeline Config',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client',
          tooltips: 'Client name for this run',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process_name",
          label: 'Process Name',
          tooltips:
              'Process submitted for execution, will pick up client-specific rule config',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "main_object_type",
          label: 'Main Object Type',
          tooltips: 'Type of object contained in the main input source',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "main_input_registry_key",
          label: 'Main Input Registry',
          tooltips:
              'Main input from previously loaded file, this specify the input session id',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "main_input_file_key",
          label: 'Main Input File Key',
          tooltips:
              'Start the process by loading the this file and then execute the rule process',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "merged_input_registry_keys",
          label: 'Merge-In Input Registry',
          tooltips:
              'Indicate the session id of the input sources to be merged with the main input source',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the pipeline execution',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "input_session_id",
          label: 'Input Session',
          tooltips: 'Input session used (overriding input registry)',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session identifier',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "user_email",
          label: 'User',
          tooltips: 'Who submitted the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the pipeline was submitted',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Pipeline Execution Status Details Data Table
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
          column: "session_id",
          formStateKey: FSK.sessionId),
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

  // Process Name Table
  DTKeys.processNameTable: TableConfig(
    key: DTKeys.processNameTable,
    schemaName: 'jetsapi',
    tableName: 'process_config',
    label: 'Rule Processes',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processName,
        columnIdx: 1,
      ),
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
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Business Rules Process name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "description",
          label: 'Process description',
          tooltips: '',
          isNumeric: false),
    ],
    sortColumnName: 'process_name',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Client Name Table
  DTKeys.clientsNameTable: TableConfig(
    key: DTKeys.clientsNameTable,
    schemaName: 'jetsapi',
    tableName: 'client_registry',
    label: 'Clients',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.client,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "client",
          label: 'Client Name',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "details",
          label: 'Client details',
          tooltips: '',
          isNumeric: false),
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Object Type Registry Data Table
  DTKeys.objectTypeRegistryTable: TableConfig(
    key: DTKeys.objectTypeRegistryTable,
    schemaName: 'jetsapi',
    tableName: 'object_type_registry',
    label: 'Object Type Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'The type of object the file contains',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "entity_rdf_type",
          label: 'Class Name',
          tooltips: 'Entity class name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "details",
          label: 'Description',
          tooltips: 'Details about the class',
          isNumeric: false),
    ],
    sortColumnName: 'object_type',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // File Key Staging Data Table
  DTKeys.fileKeyStagingTable: TableConfig(
    key: DTKeys.fileKeyStagingTable,
    schemaName: 'jetsapi',
    tableName: 'file_key_staging',
    label: 'File Key Staging',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    defaultToAllRows: true, // when where clause fails
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(
          column: "object_type", formStateKey: FSK.objectType),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.loaderOk,
          key: 'loadFile',
          label: 'Load File',
          style: ActionStyle.primary,
          isEnabledWhenHavingSelectedRows: true),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
          DataTableFormStateOtherColumnConfig(
            stateKey: FSK.fileKey,
            columnIdx: 3,
          ),
        ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Primary Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Client providing the input files',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'The type of object the file contains',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key or path',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the file was received',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Data Table
  DTKeys.processInputTable: TableConfig(
    key: DTKeys.processInputTable,
    schemaName: 'jetsapi',
    tableName: 'process_input',
    label: 'Process Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addProcessInput',
          label: 'Add/Update Process Input',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addProcessInput,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.objectType: 2,
            FSK.sourceType: 4,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'configureMapping',
          label: 'Configure Mapping',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.processMapping,
          navigationParams: {
            FSK.tableName: 3,
            FSK.processInputKey: 0,
            FSK.objectType: 2
          }),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.tableName,
        columnIdx: 3,
      ),
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'Canonical model for the Object Type',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: "Status of the Process Input and it's mapping",
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Source Config Table
  DTKeys.sourceConfigTable: TableConfig(
    key: DTKeys.sourceConfigTable,
    schemaName: 'jetsapi',
    tableName: 'source_config',
    label: 'Source Config',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
        actionType: DataTableActionType.showDialog,
        key: 'addSourceConfig',
        label: 'Add/Update Source Config',
        style: ActionStyle.primary,
        isVisibleWhenCheckboxVisible: null,
        isEnabledWhenHavingSelectedRows: null,
        configForm: FormKeys.addSourceConfig,
        navigationParams: {
          FSK.key: 0,
          FSK.client: 1,
          FSK.objectType: 2,
          FSK.domainKeysJson: 4,
        }),
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
        stateKey: FSK.client,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.objectType,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.tableName,
        columnIdx: 3,
      ),
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where to load the file',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 4,
          name: "domain_keys_json",
          label: 'Domain Keys (json)',
          tooltips: 'Column(s) for row''s domain key(s) (json-encoded string)',
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
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Mapping Data Table
  DTKeys.processMappingTable: TableConfig(
    key: DTKeys.processMappingTable,
    schemaName: 'jetsapi',
    tableName: 'process_mapping',
    label: 'Process Input Mapping',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "table_name", formStateKey: FSK.tableName)
    ],
    actions: [],
    // No formStateConfig since rows are not selectable
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
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the Process Input data reside',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "data_property",
          label: 'Target Data Property',
          tooltips: 'Canonical model data property',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "input_column",
          label: 'Source Input Column',
          tooltips: 'Column from the input data',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "function_name",
          label: 'Cleansing Function',
          tooltips: 'Function to cleanse input data',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "argument",
          label: 'Cleansing Function Argument',
          tooltips:
              "Argument for the cleansing function (is either required or ignored)",
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "default_value",
          label: 'Default Value',
          tooltips:
              "Data Property default value if none in the input or the cleansing function returned null",
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "error_message",
          label: 'Error Message',
          tooltips:
              "Error message if no value is provided in the input or returned by cleansing function",
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'data_property',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Rule Config Data Table
  DTKeys.ruleConfigTable: TableConfig(
    key: DTKeys.ruleConfigTable,
    schemaName: 'jetsapi',
    tableName: 'rule_config',
    label: 'Rules Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "process_name", formStateKey: FSK.processName),
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'ruleConfigAction',
          label: 'Edit Rule Configuration Triples',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          isEnabledWhenWhereClauseSatisfied: true,
          configForm: FormKeys.rulesConfig,
          stateFormNavigationParams: {
            FSK.processConfigKey: DTKeys.processNameTable,
            FSK.client: FSK.client,
            FSK.processName: FSK.processName
          }),
    ],
    // No formStateConfig since rows are not selectable
    columns: [
      ColumnConfig(
          index: 0,
          name: "process_config_key",
          label: 'Process Config Key',
          tooltips: '',
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
          label: 'Process',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "subject",
          label: 'Subject',
          tooltips: 'Rule config subject',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "predicate",
          label: 'Predicate',
          tooltips: 'Rule config predicate',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "object",
          label: 'Object',
          tooltips: 'Rule config object',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "rdf_type",
          label: 'Object' 's rdf type',
          tooltips: 'Object' 's rdf type',
          isNumeric: false),
    ],
    sortColumnName: 'subject',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Pipeline Config Data Table for Pipeline Config Forms
  DTKeys.pipelineConfigTable: TableConfig(
    key: DTKeys.pipelineConfigTable,
    schemaName: 'jetsapi',
    tableName: 'pipeline_config',
    label: 'Pipeline Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'newPipelineConfig',
          label: 'New Pipeline Config',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.pipelineConfig),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'configurePipeline',
          label: 'Configure Pipeline',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.pipelineConfig,
          navigationParams: {
            FSK.key: 0,
            FSK.processName: 1,
            FSK.client: 2,
            FSK.processConfigKey: 3,
            FSK.mainProcessInputKey: 4,
            FSK.mergedProcessInputKeys: 5,
            FSK.mainObjectType: 6,
            FSK.mainSourceType: 7,
            FSK.automated: 8,
            FSK.description: 9
          }),
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
          name: "process_name",
          label: 'Process',
          tooltips: 'Process Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client',
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process_config_key",
          label: 'Process Config',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "main_process_input_key",
          label: 'Main Process Input',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 5,
          name: "merged_process_input_keys",
          label: 'Merged Process Inputs',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 6,
          name: "main_object_type",
          label: 'Main Object Type',
          tooltips: 'Object Type of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "main_source_type",
          label: 'Main Object Type',
          tooltips: 'Source of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "automated",
          label: 'Automated',
          tooltips: 'Is pipeline automated? (true: 1, false: 0)',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "description",
          label: 'Description',
          tooltips: 'Pipeline description',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfig)
  // for selecting FSK.mainProcessInputKey
  FSK.mainProcessInputKey: TableConfig(
    key: FSK.mainProcessInputKey,
    schemaName: 'jetsapi',
    tableName: 'process_input',
    label: 'Main Process Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainObjectType,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainSourceType,
        columnIdx: 4,
      ),
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'Canonical model for the Object Type',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: "Status of the Process Input and it's mapping",
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfig)
  // for selecting FSK.mergedProcessInputKeys
  FSK.mergedProcessInputKeys: TableConfig(
    key: FSK.mergedProcessInputKeys,
    schemaName: 'jetsapi',
    tableName: 'process_input',
    label: 'Merged Process Inputs',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
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
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'Canonical model for the Object Type',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: "Status of the Process Input and it's mapping",
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Pipeline Config Data Table for Pipeline Execution Forms
  FSK.pipelineConfigKey: TableConfig(
    key: FSK.pipelineConfigKey,
    schemaName: 'jetsapi',
    tableName: 'pipeline_config',
    label: 'Pipeline Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.processName, columnIdx: 1),
      DataTableFormStateOtherColumnConfig(stateKey: FSK.client, columnIdx: 2),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainObjectType, columnIdx: 6),
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
          name: "process_name",
          label: 'Process',
          tooltips: 'Process Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client',
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process_config_key",
          label: 'Process Config',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "main_process_input_key",
          label: 'Main Process Input',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 5,
          name: "merged_process_input_keys",
          label: 'Merged Process Inputs',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 6,
          name: "main_object_type",
          label: 'Main Object Type',
          tooltips: 'Object Type of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "description",
          label: 'Description',
          tooltips: 'Pipeline description',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Input Registry Table for Home screen
  DTKeys.inputRegistryTable: TableConfig(
    key: DTKeys.inputRegistryTable,
    schemaName: 'jetsapi',
    tableName: 'input_registry',
    label: 'Input Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewDomainTable',
          label: 'View Loaded Data',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: domainTableViewerPath,
          navigationParams: {'table': 5, 'session_id': 6}),
      ActionConfig(
          actionType: DataTableActionType.refreshTable,
          key: 'refreshTable',
          label: 'Refresh',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
    ],
    formStateConfig:
      DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainInputFileKey, columnIdx: 3),
      ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Input Registry Key',
          isNumeric: true,
          isHidden: false),
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File Key of the loaded file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the data reside',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Session ID of the file load job',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Input Registry Table for Pipeline Exec Dialog (FormKeys.startPipeline)
  // for selecting FSK.mainInputRegistryKey
  FSK.mainInputRegistryKey: TableConfig(
    key: FSK.mainInputRegistryKey,
    schemaName: 'jetsapi',
    tableName: 'input_registry',
    label: 'Main Process Input Source',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
    ],
    actions: [],
    formStateConfig:
      DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainInputFileKey, columnIdx: 3),
      ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File Key of the loaded file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Session ID of the file load job',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // File Key Staging Data Table for Pipeline Exec Dialog (FormKeys.startPipeline)
  // for selecting FSK.mainInputFileKey
  DTKeys.fileKeyStagingForPipelineExecTable: TableConfig(
    key: DTKeys.fileKeyStagingForPipelineExecTable,
    schemaName: 'jetsapi',
    tableName: 'file_key_staging',
    label: 'Main Input Source - File Key Staging',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainInputFileKey, columnIdx: 3),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Primary Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Client providing the input files',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'The type of object the file contains',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key or path',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the file was received',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Input Registry Table for Pipeline Exec Dialog (FormKeys.startPipeline)
  // for selecting FSK.mergeInputRegistryKeys
  FSK.mergedInputRegistryKeys: TableConfig(
    key: FSK.mergedInputRegistryKeys,
    schemaName: 'jetsapi',
    tableName: 'input_registry',
    label: 'Merged Process Input Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
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
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File Key of the loaded file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_type",
          label: 'Source Type',
          tooltips: 'Source of the input data, either File or Domain Table',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Session ID of the file load job',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
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
      label: 'Staging Table or Domain Table Data',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [
        WhereClause(column: "session_id", formStateKey: FSK.sessionId),
      ],
      actions: [],
      columns: [],
      sortColumnName: '',
      sortAscending: false,
      rowsPerPage: 10),

  // Users Administration Data Table
  DTKeys.usersTable: TableConfig(
    key: DTKeys.usersTable,
    schemaName: 'jetsapi',
    tableName: 'users',
    label: 'User Administration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'toggleUserActive',
          label: 'Toggle Active',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.toggleUserActive),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteUser',
          label: 'Delete User',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteUser),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: [
          DataTableFormStateOtherColumnConfig(
              stateKey: FSK.isActive, columnIdx: 2),
        ]),
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
          name: "is_active",
          label: 'Active User?',
          tooltips: 'Is user active? (true:1, false:0)',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Last Updated',
          isNumeric: false),
    ],
    sortColumnName: 'name',
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

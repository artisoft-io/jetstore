import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/modules/workspace_ide/data_table_config.dart';

// To avoid duplication
final inputRegistryColumns = [
  ColumnConfig(
      index: 0,
      table: "input_registry",
      name: "key",
      label: 'Key',
      tooltips: 'Input Registry Key',
      isNumeric: true,
      isHidden: false),
  ColumnConfig(
      index: 1,
      table: "input_registry",
      name: "client",
      label: 'Client',
      tooltips: 'Client the file came from',
      isNumeric: false),
  ColumnConfig(
      index: 2,
      table: "input_registry",
      name: "org",
      label: 'Organization',
      tooltips: 'Client' 's org the file came from',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      table: "input_registry",
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Domain Key supported by the input records',
      isNumeric: false),
  ColumnConfig(
      index: 4,
      table: "source_period",
      name: "year",
      label: 'Year',
      tooltips: 'Year the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 5,
      table: "source_period",
      name: "month",
      label: 'Month',
      tooltips: 'Month of the year the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 6,
      table: "source_period",
      name: "day",
      label: 'Day',
      tooltips: 'Day of the month the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 7,
      table: "input_registry",
      name: "file_key",
      label: 'File Key',
      tooltips: 'File Key of the loaded file',
      isNumeric: false),
  ColumnConfig(
      index: 8,
      table: "input_registry",
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 9,
      table: "input_registry",
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false),
  ColumnConfig(
      index: 10,
      table: "input_registry",
      name: "session_id",
      label: 'Session ID',
      tooltips: 'Session ID of the file load job',
      isNumeric: false),
  ColumnConfig(
      index: 11,
      table: "input_registry",
      name: "user_email",
      label: 'User',
      tooltips: 'Who created the record',
      isNumeric: false),
  ColumnConfig(
      index: 12,
      table: "input_registry",
      name: "last_update",
      label: 'Loaded At',
      tooltips: 'Indicates when the record was created',
      isNumeric: false),
  ColumnConfig(
      index: 13,
      table: "input_registry",
      name: "source_period_key",
      label: 'Period',
      tooltips: '',
      isHidden: true,
      isNumeric: true),
];

final processInputColumns = [
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
      name: "org",
      label: 'Organization',
      tooltips: 'Client' 's organization the file came from',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Pipeline Grouping Domain Key',
      isNumeric: false),
  ColumnConfig(
      index: 4,
      name: "entity_rdf_type",
      label: 'Domain Class',
      tooltips: 'Canonical model for the source data',
      isNumeric: false),
  ColumnConfig(
      index: 5,
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 6,
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false,
      isHidden: false),
  ColumnConfig(
      index: 7,
      name: "lookback_periods",
      label: 'Lookback Periods',
      tooltips: 'Number of periods included in the rule session',
      isNumeric: true),
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
];

final fileKeyStagingColumns = [
  ColumnConfig(
      index: 0,
      table: "file_key_staging",
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
      name: "org",
      label: 'Organization',
      tooltips: 'Client' 's organization',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "object_type",
      label: 'Object Type',
      tooltips: 'The type of object the file contains',
      isNumeric: false),
  ColumnConfig(
      index: 4,
      name: "file_key",
      label: 'File Key',
      tooltips: 'File key or path',
      isNumeric: false),
  ColumnConfig(
      index: 5,
      name: "year",
      label: 'Year',
      tooltips: 'Year the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 6,
      name: "month",
      label: 'Month',
      tooltips: 'Month of the year the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 7,
      name: "day",
      label: 'Day',
      tooltips: 'Day of the month the file was received',
      isNumeric: true),
  ColumnConfig(
      index: 8,
      name: "last_update",
      label: 'Last Update',
      tooltips: 'When the file was received',
      isNumeric: false),
  ColumnConfig(
      index: 9,
      name: "source_period_key",
      label: 'Source Period Key',
      tooltips: '',
      isHidden: true,
      isNumeric: true),
];

final Map<String, TableConfig> _tableConfigurations = {
  // Input Loader Status Data Table
  DTKeys.inputLoaderStatusTable: TableConfig(
    key: DTKeys.inputLoaderStatusTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_loader_status'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period')
    ],
    label: 'File Loader Status',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
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
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          table: "input_loader_status",
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
          name: "org",
          label: 'Organization',
          tooltips: 'Client' 's organization',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of object in file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 7,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 8,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "load_count",
          label: 'Records Count',
          tooltips: 'Number of records loaded',
          isNumeric: true),
      ColumnConfig(
          index: 10,
          name: "bad_row_count",
          label: 'Bad Records',
          tooltips: 'Number of Bad Records',
          isNumeric: true),
      ColumnConfig(
          index: 11,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the load',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "error_message",
          label: 'Error Message',
          tooltips: 'Error that occured during execution',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 600),
      ColumnConfig(
          index: 13,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline Job Key',
          isNumeric: false),
      ColumnConfig(
          index: 14,
          name: "user_email",
          label: 'User',
          tooltips: 'Who loaded the file',
          isNumeric: false),
      ColumnConfig(
          index: 15,
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
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_execution_status'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period')
    ],
    label: 'Pipeline Execution Status',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
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
          actionType: DataTableActionType.showScreen,
          key: 'viewStatusDetails',
          label: 'View Execution Details',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: executionStatusDetailsPath,
          navigationParams: {'session_id': 13}),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewProcessErrors',
          label: 'View Process Errors',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: processErrorsPath,
          navigationParams: {'session_id': 13}),
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
          table: "pipeline_execution_status",
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
          label: 'Domain Key',
          tooltips: 'Domain Key of the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 7,
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 8,
          name: "main_input_registry_key",
          label: 'Main Input Registry',
          tooltips:
              'Main input from previously loaded file, this specify the input session id',
          isNumeric: true),
      ColumnConfig(
          index: 9,
          name: "main_input_file_key",
          label: 'Main Input File Key',
          tooltips:
              'Start the process by loading the this file and then execute the rule process',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "merged_input_registry_keys",
          label: 'Merge-In Input Registry',
          tooltips:
              'Indicate the session id of the input sources to be merged with the main input source',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the pipeline execution',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "input_session_id",
          label: 'Input Session',
          tooltips: 'Input session used (overriding input registry)',
          isNumeric: false),
      ColumnConfig(
          index: 13,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session identifier',
          isNumeric: false),
      ColumnConfig(
          index: 14,
          name: "user_email",
          label: 'User',
          tooltips: 'Who submitted the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 15,
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
    fromClauses: [
      FromClause(
          schemaName: 'jetsapi', tableName: 'pipeline_execution_details'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period')
    ],
    label: 'Pipeline Execution Details',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "session_id", formStateKey: FSK.sessionId),
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          table: "pipeline_execution_details",
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
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 4,
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the pipeline shard',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "error_message",
          label: 'Error Message',
          tooltips: 'Error that occured during execution',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 600),
      ColumnConfig(
          index: 8,
          name: "shard_id",
          label: 'Shard ID',
          tooltips: 'Pipeline shard ID',
          isNumeric: true),
      ColumnConfig(
          index: 9,
          name: "input_records_count",
          label: 'Input Records Count',
          tooltips: 'Number of input records',
          isNumeric: true),
      ColumnConfig(
          index: 10,
          name: "rete_sessions_count",
          label: 'Rete Sessions Count',
          tooltips: 'Number of rete sessions',
          isNumeric: true),
      ColumnConfig(
          index: 11,
          name: "output_records_count",
          label: 'Output Records Count',
          tooltips: 'Number of output records',
          isNumeric: true),
      ColumnConfig(
          index: 12,
          name: "main_input_session_id",
          label: 'Input Session ID',
          tooltips: 'Session ID of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 13,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session ID',
          isNumeric: false),
      ColumnConfig(
          index: 14,
          name: "user_email",
          label: 'User',
          tooltips: 'Who started the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 15,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'shard_id',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Pipeline Execution Errors (process_erors) Table
  DTKeys.processErrorsTable: TableConfig(
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_errors'),
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_execution_status')
    ],
    key: DTKeys.processErrorsTable,
    label: 'Pipeline Execution Errors',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          table: "process_errors",
          column: "session_id",
          formStateKey: FSK.sessionId),
      WhereClause(
          column: "pipeline_execution_status_key",
          joinWith: "pipeline_execution_status.key"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'showErrorInputRecords',
          label: 'View Input Records',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.setupShowInputRecords,
          configForm: FormKeys.viewInputRecords,
          // Copy state data from formState to dialogFormState
          stateFormNavigationParams: {
            // keys that will be set by ActionKeys.setupShowInputRecords
            // FSK.sessionId,
            // FSK.domainKey,
            // FSK.tableName,
            FSK.pipelineExectionStatusKey: FSK.pipelineExectionStatusKey,
            FSK.objectType: FSK.objectType,
            FSK.processName: FSK.processName,
            FSK.domainKey: FSK.domainKey,
          }),
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'showReteTriples',
          label: 'View Rete Triples',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.setupShowReteTriples,
          configForm: FormKeys.viewReteTriples,
          // Copy state data from formState to dialogFormState
          stateFormNavigationParams: {
            // keys that will be set by the Action:
            // FSK.reteSessionTriples
            FSK.key: DTKeys.processErrorsTable,
          }),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.pipelineExectionStatusKey,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processName,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.objectType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.domainKey,
        columnIdx: 4,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "process_errors",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: false),
      ColumnConfig(
          index: 1,
          name: "pipeline_execution_status_key",
          table: "process_errors",
          label: 'Process Execution Key',
          tooltips: 'Key from process_execution_status table',
          isNumeric: true),
      ColumnConfig(
          index: 2,
          name: "process_name",
          table: "pipeline_execution_status",
          label: 'Process Name',
          tooltips: 'Process executed, this resolves to a specific rule set',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "main_object_type",
          table: "pipeline_execution_status",
          label: 'Domain Key',
          tooltips: 'Domain Key of the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "grouping_key",
          table: "process_errors",
          label: 'Domain Key',
          tooltips: 'Domain Key of the associated row',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "row_jets_key",
          table: "process_errors",
          label: 'Row jets:key',
          tooltips: 'JetStore row' 's primary key',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "input_column",
          table: "process_errors",
          label: 'Input Column',
          tooltips:
              'Input Column of the error, available if error results from mapping',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "error_message",
          table: "process_errors",
          label: 'Error Message',
          tooltips: 'Error that occured during execution',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 600),
      ColumnConfig(
          index: 8,
          name: "session_id",
          table: "process_errors",
          label: 'Session ID',
          tooltips: 'Data Pipeline session ID',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "rete_session_saved",
          table: "process_errors",
          label: 'Rete Triples Saved',
          tooltips: 'Indicated if the rete triples were saved',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "last_update",
          table: "process_errors",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'grouping_key',
    sortAscending: true,
    rowsPerPage: 50,
  ),

  // View Input Records from Error Table
  DTKeys.inputRecordsFromProcessErrorTable: TableConfig(
      key: DTKeys.inputRecordsFromProcessErrorTable,
      fromClauses: [FromClause(schemaName: 'public', tableName: '')],
      label: 'Input Records for Process Errors',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [
        WhereClause(column: "session_id", formStateKey: FSK.sessionId),
        WhereClause(
            column: FSK.domainKeyColumn,
            lookupColumnInFormState: true,
            formStateKey: FSK.domainKey),
      ],
      actions: [],
      columns: [],
      sortColumnName: '',
      sortAscending: false,
      rowsPerPage: 50),

  // View RDFSession Triples as Table
  DTKeys.reteSessionTriplesTable: TableConfig(
      key: DTKeys.reteSessionTriplesTable,
      fromClauses: [FromClause(schemaName: 'public', tableName: 'triples')],
      label: 'Rule Execution Working Memory as Triples',
      apiPath: '/dataTable',
      modelStateFormKey: FSK.reteSessionTriples,
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [],
      actions: [],
      columns: [
        ColumnConfig(
            index: 0,
            name: "subject",
            label: 'Subject',
            tooltips: 'Subject of the Triple',
            isNumeric: false),
        ColumnConfig(
            index: 1,
            name: "predicate",
            label: 'Predicate',
            tooltips: 'Predicate of the Triple',
            isNumeric: false),
        ColumnConfig(
            index: 2,
            name: "object",
            label: 'Object',
            tooltips: 'Object of the Triple',
            isNumeric: false),
        ColumnConfig(
            index: 3,
            name: "object_type",
            label: 'Object Type',
            tooltips: '',
            isNumeric: false),
      ],
      sortColumnName: 'subject',
      sortAscending: false,
      rowsPerPage: 1000000),

  // Client Admin Table used for Client & Organization Admin form
  DTKeys.clientAdminTable: TableConfig(
    key: DTKeys.clientAdminTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry')
    ],
    label: 'Client Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addClient',
          label: 'Add Client',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addClient),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'exportClientConfig',
          label: 'Export Client Configuration',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.exportClientConfig),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteClient',
          label: 'Delete Client',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteClient),
    ],
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

  // Client Table for Client single selection list
  DTKeys.clientTable: TableConfig(
    key: DTKeys.clientTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry')
    ],
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
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 100,
  ),

  // Org Name Table used for Client & Organization Admin form
  DTKeys.orgNameTable: TableConfig(
    key: DTKeys.orgNameTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_org_registry')
    ],
    label: 'Client Organization Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    defaultToAllRows: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addOrg',
          label: 'Add Organization',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenWhereClauseSatisfied: true,
          configForm: FormKeys.addOrg,
          stateFormNavigationParams: {FSK.client: FSK.client}),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteOrg',
          label: 'Delete Organization',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteOrg),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.org,
        columnIdx: 1,
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
          name: "org",
          label: 'Client Organization',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "details",
          label: 'Organization details',
          tooltips: '',
          isNumeric: false),
    ],
    sortColumnName: 'org',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Client Name Table used for Process & Rules Config form
  DTKeys.clientsAndProcessesTableView: TableConfig(
    key: DTKeys.clientsAndProcessesTableView,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_config'),
    ],
    label: 'Client Rules Configuration',
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
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processConfigKey,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processName,
        columnIdx: 2,
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
          table: "process_config",
          name: "key",
          label: 'Process Config Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 2,
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Business Rules Process name',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          table: "process_config",
          name: "description",
          label: 'Process description',
          tooltips: '',
          isNumeric: false),
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // File Key Staging Data Table used to load files
  DTKeys.fileKeyStagingTable: TableConfig(
    key: DTKeys.fileKeyStagingTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'file_key_staging'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period'),
    ],
    label: 'File Key Staging',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    defaultToAllRows: true, // when where clause fails
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "org", formStateKey: FSK.org),
      WhereClause(column: "object_type", formStateKey: FSK.objectType),
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.loaderOk,
          key: 'loadFile',
          label: 'Load File',
          style: ActionStyle.primary,
          isEnabledWhenHavingSelectedRows: true),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'previewInputFile',
          label: 'Preview File',
          style: ActionStyle.secondary,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: filePreviewPath,
          navigationParams: {'file_key': 4}),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.syncFileKey,
          key: 'syncFileKey',
          label: 'Sync File Keys',
          style: ActionStyle.secondary),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.fileKey,
        columnIdx: 4,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourcePeriodKey,
        columnIdx: 9,
      ),
    ]),
    columns: fileKeyStagingColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Input Source Mapping: use Source Config to select table
  DTKeys.inputSourceMapping: TableConfig(
    key: DTKeys.inputSourceMapping,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_config'),
      FromClause(schemaName: 'jetsapi', tableName: 'object_type_registry'),
    ],
    label: 'Input Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          table: "source_config",
          column: "object_type",
          joinWith: "object_type_registry.object_type"),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.client,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.org,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.objectType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.tableName,
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
          name: "org",
          label: 'Organization',
          tooltips: 'Client' 's organization',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "object_type",
          table: "source_config",
          label: 'Object Type',
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where to load the file',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 5,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'rdf:type of the Domain Class',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 6,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Indicates when the record was last updated',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Data Table
  // on Process Input Configuration Screen
  DTKeys.processInputTable: TableConfig(
    key: DTKeys.processInputTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Process Input Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addProcessInput',
          label: 'Add/Update Process Input Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addProcessInput,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
          }),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Source Config Table
  DTKeys.sourceConfigTable: TableConfig(
    key: DTKeys.sourceConfigTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_config')
    ],
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
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.automated: 4,
            FSK.domainKeysJson: 6,
            FSK.codeValuesMappingJson: 7,
            FSK.inputColumnsJson: 8,
            FSK.inputColumnsPositionsCsv: 9,
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.dropTable,
          key: 'dropStagingTable',
          label: 'Drop Staging Table',
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          style: ActionStyle.secondary),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.client,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.org,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.objectType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.automated,
        columnIdx: 4,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.tableName,
        columnIdx: 5,
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
          name: "org",
          label: 'Organization',
          tooltips: 'Client' 's organization',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "automated",
          label: 'Automated',
          tooltips: 'Is load automated? (true: 1, false: 0)',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where to load the file',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 6,
          name: "domain_keys_json",
          label: 'Domain Keys (json)',
          tooltips: 'Column(s) for row' 's domain key(s) (json-encoded string)',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 500),
      ColumnConfig(
          index: 7,
          name: "code_values_mapping_json",
          label: 'Code Value Mapping (json)',
          tooltips:
              'Client-specific code values mapping to canonical codes (json-encoded string)',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 500),
      ColumnConfig(
          index: 8,
          name: "input_columns_json",
          label: 'Input Columns (json)',
          tooltips:
              'Column names for HEADERLESS FILES ONLY (json-encoded string)',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 500),
      ColumnConfig(
          index: 9,
          name: "input_columns_positions_csv",
          label: 'Fixed-Width Column Positions (csv)',
          tooltips: 'Column names & position for FIXED-WIDTH ONLY (csv)',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 500),
      ColumnConfig(
          index: 10,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Indicates when the record was last updated',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Mapping Data Table
  DTKeys.processMappingTable: TableConfig(
    key: DTKeys.processMappingTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_mapping')
    ],
    label: 'Input Source Mapping',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "table_name", formStateKey: FSK.tableName)
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'configureMapping',
          label: 'Configure Mapping',
          style: ActionStyle.primary,
          isEnabledWhenStateHasKeys: [
            FSK.tableName,
            FSK.objectType,
          ],
          configForm: FormKeys.processMapping,
          stateFormNavigationParams: {
            FSK.tableName: FSK.tableName,
            FSK.objectType: FSK.objectType,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'loadRawRows',
          label: 'Load Raw Rows',
          style: ActionStyle.secondary,
          configForm: FormKeys.loadRawRows),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'downloadMappingRows',
          label: 'Download Mapping',
          style: ActionStyle.secondary,
          isEnabledWhenWhereClauseSatisfied: true,
          actionName: ActionKeys.downloadMapping),
    ],
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
    fromClauses: [FromClause(schemaName: 'jetsapi', tableName: 'rule_config')],
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
            FSK.processConfigKey: FSK.processConfigKey,
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
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_config')
    ],
    label: 'Pipeline Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'configurePipeline',
          label: 'Add/Update Pipeline Configuration',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configScreenPath: pipelineConfigEditFormPath,
          configForm: FormKeys.pipelineConfigEditForm,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.processName: 2,
            FSK.processConfigKey: 3,
            FSK.mainProcessInputKey: 4,
            FSK.mergedProcessInputKeys: 5,
            FSK.mainObjectType: 6,
            FSK.mainSourceType: 7,
            FSK.sourcePeriodType: 8,
            FSK.automated: 9,
            FSK.description: 10,
            FSK.maxReteSessionSaved: 11,
            FSK.injectedProcessInputKeys: 12
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
          name: "client",
          label: 'Client',
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "process_name",
          label: 'Process',
          tooltips: 'Process Name',
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
          label: 'Domain Key',
          tooltips: 'Domain Key of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "main_source_type",
          label: 'Main Source Type',
          tooltips: 'Source type of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "source_period_type",
          label: 'Pipeline Frequency',
          tooltips: 'How often the pipeline execute',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "automated",
          label: 'Automated',
          tooltips: 'Is pipeline automated? (true: 1, false: 0)',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "description",
          label: 'Description',
          tooltips: 'Pipeline description',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "max_rete_sessions_saved",
          label: 'Max Rete Session Saved',
          tooltips: 'Max Rete Session Saved',
          isNumeric: true),
      ColumnConfig(
          index: 12,
          name: "injected_process_input_keys",
          label: 'Injected Data Process Inut',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 13,
          name: "user_email",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 14,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfigEditForm)
  // for selecting FSK.mainProcessInputKey
  FSK.mainProcessInputKey: TableConfig(
    key: FSK.mainProcessInputKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Main Process Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addProcessInput',
          label: 'Add/Update Process Input Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addProcessInput,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
          }),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainObjectType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainSourceType,
        columnIdx: 5,
      ),
    ]),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfigEditForm)
  // for selecting FSK.mergedProcessInputKeys
  FSK.mergedProcessInputKeys: TableConfig(
    key: FSK.mergedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Merged Process Inputs',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
      WhereClause(
          column: "source_type", defaultValue: ['file', 'domain_table']),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addProcessInput',
          label: 'Add/Update Process Input Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addProcessInput,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
          }),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfigEditForm)
  // for selecting FSK.injectedProcessInputKeys
  FSK.injectedProcessInputKeys: TableConfig(
    key: FSK.injectedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Injected Data Process Inputs',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
      WhereClause(column: "source_type", defaultValue: ['alias_domain_table']),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addProcessInput',
          label: 'Add/Update Process Input Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addProcessInput,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
          }),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Pipeline Config Data Table for Pipeline Execution Dialog
  FSK.pipelineConfigKey: TableConfig(
    key: FSK.pipelineConfigKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_config'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Pipeline Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          column: "main_process_input_key", joinWith: "process_input.key"),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.processName, columnIdx: 2),
      DataTableFormStateOtherColumnConfig(stateKey: FSK.client, columnIdx: 1),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainProcessInputKey, columnIdx: 4),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mergedProcessInputKeys, columnIdx: 5),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainObjectType, columnIdx: 6),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainSourceType, columnIdx: 7),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.sourcePeriodType, columnIdx: 9),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainTableName, columnIdx: 10),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.injectedProcessInputKeys, columnIdx: 11),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "pipeline_config",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          table: "pipeline_config",
          label: 'Client',
          tooltips: 'Client the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "process_name",
          table: "pipeline_config",
          label: 'Process',
          tooltips: 'Process Name',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process_config_key",
          table: "pipeline_config",
          label: 'Process Config',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "main_process_input_key",
          table: "pipeline_config",
          label: 'Main Process Input',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 5,
          name: "merged_process_input_keys",
          table: "pipeline_config",
          label: 'Merged Process Inputs',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 6,
          name: "main_object_type",
          table: "pipeline_config",
          label: 'Domain Key',
          tooltips: 'Domain Key of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "main_source_type",
          table: "pipeline_config",
          label: 'Main Source Type',
          tooltips: 'Source Type is file or domain_table',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "description",
          table: "pipeline_config",
          label: 'Description',
          tooltips: 'Pipeline description',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "source_period_type",
          table: "pipeline_config",
          label: 'Frequency',
          tooltips: 'Frequency of execution',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "table_name",
          table: "process_input",
          label: 'Main Table Name',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 11,
          name: "injected_process_input_keys",
          table: "pipeline_config",
          label: 'Injected Data Process Inputs',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 12,
          name: "user_email",
          table: "pipeline_config",
          label: 'User',
          tooltips: 'Who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 13,
          name: "last_update",
          table: "pipeline_config",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 100,
  ),

  // // Source Config Table for Pipeline Execution Forms
  // FSK.sourcePeriodKey: TableConfig(
  //   key: FSK.sourcePeriodKey,
  //   fromClauses: [
  //     FromClause(schemaName: 'jetsapi', tableName: 'source_period')
  //   ],
  //   label: 'Source Period of the Input Sources',
  //   apiPath: '/dataTable',
  //   isCheckboxVisible: true,
  //   isCheckboxSingleSelect: true,
  //   whereClauses: [
  //     WhereClause(
  //         column: "day",
  //         defaultValue: ["1"],
  //         predicate: FormStatePredicate(
  //             formStateKey: FSK.sourcePeriodType,
  //             expectedValue: 'month_period')),
  //     WhereClause(
  //         column: "day",
  //         defaultValue: ["99"],
  //         predicate: FormStatePredicate(formStateKey: FSK.sourcePeriodType)),
  //   ],
  //   actions: [],
  //   formStateConfig:
  //       DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
  //   columns: [
  //     ColumnConfig(
  //         index: 0,
  //         name: "key",
  //         label: 'Key',
  //         tooltips: 'Row Primary Key',
  //         isNumeric: true,
  //         isHidden: true),
  //     ColumnConfig(
  //         index: 1,
  //         name: "year",
  //         label: 'Year',
  //         tooltips: 'Year the file was received',
  //         isNumeric: true),
  //     ColumnConfig(
  //         index: 2,
  //         name: "month",
  //         label: 'Month',
  //         tooltips: 'Month of the year the file was received',
  //         isNumeric: true),
  //     ColumnConfig(
  //         index: 3,
  //         name: "day",
  //         label: 'Day',
  //         tooltips: 'Day of the month the file was received',
  //         isNumeric: true),
  //   ],
  //   sortColumnName: 'year',
  //   sortAscending: false,
  //   rowsPerPage: 10,
  // ),

  // Input Registry Table for Home screen
  DTKeys.inputRegistryTable: TableConfig(
    key: DTKeys.inputRegistryTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period')
    ],
    label: 'File and Domain Table Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewDomainTable',
          label: 'View Loaded Data',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: domainTableViewerPath,
          navigationParams: {'table_name': 9, 'session_id': 10}),
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
    columns: inputRegistryColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Table to show the main process_input of the selected pipeline above
  // this is informative to the user
  DTKeys.mainProcessInputTable: TableConfig(
    key: DTKeys.mainProcessInputTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Main Process Input Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.mainProcessInputKey),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
    noFooter: true,
  ),

  // Table to show the injected_process_input of the selected pipeline above
  // this is informative to the user
  DTKeys.injectedProcessInputTable: TableConfig(
    key: DTKeys.injectedProcessInputTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Injected Data Process Input Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.injectedProcessInputKeys),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
    noFooter: true,
  ),

  // Input Registry Table for Pipeline Exec Dialog (FormKeys.startPipeline)
  // for selecting FSK.mainInputRegistryKey
  FSK.mainInputRegistryKey: TableConfig(
    key: FSK.mainInputRegistryKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period'),
    ],
    label: 'Select the Main Process Source',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
      WhereClause(column: "source_type", formStateKey: FSK.mainSourceType),
      WhereClause(column: "table_name", formStateKey: FSK.mainTableName),
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.mainInputFileKey, columnIdx: 7),
      DataTableFormStateOtherColumnConfig(stateKey: FSK.year, columnIdx: 4),
      DataTableFormStateOtherColumnConfig(stateKey: FSK.month, columnIdx: 5),
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.sourcePeriodKey, columnIdx: 13),
    ]),
    columns: inputRegistryColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 50,
  ),

  // Table to show the merge process_input of the selected pipeline above
  // this is informative to the user
  DTKeys.mergeProcessInputTable: TableConfig(
    key: DTKeys.mergeProcessInputTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Merge Process Input Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.mergedProcessInputKeys),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
    noFooter: true,
  ),

  // Input Registry Table for Pipeline Exec Dialog (FormKeys.startPipeline)
  // for selecting FSK.mergeInputRegistryKeys
  FSK.mergedInputRegistryKeys: TableConfig(
    key: FSK.mergedInputRegistryKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period')
    ],
    label: 'Select the Merged Process Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(
          table: "input_registry", column: "client", formStateKey: FSK.client),
      WhereClause(
          table: "input_registry",
          column: "object_type",
          formStateKey: FSK.mainObjectType),
      WhereClause(
          table: "process_input",
          column: "key",
          formStateKey: FSK.mergedProcessInputKeys),
      WhereClause(
          table: "input_registry",
          column: "client",
          joinWith: "process_input.client"),
      WhereClause(
          table: "input_registry",
          column: "org",
          joinWith: "process_input.org"),
      WhereClause(
          table: "input_registry",
          column: "table_name",
          joinWith: "process_input.table_name"),
      WhereClause(column: "year", formStateKey: FSK.year),
      WhereClause(column: "month", formStateKey: FSK.month),
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: inputRegistryColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 50,
  ),

  // Domain Table Viewer Data Table
  DTKeys.inputTable: TableConfig(
      key: DTKeys.inputTable,
      fromClauses: [FromClause(schemaName: 'public', tableName: '')],
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
      rowsPerPage: 50),

  // Input File Viewer Data Table
  DTKeys.inputFileViewerTable: TableConfig(
      key: DTKeys.inputFileViewerTable,
      fromClauses: [FromClause(schemaName: 'public', tableName: 's3')],
      label: 'Input File Viewer',
      apiPath: '/dataTable',
      apiAction: 'preview_file',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [
        WhereClause(column: "file_key", formStateKey: FSK.fileKey),
      ],
      actions: [],
      columns: [],
      sortColumnName: '',
      sortAscending: false,
      rowsPerPage: 50),

  // Users Administration Data Table
  DTKeys.usersTable: TableConfig(
    key: DTKeys.usersTable,
    fromClauses: [FromClause(schemaName: 'jetsapi', tableName: 'users')],
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
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: [
      DataTableFormStateOtherColumnConfig(stateKey: FSK.isActive, columnIdx: 2),
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
    config = getWorkspaceTableConfig(key);
    if (config == null) {
      throw Exception(
          'ERROR: Invalid program configuration: table configuration $key not found');
    }
  }
  return config;
}

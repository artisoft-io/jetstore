import 'package:jetsclient/modules/user_flows/client_registry/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/home_filters/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/register_file_key/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/data_table_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/data_table_config.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/modules/rete_session/model_handlers.dart';
import 'package:jetsclient/modules/workspace_ide/data_table_config.dart';

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
      name: "day_period",
      table: "source_period",
      label: 'Day Period',
      tooltips: 'Day Period since begin of epoch',
      isNumeric: false),
  ColumnConfig(
      index: 9,
      name: "last_update",
      label: 'Last Update',
      tooltips: 'When the file was received',
      isNumeric: false),
  ColumnConfig(
      index: 10,
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
          name: "load_count",
          label: 'Records Count',
          tooltips: 'Number of records loaded',
          isNumeric: true),
      ColumnConfig(
          index: 9,
          name: "status",
          label: 'Status',
          tooltips: 'Status of the load',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key',
          isNumeric: false,
          cellFilter: (text) {
            if (text == null) return null;
            if (globalWorkspaceFileKeyLabelRe != null) {
              RegExpMatch? match =
                  globalWorkspaceFileKeyLabelRe!.firstMatch(text);
              if (match != null) {
                return match[1];
              }
            }
            final start = text.lastIndexOf('/');
            if (start >= 0) {
              return '...${text.substring(start)}';
            } else {
              return text;
            }
          }),
      ColumnConfig(
          index: 11,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline Job Key',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "bad_row_count",
          label: 'Bad Records',
          tooltips: 'Number of Bad Records',
          isNumeric: true),
      ColumnConfig(
          index: 13,
          name: "error_message",
          label: 'Error Message',
          tooltips: 'Error that occured during execution',
          isNumeric: false,
          maxLines: 3,
          columnWidth: 400,
          cellFilter: (text) => text?.replaceFirst(
              'File contains 0 bad rows,recovered error: ', '')),
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
          actionType: DataTableActionType.showScreen,
          key: 'startPipeline',
          label: 'Start Pipeline',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          capability: 'run_pipelines',
          configScreenPath: ufStartPipelinePath),
      ActionConfig(
          actionType: DataTableActionType.refreshTable,
          key: 'refreshTable',
          label: 'Refresh',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'setHomeFilters',
          label: 'Set Filters',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configScreenPath: ufHomeFiltersPath),
      ActionConfig(
          actionType: DataTableActionType.clearHomeFilters,
          key: 'clearHomeFilters',
          label: 'Clear Filters',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
    ],
    secondRowActions: [
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
          actionType: DataTableActionType.showScreen,
          key: 'viewProcessErrors',
          label: 'View Process Errors',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: processErrorsPath,
          navigationParams: {'session_id': 10}),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'viewFailureDetails',
          label: 'View Failure Details',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.showFailureDetails,
          navigationParams: {'session_id': 10, 'failure_details': 12}),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'viewExecStatsDetails',
          label: 'View Execution Stats',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          configScreenPath: executionStatsDetailsPath,
          navigationParams: {'session_id': 10}),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.failureDetails,
        columnIdx: 12,
      ),
    ]),
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
          isNumeric: false,
          isHidden: true),
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
          name: "status",
          label: 'Status',
          tooltips: 'Status of the pipeline execution',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "main_input_file_key",
          label: 'Main Input File Key',
          tooltips:
              'Start the process by loading the this file and then execute the rule process',
          isNumeric: false,
          cellFilter: (text) {
            if (text == null) return null;
            if (globalWorkspaceFileKeyLabelRe != null) {
              RegExpMatch? match =
                  globalWorkspaceFileKeyLabelRe!.firstMatch(text);
              if (match != null) {
                return match[1];
              }
            }
            final start = text.lastIndexOf('/');
            if (start >= 0) {
              return '...${text.substring(start)}';
            } else {
              return text;
            }
          }),
      ColumnConfig(
          index: 10,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session identifier',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "input_session_id",
          label: 'Input Session',
          tooltips: 'Input session used (overriding input registry)',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "failure_details",
          label: 'Failure Details',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 13,
          name: "main_input_registry_key",
          label: 'Main Input Registry',
          tooltips:
              'Main input from previously loaded file, this specify the input session id',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 14,
          name: "merged_input_registry_keys",
          label: 'Merge-In Input Registry',
          tooltips:
              'Indicate the session id of the input sources to be merged with the main input source',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 15,
          name: "user_email",
          label: 'User',
          tooltips: 'Who submitted the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 16,
          name: "run_duration",
          label: 'Duration',
          tooltips: 'Run duration',
          calculatedAs: 'AGE(last_update, start_time)',
          isNumeric: false),
      ColumnConfig(
          index: 17,
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
          name: "jets_partition",
          label: 'Jets Partition',
          tooltips: 'CPIPES partition',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "cpipes_step_id",
          label: 'Step Id',
          tooltips: 'CPIPES Step Id',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "input_files_size_mb",
          label: 'Input Files Size (Mb)',
          tooltips: 'Total size in Mb of input files',
          isNumeric: true),
      ColumnConfig(
          index: 12,
          name: "input_records_count",
          label: 'Input Records Count',
          tooltips: 'Number of input records',
          isNumeric: true),
      ColumnConfig(
          index: 13,
          name: "input_bad_records_count",
          label: 'Input Bad Records Count',
          tooltips: 'Number of bad input records',
          isNumeric: true),
      ColumnConfig(
          index: 14,
          name: "rete_sessions_count",
          label: 'Rete Sessions Count',
          tooltips: 'Number of rete sessions',
          isNumeric: true),
      ColumnConfig(
          index: 15,
          name: "output_records_count",
          label: 'Output Records Count',
          tooltips: 'Number of output records',
          isNumeric: true),
      ColumnConfig(
          index: 16,
          name: "main_input_session_id",
          label: 'Input Session ID',
          tooltips: 'Session ID of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 17,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline session ID',
          isNumeric: false),
      ColumnConfig(
          index: 18,
          name: "user_email",
          label: 'User',
          tooltips: 'Who started the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 19,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnName: 'shard_id',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // CPIPES Execution Status Details Data Table
  DTKeys.cpipesExecDetailsTable: TableConfig(
    key: DTKeys.cpipesExecDetailsTable,
    fromClauses: [
      FromClause(
          schemaName: 'jetsapi', tableName: 'cpipes_execution_status_details')
    ],
    label: 'CPIPES Execution Details',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "session_id", formStateKey: FSK.sessionId),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: [
      ColumnConfig(
          index: 0,
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Process executed',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "cpipes_step_id",
          label: 'CPIPES Step Id',
          tooltips: 'Compute Pipes Step Id',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "nbr_nodes",
          label: 'Nbr Nodes',
          tooltips: 'Number of nodes used for the step',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          name: "total_input_files_size_mb",
          label: 'Total Input Files Size (Mb)',
          tooltips: 'Total size in Mb of input files',
          isNumeric: true),
      ColumnConfig(
          index: 4,
          name: "total_input_records_count",
          label: 'Total Input Records Count',
          tooltips: 'Total number of input records',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "total_output_records_count",
          label: 'Total Output Records Count',
          tooltips: 'Total number of output records',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'JetStore session id',
          isNumeric: false),
    ],
    sortColumnName: 'total_input_files_size_mb',
    sortAscending: false,
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
          actionType: DataTableActionType.showDialog,
          key: 'showErrorInputRecords',
          label: 'View Input Records',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.viewInputRecords,
          // Copy state data from formState to dialogFormState
          stateFormNavigationParams: {
            FSK.pipelineExectionStatusKey: FSK.pipelineExectionStatusKey,
            FSK.objectType: FSK.objectType,
            FSK.processName: FSK.processName,
            FSK.domainKey: FSK.domainKey,
          }),
      // Rete Session as Triples V1
      // ActionConfig(
      //     actionType: DataTableActionType.doActionShowDialog,
      //     key: 'showReteTriples',
      //     label: 'View Rete Triples',
      //     style: ActionStyle.secondary,
      //     isVisibleWhenCheckboxVisible: true,
      //     isEnabledWhenHavingSelectedRows: true,
      //     actionName: ActionKeys.setupShowReteTriples,
      //     configForm: FormKeys.viewReteTriples,
      //     // Copy state data from formState to dialogFormState
      //     stateFormNavigationParams: {
      //       // keys that will be set by the Action:
      //       // FSK.reteSessionTriples
      //       FSK.key: DTKeys.processErrorsTable,
      //     }),
      // Rete Session as Triples V2 - Rete Session Explorer
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'showReteTriplesV2',
          label: 'View Rule Session',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.setupShowReteTriplesV2,
          configForm: FormKeys.viewReteTriplesV2,
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

  // View Input Records for a rule process with an exception (from Error Table)
  DTKeys.inputRecordsFromProcessErrorTable: TableConfig(
      key: DTKeys.inputRecordsFromProcessErrorTable,
      label: 'Input Records for Process Errors',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      fromClauses: [
        FromClause(schemaName: 'public', tableName: ''),
        FromClause(schemaName: '', tableName: 'sessions'),
      ],
      withClauses: [
        WithClause(withName: 'sessions', asStatement: """
            SELECT sr.session_id AS sess_id
            FROM
              jetsapi.pipeline_execution_status pe,
              jetsapi.source_period sp,
              jetsapi.session_registry sr,
              "{table_name}" mc
            WHERE pe.key = CASE {lookback_periods} WHEN 0 THEN NULL ELSE {pipeline_execution_status_key} END
              AND pe.source_period_key = sp.key 
              AND sr.session_id = mc.session_id
              AND sr.month_period >= (sp.month_period - {lookback_periods})
              AND sr.month_period <= sp.month_period
            UNION
            SELECT '{session_id}'""", stateVariables: [
          FSK.pipelineExectionStatusKey,
          FSK.tableName,
          FSK.lookbackPeriods,
          FSK.sessionId,
        ]),
      ],
      whereClauses: [
        WhereClause(column: "session_id", joinWith: "sessions.sess_id"),
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

  // View RDFSession Triples as Table V1
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

  // View RDFSession as Table V2 - 1. rdf:type in current session
  DTKeys.reteSessionRdfTypeTable: TableConfig(
      key: DTKeys.reteSessionRdfTypeTable,
      fromClauses: [],
      label: 'Class Name',
      apiPath: '/dataTable',
      modelStateFormKey: FSK.reteSessionRdfTypes,
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [],
      actions: [],
      formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.entityRdfType,
          columnIdx: 0,
        ),
      ]),
      columns: [
        ColumnConfig(
            index: 0,
            name: "entity_rdf_type",
            label: 'Class Name',
            tooltips: 'Entity Class Name Filter',
            isNumeric: false),
      ],
      sortColumnName: 'entity_rdf_type',
      sortAscending: true,
      noFooter: true,
      rowsPerPage: 1000000),

  // View RDFSession as Table V2 - 2. entity_key_by_type
  DTKeys.reteSessionEntityKeyTable: TableConfig(
      key: DTKeys.reteSessionEntityKeyTable,
      fromClauses: [],
      label: 'Entity Key',
      apiPath: '/dataTable',
      modelStateHandler: reteSessionEntityKeyStateHandler,
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [
        WhereClause(column: FSK.entityRdfType, formStateKey: FSK.entityRdfType),
      ],
      actions: [],
      formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.entityKey,
          columnIdx: 0,
        ),
      ]),
      columns: [
        ColumnConfig(
            index: 0,
            name: "entity_key",
            label: 'Entity Key',
            tooltips: 'Entity Key Filter',
            isNumeric: false),
      ],
      sortColumnName: 'entity_key',
      sortAscending: true,
      noFooter: true,
      rowsPerPage: 1000000),

  // View RDFSession as Table V2 - 3. entity_details_by_key
  DTKeys.reteSessionEntityDetailsTable: TableConfig(
      key: DTKeys.reteSessionEntityDetailsTable,
      fromClauses: [],
      label: 'Entity Details',
      apiPath: '/dataTable',
      modelStateHandler: reteSessionEntityDetailsStateHandler,
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [
        WhereClause(column: FSK.entityKey, formStateKey: FSK.entityKey),
      ],
      actions: [
        ActionConfig(
            actionType: DataTableActionType.doAction,
            key: 'visitEntity',
            label: 'Visit Object Entity',
            style: ActionStyle.primary,
            isVisibleWhenCheckboxVisible: true,
            isEnabledWhenHavingSelectedRows: true,
            actionName: ActionKeys.reteSessionVisitEntity),
      ],
      formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.entityProperty,
          columnIdx: 0,
        ),
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.entityPropertyValue,
          columnIdx: 1,
        ),
        DataTableFormStateOtherColumnConfig(
          stateKey: FSK.entityPropertyValueType,
          columnIdx: 2,
        ),
      ]),
      columns: [
        ColumnConfig(
            index: 0,
            name: "entity_property",
            label: 'Property',
            tooltips: 'Property name',
            isNumeric: false),
        ColumnConfig(
            index: 1,
            name: "entity_value",
            label: 'Value',
            tooltips: 'Property value',
            isNumeric: false),
        ColumnConfig(
            index: 2,
            name: "entity_value_type",
            label: 'Type',
            tooltips: 'Value type',
            isNumeric: false),
      ],
      sortColumnName: 'entity_property',
      sortAscending: true,
      noFooter: true,
      rowsPerPage: 1000000),

  // Client Name Table used for Process & Rules Config form
  DTKeys.clientsAndProcessesTableView: TableConfig(
    key: DTKeys.clientsAndProcessesTableView,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_config'),
    ],
    label: 'Select a Rules Process for configuration',
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
    label: 'File Staging Area',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
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
          capability: 'run_pipelines',
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
          capability: 'run_pipelines',
          style: ActionStyle.secondary),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.fileKey,
        columnIdx: 4,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourcePeriodKey,
        columnIdx: 10,
      ),
    ]),
    columns: fileKeyStagingColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 20,
  ),

  // File Key Staging Data Table used to multi-load files
  DTKeys.fileKeyStagingMultiLoadTable: TableConfig(
    key: DTKeys.fileKeyStagingMultiLoadTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'file_key_staging'),
      FromClause(schemaName: '', tableName: 'sp'),
    ],
    label: 'File Keys Selected',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    defaultToAllRows: false, // when where clause fails
    withClauses: [
      WithClause(
          withName: "sp",
          asStatement: '''SELECT sp1.* 
          FROM jetsapi.source_period sp1, jetsapi.source_period sp2 
          WHERE sp1.day_period >= sp2.day_period 
            AND sp2.key = {source_period_key}''',
          stateVariables: [FSK.sourcePeriodKey])
    ],
    distinctOnClauses: ["file_key"],
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "org", formStateKey: FSK.org),
      WhereClause(column: "object_type", formStateKey: FSK.objectType),
      WhereClause(column: "source_period_key", joinWith: "sp.key"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.loaderMultiOk,
          key: 'loadMultiFile',
          label: 'Load Selected Files',
          style: ActionStyle.primary,
          capability: 'run_pipelines',
          isEnabledWhenHavingSelectedRows: true),
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
    sortColumnName: 'file_key',
    sortAscending: false,
    rowsPerPage: 50,
  ),

  // Source Period Table for Load ALL Files Dialog
  FSK.fromSourcePeriodKey: TableConfig(
    key: FSK.fromSourcePeriodKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_period'),
      FromClause(schemaName: 'jetsapi', tableName: 'file_key_staging'),
    ],
    label: 'Select the FROM date to load the files from',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          table: "file_key_staging",
          column: "client",
          formStateKey: FSK.client),
      WhereClause(
          table: "file_key_staging", column: "org", formStateKey: FSK.org),
      WhereClause(
          table: "file_key_staging",
          column: "object_type",
          formStateKey: FSK.objectType),
      WhereClause(
          table: "source_period",
          column: "key",
          joinWith: "file_key_staging.source_period_key"),
    ],
    distinctOnClauses: ["source_period.day_period"],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.fromDayPeriod,
        columnIdx: 4,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          table: "source_period",
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          table: "source_period",
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 2,
          table: "source_period",
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          table: "source_period",
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 4,
          table: "source_period",
          name: "day_period",
          label: 'Day Period',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
    ],
    sortColumnName: 'day_period',
    sortAscending: true,
    rowsPerPage: 50,
  ),
  FSK.toSourcePeriodKey: TableConfig(
    key: FSK.toSourcePeriodKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_period'),
      FromClause(schemaName: 'jetsapi', tableName: 'file_key_staging'),
    ],
    label: 'Select the TO date to load the files from',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          table: "file_key_staging",
          column: "client",
          formStateKey: FSK.client),
      WhereClause(
          table: "file_key_staging", column: "org", formStateKey: FSK.org),
      WhereClause(
          table: "file_key_staging",
          column: "object_type",
          formStateKey: FSK.objectType),
      WhereClause(
          table: "source_period",
          column: "key",
          joinWith: "file_key_staging.source_period_key"),
    ],
    distinctOnClauses: ["source_period.day_period"],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.toDayPeriod,
        columnIdx: 4,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          table: "source_period",
          name: "key",
          label: 'Key',
          tooltips: 'Row Primary Key',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          table: "source_period",
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 2,
          table: "source_period",
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          table: "source_period",
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 4,
          table: "source_period",
          name: "day_period",
          label: 'Day Period',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
    ],
    sortColumnName: 'day_period',
    sortAscending: false,
    rowsPerPage: 50,
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
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
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

  // Rule Configv2 Table to Add/Edit Rule Configuration
  DTKeys.ruleConfigv2Table: TableConfig(
    key: DTKeys.ruleConfigv2Table,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'rule_configv2')
    ],
    label: 'Rules Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'configureRulesv2',
          label: 'Add/Update Rule Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.rulesConfigv2Dialog,
          navigationParams: {
            FSK.key: 0,
            FSK.client: 1,
            FSK.processName: 2,
            FSK.processConfigKey: 3,
            FSK.ruleConfigJson: 4
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteRuleConfig',
          label: 'Delete',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'client_config',
          actionName: ActionKeys.deleteRuleConfigv2),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
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
          name: "rule_config_json",
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
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
      rowsPerPage: 10),

  // Query Tool Result Viewer Data Table
  DTKeys.queryToolResultSetTable: TableConfig(
      key: DTKeys.queryToolResultSetTable,
      fromClauses: [FromClause(schemaName: 'public', tableName: '')],
      apiAction: 'raw_query_tool', // will be overriden for data management stmt
      requestColumnDef: true,
      label: 'Query Result',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [
        WhereClause(column: '', formStateKey: FSK.queryReady),
      ],
      actions: [],
      columns: [],
      sortColumnName: '',
      sortAscending: false,
      rowsPerPage: 1),

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

  // Users Administration Data Table - list of users
  DTKeys.usersTable: TableConfig(
    key: DTKeys.usersTable,
    fromClauses: [FromClause(schemaName: 'jetsapi', tableName: 'users')],
    label: 'Registered Users',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'editUserProfile',
          label: 'Update User Profile',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.editUserProfile,
          navigationParams: {
            FSK.userName: 0,
            FSK.userEmail: 1,
            FSK.isActive: 2,
            FSK.userRoles: 3,
            DTKeys.userRolesTable: 3,
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteUser',
          label: 'Delete User',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'user_profile',
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
          name: "roles",
          label: 'Roles',
          tooltips: 'User Roles',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 4,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Last Updated',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortAscending: true,
    rowsPerPage: 10,
  ),

  // Users Administration Data Table - list of roles
  DTKeys.userRolesTable: TableConfig(
    key: DTKeys.userRolesTable,
    fromClauses: [FromClause(schemaName: 'jetsapi', tableName: 'roles')],
    label: 'Select Roles',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.userRoles, columnIdx: 0),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "role",
          label: 'Role Name',
          tooltips: 'Role to assign to user',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "details",
          label: 'Details',
          tooltips: 'Role details',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Last Updated',
          isNumeric: false),
    ],
    sortColumnName: 'role',
    sortAscending: true,
    rowsPerPage: 20,
  ),
};

TableConfig getTableConfig(String key) {
  var config = _tableConfigurations[key];
  if (config != null) return config;
  config = getWorkspaceTableConfig(key);
  if (config != null) return config;
  config = getClientRegistryTableConfig(key);
  if (config != null) return config;
  config = getConfigureFileTableConfig(key);
  if (config != null) return config;
  config = getPipelineConfigTableConfig(key);
  if (config != null) return config;
  config = getLoadFilesTableConfig(key);
  if (config != null) return config;
  config = getRegisterFileKeyTableConfig(key);
  if (config != null) return config;
  config = getStartPipelineTableConfig(key);
  if (config != null) return config;
  config = getWorkspacePullTableConfig(key);
  if (config != null) return config;
  config = getFileMappingTableConfig(key);
  if (config != null) return config;
  config = getHomeFiltersTableConfig(key);
  if (config != null) return config;
  throw Exception(
      'ERROR: Invalid program configuration: table configuration $key not found');
}

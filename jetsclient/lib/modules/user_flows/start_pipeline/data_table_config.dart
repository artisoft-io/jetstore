import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

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
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false),
  ColumnConfig(
      index: 8,
      table: "input_registry",
      name: "session_id",
      label: 'Session ID',
      tooltips: 'Session ID of the file load job',
      isNumeric: false),
  ColumnConfig(
      index: 9,
      table: "input_registry",
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 10,
      table: "input_registry",
      name: "file_key",
      label: 'File Key',
      tooltips: 'File Key of the loaded file',
      isNumeric: false,
          cellFilter: (text) {
            if(text == null) return null;
            return '...${text.substring(text.lastIndexOf('/'))}';
          }),
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


final baseProcessInputColumns = [
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
];

final processInputColumns = baseProcessInputColumns + [
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

/// Source Config User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {

  // Pipeline Config Data Table for Pipeline Execution Dialog
  FSK.pipelineConfigKey: TableConfig(
    key: FSK.pipelineConfigKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_config'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_input'),
    ],
    label: 'Select a Pipeline Configuration',
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
          stateKey: FSK.description, columnIdx: 8),
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
    sortColumnName: 'process_name',
    sortAscending: true,
    rowsPerPage: 100,
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
          stateKey: FSK.mainInputFileKey, columnIdx: 10),
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
          navigationParams: {'table_name': 7, 'session_id': 8}),
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


  // Summary of selected data sources
  DTKeys.spSummaryDataSources: TableConfig(
    key: DTKeys.spSummaryDataSources,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry')
    ],
    label: 'Data Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    noFooter: true,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.spAllDataSourceKeys),
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
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Pipeline Grouping Domain Key',
      isNumeric: false),
  ColumnConfig(
      index: 2,
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false,
      isHidden: false),
  ColumnConfig(
      index: 4,
      name: "file_key",
      label: 'File Key',
      tooltips: 'File Key from s3',
      isNumeric: false),
    ],
    sortColumnName: 'source_type',
    sortAscending: false,
    rowsPerPage: 100,
  ),

  // for showing the injected data source
  DTKeys.spInjectedProcessInput: TableConfig(
    key: DTKeys.spInjectedProcessInput,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Historical Data Sources',
    apiPath: '/dataTable',
    noFooter: true,
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.injectedProcessInputKeys),
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
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Pipeline Grouping Domain Key',
      isNumeric: false),
  ColumnConfig(
      index: 2,
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false,
      isHidden: false),
  ColumnConfig(
      index: 4,
      name: "lookback_periods",
      label: 'Lookback Periods',
      tooltips: 'Number of periods included in the rule session',
      isNumeric: true),

    ],
    sortColumnName: 'source_type',
    sortAscending: false,
    rowsPerPage: 20,
  ),
};

TableConfig? getStartPipelineTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

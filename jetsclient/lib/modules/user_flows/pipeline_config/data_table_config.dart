import 'package:jetsclient/modules/user_flows/start_pipeline/data_table_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Source Config User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
  // Static choice table
  FSK.pcAddOrEditPipelineConfigOption: TableConfig(
      key: FSK.pcAddOrEditPipelineConfigOption,
      fromClauses: [],
      label: 'Select one of the following options:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        ['New Pipeline Configuration', 'ufAddOption', '0'],
        ['Edit an Existing Pipeline Configuration', 'ufEditOption', '1'],
      ],
      formStateConfig:
          DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: []),
      columns: [
        ColumnConfig(
            index: 0,
            name: "option_description",
            label: 'Select one of the following option',
            tooltips: 'Select one of the option',
            isNumeric: false),
        ColumnConfig(
            index: 1,
            name: "option",
            label: '',
            tooltips: '',
            isNumeric: true,
            isHidden: true),
        ColumnConfig(
            index: 2,
            name: "option_order",
            label: '',
            tooltips: '',
            isNumeric: true,
            isHidden: true),
      ],
      sortColumnName: 'option_order',
      sortAscending: true,
      noFooter: true,
      noCopy2Clipboard: true,
      rowsPerPage: 1000000),

  // Pipeline Config Data Table for Pipeline Config Forms
  DTKeys.pcPipelineConfigTable: TableConfig(
    key: DTKeys.pcPipelineConfigTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'pipeline_config'),
      FromClause(schemaName: 'jetsapi', tableName: 'process_config'),
    ],
    label: 'Select a Pipeline Configuration',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          column: 'process_name',
          table: 'pipeline_config',
          joinWith: 'process_config.process_name')
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deletePipelineConfig',
          label: 'Delete',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'client_config',
          actionName: ActionKeys.deletePipelineConfig),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.client,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processName,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.processConfigKey,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainProcessInputKey,
        columnIdx: 4,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mergedProcessInputKeys,
        columnIdx: 5,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainObjectType,
        columnIdx: 6,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.mainSourceType,
        columnIdx: 7,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourcePeriodType,
        columnIdx: 8,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.automated,
        columnIdx: 9,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.description,
        columnIdx: 10,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.maxReteSessionSaved,
        columnIdx: 11,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.injectedProcessInputKeys,
        columnIdx: 12,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.ruleConfigJson,
        columnIdx: 13,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.entityRdfType,
        columnIdx: 14,
      ),
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
          tooltips: 'Source type of main input table',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "source_period_type",
          table: "pipeline_config",
          label: 'Pipeline Frequency',
          tooltips: 'How often the pipeline execute',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 9,
          name: "automated",
          table: "pipeline_config",
          label: 'Automated',
          tooltips: 'Is pipeline automated? (true: 1, false: 0)',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "description",
          table: "pipeline_config",
          label: 'Description',
          tooltips: 'Pipeline description',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "max_rete_sessions_saved",
          table: "pipeline_config",
          label: 'Max Rete Session Saved',
          tooltips: 'Max Rete Session Saved',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 12,
          name: "injected_process_input_keys",
          table: "pipeline_config",
          label: 'Data Sources of Historical Data',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 13,
          name: "rule_config_json",
          table: "pipeline_config",
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 14,
          name: "input_rdf_types",
          table: "process_config",
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 15,
          name: "last_update",
          table: "pipeline_config",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'process_name',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfigEditForm)
  // for selecting FSK.mainProcessInputKey
  DTKeys.pcMainProcessInputKey: TableConfig(
    key: DTKeys.pcMainProcessInputKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Select the Main Data Source',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(
          column: "source_type", defaultValue: ['file', 'domain_table']),
      WhereClause(column: "entity_rdf_type", formStateKey: FSK.entityRdfType),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'addProcessInput',
          label: 'Add/Update Data Source Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          actionName: ActionKeys.pcSetProcessInputRegistryKey,
          configForm: FormKeys.pcNewProcessInputDialog,
          navigationParams: {
            FSK.key: 0,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
            FSK.whereSourceType: '{file,domain_table}',
          },
          stateFormNavigationParams: {
            FSK.client: FSK.client,
            FSK.processName: FSK.processName,
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

  // Process Input Registry Table for New Process Input Dialog (FormKeys.pcNewProcessInputDialog)
  DTKeys.pcProcessInputRegistry: TableConfig(
    key: DTKeys.pcProcessInputRegistry,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input_registry')
    ],
    // label: 'Select a Process Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    // defaultToAllRows: true,
    whereClauses: [
      WhereClause(
          column: "client",
          formStateKey: FSK.client,
          orWith: WhereClause(column: "client", defaultValue: [''])),
      WhereClause(column: "source_type", formStateKey: FSK.whereSourceType),
      WhereClause(column: "process_name", formStateKey: FSK.processName),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.org,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.objectType,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.entityRdfType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourceType,
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
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "org",
          label: 'Vendor',
          tooltips: 'Client' 's vendor the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Domain Key',
          tooltips: 'Pipeline Grouping Domain Key',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'Canonical model for the source data',
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
          isNumeric: false,
          isHidden: false),
    ],
    sortColumnName: 'table_name',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Process Input Registry Table for New Process Input Dialog (FormKeys.pcNewProcessInputDialog4MI)
  // This version of the table is for Merged and Injected Process Inputs (it has
  // and additional WhereClause on the object_type to match the FSK.mainObjectType)
  DTKeys.pcProcessInputRegistry4MI: TableConfig(
    key: DTKeys.pcProcessInputRegistry4MI,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input_registry')
    ],
    // label: 'Select a Process Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    // defaultToAllRows: true,
    whereClauses: [
      WhereClause(
          column: "client",
          formStateKey: FSK.client,
          orWith: WhereClause(column: "client", defaultValue: [''])),
      WhereClause(column: "source_type", formStateKey: FSK.whereSourceType),
      WhereClause(column: "process_name", formStateKey: FSK.processName),
      WhereClause(column: "object_type", formStateKey: FSK.objectType),
    ],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      // NOTE: Do not set using stateKey same as in WhereClause
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.org,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.entityRdfType,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourceType,
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
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "org",
          label: 'Vendor',
          tooltips: 'Client' 's vendor the file came from',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "object_type",
          label: 'Domain Key',
          tooltips: 'Pipeline Grouping Domain Key',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "entity_rdf_type",
          label: 'Domain Class',
          tooltips: 'Canonical model for the source data',
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
          isNumeric: false,
          isHidden: false),
    ],
    sortColumnName: 'table_name',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // for selecting FSK.mergedProcessInputKeys
  DTKeys.pcViewMergedProcessInputKeys: TableConfig(
    key: DTKeys.pcViewMergedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Merged Data Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.mergedProcessInputKeys),
    ],
    refreshOnKeyUpdateEvent: [
      DTKeys.pcMergedProcessInputKeys,
      FSK.mergedProcessInputKeys
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.toggleCheckboxVisible,
          key: 'toggleRowSelection',
          label: 'Show/Hide Select Row',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.pcRemoveMergedProcessInput,
          key: 'removeMergedProcessInput',
          label: 'Remove',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // for selecting FSK.injectedProcessInputKeys
  DTKeys.pcViewInjectedProcessInputKeys: TableConfig(
    key: DTKeys.pcViewInjectedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Data Sources of Historical Data',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.injectedProcessInputKeys),
    ],
    refreshOnKeyUpdateEvent: [
      DTKeys.pcInjectedProcessInputKeys,
      FSK.injectedProcessInputKeys
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.toggleCheckboxVisible,
          key: 'toggleRowSelection',
          label: 'Show/Hide Select Row',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.pcRemoveInjectedProcessInput,
          key: 'removeInjectedProcessInput',
          label: 'Remove',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // for selecting FSK.injectedProcessInputKeys
  DTKeys.pcSummaryProcessInputs: TableConfig(
    key: DTKeys.pcSummaryProcessInputs,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Data Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    noFooter: true,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.ufAllProcessInputKeys),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 100,
  ),

  // for selecting FSK.mergedProcessInputKeys
  DTKeys.pcMergedProcessInputKeys: TableConfig(
    key: DTKeys.pcMergedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Select a Data Source to Merge',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
      WhereClause(
          column: "source_type", defaultValue: ['file', 'domain_table']),
      WhereClause(column: "entity_rdf_type", formStateKey: FSK.entityRdfType),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'addProcessInput',
          label: 'Add/Update Data Source Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          actionName: ActionKeys.pcSetProcessInputRegistryKey,
          configForm: FormKeys.pcNewProcessInputDialog4MI,
          navigationParams: {
            FSK.key: 0,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
            FSK.whereSourceType: '{file,domain_table}',
          },
          stateFormNavigationParams: {
            FSK.client: FSK.client,
            FSK.processName: FSK.processName,
            FSK.objectType: FSK.mainObjectType,
          }),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // for selecting FSK.injectedProcessInputKeys
  DTKeys.pcInjectedProcessInputKeys: TableConfig(
    key: DTKeys.pcInjectedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Select a Data Source to Inject',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "object_type", formStateKey: FSK.mainObjectType),
      WhereClause(column: "source_type", defaultValue: ['alias_domain_table']),
      WhereClause(column: "entity_rdf_type", formStateKey: FSK.entityRdfType),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doActionShowDialog,
          key: 'addProcessInput',
          label: 'Add/Update Data Source Configuration',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          actionName: ActionKeys.pcSetProcessInputRegistryKey,
          configForm: FormKeys.pcNewProcessInputDialog4MI,
          navigationParams: {
            FSK.key: 0,
            FSK.org: 2,
            FSK.objectType: 3,
            FSK.entityRdfType: 4,
            FSK.sourceType: 5,
            FSK.tableName: 6,
            FSK.lookbackPeriods: 7,
            FSK.whereSourceType: '{alias_domain_table}',
          },
          stateFormNavigationParams: {
            FSK.client: FSK.client,
            FSK.processName: FSK.processName,
            FSK.objectType: FSK.mainObjectType,
            FSK.sourceType: FSK.sourceType,
          }),
    ],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
    columns: processInputColumns,
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),
};

TableConfig? getPipelineConfigTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

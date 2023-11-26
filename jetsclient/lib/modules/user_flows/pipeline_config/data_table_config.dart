import 'package:jetsclient/modules/data_table_config_impl.dart';
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
      formStateConfig: DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: []),
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
      rowsPerPage: 1000000),


  // Pipeline Config Data Table for Pipeline Config Forms
  DTKeys.pcPipelineConfigTable: TableConfig(
    key: DTKeys.pcPipelineConfigTable,
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
          isNumeric: false,
          isHidden: true),
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
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 12,
          name: "injected_process_input_keys",
          label: 'Injected Data Process Inut',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 13,
          name: "rule_config_json",
          label: '',
          tooltips: '',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 14,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the record was created',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 50,
  ),

  // Process Input Table for Pipeline Config Dialog (FormKeys.pipelineConfigEditForm)
  // for selecting FSK.mainProcessInputKey
  DTKeys.pcMainProcessInputKey: TableConfig(
    key: DTKeys.pcMainProcessInputKey,
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
    actions: [],
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


  // for selecting FSK.mergedProcessInputKeys
  DTKeys.pcViewMergedProcessInputKeys: TableConfig(
    key: DTKeys.pcViewMergedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Merged Process Inputs',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          column: "key", formStateKey: FSK.mergedProcessInputKeys),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.pcRemoveMergedProcessInput,
          key: 'removeMergedProcessInput',
          label: 'Remove',
          style: ActionStyle.primary,
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
    label: 'Injected Process Inputs',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(
          column: "key", formStateKey: FSK.injectedProcessInputKeys),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.pcRemoveInjectedProcessInput,
          key: 'removeInjectedProcessInput',
          label: 'Remove',
          style: ActionStyle.primary,
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

  // for selecting FSK.mergedProcessInputKeys
  DTKeys.pcMergedProcessInputKeys: TableConfig(
    key: DTKeys.pcMergedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Select a Process Input to Merge',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
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

  // for selecting FSK.injectedProcessInputKeys
  DTKeys.pcInjectedProcessInputKeys: TableConfig(
    key: DTKeys.pcInjectedProcessInputKeys,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Select a Process Input to Inject',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
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

};

TableConfig? getPipelineConfigTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Client Registry User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {

  // Input Source Mapping: use Source Config to select table
  DTKeys.fmInputSourceMappingUF: TableConfig(
    key: DTKeys.fmInputSourceMappingUF,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_config'),
      FromClause(schemaName: 'jetsapi', tableName: 'object_type_registry'),
    ],
    label: 'Select a Data Source',
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

  // File Mapping Data Table
  DTKeys.fmFileMappingTableUF: TableConfig(
    key: DTKeys.fmFileMappingTableUF,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_mapping')
    ],
    label: 'File Mapping',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    whereClauses: [
      WhereClause(column: "table_name", formStateKey: FSK.tableName)
    ],
    actions: [
      // ActionConfig(
      //     actionType: DataTableActionType.showDialog,
      //     key: 'configureMapping',
      //     label: 'Mapping Dialog',
      //     style: ActionStyle.primary,
      //     isEnabledWhenStateHasKeys: [
      //       FSK.tableName,
      //       FSK.objectType,
      //     ],
      //     configForm: FormKeys.fmMappingFormUF,
      //     // configForm: FormKeys.processMapping,
      //     stateFormNavigationParams: {
      //       FSK.tableName: FSK.tableName,
      //       FSK.objectType: FSK.objectType,
      //     }),
      ActionConfig(
          actionType: DataTableActionType.showScreen,
          key: 'configureMappingPage',
          label: 'Configure Mapping',
          style: ActionStyle.secondary,
          isEnabledWhenStateHasKeys: [
            FSK.tableName,
            FSK.objectType,
          ],
          configForm: FormKeys.fmMappingFormUF,
          // configForm: FormKeys.processMapping,
          configScreenPath: ufMappingPath,
          stateFormNavigationParams: {
            FSK.tableName: FSK.tableName,
            FSK.objectType: FSK.objectType,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'loadRawRows',
          label: 'Paste File Mapping',
          style: ActionStyle.primary,
          capability: 'client_config',
          configForm: FormKeys.loadRawRows),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'downloadMappingRows',
          label: 'Download File Mapping',
          style: ActionStyle.primary,
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

};

TableConfig? getFileMappingTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

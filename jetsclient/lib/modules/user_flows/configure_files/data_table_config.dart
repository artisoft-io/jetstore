import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Source Config User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
  // Static choice table
  FSK.scAddOrEditSourceConfigOption: TableConfig(
      key: FSK.scAddOrEditSourceConfigOption,
      fromClauses: [],
      label: 'Select one of the following options:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        ['Create a file configuration', 'ufAddOption', '0'],
        ['Edit an existing file configuration', 'ufEditOption', '1'],
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

  // Table to Select a an existing Source Config
  // Source Config Table
  FSK.scSourceConfigKey: TableConfig(
    key: FSK.scSourceConfigKey,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_config')
    ],
    label: 'Select a Data Source',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.dropTable,
          key: 'dropStagingTable',
          label: 'Drop Staging Table',
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'run_pipelines',
          style: ActionStyle.primary),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteSourceConfig',
          label: 'Delete',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'client_config',
          actionName: ActionKeys.deleteSourceConfig),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
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
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.domainKeysJson,
        columnIdx: 6,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.codeValuesMappingJson,
        columnIdx: 7,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.inputColumnsJson,
        columnIdx: 8,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.inputColumnsPositionsCsv,
        columnIdx: 9,
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
          isNumeric: true,
          isHidden: true),
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
          isHidden: true),
      ColumnConfig(
          index: 7,
          name: "code_values_mapping_json",
          label: 'Code Value Mapping (json)',
          tooltips:
              'Client-specific code values mapping to canonical codes (json-encoded string)',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 8,
          name: "input_columns_json",
          label: 'Input Columns (json)',
          tooltips:
              'Column names for HEADERLESS FILES ONLY (json-encoded string)',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 9,
          name: "input_columns_positions_csv",
          label: 'Fixed-Width Column Positions (csv)',
          tooltips: 'Column names & position for FIXED-WIDTH ONLY (csv)',
          isNumeric: false,
          isHidden: true),
      ColumnConfig(
          index: 10,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Indicates when the record was last updated',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 20,
  ),

  // File Structure Options: Csv, Headerless CSV or Fixed-width
  FSK.scCsvOrFixedOption: TableConfig(
      key: FSK.scCsvOrFixedOption,
      fromClauses: [],
      label: 'Select one of the following options:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        ['CSV file with headers (most common)', 'scCsvOption', '0'],
        ['Headerless CSV file', 'scHeaderlessCsvOption', '1'],
        ['Fixed-width file', 'scFixedWidthOption', '2'],
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
};

TableConfig? getConfigureFileTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

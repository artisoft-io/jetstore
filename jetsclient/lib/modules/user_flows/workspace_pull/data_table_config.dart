import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Source Config User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
  // Static choice table
  DTKeys.otherWorkspaceActionOptions: TableConfig(
      key: DTKeys.otherWorkspaceActionOptions,
      fromClauses: [],
      label: 'Select any of the following options:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: false,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        [
          'Compile workspace (when pulling data model, rule, or export/report changes)',
          'wpCompileWorkspaceOption',
          '0'
        ],
        [
          'Load ALL client configurations (when pulling client configuration changes)',
          'wpLoadClientConfgOption',
          '1'
        ],
        [
          'Load SELECTED client configurations (click next to select the clients)',
          'wpLoadSelectedClientConfgOption',
          '2'
        ],
      ],
      formStateConfig:
          DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: []),
      columns: [
        ColumnConfig(
            index: 0,
            name: "option_description",
            label: 'Select any of the applicable option',
            tooltips: 'Select all the applicable option',
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
  // Confirm choice table
  DTKeys.wpPullWorkspaceConfirmOptions: TableConfig(
      key: DTKeys.wpPullWorkspaceConfirmOptions,
      fromClauses: [],
      label: 'The following action will be taken once the workspace is pulled:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: false,
      isReadOnly: true,
      showSelectedOnly: true,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        ['Workspace will be compiled', 'wpCompileWorkspaceOption', '0'],
        [
          'ALL client configuration will be loaded into database',
          'wpLoadClientConfgOption',
          '1'
        ],
        [
          'SELECTED client configuration will be loaded into database',
          'wpLoadSelectedClientConfgOption',
          '2'
        ],
      ],
      formStateConfig:
          DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: []),
      columns: [
        ColumnConfig(
            index: 0,
            name: "option_description",
            label: 'Action to be taken once workspace is refreshed',
            tooltips: 'Go to previouos page to update the selected actions',
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

  // Load Client Config UF Tables
  FSK.wpClientList: TableConfig(
    key: FSK.wpClientList,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry')
    ],
    label: 'Select Clients',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    noCopy2Clipboard: true,
    whereClauses: [],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
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
    rowsPerPage: 100,
  ),
  FSK.wpClientListRO: TableConfig(
    key: FSK.wpClientListRO,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry')
    ],
    label: 'Clients Selected',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    noCopy2Clipboard: true,
    isReadOnly: true,
    showSelectedOnly: true,
    whereClauses: [],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
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
    rowsPerPage: 100,
  ),

};

TableConfig? getWorkspacePullTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

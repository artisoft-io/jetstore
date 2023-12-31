import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Client Registry User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
  // Static choice table
  FSK.ufClientOrVendorOption: TableConfig(
      key: FSK.ufClientOrVendorOption,
      fromClauses: [],
      label: 'Select one of the following options:',
      apiPath: '',
      isCheckboxVisible: true,
      isCheckboxSingleSelect: true,
      whereClauses: [],
      actions: [],
      staticTableModel: [
        ['Create a client and add vendors', 'ufClientOption', '0'],
        ['Add vendors to an existing client', 'ufVendorOption', '1'],
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
      noCopy2Clipboard: true,
      rowsPerPage: 1000000),

  // Client Table for Selecting Client
  FSK.client: TableConfig(
    key: FSK.client,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_registry')
    ],
    label: 'Select a Clients',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteClient',
          label: 'Delete Client',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'client_config',
          actionName: ActionKeys.deleteClient),
    ],
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
      ColumnConfig(
          index: 2,
          name: "last_update",
          label: 'Last Updated At',
          tooltips: 'Indicates when the record was last updated',
          isNumeric: false),
    ],
    sortColumnName: 'client',
    sortAscending: true,
    rowsPerPage: 100,
  ),

  // Org Table 
  FSK.org: TableConfig(
    key: FSK.org,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'client_org_registry')
    ],
    label: 'Client Organization Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    defaultToAllRows: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addVendorDialog',
          label: 'Add Vendor/Org',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.ufVendor,
          capability: 'client_config',
          stateFormNavigationParams: {
            FSK.client: FSK.client,
          }),
      ActionConfig(
          actionType: DataTableActionType.toggleCheckboxVisible,
          key: 'toggleRowSelection',
          label: 'Show/Hide Select Row',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteOrg',
          label: 'Delete Organization',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'client_config',
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
    rowsPerPage: 50,
  ),
};

TableConfig? getClientRegistryTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

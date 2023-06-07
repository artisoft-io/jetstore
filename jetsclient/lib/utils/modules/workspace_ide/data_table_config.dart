import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';


final Map<String, TableConfig> _tableConfigurations = {
  // Workspace Registry
  DTKeys.workspaceRegistryTable: TableConfig(
    key: DTKeys.workspaceRegistryTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'workspace_registry')
    ],
    label: 'Workspace Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addWorkspace',
          label: 'Add Workspace',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addWorkspace),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'compileWorkspace',
          label: 'Compile Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          // actionName: ActionKeys.compileWorkspace
          ),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteWorkspace',
          label: 'Delete Workspace',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          // actionName: ActionKeys.deleteWorkspace
          ),
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

};

TableConfig? getWorkspaceTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

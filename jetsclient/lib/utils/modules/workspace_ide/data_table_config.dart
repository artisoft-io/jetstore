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
          label: 'Add/Update Workspace',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsURI: 2,
            FSK.description: 3,
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'compileWorkspace',
          label: 'Compile Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.compileWorkspace),
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
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsName,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsURI,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.description,
        columnIdx: 3,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "workspace_name",
          label: 'Workspace Name',
          tooltips: 'Workspace identifier',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "workspace_uri",
          label: 'Workspace Repo',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "description",
          label: 'Description',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "user_email",
          label: 'User Email',
          tooltips: 'User who made the last change',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'Last time the workspace was compiled',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

};

TableConfig? getWorkspaceTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

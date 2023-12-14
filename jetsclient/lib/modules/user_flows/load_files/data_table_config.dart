import 'package:jetsclient/modules/data_table_config_impl.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Load Files User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {

  // Source Config Table
  DTKeys.lfSourceConfigTable: TableConfig(
    key: DTKeys.lfSourceConfigTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'source_config')
    ],
    label: 'Select a File Data Source Configurations',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          actionName: ActionKeys.lfDropTable,
          key: 'dropStagingTable',
          label: 'Drop Staging Table',
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          capability: 'run_pipelines',
          style: ActionStyle.primary),
    ],
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
          label: 'Vendor',
          tooltips: 'Client' 's Vendor/Organization',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "object_type",
          label: 'Object Type',
          tooltips: 'Type of objects in file',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "table_name",
          label: 'Staging Table Name',
          tooltips: 'Table where to load the file',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 5,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Indicates when the record was last updated',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 20,
  ),


  // File Key Staging Data Table used to load files
  DTKeys.lfFileKeyStagingTable: TableConfig(
    key: DTKeys.lfFileKeyStagingTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'file_key_staging'),
      FromClause(schemaName: 'jetsapi', tableName: 'source_period'),
    ],
    label: 'Select File(s) to Load',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "client", formStateKey: FSK.client),
      WhereClause(column: "org", formStateKey: FSK.org),
      WhereClause(column: "object_type", formStateKey: FSK.objectType),
      WhereClause(column: "source_period_key", joinWith: "source_period.key"),
    ],
    actions: [
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
          actionName: ActionKeys.lfSyncFileKey,
          key: 'syncFileKey',
          label: 'Sync File Keys',
          capability: 'run_pipelines',
          style: ActionStyle.primary),
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
};

TableConfig? getLoadFilesTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

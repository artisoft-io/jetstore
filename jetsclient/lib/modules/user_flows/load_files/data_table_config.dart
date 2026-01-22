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
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry'),
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
      WhereClause(column: "source_type", defaultValue: ['file']),
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
        stateKey: FSK.inputRegistrySessionId,
        columnIdx: 10,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sourcePeriodKey,
        columnIdx: 12,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          table: "input_registry",
          name: "key",
          label: 'Primary Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Client providing the input files',
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
          tooltips: 'The type of object the file contains',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "file_key",
          label: 'File Key',
          tooltips: 'File key or path',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "year",
          label: 'Year',
          tooltips: 'Year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "month",
          label: 'Month',
          tooltips: 'Month of the year the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 7,
          name: "day",
          label: 'Day',
          tooltips: 'Day of the month the file was received',
          isNumeric: true),
      ColumnConfig(
          index: 8,
          name: "day_period",
          table: "source_period",
          label: 'Day Period',
          tooltips: 'Day Period since begin of epoch',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false),
      ColumnConfig(
          index: 10,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Session ID of the file upload',
          isNumeric: false),
      ColumnConfig(
          index: 11,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the file was received',
          isNumeric: false),
      ColumnConfig(
          index: 12,
          name: "source_period_key",
          label: 'Source Period Key',
          tooltips: '',
          isHidden: true,
          isNumeric: true),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 20,
  ),
};

TableConfig? getLoadFilesTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

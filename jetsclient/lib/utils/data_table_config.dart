import 'package:flutter/material.dart';

import 'package:jetsclient/utils/constants.dart';

enum ActionStyle { primary, secondary, danger }

class TableConfig {
  TableConfig(
      {required this.key,
      required this.schemaName,
      required this.tableName,
      this.label = "",
      required this.apiPath,
      required this.isCheckboxVisible,
      required this.isCheckboxSingleSelect,
      required this.actions,
      required this.columns,
      required this.whereClauses,
      this.formStateConfig,
      required this.sortColumnIndex,
      required this.sortAscending,
      required this.rowsPerPage});
  final String key;
  final String schemaName;
  final String tableName;
  final String label;
  final String apiPath;
  final bool isCheckboxVisible;
  final bool isCheckboxSingleSelect;
  final List<ActionConfig> actions;
  final List<ColumnConfig> columns;
  final List<WhereClause> whereClauses;
  final DataTableFormStateConfig? formStateConfig;
  final int sortColumnIndex;
  final bool sortAscending;
  final int rowsPerPage;
}

class ActionConfig {
  ActionConfig(
      {required this.key,
      required this.label,
      this.isTableEditablePrecondition,
      this.isEnabledWhenTableEditablePrecondition,
      required this.style,
      this.configTable,
      this.configForm,
      this.apiKey});
  final String key;
  final String label;
  final bool? isTableEditablePrecondition;
  final bool? isEnabledWhenTableEditablePrecondition;
  final ActionStyle style;
  final String? configTable;
  final String? configForm;
  final String? apiKey;

  bool predicate(bool isTableEditable) {
    if (isTableEditablePrecondition != null) {
      return isTableEditablePrecondition == isTableEditable;
    }
    return true;
  }

  bool isEnabled(bool isTableEditable) {
    if (isEnabledWhenTableEditablePrecondition != null) {
      return isEnabledWhenTableEditablePrecondition == isTableEditable;
    }
    return true;
  }

  ButtonStyle buttonStyle(ThemeData td) {
    switch (style) {
      case ActionStyle.danger:
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onErrorContainer,
          backgroundColor: td.colorScheme.errorContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

      case ActionStyle.secondary:
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onPrimaryContainer,
          backgroundColor: td.colorScheme.primaryContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

      default: // primary
        return ElevatedButton.styleFrom(
          foregroundColor: td.colorScheme.onSecondaryContainer,
          backgroundColor: td.colorScheme.secondaryContainer,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));
    }
  }
}

class ColumnConfig {
  ColumnConfig({
    required this.index,
    required this.name,
    required this.label,
    required this.tooltips,
    required this.isNumeric,
  });
  final int index;
  final String name;
  final String label;
  final String tooltips;
  final bool isNumeric;
}

class WhereClause {
  WhereClause({
    required this.column,
    this.formStateKey,
    this.defaultValue = const [],
  });
  final String column;
  final String? formStateKey;
  final List<String> defaultValue;
}

class DataTableFormStateConfig {
  DataTableFormStateConfig(
      {required this.keyColumnIdx, required this.otherColumns});
  final int keyColumnIdx;
  final List<DataTableFormStateOtherColumnConfig> otherColumns;
}

class DataTableFormStateOtherColumnConfig {
  DataTableFormStateOtherColumnConfig({
    required this.stateKey,
    required this.columnIdx,
  });
  final String stateKey;
  final int columnIdx;
}

final Map<String, TableConfig> _tableConfigurations = {
  //* DEMO FORM - DEMO MAIN DATA TABLE
  "dataTableDemoMainTableConfig": TableConfig(
    key: "dataTableDemoMainTableConfig",
    schemaName: 'jetsapi',
    tableName: 'process_input',
    label: 'Client Input',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          key: 'new',
          label: 'New Row',
          style: ActionStyle.primary,
          configTable: "processConfigTable",
          configForm: "newPipeline"),
      ActionConfig(
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isTableEditablePrecondition: false),
      ActionConfig(
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isTableEditablePrecondition: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true),
    ],
    // FORM STATE CONFIG
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: "dataTableDemoClient",
        columnIdx: 1,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Input Data Table Name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "client",
          label: 'Client',
          tooltips: 'Secondary Key',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "input_type",
          label: 'Input Type',
          tooltips: 'Table Type',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "entity_rdf_type",
          label: 'RDF Type',
          tooltips: 'Entity rdf type',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "grouping_column",
          label: 'Grouping Column',
          tooltips: 'Input record grouping column',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "key_column",
          label: 'Key Column',
          tooltips: 'Input record key column',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "user_email",
          label: 'User',
          tooltips: 'User who created the record',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the record was created or last update',
          isNumeric: false),
    ],
    sortColumnIndex: 0,
    sortAscending: true,
    rowsPerPage: 10,
  ),
  //* DEMO FORM - DEMO SUPPORT DATA TABLE
  "dataTableDemoSupportTableConfig": TableConfig(
    key: "dataTableDemoSupportTableConfig",
    schemaName: 'jetsapi',
    tableName: 'process_mapping',
    label: 'Input Mapping',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "table_name", formStateKey: "dataTableDemoMainTable")
    ],
    actions: [
      ActionConfig(
          key: 'new',
          label: 'New Row',
          style: ActionStyle.primary,
          configTable: "processConfigTable",
          configForm: "newPipeline"),
      ActionConfig(
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isTableEditablePrecondition: false),
      ActionConfig(
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isTableEditablePrecondition: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true),
    ],
    // FORM STATE CONFIG
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: "dataProperties",
        columnIdx: 3,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Primary Key',
          tooltips: 'Sequence Primary Key',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Input Data Table Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "input_column",
          label: 'Input Column',
          tooltips: 'Input column',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "data_property",
          label: 'Data Property',
          tooltips: 'Entity data property',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "function_name",
          label: 'Mapping Function',
          tooltips: 'Function applied to input data',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "argument",
          label: 'Argument',
          tooltips: 'Argument for mapping function',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "default_value",
          label: 'Default',
          tooltips: 'Default value if the mapping function does not yield anything',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "error_message",
          label: 'Error Message',
          tooltips: 'Alternate to default value, generate an error if no data is available',
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "user_email",
          label: 'User',
          tooltips: 'User who created or last updated the record',
          isNumeric: false),
      ColumnConfig(
          index: 9,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'When the record was created or last update',
          isNumeric: false),
    ],
    sortColumnIndex: 2,
    sortAscending: true,
    rowsPerPage: 10,
  ),
  //* DEMO ** TABLE ** CODE
  DTKeys.pipelineDemo: TableConfig(
    key: DTKeys.pipelineDemo,
    schemaName: 'jetsapi',
    tableName: 'pipelines',
    label: 'Data Pipeline',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          key: 'new',
          label: 'New Row',
          style: ActionStyle.primary,
          configTable: "processConfigTable",
          configForm: "newPipeline"),
      ActionConfig(
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isTableEditablePrecondition: false),
      ActionConfig(
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isTableEditablePrecondition: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true),
    ],
    // no need for formState here since isCheckboxVisible is false
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: 'Process Session ID',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "user_name",
          label: 'Submitted By',
          tooltips: 'Submitted By',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "client",
          label: 'Client',
          tooltips: 'Client',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "process",
          label: 'Process',
          tooltips: 'Process',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "status",
          label: 'Status',
          tooltips: 'Execution Status',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "submitted_at",
          label: 'Submitted At',
          tooltips: 'Submitted At',
          isNumeric: false),
    ],
    sortColumnIndex: 0,
    sortAscending: true,
    rowsPerPage: 10,
  ),
  //* DEMO CODE
  DTKeys.registryDemo: TableConfig(
    key: DTKeys.registryDemo,
    schemaName: 'jetsapi',
    tableName: 'input_registry',
    label: 'Input File Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          key: 'new',
          label: 'Load New File',
          style: ActionStyle.primary,
          configTable: "processConfigTable",
          configForm: "newPipeline"),
      ActionConfig(
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isTableEditablePrecondition: false),
      ActionConfig(
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isTableEditablePrecondition: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true),
    ],
    //* DEMO CODE
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 1, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.sessionId,
        columnIdx: 2,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "file_name",
          label: 'File Name',
          tooltips: 'Input File Name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "table_name",
          label: 'Table Name',
          tooltips: 'Table where the file was loaded',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "session_id",
          label: 'Session ID',
          tooltips: 'Data Pipeline Job Key',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "load_count",
          label: 'Records Count',
          tooltips: 'Number of records loaded',
          isNumeric: true),
      ColumnConfig(
          index: 4,
          name: "bad_row_count",
          label: 'Bad Records',
          tooltips: 'Number of Bad Records',
          isNumeric: true),
      ColumnConfig(
          index: 5,
          name: "node_id",
          label: 'Node ID',
          tooltips: 'Node ID containing there records',
          isNumeric: true),
      ColumnConfig(
          index: 6,
          name: "last_update",
          label: 'Loaded At',
          tooltips: 'Indicates when the file was loaded',
          isNumeric: false),
    ],
    sortColumnIndex: 6,
    sortAscending: false,
    rowsPerPage: 10,
  ),
  DTKeys.usersTable: TableConfig(
    key: DTKeys.usersTable,
    schemaName: 'jetsapi',
    tableName: 'users',
    label: 'User Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [
      ActionConfig(
          key: 'new',
          label: 'Load New File',
          style: ActionStyle.primary,
          configTable: "processConfigTable",
          configForm: "newPipeline"),
      ActionConfig(
          key: 'edit',
          label: 'Edit Table',
          style: ActionStyle.secondary,
          isTableEditablePrecondition: false),
      ActionConfig(
          key: 'save',
          label: 'Save Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true,
          apiKey: 'updatePipeline'),
      ActionConfig(
          key: 'delete',
          label: 'Delete Rows',
          style: ActionStyle.danger,
          isTableEditablePrecondition: true,
          apiKey: 'deletePipelines'),
      ActionConfig(
          key: 'cancel',
          label: 'Cancel Changes',
          style: ActionStyle.primary,
          isTableEditablePrecondition: true),
    ],
    //* DEMO CODE
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.userEmail,
        columnIdx: 2,
      )
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "user_id",
          label: 'UserID',
          tooltips: 'User ID',
          isNumeric: true),
      ColumnConfig(
          index: 1,
          name: "name",
          label: 'Name',
          tooltips: 'User Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "email",
          label: 'Email',
          tooltips: 'User Email',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "last_update",
          label: 'Last Updated',
          tooltips: 'Last Updated',
          isNumeric: false),
    ],
    sortColumnIndex: 0,
    sortAscending: true,
    rowsPerPage: 10,
  ),
  DTKeys.inputTable: TableConfig(
      key: 'inputTable',
      schemaName: 'public',
      tableName: '',
      label: 'Input Data',
      apiPath: '/dataTable',
      isCheckboxVisible: false,
      isCheckboxSingleSelect: false,
      whereClauses: [],
      actions: [],
      columns: [],
      sortColumnIndex: 0,
      sortAscending: true,
      rowsPerPage: 10),
};

TableConfig getTableConfig(String key) {
  var config = _tableConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: table configuration $key not found');
  }
  return config;
}

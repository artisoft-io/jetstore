import 'package:flutter/material.dart';

enum ActionStyle { primary, secondary, danger }

class ColumnConfig {
  ColumnConfig(
      {required this.name,
      required this.label,
      required this.tooltips,
      required this.isNumeric});
  final String name;
  final String label;
  final String tooltips;
  final bool isNumeric;
}

class TableConfig {
  TableConfig(
      {required this.key,
      required this.schemaName,
      required this.tableName,
      required this.title,
      required this.actions,
      required this.columns,
      required this.sortColumnIndex,
      required this.sortAscending,
      required this.rowsPerPage});
  final String key;
  final String schemaName;
  final String tableName;
  final String title;
  final List<ActionConfig> actions;
  final List<ColumnConfig> columns;
  final int sortColumnIndex;
  final bool sortAscending;
  final int rowsPerPage;
}

class ActionConfig {
  ActionConfig(
      {required this.key,
      required this.label,
      this.isTableEditablePrecondition,
      required this.style,
      this.configTable,
      this.configForm,
      this.apiKey});
  final String key;
  final String label;
  final bool? isTableEditablePrecondition;
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

final Map<String, TableConfig> _tableConfigurations = {
  'pipelineTable': TableConfig(
      key: 'pipelineTable',
      schemaName: 'jetsapi',
      tableName: 'pipelines',
      title: 'Data Pipeline',
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
      columns: [
        ColumnConfig(
            name: "key", label: 'Key', tooltips: 'Process Session ID', isNumeric: true),
        ColumnConfig(
            name: "user_name",
            label: 'Submitted By',
            tooltips: 'Submitted By',
            isNumeric: false),
        ColumnConfig(
            name: "client",
            label: 'Client',
            tooltips: 'Client',
            isNumeric: false),
        ColumnConfig(
            name: "process",
            label: 'Process',
            tooltips: 'Process',
            isNumeric: false),
        ColumnConfig(
            name: "status",
            label: 'Status',
            tooltips: 'Execution Status',
            isNumeric: false),
        ColumnConfig(
            name: "submitted_at",
            label: 'Submitted At',
            tooltips: 'Submitted At',
            isNumeric: false),
      ],
      sortColumnIndex: 0,
      sortAscending: true,
      rowsPerPage: 10)
};

TableConfig getTableConfig(String key) {
  var config = _tableConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: table configuration $key not found');
  }
  return config;
}

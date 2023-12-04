import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Source Config User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {

  // Summary of selected data sources
  DTKeys.spSummaryDataSources: TableConfig(
    key: DTKeys.spSummaryDataSources,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'input_registry')
    ],
    label: 'Data Sources',
    apiPath: '/dataTable',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: true,
    noFooter: true,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.spAllDataSourceKeys),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
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
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Pipeline Grouping Domain Key',
      isNumeric: false),
  ColumnConfig(
      index: 2,
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false,
      isHidden: false),
  ColumnConfig(
      index: 4,
      name: "file_key",
      label: 'File Key',
      tooltips: 'File Key from s3',
      isNumeric: false),
    ],
    sortColumnName: 'source_type',
    sortAscending: false,
    rowsPerPage: 100,
  ),

  // for showing the injected data source
  DTKeys.spInjectedProcessInput: TableConfig(
    key: DTKeys.spInjectedProcessInput,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_input')
    ],
    label: 'Historical Data Sources',
    apiPath: '/dataTable',
    noFooter: true,
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "key", formStateKey: FSK.injectedProcessInputKeys),
    ],
    actions: [],
    formStateConfig:
        DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: []),
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
      name: "object_type",
      label: 'Domain Key',
      tooltips: 'Pipeline Grouping Domain Key',
      isNumeric: false),
  ColumnConfig(
      index: 2,
      name: "source_type",
      label: 'Source Type',
      tooltips: 'Source of the input data, either File or Domain Table',
      isNumeric: false),
  ColumnConfig(
      index: 3,
      name: "table_name",
      label: 'Table Name',
      tooltips: 'Table where the data reside',
      isNumeric: false,
      isHidden: false),
  ColumnConfig(
      index: 4,
      name: "lookback_periods",
      label: 'Lookback Periods',
      tooltips: 'Number of periods included in the rule session',
      isNumeric: true),

    ],
    sortColumnName: 'source_type',
    sortAscending: false,
    rowsPerPage: 20,
  ),
};

TableConfig? getStartPipelineTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/data_table_config.dart';

/// Client Registry User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
  // Select Process as Filter
  DTKeys.hfProcessTableUF: TableConfig(
    key: DTKeys.hfProcessTableUF,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'process_config')
    ],
    label: 'Select Process(es) as filter criteria',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.processName, columnIdx: 0),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "process_name",
          label: 'Process Name',
          tooltips: 'Process Name of the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "description",
          label: 'Description',
          tooltips: 'Process description',
          isNumeric: false),
    ],
    sortColumnName: 'process_name',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Select Status as Filter
  DTKeys.hfStatusTableUF: TableConfig(
    key: DTKeys.hfStatusTableUF,
    fromClauses: [],
    label: 'Select Status(es) as filter criteria',
    apiPath: '',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [],
    actions: [],
    staticTableModel: [
      ['Completed', 'completed', '0'],
      ['Submitted', 'submitted', '1'],
      ['Pending', 'pending', '2'],
      ['Failed', 'failed', '3'],
      ['Timed Out', 'timed_out', '4'],
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.status, columnIdx: 1),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "status_name",
          label: 'Select the Status(es)',
          tooltips: 'Status of the pipeline',
          isNumeric: false),
      ColumnConfig(
          index: 1, name: "status", label: '', tooltips: '', isNumeric: false, isHidden: true),
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
      rowsPerPage: 1000000,
  ),

  // Select Status as Filter
  DTKeys.hfFileKeyFilterTypeTableUF: TableConfig(
    key: DTKeys.hfFileKeyFilterTypeTableUF,
    fromClauses: [],
    label: 'Select File Key filter type',
    apiPath: '',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [],
    staticTableModel: [
      ['None', '', '0'],
      ['Equals Value', 'equals_value', '1'],
      ['Starts With', 'starts_with', '2'],
      ['Ends With', 'ends_with', '3'],
      ['Contains', 'contains', '4'],
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
          stateKey: FSK.hfFileKeyMatchType, columnIdx: 1),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "filter_type",
          label: 'Select the File Key filter type',
          tooltips: 'Select the File Key filter type',
          isNumeric: false),
      ColumnConfig(
          index: 1, name: "file_key_filter_type", label: '', tooltips: '', isNumeric: false, isHidden: true),
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
      rowsPerPage: 1000000,
  ),
};

TableConfig? getHomeFiltersTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}

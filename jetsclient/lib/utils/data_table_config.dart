
class ColumnConfig {
  ColumnConfig(
      {required this.domainKey, required this.label, required this.tooltips, required this.isNumeric});
  final String domainKey;
  final String label;
  final String tooltips;
  final bool isNumeric;
}

class TableConfig {
  TableConfig(
      {required this.key,
      required this.title,
      required this.actions,
      required this.columns,
      required this.sortColumnIndex,
      required this.sortAscending,
      required this.rowsPerPage});
  final String key;
  final String title;
  final List<String> actions;
  final List<ColumnConfig> columns;
  final int sortColumnIndex;
  final bool sortAscending;
  final int rowsPerPage;
}

final Map<String, TableConfig> _tableConfigurations = {
  'joblist': TableConfig(
      key: 'joblist',
      title: 'Data Pipeline',
      actions: [
        'newRow',
        'editTable',
        'saveChanges',
        'deleteRows',
        'cancelChanges'
      ],
      columns: [
        ColumnConfig(domainKey: "key",label: 'Key', tooltips: 'Key', isNumeric: true),
        ColumnConfig(domainKey: "client",label: 'Client', tooltips: 'Client', isNumeric: false),
      ],
      sortColumnIndex: 0,
      sortAscending: false,
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

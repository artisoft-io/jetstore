import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table.dart';

import '../../utils/data_table_config.dart';

// typedef JetsDataModel = List<List<dynamic>>;
typedef JetsDataModel = List<dynamic>;

class JetsDataTableSource extends ChangeNotifier {
  JetsDataTableSource(this.state, this.httpClient);
  final JetsDataTableState state;
  final HttpClient httpClient;
  JetsDataModel? model;
  int _totalRowCount = 0;
  List<bool> selectedRows = <bool>[];
  int _selectedRowCount = 0;

  int get rowCount => model != null ? model!.length : 0;
  int get totalRowCount => _totalRowCount;

  int get selectedRowCount => _selectedRowCount;

  DataRow getRow(int index) {
    assert(model != null);
    return DataRow.byIndex(
      index: index,
      color: MaterialStateProperty.resolveWith<Color?>(
          (Set<MaterialState> states) {
        // All rows will have the same selected color.
        if (states.contains(MaterialState.selected)) {
          return Theme.of(state.context).colorScheme.primary.withOpacity(0.08);
        }
        // Even rows will have a grey color.
        if (index.isEven) {
          return Colors.grey.withOpacity(0.3);
        }
        return null; // Use default value for other states and odd rows.
      }),
      cells: List<DataCell>.generate(model![0].length,
          (int colIndex) => DataCell(Text(model![index][colIndex] ?? 'null'))),
      selected: selectedRows[index],
      onSelectChanged: state.isTableEditable
          ? (bool? value) {
              if (value == null) return;
              if (value && !selectedRows[index]) _selectedRowCount++;
              if (!value && selectedRows[index]) _selectedRowCount--;
              selectedRows[index] = value;
              notifyListeners();
            }
          : null,
    );
  }

  dynamic _makeQuery() {
    var schemaName = state.tableConfig.schemaName;
    var tableName = state.tableConfig.tableName;
    var columns = state.tableConfig.columns;
    List<String> columnNames = [];
    if (columns.isNotEmpty) {
      columnNames =
          List<String>.generate(columns.length, (index) => columns[index].name);
    }
    var msg = <String, dynamic>{'action': 'read'};
    msg['schema'] = schemaName;
    if (tableName.isEmpty) {
      String name = JetsRouterDelegate().currentConfiguration?.params['table'];
      msg['table'] = name;
    } else {
      msg['table'] = tableName;
    }
    msg['offset'] = state.indexOffset;
    msg['limit'] = state.rowsPerPage;
    if (columns.isNotEmpty) {
      msg['columns'] = columnNames;
      msg['sortColumn'] = columnNames[state.sortColumnIndex];
    } else {
      if(state.columnNames.isNotEmpty) {
        msg['columns'] = state.columnNames;
        msg['sortColumn'] = state.columnNames[state.sortColumnIndex];
      } else {
        msg['columns'] = [];
        msg['sortColumn'] = '';
      }
    }
    msg['sortAscending'] = state.sortAscending;
    return msg;
  }

  Future<Map<String, dynamic>?> fetchData() async {
    var result = await httpClient.sendRequest(
        path: '/dataTable',
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: json.encode(_makeQuery()));

    if (!state.mounted) return null;
    if (result.statusCode == 200) {
      // update the [model]
      return result.body;
    } else if (result.statusCode == 401) {
      const snackBar = SnackBar(
        content: Text('Session Expired, please login'),
      );
      ScaffoldMessenger.of(state.context).showSnackBar(snackBar);
      JetsRouterDelegate()(JetsRouteData(loginPath));
      return null;
    } else if (result.statusCode == 422) {
      const snackBar = SnackBar(
        content: Text('Error reading data from table'),
      );
      ScaffoldMessenger.of(state.context).showSnackBar(snackBar);
      return null;
    } else {
      const snackBar = SnackBar(
        content: Text('Unknown Error reading data from table'),
      );
      ScaffoldMessenger.of(state.context).showSnackBar(snackBar);
      return null;
    }
  }

  Future<int> getModelData() async {
    selectedRows = List<bool>.filled(state.rowsPerPage, false);
    _selectedRowCount = 0;

    var data = await fetchData();
    if (data != null) {
      // Check if we got columnDef back
      var columnDef = data['columnDef'] as List<dynamic>?;
      if (columnDef != null) {
        state.dataColumns = columnDef
          .map((m1) => ColumnConfig(
              name: m1['name'],
              label: m1['label'],
              tooltips: m1['tooltips'],
              isNumeric: m1['isnumeric']))
          .map((e) => state.makeDataColumn(e))
          .toList();
        state.columnNames = columnDef.map((e) => e['name'] as String).toList();
      }
      model = data['rows'];
      _totalRowCount = data['totalRowCount'];
      notifyListeners();
    }
    return 0;
  }

  void getModelDataSync() async {
    await getModelData();
  }

  void sortModelData(int columnIndex, bool ascending) {
    if (model == null) return;
    var sortSign = ascending ? 1 : -1;
    model!.sort((l, r) {
      if (l == r) return 0;
      // Always put null last
      if (l[columnIndex] == null) return 1;
      if (r[columnIndex] == null) return -1;
      // Check data type
      if (state.tableConfig.columns[columnIndex].isNumeric) {
        return sortSign *
            double.parse(l[columnIndex])
                .compareTo(double.parse(r[columnIndex]));
      }
      return sortSign *
          l[columnIndex].toString().compareTo(r[columnIndex].toString());
    });
  }
}

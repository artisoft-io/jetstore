import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/data_table.dart';

typedef JetsDataModel = List<List<dynamic>>;

class JetsDataTableSource extends DataTableSource {
  JetsDataTableSource(this.state, this.httpClient);
  final JetsDataTableState state;
  final HttpClient httpClient;
  JetsDataModel? model;
  List<bool> selectedRows = <bool>[];
  int _selectedRowCount = 0;

  @override
  int get rowCount => model != null ? model!.length : 0;

  @override
  bool get isRowCountApproximate => false;

  @override
  int get selectedRowCount => _selectedRowCount;

  @override
  DataRow? getRow(int index) {
    print("JetsDataTableSource.getRow called with index $index");
    if (model == null || index < state.indexOffset || index >= state.maxIndex) {
      return null;
    }
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
      cells: List<DataCell>.generate(
          model![0].length,
          (int colIndex) =>
              DataCell(Text(model![index - state.indexOffset][colIndex]))),
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

  Future<int> getModelData() async {
    print(
        "getModelData from index ${state.indexOffset} to ${state.maxIndex}) called (simulated)");
    selectedRows = List<bool>.filled(state.rowsPerPage, false);
    _selectedRowCount = 0;
    model = List<List<dynamic>>.generate(
        state.rowsPerPage,
        (index) => [
              "$index",
              "User $index",
              "ACME",
              "P$index",
              "completed",
              "2022-06-27 15:51:22"
            ]);
    return 0;
  }

  void getModelDataSync() async {
    var res = await getModelData();
    print("getModelDataSync got result $res");
  }
}

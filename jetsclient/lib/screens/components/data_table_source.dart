import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/data_table.dart';

typedef JetsDataModel = List<List<dynamic>>;

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
    debugPrint("JetsDataTableSource.getRow called with index $index");
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
          (int colIndex) => DataCell(Text(model![index][colIndex]))),
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
    debugPrint(
        "getModelData from index ${state.indexOffset} to ${state.maxIndex}) called (simulated)");
    selectedRows = List<bool>.filled(state.rowsPerPage, false);
    _selectedRowCount = 0;
    model = List<List<dynamic>>.generate(
        state.rowsPerPage,
        (index) => [
              "${state.indexOffset + index}",
              "User $index on Page ${state.currentDataPage}",
              "ACME",
              "P$index",
              "completed",
              "2022-06-27 15:51:22"
            ]);
    _totalRowCount = 50;
    return 0;
  }

  void getModelDataSync() async {
    var res = await getModelData();
    debugPrint("getModelDataSync got result $res");
  }

  void sortModelData(int columnIndex, bool ascending) {
    if (model == null) return;
    var sortSign = ascending ? 1 : -1;
    model!.sort((l, r) {
      if (l == r) return 0;
      // Always put null last
      if (l[columnIndex] == null) return 1;
      if (r[columnIndex] == null) return -1;
      return sortSign *
          l[columnIndex].toString().compareTo(r[columnIndex].toString());
    });
  }
}

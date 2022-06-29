import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';

import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:provider/provider.dart';

//* examples
typedef FncBool = void Function(bool?);
typedef OnSelectCB = void Function(bool value, int index);

class JetsDataTableWidget extends StatefulWidget {
  const JetsDataTableWidget({super.key, required this.tableConfig});
  final String tableConfig;

  @override
  State<JetsDataTableWidget> createState() => _JetsDataTableState();
}

class _JetsDataTableState extends State<JetsDataTableWidget> {
  @override
  void initState() {
    super.initState();
    tableConfig = getTableConfig(widget.tableConfig);
    sortColumnIndex = tableConfig!.sortColumnIndex;
    sortAscending = tableConfig!.sortAscending;
    rowsPerPage = tableConfig!.rowsPerPage;
    dataSource = _JetsDataTableSource(
        this, Provider.of<HttpClient>(context, listen: false));
    dataSource!.getModelDataSync();
    // dataSource!.addListener(() {
    //   setState(() {});
    // });
  }

  @override
  void dispose() {
    if (dataSource != null) {
      dataSource!.dispose();
    }
    super.dispose();
  }

  // State Data
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();
  _JetsDataTableSource? dataSource;
  bool isTableEditable = false;
  TableConfig? tableConfig;
  int sortColumnIndex = 0;
  bool sortAscending = false;
  int currentDataPage = 0;
  int rowsPerPage = 10;

  int get indexOffset => currentDataPage * rowsPerPage;
  int get maxIndex => (currentDataPage + 1) * rowsPerPage;

  // // sentinel variable used only to trigger the widget build
  // // when the dataSource is modified
  // bool _triggerBuild = true;

  List<DataColumn> get dataColumns {
    return tableConfig!.columns
        .map((e) => DataColumn(
            label: Text(e.label),
            numeric: e.isNumeric,
            tooltip: e.tooltips,
            onSort: ((columnIndex, ascending) =>
                _sortTable(columnIndex, ascending))))
        .toList();
  }

  Widget _makeActions(String actionKey) {
    switch (actionKey) {
      case 'newRow':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: () => _showDialog('New Row!'),
          child: const Text('New Row'),
        );
      case 'editTable':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: () => _showDialog('Edit Table!'),
          child: const Text('Edit Table'),
        );
      case 'saveChanges':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: null,
          child: const Text('Save Changes'),
        );
      case 'deleteRows':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: null,
          child: const Text('Delete Selected'),
        );
      case 'cancelChanges':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: null,
          child: const Text('Cancel'),
        );
      default:
        throw Exception(
            'ERROR: Invalid program configuration: Unknown DataTable action: $actionKey');
    }
  }

  void _sortTable(int columnIndex, bool ascending) {
    //* TODO
    print('_sortTable called with columnIndex: $columnIndex, asc? $ascending');
  }

  @override
  Widget build(BuildContext context) {
    // return _buildJetsDataTableWithScrollbars2(context);
    return _buildJetsDataTable2(context);
  }

  Widget _buildJetsDataTableWithScrollbars_back(BuildContext context) {
    return Scrollbar(
      thumbVisibility: true,
      trackVisibility: true,
      controller: _verticalController,
      child: SingleChildScrollView(
          scrollDirection: Axis.vertical,
          controller: _verticalController,
          child: Scrollbar(
            thumbVisibility: true,
            trackVisibility: true,
            controller: _horizontalController,
            child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                controller: _horizontalController,
                padding: const EdgeInsets.all(defaultPadding),
                child: PaginatedDataTable(
                  header: Text(tableConfig!.title),
                  actions:
                      tableConfig!.actions.map((e) => _makeActions(e)).toList(),
                  // controller: _verticalController,
                  columns: dataColumns,
                  sortColumnIndex: sortColumnIndex,
                  sortAscending: sortAscending,
                  onPageChanged: (value) =>
                      print("onPageChange with value $value"),
                  rowsPerPage: rowsPerPage,
                  onRowsPerPageChanged: (value) => setState(() {
                    rowsPerPage = value ?? rowsPerPage;
                  }),
                  source: dataSource!,
                )),
          )),
    );
  }

  Widget _buildJetsDataTableWithScrollbars(BuildContext context) {
    return Scrollbar(
      thumbVisibility: true,
      trackVisibility: true,
      controller: _verticalController,
      child: SingleChildScrollView(
          scrollDirection: Axis.vertical,
          controller: _verticalController,
          child: Scrollbar(
            thumbVisibility: true,
            trackVisibility: true,
            controller: _horizontalController,
            child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                controller: _horizontalController,
                padding: const EdgeInsets.all(defaultPadding),
                child: _buildJetsDataTable(context)),
          )),
    );
  }

  Widget _buildJetsDataTableWithScrollbars2(BuildContext context) {
    return Scrollbar(
      thumbVisibility: true,
      trackVisibility: true,
      controller: _verticalController,
      child: SingleChildScrollView(
          scrollDirection: Axis.vertical,
          controller: _verticalController,
          child: Scrollbar(
            thumbVisibility: true,
            trackVisibility: true,
            controller: _horizontalController,
            child: _buildJetsDataTable(context),
          )),
    );
  }

  // Main widget
  Widget _buildJetsDataTable(BuildContext context) {
    return PaginatedDataTable(
      header: Text(tableConfig!.title),
      //*TEST
      actions: tableConfig!.actions.map((e) => _makeActions(e)).toList(),
      columns: dataColumns,
      sortColumnIndex: sortColumnIndex,
      sortAscending: sortAscending,
      onPageChanged: (value) => print("onPageChange called with value $value"),
      rowsPerPage: rowsPerPage,
      onRowsPerPageChanged: (value) => setState(() {
        print("onRowsPerPageChanged called with value $value");
        rowsPerPage = value ?? rowsPerPage;
      }),
      source: dataSource!,
      controller: _horizontalController,
    );
  }

//TEST
  Widget _buildJetsDataTable2(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                foregroundColor:
                    Theme.of(context).colorScheme.onSecondaryContainer,
                backgroundColor:
                    Theme.of(context).colorScheme.secondaryContainer,
              ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
              onPressed: () => _showDialog('Coming Soon!'),
              child: const Text('New Pipeline'),
            ),
            const SizedBox(width: defaultPadding),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                foregroundColor:
                    Theme.of(context).colorScheme.onSecondaryContainer,
                backgroundColor:
                    Theme.of(context).colorScheme.secondaryContainer,
              ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
              onPressed: () {
                setState(() {
                  isTableEditable = !isTableEditable;
                });
              },
              child: const Text('Edit Table'),
            ),
          ],
        ),
        const SizedBox(height: defaultPadding),
        Expanded(
          flex: 8,
            child: Scrollbar(
          thumbVisibility: true,
          trackVisibility: true,
          controller: _verticalController,
          child: SingleChildScrollView(
              scrollDirection: Axis.vertical,
              controller: _verticalController,
              child: Scrollbar(
                thumbVisibility: true,
                trackVisibility: true,
                controller: _horizontalController,
                child: SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    controller: _horizontalController,
                    padding: const EdgeInsets.all(defaultPadding),
                    child: DataTable(
                      columns: List<DataColumn>.generate(
                          10,
                          (int index) => DataColumn(
                                label: Text('Item $index'),
                              )),
                      rows: List<DataRow>.generate(
                        10,
                        (int index) => DataRow.byIndex(
                          index: index,
                          color: MaterialStateProperty.resolveWith<Color?>(
                              (Set<MaterialState> states) {
                            // All rows will have the same selected color.
                            if (states.contains(MaterialState.selected)) {
                              return Theme.of(context)
                                  .colorScheme
                                  .primary
                                  .withOpacity(0.08);
                            }
                            // Even rows will have a grey color.
                            if (index.isEven) {
                              return Colors.grey.withOpacity(0.3);
                            }
                            return null; // Use default value for other states and odd rows.
                          }),
                          cells: List<DataCell>.generate(
                              10,
                              (int colIndex) => DataCell(
                                  Text('Cell row $index, col $colIndex'))),
                        ),
                      ),
                    )),
              )),
        )),
        const SizedBox(height: defaultPadding),
        Expanded(
          flex: 1,
          child: Row(
            children: [
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  foregroundColor:
                      Theme.of(context).colorScheme.onSecondaryContainer,
                  backgroundColor:
                      Theme.of(context).colorScheme.secondaryContainer,
                ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
                onPressed: () => _showDialog('Coming Soon!'),
                child: const Text('Footer New Pipeline'),
              ),
              const SizedBox(width: defaultPadding),
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  foregroundColor:
                      Theme.of(context).colorScheme.onSecondaryContainer,
                  backgroundColor:
                      Theme.of(context).colorScheme.secondaryContainer,
                ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
                onPressed: () {
                  setState(() {
                    isTableEditable = !isTableEditable;
                  });
                },
                child: const Text('Edit Table'),
              ),
            ],
          ),
        )
      ],
    );
  }

  void _showDialog(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(message),
        actions: [
          TextButton(
            child: const Text('OK'),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ],
      ),
    );
  }
}

typedef JetsDataModel = List<List<dynamic>>;

class _JetsDataTableSource extends DataTableSource {
  _JetsDataTableSource(this.state, this.httpClient);
  final _JetsDataTableState state;
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
    if (model == null || index < state.indexOffset || index >= state.maxIndex)
      return null;
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

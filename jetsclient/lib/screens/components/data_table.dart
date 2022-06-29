import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/data_table_source.dart';

//* examples
typedef FncBool = void Function(bool?);
typedef OnSelectCB = void Function(bool value, int index);

class JetsDataTableWidget extends StatefulWidget {
  const JetsDataTableWidget({super.key, required this.tableConfig});
  final String tableConfig;

  @override
  State<JetsDataTableWidget> createState() => JetsDataTableState();
}

class JetsDataTableState extends State<JetsDataTableWidget> {

  // State Data
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();
  JetsDataTableSource? dataSource;
  bool isTableEditable = false;
  TableConfig? tableConfig;
  int sortColumnIndex = 0;
  bool sortAscending = false;
  int currentDataPage = 0;
  int rowsPerPage = 10;

  int get indexOffset => currentDataPage * rowsPerPage;
  int get maxIndex => (currentDataPage + 1) * rowsPerPage;

  @override
  void initState() {
    super.initState();
    tableConfig = getTableConfig(widget.tableConfig);
    sortColumnIndex = tableConfig!.sortColumnIndex;
    sortAscending = tableConfig!.sortAscending;
    rowsPerPage = tableConfig!.rowsPerPage;
    dataSource = JetsDataTableSource(
        this, Provider.of<HttpClient>(context, listen: false));
    // Get the first batch of data
    dataSource!.getModelDataSync();
  }

  @override
  void dispose() {
    if (dataSource != null) {
      dataSource!.dispose();
    }
    super.dispose();
  }

  // Utility Function for the build
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
          onPressed: () => showAlertDialog(context, 'New Row!'),
          child: const Text('New Row'),
        );
      case 'editTable':
        return ElevatedButton(
          style: ElevatedButton.styleFrom(
            foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
            backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: () => showAlertDialog(context, 'Edit Table!'),
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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // HEADER ROW
        Row(
          children: [
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                foregroundColor:
                    Theme.of(context).colorScheme.onSecondaryContainer,
                backgroundColor:
                    Theme.of(context).colorScheme.secondaryContainer,
              ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
              onPressed: () => showAlertDialog(context, 'Coming Soon!'),
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
        // MAIN TABLE SECTION
        const SizedBox(height: defaultPadding),
        Expanded(
          flex: 8,
            child: Scrollbar(
          thumbVisibility: true,
          trackVisibility: true,
          controller: _verticalController,
          child: Scrollbar(
            thumbVisibility: true,
            trackVisibility: true,
            controller: _horizontalController,
            notificationPredicate: (e) => e.depth == 1,
            child: SingleChildScrollView(
              scrollDirection: Axis.vertical,
              controller: _verticalController,
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
            ),
          ),
        )),
        // FOOTER ROW
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
                onPressed: () => showAlertDialog(context, 'Coming Soon!'),
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
}

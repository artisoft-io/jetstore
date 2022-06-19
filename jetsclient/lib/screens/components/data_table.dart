import 'package:flutter/material.dart';
import 'package:jetsclient/screens/job_list.dart';

typedef FncBool = void Function(bool?);
typedef OnSelectCB = void Function(bool value, int index);

class JetsDataTableWidget extends StatefulWidget {
  const JetsDataTableWidget({super.key});

  @override
  State<JetsDataTableWidget> createState() => _JetsDataTableState();
}

class _JetsDataTableState extends State<JetsDataTableWidget> {
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();
  static const int numItems = 10;
  List<bool> selected = List<bool>.generate(numItems, (int index) => false);
  bool rowsSelectable = false;

  ValueNotifier<bool> tableEditable = ValueNotifier(false);

  @override
  void dispose() {
    tableEditable.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return _buildJetsDataTableWithScrollbars(context);
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

  Widget _buildJetsDataTable(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                // Foreground color
                onPrimary: Theme.of(context).colorScheme.onSecondaryContainer,
                // Background color
                primary: Theme.of(context).colorScheme.secondaryContainer,
              ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
              onPressed: () => _showDialog('Coming Soon!'),
              child: const Text('New Pipeline'),
            ),
            const SizedBox(width: defaultPadding),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                // Foreground color
                onPrimary: Theme.of(context).colorScheme.onSecondaryContainer,
                // Background color
                primary: Theme.of(context).colorScheme.secondaryContainer,
              ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
              onPressed: () => tableEditable.value = !tableEditable.value,
              child: const Text('Edit Table'),
            ),
          ],
        ),
        ValueListenableBuilder(
            valueListenable: tableEditable,
            builder: (BuildContext context, bool counterValue, Widget? child) {
              return DataTable(
                columns: List<DataColumn>.generate(
                    numItems,
                    (int index) => DataColumn(
                          label: Text('Item $index'),
                        )),
                rows: List<DataRow>.generate(
                  numItems,
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
                        numItems,
                        (int colIndex) =>
                            DataCell(Text('Cell row $index, col $colIndex'))),
                    selected: selected[index],
                    onSelectChanged: counterValue
                        ? (bool? value) {
                            setState(() {
                              selected[index] = value!;
                            });
                          }
                        : null,
                  ),
                ),
              );
            }),
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

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
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
  const JetsDataTableWidget(
      {super.key, required this.tablePath, required this.tableConfig});
  final JetsRouteData tablePath;
  final String tableConfig;

  @override
  State<JetsDataTableWidget> createState() => JetsDataTableState();
}

class JetsDataTableState extends State<JetsDataTableWidget> {
  // State Data
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();
  late final JetsDataTableSource dataSource;
  late final TableConfig tableConfig;
  bool isTableEditable = false;
  int sortColumnIndex = 0;
  bool sortAscending = false;

  // pagination state
  int currentDataPage = 0;
  int rowsPerPage = 10;
  late final List<int> availableRowsPerPage;

  late final List<DataColumn> dataColumns;

  int get indexOffset => currentDataPage * rowsPerPage;
  int get maxIndex => (currentDataPage + 1) * rowsPerPage;

  @override
  void initState() {
    super.initState();
    tableConfig = getTableConfig(widget.tableConfig);
    sortColumnIndex = tableConfig.sortColumnIndex;
    sortAscending = tableConfig.sortAscending;
    rowsPerPage = tableConfig.rowsPerPage;
    availableRowsPerPage = <int>[
      rowsPerPage,
      rowsPerPage * 2,
      rowsPerPage * 5,
      rowsPerPage * 10
    ];

    dataColumns = tableConfig.columns
        .map((e) => DataColumn(
            label: Text(e.label),
            numeric: e.isNumeric,
            tooltip: e.tooltips,
            onSort: ((columnIndex, ascending) =>
                _sortTable(columnIndex, ascending))))
        .toList();

    dataSource = JetsDataTableSource(
        this, Provider.of<HttpClient>(context, listen: false));
    dataSource.addListener(() {
      setState(() {});
    });
    // Get the first batch of data when navigated to tablePath
    JetsRouterDelegate().addListener(() {
      if (JetsRouterDelegate().currentConfiguration?.path ==
          widget.tablePath.path) {
        dataSource.getModelDataSync();
      }
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(() {
      if (JetsRouterDelegate().currentConfiguration?.path ==
          widget.tablePath.path) {
        dataSource.getModelDataSync();
      }
    });
    dataSource.removeListener(() {
      setState(() {});
    });
    dataSource.dispose();
    super.dispose();
  }

  void actionDispatcher(ActionConfig ac) {
    //* TODO
    switch (ac.key) {
      case 'new':
        showAlertDialog(context, 'New Pipeline Coming Soon!');
        break;
      case 'edit':
        setState(() => isTableEditable = true);
        break;
      case 'save':
        showAlertDialog(context, 'Save Changes Coming Soon!');
        setState(() => isTableEditable = false);
        break;
      case 'delete':
        showAlertDialog(
            context, 'Delete Pipeline (with confirmation) Coming Soon!');
        setState(() => isTableEditable = false);
        break;
      case 'cancel':
        showAlertDialog(
            context, 'Cancel changes (with confirmation) Coming Soon!');
        setState(() => isTableEditable = false);
        break;

      default:
        showAlertDialog(
            context, 'Oops something is wrong, unknown action ${ac.key}');
    }
  }

  void _sortTable(int columnIndex, bool ascending) {
    dataSource.sortModelData(columnIndex, ascending);
    setState(() {
      sortColumnIndex = columnIndex;
      sortAscending = ascending;
    });
  }

  // Functions for pagination
  bool _isLastPage() {
    return (currentDataPage + 1) * rowsPerPage >= dataSource.totalRowCount;
  }

  void _rowPerPageChanged(int? value) {
    if (value == null) return;
    setState(() {
      rowsPerPage = value;
    });
    //* TODO dot it asynch
    dataSource.getModelDataSync();
  }

  void _gotoFirstPressed() {
    setState(() {
      currentDataPage = 0;
    });
    //* TODO dot it asynch
    dataSource.getModelDataSync();
  }

  void _previousPressed() {
    setState(() {
      currentDataPage--;
    });
    //* TODO dot it asynch
    dataSource.getModelDataSync();
  }

  void _nextPressed() {
    setState(() {
      currentDataPage++;
    });
    //* TODO dot it asynch
    dataSource.getModelDataSync();
  }

  void _lastPressed() {
    var n = dataSource.totalRowCount ~/ rowsPerPage;
    var r = dataSource.totalRowCount % n;
    setState(() {
      currentDataPage = r == 0 ? n - 1 : n;
    });
    //* TODO dot it asynch
    dataSource.getModelDataSync();
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData themeData = Theme.of(context);
    final MaterialLocalizations localizations =
        MaterialLocalizations.of(context);
    // prepare the footer widgets
    final TextStyle? footerTextStyle = themeData.textTheme.caption;
    List<DropdownMenuItem<int>> rowsPerPageItems = availableRowsPerPage
        .map<DropdownMenuItem<int>>((e) => DropdownMenuItem<int>(
              value: e,
              child: Text('$e'),
            ))
        .toList();
    var footerWidgets = <Widget>[
      Container(
          width:
              14.0), // to match trailing padding in case we overflow and end up scrolling
      Text(localizations.rowsPerPageTitle),
      ConstrainedBox(
        constraints: const BoxConstraints(
            minWidth: 64.0), // 40.0 for the text, 24.0 for the icon
        child: Align(
          alignment: AlignmentDirectional.centerEnd,
          child: DropdownButtonHideUnderline(
            child: DropdownButton<int>(
              items: rowsPerPageItems,
              value: rowsPerPage,
              onChanged: _rowPerPageChanged,
              style: footerTextStyle,
            ),
          ),
        ),
      ),
      Container(width: 32.0),
      Text(
        localizations.pageRowsInfoTitle(
          indexOffset + 1,
          maxIndex + 1,
          dataSource.totalRowCount,
          false,
        ),
      ),
      Container(width: 32.0),
      IconButton(
        icon: const Icon(Icons.skip_previous),
        padding: EdgeInsets.zero,
        tooltip: localizations.firstPageTooltip,
        onPressed: currentDataPage == 0 ? null : _gotoFirstPressed,
      ),
      IconButton(
        icon: const Icon(Icons.chevron_left),
        padding: EdgeInsets.zero,
        tooltip: localizations.previousPageTooltip,
        onPressed: currentDataPage == 0 ? null : _previousPressed,
      ),
      Container(width: 24.0),
      IconButton(
        icon: const Icon(Icons.chevron_right),
        padding: EdgeInsets.zero,
        tooltip: localizations.nextPageTooltip,
        onPressed: _isLastPage() ? null : _nextPressed,
      ),
      IconButton(
        icon: const Icon(Icons.skip_next),
        padding: EdgeInsets.zero,
        tooltip: localizations.lastPageTooltip,
        onPressed: _isLastPage() ? null : _lastPressed,
      ),
      Container(width: 14.0),
    ];

    // build the data table
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // HEADER ROW
        Row(
          children: tableConfig.actions
              .where((ac) => ac.predicate(isTableEditable))
              .expand((ac) => [
                    const SizedBox(width: defaultPadding),
                    ElevatedButton(
                      style: ac.buttonStyle(themeData),
                      onPressed: () => actionDispatcher(ac),
                      child: Text(ac.label),
                    )
                  ])
              .toList(),
        ),
        // MAIN TABLE SECTION
        const SizedBox(height: defaultPadding),
        Expanded(
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
                    columns: dataColumns,
                    rows: List<DataRow>.generate(
                      dataSource.rowCount,
                      (int index) => dataSource.getRow(index),
                    ),
                  )),
            ),
          ),
        )),
        // FOOTER ROW
        const SizedBox(height: defaultPadding),
        DefaultTextStyle(
          style: footerTextStyle!,
          child: IconTheme.merge(
            data: const IconThemeData(
              opacity: 0.54,
            ),
            child: SizedBox(
              height: 56.0,
              child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                reverse: true,
                child: Row(
                  children: footerWidgets,
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

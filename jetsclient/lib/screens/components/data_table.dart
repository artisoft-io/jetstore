import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:provider/provider.dart';

import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/data_table_source.dart';

class JetsDataTableWidget extends FormField<WidgetField> {
  JetsDataTableWidget({
    required super.key,
    required this.screenPath,
    this.formFieldConfig,
    required this.tableConfig,
    this.formState,
    this.formFieldValidator,
    required this.dialogValidatorDelegate,
    required this.actionsDelegate,
  })  : assert((formState != null && formFieldConfig != null) ||
            (formState == null && formFieldConfig == null)),
        super(
          initialValue:
              formState?.getValue(formFieldConfig!.group, formFieldConfig.key),
          validator: formFieldConfig != null && formFieldValidator != null
              ? (WidgetField? value) => formFieldValidator(
                  formFieldConfig.group, formFieldConfig.key, value)
              : null,
          autovalidateMode: AutovalidateMode.disabled,
          builder: (FormFieldState<WidgetField> field) {
            final state = field as JetsDataTableState;
            final context = field.context;
            final ThemeData themeData = Theme.of(context);
            final MaterialLocalizations localizations =
                MaterialLocalizations.of(context);
            // prepare the footer widgets
            final TextStyle? footerTextStyle = themeData.textTheme.caption;
            List<DropdownMenuItem<int>> rowsPerPageItems =
                state.availableRowsPerPage
                    .map<DropdownMenuItem<int>>((e) => DropdownMenuItem<int>(
                          value: e,
                          child: Text('$e'),
                        ))
                    .toList();
            final List<DataColumn> dataColumns = state.columnsConfig
                .map((e) => state.makeDataColumn(e))
                .toList();
            var footerWidgets = <Widget>[
              Container(
                  // to match trailing padding in case we overflow and end up scrolling
                  width: 14.0),
              Text(localizations.rowsPerPageTitle),
              ConstrainedBox(
                constraints: const BoxConstraints(
                    minWidth: 64.0), // 40.0 for the text, 24.0 for the icon
                child: Align(
                  alignment: AlignmentDirectional.centerEnd,
                  child: DropdownButtonHideUnderline(
                    child: DropdownButton<int>(
                      items: rowsPerPageItems,
                      value: state.rowsPerPage,
                      onChanged: state._rowPerPageChanged,
                      style: footerTextStyle,
                    ),
                  ),
                ),
              ),
              Container(width: 32.0),
              Text(
                localizations.pageRowsInfoTitle(
                  state.indexOffset + 1,
                  state.maxIndex + 1,
                  state.dataSource.totalRowCount,
                  false,
                ),
              ),
              Container(width: 32.0),
              IconButton(
                icon: const Icon(Icons.skip_previous),
                padding: EdgeInsets.zero,
                tooltip: localizations.firstPageTooltip,
                onPressed:
                    state.currentDataPage == 0 ? null : state._gotoFirstPressed,
              ),
              IconButton(
                icon: const Icon(Icons.chevron_left),
                padding: EdgeInsets.zero,
                tooltip: localizations.previousPageTooltip,
                onPressed:
                    state.currentDataPage == 0 ? null : state._previousPressed,
              ),
              Container(width: 24.0),
              IconButton(
                icon: const Icon(Icons.chevron_right),
                padding: EdgeInsets.zero,
                tooltip: localizations.nextPageTooltip,
                onPressed: state._isLastPage() ? null : state._nextPressed,
              ),
              IconButton(
                icon: const Icon(Icons.skip_next),
                padding: EdgeInsets.zero,
                tooltip: localizations.lastPageTooltip,
                onPressed: state._isLastPage() ? null : state._lastPressed,
              ),
              Container(width: 14.0),
            ];
            // Header row - label + action buttons
            final headerRow = <Widget>[
              if (tableConfig.label.isNotEmpty)
                Text(
                  tableConfig.label,
                  style: Theme.of(context).textTheme.headline5,
                )
            ];
            headerRow.addAll(tableConfig.actions
                .where((ac) => ac.predicate(state.isTableEditable))
                .expand((ac) => [
                      const SizedBox(width: defaultPadding),
                      ElevatedButton(
                        style: ac.buttonStyle(themeData),
                        onPressed: () => ac.isEnabled(state.isTableEditable)
                            ? state.actionDispatcher(context, ac)
                            : null,
                        child: Text(ac.label),
                      )
                    ]));

            // build the data table
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Container(
                    padding: state.errorText != null
                        ? const EdgeInsets.all(4)
                        : null,
                    decoration: state.errorText != null
                        ? BoxDecoration(
                            border: Border.all(color: Colors.red, width: 2.0),
                            borderRadius: BorderRadius.circular(12))
                        : null,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // HEADER ROW
                        if (headerRow.isNotEmpty) Row(children: headerRow),
                        // MAIN TABLE SECTION
                        const SizedBox(height: defaultPadding),
                        Expanded(
                            child: Scrollbar(
                          thumbVisibility: true,
                          trackVisibility: true,
                          controller: state._verticalController,
                          child: Scrollbar(
                            thumbVisibility: true,
                            trackVisibility: true,
                            controller: state._horizontalController,
                            notificationPredicate: (e) => e.depth == 1,
                            child: SingleChildScrollView(
                              scrollDirection: Axis.vertical,
                              controller: state._verticalController,
                              child: SingleChildScrollView(
                                  scrollDirection: Axis.horizontal,
                                  controller: state._horizontalController,
                                  padding: const EdgeInsets.all(defaultPadding),
                                  child: DataTable(
                                    columns: dataColumns.isNotEmpty
                                        ? dataColumns
                                        : [const DataColumn(label: Text(' '))],
                                    rows: List<DataRow>.generate(
                                      state.dataSource.rowCount,
                                      (int index) =>
                                          state.dataSource.getRow(index),
                                    ),
                                    sortColumnIndex: state.sortColumnIndex,
                                    sortAscending: state.sortAscending,
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
                    ),
                  ),
                ),
                if (state.errorText != null)
                  Text(state.errorText!,
                      style: themeData.textTheme.bodyMedium
                          ?.copyWith(color: Colors.red)),
              ],
            );
          },
        );
  final JetsRouteData screenPath;
  final TableConfig tableConfig;
  final FormDataTableFieldConfig? formFieldConfig;
  final JetsFormState? formState;
  final JetsFormFieldValidator? formFieldValidator;
  final ValidatorDelegate dialogValidatorDelegate;
  final FormActionsDelegate actionsDelegate;

  @override
  FormFieldState<WidgetField> createState() => JetsDataTableState();
}

class JetsDataTableState extends FormFieldState<WidgetField> {
  // State Data
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();
  late final JetsDataTableSource dataSource;
  bool isTableEditable = false;
  int sortColumnIndex = 0;
  bool sortAscending = false;

  // pagination state
  int currentDataPage = 0;
  int rowsPerPage = 10;
  late final List<int> availableRowsPerPage;

  List<ColumnConfig> columnsConfig = [];
  List<String> columnNames = [];

  int get indexOffset => currentDataPage * rowsPerPage;
  int get maxIndex => (currentDataPage + 1) * rowsPerPage;
  JetsDataTableWidget get _dataTableWidget =>
      super.widget as JetsDataTableWidget;
  TableConfig get tableConfig => _dataTableWidget.tableConfig;
  JetsFormState? get formState => _dataTableWidget.formState;
  FormDataTableFieldConfig? get formFieldConfig =>
      _dataTableWidget.formFieldConfig;
  ValidatorDelegate get dialogValidatorDelegate =>
      _dataTableWidget.dialogValidatorDelegate;
  FormActionsDelegate get actionsDelegate => _dataTableWidget.actionsDelegate;

  @override
  void initState() {
    super.initState();
    sortColumnIndex = tableConfig.sortColumnIndex;
    sortAscending = tableConfig.sortAscending;
    rowsPerPage = tableConfig.rowsPerPage;
    availableRowsPerPage = <int>[
      rowsPerPage,
      rowsPerPage * 2,
      rowsPerPage * 5,
      rowsPerPage * 10
    ];

    dataSource = JetsDataTableSource(
        state: this,
        httpClient: Provider.of<HttpClient>(context, listen: false));
    dataSource.addListener(triggetRefreshListner);

    isTableEditable = tableConfig.isCheckboxVisible;

    // this may be an empty list if table is a domain table
    columnsConfig = tableConfig.columns;

    // register for change notification on the form state
    if (formState != null && formFieldConfig != null) {
      formState!.addListener(refreshOnFormStateChange);
    }

    if (_dataTableWidget.screenPath.path == homePath) {
      // Get the first batch of data when navigated to screenPath
      JetsRouterDelegate().addListener(navListener);
    } else {
      dataSource.getModelData();
    }
  }

  DataColumn makeDataColumn(ColumnConfig e) {
    return DataColumn(
        label: Text(e.label),
        numeric: e.isNumeric,
        tooltip: e.tooltips,
        onSort: ((columnIndex, ascending) =>
            _sortTable(columnIndex, ascending)));
  }

  void navListener() async {
    if (JetsRouterDelegate().currentConfiguration?.path == homePath) {
      dataSource.getModelData();
    }
  }

  void triggetRefreshListner() {
    setState(() {});
  }

  void refreshOnFormStateChange() {
    assert(formState != null);
    assert(formFieldConfig != null);
    for (final whereClause in tableConfig.whereClauses) {
      if (whereClause.formStateKey != null) {
        if (formState!
            .isKeyUpdated(formFieldConfig!.group, whereClause.formStateKey!)) {
          // where clause have changed, refresh the table, make sure to go to
          // first page of data and clear the selected rows & secondary fields
          // in the form state
          currentDataPage = 0;
          rowsPerPage = 10;
          final config = formFieldConfig!;
          formState!.clearSelectedRow(config.group, config.key);
          formState!.setValue(config.group, config.key, null);
          if (tableConfig.formStateConfig != null) {
            for (final field in tableConfig.formStateConfig!.otherColumns) {
              formState!.setValue(config.group, field.stateKey, null);
            }
          }
          dataSource.getModelData();
          return;
        }
      }
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(navListener);
    dataSource.removeListener(triggetRefreshListner);
    dataSource.dispose();
    super.dispose();
  }

  void dialogResultHandler(BuildContext context, DTActionResult? result) {
    switch (result) {
      case DTActionResult.ok:
      case DTActionResult.canceled:
        break;
      case DTActionResult.okDataTableDirty:
        // refresh the data table
        print("Refreshing the data table YAY!");
        dataSource.getModelData();
        break;
      default:
      // case null
    }
  }

  /// Dispatcher to handled the data table actions
  void actionDispatcher(BuildContext context, ActionConfig ac) {
    switch (ac.actionType) {
      // Show a modal dialog
      case DataTableActionType.showDialog:
        if (ac.configForm == null) return;
        final dialogFormKey = GlobalKey<FormState>();
        final formConfig = getFormConfig(ac.configForm!);
        final dialogFormState = formConfig.makeFormState();
        showFormDialog<DTActionResult>(
          formKey: dialogFormKey,
          screenPath: _dataTableWidget.screenPath,
          context: context,
          formState: dialogFormState,
          formConfig: formConfig,
          validatorDelegate: dialogValidatorDelegate,
          actionsDelegate: actionsDelegate,
          resultHandler: dialogResultHandler,
        );
        break;
      // case 'edit':
      //   setState(() => isTableEditable = true);
      //   break;
      // case 'save':
      //   showAlertDialog(context, 'Save Changes Coming Soon!');
      //   setState(() => isTableEditable = false);
      //   break;
      // case 'delete':
      //   showAlertDialog(
      //       context, 'Delete Pipeline (with confirmation) Coming Soon!');
      //   setState(() => isTableEditable = false);
      //   break;
      // case 'cancel':
      //   showAlertDialog(
      //       context, 'Cancel changes (with confirmation) Coming Soon!');
      //   setState(() => isTableEditable = false);
      //   break;

      default:
        showAlertDialog(
            context, 'Oops something is wrong, unknown action ${ac.key}');
    }
  }

  void _sortTable(int columnIndex, bool ascending) async {
    //* TODO add sort on client side with time-based order from server
    // dataSource.sortModelData(columnIndex, ascending);
    setState(() {
      currentDataPage = 0;
      if (columnIndex != sortColumnIndex) {
        sortColumnIndex = columnIndex;
        sortAscending = true;
      } else {
        sortColumnIndex = columnIndex;
        sortAscending = !sortAscending;
      }
      dataSource.getModelData();
    });
  }

  // Functions for pagination
  bool _isLastPage() {
    return (currentDataPage + 1) * rowsPerPage >= dataSource.totalRowCount;
  }

  void _rowPerPageChanged(int? value) async {
    if (value == null) return;
    rowsPerPage = value;
    dataSource.getModelData();
  }

  void _gotoFirstPressed() async {
    currentDataPage = 0;
    dataSource.getModelData();
  }

  void _previousPressed() async {
    currentDataPage--;
    dataSource.getModelData();
  }

  void _nextPressed() async {
    currentDataPage++;
    dataSource.getModelData();
  }

  void _lastPressed() async {
    var n = dataSource.totalRowCount ~/ rowsPerPage;
    var r = dataSource.totalRowCount % n;
    currentDataPage = r == 0 ? n - 1 : n;
    dataSource.getModelData();
  }
}

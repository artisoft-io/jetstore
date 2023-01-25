import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:provider/provider.dart';

import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/data_table_source.dart';

List<String>? castInitialValue(
    int? group, String? key, JetsFormState? formState) {
  if (group == null || key == null || formState == null) return null;
  var value = formState.getValue(group, key);
  if (value is String) return [value];
  return value;
}

class JetsDataTableWidget extends FormField<WidgetField> {
  JetsDataTableWidget({
    required super.key,
    required this.screenPath,
    this.formFieldConfig,
    required this.tableConfig,
    this.formState,
    required this.validatorDelegate,
    required this.actionsDelegate,
  })  : assert((formState != null && formFieldConfig != null) ||
            (formState == null && formFieldConfig == null)),
        super(
          initialValue: castInitialValue(
              formFieldConfig?.group, formFieldConfig?.key, formState),
          validator: formFieldConfig != null
              ? (WidgetField? value) => validatorDelegate(
                  formState!, formFieldConfig.group, formFieldConfig.key, value)
              : null,
          autovalidateMode: formFieldConfig != null
              ? formFieldConfig.autovalidateMode
              : AutovalidateMode.disabled,
          builder: (FormFieldState<WidgetField> field) {
            final state = field as JetsDataTableState;
            final context = field.context;
            final ThemeData themeData = Theme.of(context);
            final MaterialLocalizations localizations =
                MaterialLocalizations.of(context);
            // prepare the footer widgets
            final TextStyle? footerTextStyle = themeData.textTheme.bodySmall;
            List<DropdownMenuItem<int>> rowsPerPageItems =
                state.availableRowsPerPage
                    .map<DropdownMenuItem<int>>((e) => DropdownMenuItem<int>(
                          value: e,
                          child: Text('$e'),
                        ))
                    .toList();
            final List<DataColumn> dataColumns = state.columnsConfig
                .where((e) => !e.isHidden)
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
                Padding(
                  padding: const EdgeInsets.fromLTRB(defaultPadding, 0, 0, 0),
                  child: Text(
                    tableConfig.label,
                    style: Theme.of(context).textTheme.headlineSmall,
                  ),
                )
            ];
            headerRow.addAll(tableConfig.actions
                .where((ac) => ac.isVisible(state))
                .expand((ac) => [
                      const SizedBox(width: defaultPadding),
                      ElevatedButton(
                        style: buttonStyle(ac.style, themeData),
                        onPressed: ac.isEnabled(state)
                            ? () => state.actionDispatcher(context, ac)
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
  final ValidatorDelegate validatorDelegate;
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
  int? sortColumnIndex;
  String sortColumnName = '';
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
      _dataTableWidget.validatorDelegate;
  FormActionsDelegate get actionsDelegate => _dataTableWidget.actionsDelegate;

  @override
  void initState() {
    super.initState();

    // this may be an empty list if table is a domain table
    columnsConfig = tableConfig.columns;
    // sortColumnIndex may not be resolved until we get the columns
    setSortingColumn();
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

    // register for change notification on the form state
    if (formState != null && formFieldConfig != null) {
      formState!.addListener(refreshOnFormStateChange);
    }

    if (JetsRouterDelegate().user.isAuthenticated) {
      dataSource.getModelData();
    } else {
      // Get the first batch of data when navigated to screenPath
      JetsRouterDelegate().addListener(navListener);
    }
  }

  /// Get the sort column index as seen by the data table,
  /// i.e., the position of sortColumnName among the visible columns
  /// If the column is not visible or not found, defaults to the first
  /// visible column
  void setSortingColumn({int columnIndex = -1}) {
    if (columnsConfig.isEmpty) return;
    var filteredColumns = columnsConfig.where((e) => !e.isHidden);
    if (filteredColumns.isEmpty) {
      print("error: table has no visible columns!");
      sortColumnIndex = null;
      sortColumnName = '';
      return;
    }
    if (columnIndex < 0 || columnIndex >= filteredColumns.length) {
      // Use the configuration setting, which is specified by column name
      var sortPos = 0;
      for (var col in filteredColumns) {
        if (col.name == tableConfig.sortColumnName) {
          if (col.isHidden) {
            print("error: table sort column is not visible!");
            sortColumnIndex = null;
            sortColumnName = '';
            return;
          } else {
            sortColumnIndex = sortPos;
            sortColumnName = col.name;
            return;
          }
        }
        sortPos++;
      }
    } else {
      // use columnIndex, which came from gesture
      var col = filteredColumns.elementAt(columnIndex);
      sortColumnIndex = columnIndex;
      sortColumnName = col.name;
      return;
    }
    print("error: table sort column unexpected fall through!");
    sortColumnIndex = null;
    sortColumnName = '';
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

  void _refreshTable() {
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
  }

  void refreshOnFormStateChange() {
    assert(formState != null);
    assert(formFieldConfig != null);
    var group = formFieldConfig!.group;
    for (final whereClause in tableConfig.whereClauses) {
      if (whereClause.formStateKey != null) {
        // print(
        //     "whereClause on group ${formFieldConfig!.group}, key ${whereClause.formStateKey} for ${formFieldConfig?.key}");
        if (formState!.isKeyUpdated(group, whereClause.formStateKey!)) {
          // where clause have changed, refresh the table, make sure to go to
          // first page of data and clear the selected rows & secondary fields
          // in the form state
          _refreshTable();
          break;
        }
      }
    }
    for (final key in tableConfig.refreshOnKeyUpdateEvent) {
      if (formState!.isKeyUpdated(group, key)) {
        _refreshTable();
        break;
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

  void dialogResultHandler(BuildContext context, JetsFormState dialogFormState,
      DTActionResult? result) {
    switch (result) {
      case DTActionResult.ok:
      case DTActionResult.canceled:
        break;
      case DTActionResult.okDataTableDirty:
        // refresh the data table
        dataSource.getModelData();
        break;
      case DTActionResult.statusError:
        var msg = dialogFormState.getValue(0, FSK.serverError);
        if (msg != null) {
          showAlertDialog(context, msg);
        }
        break;
      default:
      // case null
    }
  }

  /// Dispatcher to handled the data table actions
  void actionDispatcher(BuildContext context, ActionConfig ac) async {
    switch (ac.actionType) {
      // Show a modal dialog
      case DataTableActionType.showDialog:
        if (ac.configForm == null) return;
        final dialogFormKey = GlobalKey<FormState>();
        final formConfig = getFormConfig(ac.configForm!);
        final dialogFormState =
            formConfig.makeFormState(parentFormState: formState);
        // check if we expect to have a selected row
        JetsRow? row = dataSource.getFirstSelectedRow();
        if (row == null && ac.isEnabledWhenHavingSelectedRows == true) return;
        // add state information to dialogFormState if navigationParams exists
        if (ac.stateFormNavigationParams != null) {
          ac.stateFormNavigationParams?.forEach((key, npKey) {
            var value = formState?.getValue(0, npKey);
            if (value is List<String>) {
              dialogFormState.setValue(0, key, value[0]);
            } else {
              dialogFormState.setValue(0, key, value);
            }
          });
        } else {
          ac.navigationParams?.forEach((key, value) {
            if (value is String?) {
              dialogFormState.setValue(0, key, value);
            } else {
              if (row != null) {
                dialogFormState.setValue(0, key, row[value]);
              }
            }
          });
        }
        // reset the updated keys since these updates is to put default values
        // and is not from user interactions
        dialogFormState.resetUpdatedKeys(0);
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
      // Navigate to a page
      case DataTableActionType.showScreen:
        if (ac.configScreenPath == null) return;
        // find the first selected row
        JetsRow? row = dataSource.getFirstSelectedRow();
        // check if no row is selected while we expect to have one selected
        if (row == null && ac.isEnabledWhenHavingSelectedRows == true) return;
        var params = ac.navigationParams?.map((key, value) {
          if (value is String?) return MapEntry(key, value);
          return MapEntry(key, row![value]);
        });
        // print("NAVIGATING to ${ac.configScreenPath}, with ${params}");
        JetsRouterDelegate()(
            JetsRouteData(ac.configScreenPath!, params: params));
        break;
      // Refresh data table
      case DataTableActionType.refreshTable:
        _refreshTable();
        break;
      // Call server to do an action
      case DataTableActionType.doAction:
        JetsRow? row = dataSource.getFirstSelectedRow();
        // check if no row is selected while we expect to have one selected
        if (row == null && ac.isEnabledWhenHavingSelectedRows == true) return;
        if (formState == null || ac.actionName == null) return;
        
        // perform the action then refresh the table
        formState!.addCallback(_refreshTable);
        await actionsDelegate(
            context, GlobalKey<FormState>(), formState!, ac.actionName!,
            group: 0);
        // _refreshTable();
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
    // dataSource.sortModelData(columnIndex, ascending);
    setSortingColumn(columnIndex: columnIndex);
    dataSource.getModelData();
    setState(() {
      currentDataPage = 0;
      if (columnIndex != sortColumnIndex) {
        sortAscending = true;
      } else {
        sortAscending = !sortAscending;
      }
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

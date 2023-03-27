import 'dart:convert';
import 'package:flutter/material.dart';

import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/data_table.dart';

class JetsDataTableSource extends ChangeNotifier {
  JetsDataTableSource({required this.state, required this.httpClient});
  final JetsDataTableState state;
  final HttpClient httpClient;
  JetsDataModel? model;
  int _totalRowCount = 0;
  List<bool> selectedRows = <bool>[];
  bool _whereClauseSatisfied = false;

  int get rowCount => model != null ? model!.length : 0;
  int get totalRowCount => _totalRowCount;
  bool get isWhereClauseSatisfied => _whereClauseSatisfied;
  bool get isWhereClauseSatisfiedOrDefaultToAllRows =>
      _whereClauseSatisfied || state.tableConfig.defaultToAllRows;

  /// returns true if state has all keys defined
  bool stateHasKeys(int group, List<String> keys) {
    if (state.formState == null) {
      return false;
    }
    JetsFormState fs = state.formState!;
    for (var key in keys) {
      if (fs.getValue(group, key) == null) return false;
    }
    return true;
  }

  /// returns true if table has selected row(s)
  bool hasSelectedRows() {
    for (var b in selectedRows) {
      if (b) return true;
    }
    return false;
  }

  /// returns the first selected row
  JetsRow? getFirstSelectedRow() {
    for (int i = 0; i < rowCount; i++) {
      if (selectedRows[i]) {
        return model?[i];
      }
    }
    return null;
  }

  /// Update the form state:
  /// This is in response to a gesture of selecting or de-selecting a row.
  ///  if [isAdd] is true, add row identified by index to the form state
  ///  otherwise remove it.
  ///  This takes in consideration the row key as well as secondary keys
  ///  defined in [DataTableFormStateConfig] of the table's config.
  ///  Note that if the row's key column is null, then the form state is not updated
  void _updateFormState(int index, bool isAdd) {
    // Get the row key value (rowKeyValue)
    var formStateConfig = state.tableConfig.formStateConfig;
    var formState = state.formState;
    if (formStateConfig == null || model == null || formState == null) {
      return;
    }
    JetsRow row = model![index];
    var rowKeyValue = row[formStateConfig.keyColumnIdx];
    if (rowKeyValue == null) return;
    var config = state.formFieldConfig!;
    if (isAdd) {
      // Add row to the selected rows
      formState.addSelectedRow(config.group, config.key, rowKeyValue, row);
    } else {
      // row is removed from the selected rows
      formState.removeSelectedRow(config.group, config.key, rowKeyValue);
    }
    // update the row (primary) key to form state
    // use the selected rows to have those selected on other data page
    formState.resetUpdatedKeys(config.group);
    var selRowPKs = <String>[];
    Iterable<JetsRow>? itor = formState.selectedRows(config.group, config.key);
    if (itor != null) {
      for (final JetsRow selRow in itor) {
        final value = selRow[formStateConfig.keyColumnIdx];
        if (value != null) {
          selRowPKs.add(value);
        }
      }
    }
    // save the selected primary keys
    if (selRowPKs.isNotEmpty) {
      formState.setValue(config.group, config.key, selRowPKs);
      state.didChange(selRowPKs);
    } else {
      formState.setValue(config.group, config.key, null);
      state.didChange(null);
    }

    if (formStateConfig.otherColumns.isEmpty) {
      formState.notifyListeners();
      return;
    }
    // Reset the secondary keys in form state.
    // Note that secondary keys MUST be set in form state ONLY by
    // the this data table (the one with the primary key)
    // Other widgets associated with the form can read these value
    // but should not update it since they are reset here regardless
    // of the other widgets.
    List<Set<String>> secondaryValues =
        List.generate(formStateConfig.otherColumns.length, (_) => <String>{});
    itor = formState.selectedRows(config.group, config.key);
    if (itor != null) {
      for (final JetsRow selRow in itor) {
        for (var i = 0; i < formStateConfig.otherColumns.length; i++) {
          final otherColConfig = formStateConfig.otherColumns[i];
          final value = selRow[otherColConfig.columnIdx];
          if (value != null) {
            secondaryValues[i].add(value);
          }
        }
      }
    }
    for (var i = 0; i < secondaryValues.length; i++) {
      final otherColConfig = formStateConfig.otherColumns[i];
      if (secondaryValues[i].isEmpty) {
        formState.setValue(config.group, otherColConfig.stateKey, null);
      } else {
        formState.setValue(
            config.group, otherColConfig.stateKey, secondaryValues[i].toList());
      }
    }
    formState.notifyListeners();
  }

  /// Update table's selected rows based on form state
  void updateTableFromFormState() {
    var formStateConfig = state.tableConfig.formStateConfig;
    var formState = state.formState;
    if (formStateConfig == null || model == null || formState == null) {
      return;
    }
    var config = state.formFieldConfig!;
    // Expecting WidgetField from form state
    // Although it may be a String if the formState was initialized from
    // a Data Table row (case update a record)
    var value = formState.getValue(config.group, config.key);
    if (value == null) return;
    assert((value is String) || (value is List<String>), 'Unexpected type');
    if (value.isEmpty) return;
    WidgetField? selValues = [];
    if (value is List<String>) {
      selValues = value;
    } else {
      String str = value;
      if (str[0] == '{') {
        selValues = str.substring(1, str.length - 1).split(',');
      } else {
        selValues = [value];
      }
    }

    // update selectedRows based on form state,
    // also drop selected row in form state that are no longer in the model
    // in case the where clause has changed
    for (int index = 0; index < model!.length; index++) {
      final JetsRow row = model![index];
      var rowKeyValue = row[formStateConfig.keyColumnIdx];
      if (rowKeyValue != null && selValues.contains(rowKeyValue)) {
        selectedRows[index] = true;
      }
    }
  }

  void _onSelectChanged(int index, bool value) {
    if (state.tableConfig.isCheckboxSingleSelect && value) {
      for (int i = 0; i < model!.length; i++) {
        if (selectedRows[i]) {
          selectedRows[i] = false;
          _updateFormState(i, false);
        }
      }
    }
    selectedRows[index] = value;
    _updateFormState(index, value);
    notifyListeners();
  }

  DataRow getRow(int index) {
    assert(model != null);
    // print("getRow Called with index $index which has key ${model![index][0]} ");
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
      cells: state.columnsConfig
          .where((e) => !e.isHidden)
          .map((e) => e.maxLines > 0
              ? DataCell(SizedBox(
                  width: e.columnWidth, //SET width
                  child: Text(
                    model![index][e.index] ?? 'null',
                    maxLines: e.maxLines,
                  )))
              : DataCell(Text(model![index][e.index] ?? 'null')))
          .toList(),
      selected: selectedRows[index],
      onSelectChanged: state.isTableEditable && isWhereClauseSatisfied
          ? (bool? value) {
              if (value == null) return;
              _onSelectChanged(index, value);
            }
          : null,
    );
  }

  dynamic _makeQuery() {
    final columns = state.tableConfig.columns;
    final config = state.formFieldConfig; // when datatable is in a form
    // List of Column for select stmt
    List<Map<String, String>> selectColumns = [];
    if (columns.isNotEmpty) {
      selectColumns = List<Map<String, String>>.generate(
          columns.length,
          (index) => <String, String>{
                'table': columns[index].table ?? '',
                'column': columns[index].name
              });
    }
    var msg = <String, dynamic>{'action': state.tableConfig.apiAction};
    // from clauses (table name(s))
    List<Map<String, String>> fromClauses = [];
    for (final fc in state.tableConfig.fromClauses) {
      var table = fc.tableName;
      if (table.isEmpty) {
        table = JetsRouterDelegate().currentConfiguration?.params['table'];
      }
      fromClauses.add(<String, String>{
        'schema': fc.schemaName,
        'table': table,
      });
    }
    msg['fromClauses'] = fromClauses;

    // add WHERE clauses
    List<Map<String, dynamic>> whereClauses = [];
    for (final wc in state.tableConfig.whereClauses) {
      var value =
          JetsRouterDelegate().currentConfiguration?.params[wc.formStateKey];
      if (value != null) {
        whereClauses.add(<String, dynamic>{
          'table': wc.table ?? '',
          'column': wc.column,
          'values': [value],
        });
      } else if (config == null || wc.formStateKey == null) {
        if (wc.defaultValue.isNotEmpty) {
          whereClauses.add(<String, dynamic>{
            'table': wc.table ?? '',
            'column': wc.column,
            'values': wc.defaultValue,
          });
        } else if (wc.joinWith != null) {
          whereClauses.add(<String, dynamic>{
            'table': wc.table ?? '',
            'column': wc.column,
            'joinWith': wc.joinWith,
          });
        }
      } else {
        var values = state.formState?.getValue(config.group, wc.formStateKey!);
        if (values != null) {
          if (values is String?) {
            whereClauses.add(<String, dynamic>{
              'table': wc.table ?? '',
              'column': wc.column,
              'values': [values],
            });
          } else {
            assert(
                values is List<String>?, "Incorrect data type in form state");
            var l = values as List<String>;
            if (l.isNotEmpty) {
              whereClauses.add(<String, dynamic>{
                'table': wc.table ?? '',
                'column': wc.column,
                'values': values,
              });
            }
          }
        } else {
          if (wc.defaultValue.isNotEmpty) {
            whereClauses.add(<String, dynamic>{
              'table': wc.table ?? '',
              'column': wc.column,
              'values': wc.defaultValue,
            });
          }
        }
      }
    }
    if (whereClauses.isNotEmpty) {
      msg['whereClauses'] = whereClauses;
    }
    msg['offset'] = state.indexOffset;
    msg['limit'] = state.rowsPerPage;
    if (columns.isNotEmpty) {
      msg['columns'] = selectColumns;
      msg['sortColumn'] = state.sortColumnName;
    } else {
      if (state.columnNameMaps.isNotEmpty) {
        msg['columns'] = state.columnNameMaps;
        msg['sortColumn'] = state.sortColumnName;
      } else {
        msg['columns'] = [];
        msg['sortColumn'] = '';
      }
    }
    msg['sortAscending'] = state.sortAscending;
    // print("*** Query object $msg");
    return msg;
  }

  Future<Map<String, dynamic>?> fetchData() async {
    var result = await httpClient.sendRequest(
        path: state.tableConfig.apiPath,
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

  void getModelData() async {
    selectedRows = List<bool>.filled(state.rowsPerPage, false);
    // Check if this data table widget is part of a form and depend on
    // row selection of another widget, if so let's make sure that
    // widget has data selected
    final config = state.formFieldConfig;
    var hasBlockingFilter = false;
    if (config != null) {
      for (final wc in state.tableConfig.whereClauses) {
        if (wc.defaultValue.isNotEmpty) continue;
        if (wc.formStateKey != null) {
          final value =
              state.formState?.getValue(config.group, wc.formStateKey!);
          if (value == null) {
            hasBlockingFilter = true;
          }
        }
      }
    }
    _whereClauseSatisfied = true;
    if (hasBlockingFilter) {
      _whereClauseSatisfied = false;
      if (!state.tableConfig.defaultToAllRows) {
        model = null;
        _totalRowCount = 0;
        notifyListeners();
        return;
      }
    }
    var data = await fetchData();
    if (data != null) {
      // Check if we got columnDef back
      var columnDef = data['columnDef'] as List<dynamic>?;
      if (columnDef != null) {
        state.columnsConfig = columnDef
            .map((m1) => ColumnConfig(
                  index: m1['index'],
                  name: m1['name'],
                  label: m1['label'],
                  tooltips: m1['tooltips'] ?? '',
                  isNumeric: m1['isnumeric'] ?? false,
                  maxLines: m1['maxLines'] ?? 0,
                  columnWidth: m1['columnWidth'] ?? 0,
                ))
            .toList();
        state.columnNameMaps = columnDef
            .map((e) => <String, String>{'column': e['name'] as String})
            .toList();
        state.setSortingColumn(columnIndex: 0);
      }
      // The table's label
      state.label = data['label'] ?? state.label;
      // The table's rows
      final rows = data['rows'] as List;
      model = rows.map((e) => (e as List).cast<String?>()).toList();
      // model = rows.cast<JetsRow>().toList();
      _totalRowCount = data['totalRowCount'];
      // Set selectedRows based on form state
      updateTableFromFormState();
      notifyListeners();
    }
  }

  //* TODO currently not used, do we need local sort?
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
            double.parse(l[columnIndex]!)
                .compareTo(double.parse(r[columnIndex]!));
      }
      return sortSign *
          l[columnIndex].toString().compareTo(r[columnIndex].toString());
    });
  }
}

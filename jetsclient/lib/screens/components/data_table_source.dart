import 'dart:convert';
import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/data_table.dart';

class JetsDataTableSource extends ChangeNotifier {
  JetsDataTableSource({required this.state});
  final JetsDataTableState state;
  JetsDataModel? model;
  int _totalRowCount = 0;
  List<bool> selectedRows = <bool>[];
  bool _whereClauseSatisfied = false;
  bool _addWhereClauseOnClient = true;

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
    if (model == null) return null;
    final sz = min(selectedRows.length, model!.length);
    for (int i = 0; i < sz; i++) {
      if (selectedRows[i]) {
        return model?[i];
      }
    }
    return null;
  }

  void _resetSecondaryKeys(
      final DataTableFormStateConfig formStateConfig, JetsFormState formState) {
    // Reset the secondary keys in form state.
    // Note that secondary keys MUST be set in form state ONLY by
    // the this data table (the one with the primary key)
    // Other widgets associated with the form can read these value
    // but should not update it since they are reset here regardless
    // of the other widgets.
    final config = state.formFieldConfig!;
    List<Set<String>> secondaryValues =
        List.generate(formStateConfig.otherColumns.length, (_) => <String>{});
    Iterable<JetsRow>? itor = formState.selectedRows(config.group, config.key);
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
        // //DEV
        // print(
        //     "${config.key}: Secondary Value ${otherColConfig.stateKey} Set to null");
        formState.setValue(config.group, otherColConfig.stateKey, null);
      } else {
        // //DEV
        // print(
        //     "${config.key}: Secondary Value ${otherColConfig.stateKey} Set to ${secondaryValues[i].toList().join(",")}");
        formState.setValue(
            config.group, otherColConfig.stateKey, secondaryValues[i].toList());
      }
    }
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
    final formStateConfig = state.tableConfig.formStateConfig;
    var formState = state.formState;
    if (formStateConfig == null || model == null || formState == null) {
      return;
    }
    JetsRow row = model![index];
    var rowKeyValue = row[formStateConfig.keyColumnIdx];
    if (rowKeyValue == null) return;
    final config = state.formFieldConfig!;
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
    // //DEV
    // if (isAdd) {
    //   print("${config.key}: Adding Selected Row $index");
    // } else {
    //   print("${config.key}: Removing Selected Row $index");
    // }
    _resetSecondaryKeys(formStateConfig, formState);
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
    formState.clearSelectedRow(config.group, config.key);
    final sz = min(selectedRows.length, model!.length);
    for (int index = 0; index < sz; index++) {
      final JetsRow row = model![index];
      var rowKeyValue = row[formStateConfig.keyColumnIdx];
      if (rowKeyValue != null && selValues.contains(rowKeyValue)) {
        selectedRows[index] = true;
        formState.addSelectedRow(config.group, config.key, rowKeyValue, row);
      }
    }
  }

  void _onSelectChanged(int index, bool value) {
    if (state.tableConfig.isCheckboxSingleSelect && value) {
      final sz = min(selectedRows.length, model!.length);
      for (int i = 0; i < sz; i++) {
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
              ? DataCell(
                  SizedBox(
                      width: e.columnWidth, //SET width
                      child: Text(model![index][e.index] ?? 'null',
                          maxLines: e.maxLines)), onLongPress: () {
                  Clipboard.setData(
                      ClipboardData(text: model![index][e.index] ?? 'null'));
                  ScaffoldMessenger.of(state.context).showSnackBar(
                      const SnackBar(content: Text("Copied to Clipboard")));
                })
              : DataCell(Text(model![index][e.index] ?? 'null'),
                  onLongPress: () {
                  Clipboard.setData(
                      ClipboardData(text: model![index][e.index] ?? 'null'));
                  ScaffoldMessenger.of(state.context).showSnackBar(
                      const SnackBar(content: Text("Copied to Clipboard")));
                }))
          .toList(),
      selected: selectedRows.length > index ? selectedRows[index] : false,
      onSelectChanged: state.isTableEditable && isWhereClauseSatisfied
          ? (bool? value) {
              if (value == null) return;
              _onSelectChanged(index, value);
            }
          : null,
    );
  }

  Map<String, dynamic>? _makeWhereClause(WhereClause wc) {
    final config = state.formFieldConfig; // when datatable is in a form

    // Check if column name is in formState
    var columnName = wc.column;
    if (wc.lookupColumnInFormState && config != null) {
      final v = state.formState?.getValue(config.group, wc.column);
      assert(v is String, "Error: Column Name not found in stateForm");
      columnName = v;
    }

    // Check if the Wehereclause column is client
    if (wc.column == 'client') {
      _addWhereClauseOnClient = false;
    }

    // Check if value is comming from screen param (navigation param)
    // only for case where there is no formState or it's not a dialog (isDialog = false)
    if (state.formState == null ||
        (state.formState!.activeFormWidgetState != null &&
            !state.formState!.activeFormWidgetState!.isDialog)) {
      var value =
          JetsRouterDelegate().currentConfiguration?.params[wc.formStateKey];
      if (value != null) {
        return <String, dynamic>{
          'table': wc.table ?? '',
          'column': columnName,
          'values': [value],
        };
      }
    }

    // Check if whereclause has a predicate to satisfy
    var predicateSatisfied = true;
    if (config != null && wc.predicate != null) {
      var value =
          state.formState?.getValue(config.group, wc.predicate!.formStateKey);
      if (wc.predicate!.formStateKey == FSK.client) {
        _addWhereClauseOnClient = false;
      }
      if (value != wc.predicate!.expectedValue) {
        predicateSatisfied = false;
      }
    }
    if (!predicateSatisfied) {
      return null;
    }

    if (config == null || wc.formStateKey == null) {
      if (wc.defaultValue.isNotEmpty) {
        return <String, dynamic>{
          'table': wc.table ?? '',
          'column': columnName,
          'values': wc.defaultValue,
        };
      }
      if (wc.joinWith != null) {
        return <String, dynamic>{
          'table': wc.table ?? '',
          'column': columnName,
          'joinWith': wc.joinWith,
        };
      }
    } else {
      var values = state.formState?.getValue(config.group, wc.formStateKey!);
      if (values != null) {
        if (values is String?) {
          return <String, dynamic>{
            'table': wc.table ?? '',
            'column': columnName,
            'values': [values],
          };
        }
        assert(values is List<String?>?, "Incorrect data type in form state");
        var l = values as List<String?>;
        if (l.isNotEmpty) {
          return <String, dynamic>{
            'table': wc.table ?? '',
            'column': columnName,
            'values': values,
          };
        }
      } else {
        if (wc.defaultValue.isNotEmpty) {
          return <String, dynamic>{
            'table': wc.table ?? '',
            'column': columnName,
            'values': wc.defaultValue,
          };
        }
      }
    }
    return null;
  }

  dynamic _makeQuery() {
    final columns = state.tableConfig.columns;
    final config = state.formFieldConfig;
    // reset _addWhereClauseOnClient
    _addWhereClauseOnClient = true;
    // Check if there is a select client in context
    if (JetsRouterDelegate().selectedClient == null) {
      _addWhereClauseOnClient = false;
    }
    // List of Column for select stmt
    var hasClientColumn = false;
    List<Map<String, String>> selectColumns = [];
    if (columns.isNotEmpty) {
      selectColumns = List<Map<String, String>>.generate(
          columns.length,
          (index) => <String, String>{
                'table': columns[index].table ?? '',
                'column': columns[index].name
              });
      for (final col in columns) {
        if (col.name == 'client') {
          hasClientColumn = true;
        }
      }
    }
    if (!hasClientColumn) {
      _addWhereClauseOnClient = false;
    }
    // The message aka DataTableAction (from data_table_action.go)
    var msg = <String, dynamic>{'action': state.tableConfig.apiAction};

    // With clauses
    List<Map<String, String>> withClauses = [];
    for (final wc in state.tableConfig.withClauses) {
      var stmt = wc.asStatement;
      for (final k in wc.stateVariables) {
        if (config == null) {
          print(
              "ERROR: Table having WITH statement but FormDataTableFieldConfig is null!");
        }
        final v = state.formState?.getValue(config!.group, k);
        stmt = stmt.replaceAll(RegExp('{$k}'), v ?? 'NULL');
      }
      stmt = stmt.replaceAll(RegExp("'NULL'"), 'NULL');
      withClauses.add(<String, String>{
        'name': wc.withName,
        'stmt': stmt,
      });
    }
    msg['withClauses'] = withClauses;

    // from clauses (table name(s))
    List<Map<String, String>> fromClauses = [];
    for (final fc in state.tableConfig.fromClauses) {
      var table = fc.tableName;
      if (table.isEmpty) {
        var v = state.formState
            ?.getValue(state.formFieldConfig!.group, 'table_name');
        v ??= JetsRouterDelegate().currentConfiguration?.params['table_name'];
        if (v != null) {
          table = v;
        } else {
          print("Error: Don't have a table_name!");
        }
      }
      if (fc.asTableName.isNotEmpty) {
        fromClauses.add(<String, String>{
          'schema': fc.schemaName,
          'table': table,
          'asTable': fc.asTableName,
        });
      } else {
        fromClauses.add(<String, String>{
          'schema': fc.schemaName,
          'table': table,
        });
      }
    }
    msg['fromClauses'] = fromClauses;

    // Distinct On clause
    if (state.tableConfig.distinctOnClauses.isNotEmpty) {
      msg['distinctOnClauses'] = state.tableConfig.distinctOnClauses;
    }

    // add WHERE clauses
    List<Map<String, dynamic>> whereClauses = [];
    for (final wc in state.tableConfig.whereClauses) {
      var wcValue = _makeWhereClause(wc);
      if (wcValue != null) {
        whereClauses.add(wcValue);
      }
    }
    // if _addWhereClauseOnClient is still true, then add to where clause
    if (_addWhereClauseOnClient) {
      // Add to where clause
      whereClauses.add(<String, dynamic>{
        'table': state.tableConfig.fromClauses[0].tableName,
        'column': 'client',
        'values': [JetsRouterDelegate().selectedClient!],
      });
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
    // Check if we have a raw query / query tool
    dynamic query;
    if (state.tableConfig.apiAction == 'raw_query') {
      final group = state.formFieldConfig?.group ?? 0;
      var action = 'raw_query';
      query = state.formState?.getValue(group, FSK.rawQueryReady);
      if (query == null) {
        query = state.formState?.getValue(group, FSK.rawDdlQueryReady);
        if (query != null) {
          action = 'exec_ddl';
        }
      }

      query ??= state.tableConfig.sqlQuery?.sqlQuery;
      if (query != null) {
        query = query as String;
        var vars = state.tableConfig.sqlQuery?.stateVariables;
        if (vars != null) {
          for (final k in vars) {
            final v = state.formState?.getValue(group, k);
            query = query.replaceAll(RegExp('{$k}'), v ?? 'NULL');
          }
        }
        // The message aka DataTableAction (from data_table_action.go)
        var msg = <String, dynamic>{'action': action};
        msg['query'] = query;
        if (state.tableConfig.requestColumnDef) {
          msg['requestColumnDef'] = true;
        }
        query = msg;
      }
    } else {
      query = _makeQuery();
    }
    var result = await HttpClientSingleton().sendRequest(
        path: state.tableConfig.apiPath,
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: json.encode(query));

    if (!state.mounted) return null;
    if (result.statusCode == 200) {
      // update the [model]
      print("*** Data Table Got Data");
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
      showAlertDialog(state.context, result.body['error']);
      return null;
    } else {
      const snackBar = SnackBar(
        content: Text('Unknown Error reading data from table'),
      );
      ScaffoldMessenger.of(state.context).showSnackBar(snackBar);
      showAlertDialog(state.context, result.body['error']);
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
          var value = state.formState?.getValue(config.group, wc.formStateKey!);
          value ??= JetsRouterDelegate()
              .currentConfiguration
              ?.params[wc.formStateKey!];
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
        print("*** Table has blocking filter, no refresh");
        return;
      }
    }
    Map<String, dynamic>? data;
    if (state.tableConfig.modelStateFormKey != null) {
      data = state.formState?.getValue(0, state.tableConfig.modelStateFormKey!);
    } else {
      data = await fetchData();
    }
    model = null;
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
      _totalRowCount = data['totalRowCount'] ?? model!.length;
      // Set selectedRows based on form state
      updateTableFromFormState();
      notifyListeners();
      // reset the form state variable used for notification only
      final group = state.formFieldConfig?.group ?? 0;
      state.formState?.setValue(group, FSK.rawQueryReady, null);
      state.formState?.setValue(group, FSK.rawDdlQueryReady, null);
      state.formState?.setValue(group, FSK.queryReady, null);
      state.formState?.resetUpdatedKeys(group);
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

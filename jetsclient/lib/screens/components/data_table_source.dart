import 'dart:convert';
import 'package:flutter/material.dart';

import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/data_table.dart';

// typedef JetsDataModel = List<List<String?>>;
typedef JetsDataModel = List<dynamic>;

class JetsDataTableSource extends ChangeNotifier {
  JetsDataTableSource({required this.state, required this.httpClient});
  final JetsDataTableState state;
  final HttpClient httpClient;
  JetsDataModel? model;
  int _totalRowCount = 0;
  List<bool> selectedRows = <bool>[];

  int get rowCount => model != null ? model!.length : 0;
  int get totalRowCount => _totalRowCount;

  //// Update the form state:
  ///  if [isAdd] is true, add row identified by index to the form state
  ///  otherwise remove it.
  ///  This takes in consideration the row key as well as secondary keys
  ///  defined in [DataTableFormStateConfig] of the table's config.
  ///  Note that if the row's key column is null, then the form state is not updated
  void updateFormState(int index, bool isAdd) {
    // Get the row key value (rowKeyValue)
    var formStateConfig = state.tableConfig.formStateConfig;
    var formState = state.formState;
    if (formStateConfig == null || model == null || formState == null) {
      return;
    }
    var row = model![index];
    var rowKeyValue = row?[formStateConfig.keyColumnIdx];
    if (rowKeyValue == null) return;
    var config = state.formFieldConfig!;
    var selValues =
        formState.getValue(config.group, config.key) as Set<String>?;
    if (isAdd) {
      // row is added to the selected rows
      formState.addSelectedRow(config.group, config.key, index, row);
      // add the row (primary) key to form state
      if (selValues != null) {
        var isChanged = selValues.add(rowKeyValue);
        if (isChanged) formState.notifyListeners();
      } else {
        formState.setValue(config.group, config.key, <String>{rowKeyValue});
      }
    } else {
      // row is removed to the selected rows
      formState.removeSelectedRow(config.group, config.key, index);
      if (selValues != null) {
        var isChanged = selValues.remove(rowKeyValue);
        if (isChanged) formState.notifyListeners();
      }
    }
    // Reset the secondary keys in form state.
    // Note that secondary keys MUST be set in form state ONLY by
    // the this data table (the one with the primary key)
    // Other widgets associated with the form can read these value
    // but should not update it since they are reset here regardless
    // of the other widgets.
    for (final otherColConfig in formStateConfig.otherColumns) {
      formState.setValue(config.group, otherColConfig.stateKey, <String>{});
    }
    var itor = formState.selectedRows(config.group, config.key)
        as Iterable<List<String?>>?;
    if (itor != null) {
      for (final selRow in itor) {
        for (final otherColConfig in formStateConfig.otherColumns) {
          var value = selRow[otherColConfig.columnIdx];
          if (value != null) {
            formState
                .getValue(config.group, otherColConfig.stateKey)
                .add(value);
          }
        }
      }
    }
  }

  //// Update table's selected rows based on form state
  void updateTableFromFormState() {
    var formStateConfig = state.tableConfig.formStateConfig;
    var formState = state.formState;
    if (formStateConfig == null || model == null || formState == null) {
      return;
    }
    var config = state.formFieldConfig!;
    var selValues =
        formState.getValue(config.group, config.key) as Set<String>?;
    if (selValues == null) return;
    for (int index = 0; index < model!.length; index++) {
      var row = model![index] as List<String?>;
      var rowKeyValue = row[formStateConfig.keyColumnIdx];
      if (rowKeyValue != null && selValues.contains(rowKeyValue)) {
        selectedRows[index] = true;
      }
    }
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
      cells: List<DataCell>.generate(model![0].length,
          (int colIndex) => DataCell(Text(model![index][colIndex] ?? 'null'))),
      selected: selectedRows[index],
      onSelectChanged: state.isTableEditable
          ? (bool? value) {
              if (value == null) return;
              selectedRows[index] = value;
              updateFormState(index, value);
              notifyListeners();
            }
          : null,
    );
  }

  dynamic _makeQuery() {
    var schemaName = state.tableConfig.schemaName;
    var tableName = state.tableConfig.tableName;
    var columns = state.tableConfig.columns;
    List<String> columnNames = [];
    if (columns.isNotEmpty) {
      columnNames =
          List<String>.generate(columns.length, (index) => columns[index].name);
    }
    var msg = <String, dynamic>{'action': 'read'};
    msg['schema'] = schemaName;
    if (tableName.isEmpty) {
      String name = JetsRouterDelegate().currentConfiguration?.params['table'];
      msg['table'] = name;
    } else {
      msg['table'] = tableName;
    }
    // add where clauses
    List<Map<String, dynamic>> whereClauses = [];
    var config = state.formFieldConfig;
    for (final wc in state.tableConfig.whereClauses) {
      if (config == null || wc.formStateKey == null) {
        if (wc.defaultValue.isNotEmpty) {
          whereClauses.add(<String, dynamic>{wc.column: wc.defaultValue});
        }
      } else {
        var values = state.formState?.getValue(config.group, wc.formStateKey!);
        if (values != null) {
          assert(values is String || values is Set<String>,
              "Invalid type in form state");
          if (values is String) {
            whereClauses.add(<String, dynamic>{
              wc.column: [values]
            });
          } else {
            whereClauses.add(<String, dynamic>{wc.column: values.toList()});
          }
        } else {
          if (wc.defaultValue.isNotEmpty) {
            whereClauses.add(<String, dynamic>{wc.column: wc.defaultValue});
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
      msg['columns'] = columnNames;
      msg['sortColumn'] = columnNames[state.sortColumnIndex];
    } else {
      if (state.columnNames.isNotEmpty) {
        msg['columns'] = state.columnNames;
        msg['sortColumn'] = state.columnNames[state.sortColumnIndex];
      } else {
        msg['columns'] = [];
        msg['sortColumn'] = '';
      }
    }
    msg['sortAscending'] = state.sortAscending;
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
                tooltips: m1['tooltips'],
                isNumeric: m1['isnumeric']))
            .toList();
        state.columnNames = columnDef.map((e) => e['name'] as String).toList();
      }
      model = data['rows'];
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
            double.parse(l[columnIndex])
                .compareTo(double.parse(r[columnIndex]));
      }
      return sortSign *
          l[columnIndex].toString().compareTo(r[columnIndex].toString());
    });
  }
}

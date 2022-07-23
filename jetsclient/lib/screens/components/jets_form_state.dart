import 'dart:convert';
import 'package:flutter/material.dart';

//// Jets Form validator
typedef ValidatorDelegate = String? Function(int, String, dynamic);

typedef InternalStateMap = List<Map<String, dynamic>>;
typedef InternalSelectedRowMap = List<Map<String, Map<int, dynamic>>>;

class JetsFormState extends ChangeNotifier {
  JetsFormState({required this.groupCount})
      : _state = InternalStateMap.filled(groupCount, <String, dynamic>{}),
        _selectedRows = InternalSelectedRowMap.filled(
            groupCount, <String, Map<int, dynamic>>{});
  final int groupCount;
  //// The actual state of the form, keyed by validation group (list item)
  ///  and widget key
  final InternalStateMap _state;
  //// Keep track of selected rows for data table form widgets
  ///  using the same keying as [_state] does.
  final InternalSelectedRowMap _selectedRows;

  void setValue(int group, String key, dynamic value) {
    assert(group < groupCount, "invalid group");
    var didit = false;
    if (value == null) {
      // remove the binding
      didit = _state[group].remove(key);
    } else {
      _state[group][key] = value;
      didit = true;
    }
    if (didit) notifyListeners();
  }

  //// return the state of validation [group]
  Map<String, dynamic> getState(int group) {
    assert(group < groupCount, "invalid group");
    return _state[group];
  }

  //// Get a value from the state at
  /// validation group [group] and key [key]
  dynamic getValue(int group, String key) {
    assert(group < groupCount, "invalid groupCount");
    print("FormState.getValue called for group $group, key $key");
    return _state[group][key];
  }

  //// Add a selected row from a data table
  void addSelectedRow(int group, String key, int rowIndex, dynamic row) {
    assert(group < groupCount, "invalid groupCount");
    var widgetSelectedRows = _selectedRows[group][key];
    if (widgetSelectedRows == null) {
      _selectedRows[group][key] = <int, dynamic>{rowIndex: row};
    } else {
      widgetSelectedRows[rowIndex] = row;
    }
  }

  //// Remove a selected row from a data table
  void removeSelectedRow(int group, String key, int rowIndex) {
    assert(group < groupCount, "invalid groupCount");
    var widgetSelectedRows = _selectedRows[group][key];
    if (widgetSelectedRows == null) return;
    widgetSelectedRows.remove(rowIndex);
  }

  //// return an [Iterable] over the selected rows
  Iterable<dynamic>? selectedRows(int group, String key) {
    assert(group < groupCount, "invalid groupCount");
    var rows = _selectedRows[group][key];
    if (rows == null) return null;
    return rows.values;
  }

  //// encode a validation group,
  //// return json string representing Map<String, dynamic>
  String encodeState(int group) {
    assert(group < groupCount, "invalid groupCount");
    return jsonEncode(getState(group));
  }

  //// encode the full state,
  //// return json string representing List<Map<String, dynamic>>
  String encodeFullState() {
    return jsonEncode(_state);
  }

  // Return value of find first occurence of [key] across groups
  dynamic findFirst(String key) {
    for (int i = 0; i < _state.length; i++) {
      if (_state[i].containsKey(key)) {
        return _state[i][key];
      }
    }
  }
}

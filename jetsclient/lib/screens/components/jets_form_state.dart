import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';

/// Jets Form validator
/// The last argument is either String? or List<String?>
typedef ValidatorDelegate = String? Function(int, String, dynamic);

/// Selected rows mapping, key is [JetsDataModel] model's row index
typedef SelectedRows = Map<int, JetsRow>;

/// Widget field model
/// Note that [JetsTextFormField] and [JetsDropdownButtonFormField] data element
/// is a single String which is whapped in a list.
/// [JetsDataTableWidget] allow multi-select and therefore emits List<String>
/// where the string is the row key field or a secondary field per
/// [DataTableFormStateConfig] configuration of the data table
typedef WidgetField = List<String>;

/// Validation group model
/// Validation group have a mapping
/// of widget's key to a List<String> being the data element of the widget.
typedef ValidationGroup = Map<String, WidgetField>;

/// [JetsFormState] internal model for form widget data elements.
/// Consist of List of validation groups.
typedef InternalFormState = List<ValidationGroup>;

/// Mapping of widget's key with associated selected rows.
/// Note this applied only to data table widgets.
typedef ValidationGroupSelectedRow = Map<String, SelectedRows>;

/// List of [SelectedRows] by validation groups.
/// Each validation group can have multiple data table, the
/// selected rows are grouped by widget's key.
typedef InternalSelectedRow = List<ValidationGroupSelectedRow>;

class JetsFormState extends ChangeNotifier {
  JetsFormState({required this.groupCount})
      : _state = InternalFormState.filled(groupCount, <String, WidgetField>{}),
        _selectedRows =
            InternalSelectedRow.filled(groupCount, <String, SelectedRows>{});
  final int groupCount;
  //// The actual state of the form, keyed by validation group (list item)
  ///  and widget key
  final InternalFormState _state;
  //// Keep track of selected rows for data table form widgets
  ///  using the same keying as [_state] does.
  final InternalSelectedRow _selectedRows;

  void setValue(int group, String key, dynamic value) {
    assert(group < groupCount, "invalid group");
    assert((value is String?) || (value is WidgetField?),
        "form state values are expected to be String? or WidgetField? (List<String>?), got ${value.runtimeType}");
    var didit = false;
    if (value == null) {
      // remove the binding if any
      didit = _state[group].remove(key) != null;
    } else {
      if (value is String) {
        _state[group][key] = [value];
      } else {
        _state[group][key] = value;
      }
      didit = true;
    }
    if (didit) notifyListeners();
  }

  /// return the model value (aka state) for validation [group]
  ValidationGroup getState(int group) {
    assert(group < groupCount, "invalid group");
    return _state[group];
  }

  /// Get a value from the state at
  /// validation group [group] and key [key]
  /// [key] is widget key.
  WidgetField? getValue(int group, String key) {
    assert(group < groupCount, "invalid groupCount");
    final value = _state[group][key];
    //*
    print(
        "FormState.getValue called for group $group, key $key, returning $value");
    return value;
  }

  /// Add a selected row from a data table
  void addSelectedRow(int group, String key, int rowIndex, JetsRow row) {
    assert(group < groupCount, "invalid groupCount");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) {
      _selectedRows[group][key] = <int, JetsRow>{rowIndex: row};
    } else {
      selectedRows[rowIndex] = row;
    }
  }

  /// Remove a selected row from a data table
  void removeSelectedRow(int group, String key, int rowIndex) {
    assert(group < groupCount, "invalid groupCount");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) return;
    selectedRows.remove(rowIndex);
  }

  /// return an [Iterable] over the selected rows
  Iterable<JetsRow>? selectedRows(int group, String key) {
    assert(group < groupCount, "invalid groupCount");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) return null;
    return selectedRows.values;
  }

  /// encode a validation group,
  /// return json string representing [ValidationGroup]
  String encodeState(int group) {
    assert(group < groupCount, "invalid groupCount");
    return jsonEncode(getState(group));
  }

  /// encode the full state,
  /// return json string representing [InternalFormState]
  String encodeFullState() {
    return jsonEncode(_state);
  }

  // Return value of find first occurence of [key] across groups
  WidgetField? findFirst(String key) {
    for (int i = 0; i < _state.length; i++) {
      if (_state[i].containsKey(key)) {
        return _state[i][key];
      }
    }
    return null;
  }
}

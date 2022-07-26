import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';

/// Jets Form validator
/// The last argument is either String? or List<String?>
typedef ValidatorDelegate = String? Function(int, String, dynamic);

/// Selected rows mapping, key is row primary key
typedef SelectedRows = Map<String, JetsRow>;

/// Data Table Widget field model
/// Note that [JetsTextFormField] and [JetsDropdownButtonFormField] data element
/// is a single String while
/// [JetsDataTableWidget] allow multi-select and therefore emits List<String>
/// where the string is the row key field or a secondary field per
/// [DataTableFormStateConfig] configuration of the data table
/// As a result, the Form State Model has a dynamic as data element for
/// the widget, and the client to FormState data is responsible
/// to cast it to either String? or List<String>?
typedef WidgetField = List<String>;

/// Validation group model
/// Validation group have a mapping
/// of widget's key to a String or List<String> depending on the widget.
typedef ValidationGroup = Map<String, dynamic>;

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

/// For each group, list of keys that have updated value
typedef InternalUpdatedKeys = List<Set<String>>;

class JetsFormState extends ChangeNotifier {
  JetsFormState({required this.groupCount})
      : _state = InternalFormState.filled(groupCount, <String, dynamic>{}),
        _selectedRows =
            InternalSelectedRow.filled(groupCount, <String, SelectedRows>{}),
        _updatedKeys = InternalUpdatedKeys.filled(groupCount, <String>{});
  final int groupCount;

  /// The actual state of the form, keyed by validation group (list item)
  ///  and widget key
  final InternalFormState _state;

  /// Keep track of selected rows for data table form widgets
  ///  using the same keying as [_state] does (group and widget key).
  /// The selected rows are in a map keyed by the row's primary key
  final InternalSelectedRow _selectedRows;

  /// Keep track of keys that have value that changed
  final InternalUpdatedKeys _updatedKeys;

  void resetUpdatedKeys(int group) {
    assert(group < groupCount, "invalid group");
    _updatedKeys[group].clear();
  }

  /// [group] is validation group
  /// [key] is widget key
  void markKeyAsUpdated(int group, String key) {
    assert(group < groupCount, "invalid group");
    _updatedKeys[group].add(key);
  }

  /// [group] is validation group
  /// [key] is widget key
  bool isKeyUpdated(int group, String key) {
    assert(group < groupCount, "invalid group");
    return _updatedKeys[group].contains(key);
  }

  void setValue(int group, String key, dynamic value) {
    assert(group < groupCount, "invalid group");
    assert((value is String?) || (value is WidgetField?),
        "form state values are expected to be String? or WidgetField? (List<String>?), got ${value.runtimeType}");
    // //*
    // print(
    //     "FormState.setValue called for group $group, key $key, with value $value");
    var didit = false;
    if (value == null) {
      // remove the binding if any
      didit = _state[group].remove(key) != null;
    } else {
      final oldValue = _state[group][key];
      if (oldValue != value) {
        _state[group][key] = value;
        didit = true;
      }
    }
    if (didit) markKeyAsUpdated(group, key);
  }

  /// return the model value (aka state) for validation [group]
  ValidationGroup getState(int group) {
    assert(group < groupCount, "invalid group");
    return _state[group];
  }

  /// Get a value from the state at
  /// validation group [group] and key [key]
  /// [key] is widget key.
  dynamic getValue(int group, String key) {
    assert(group < groupCount, "invalid group");
    final value = _state[group][key];
    // //*
    // print(
    //     "FormState.getValue called for group $group, key $key, returning $value");
    return value;
  }

  /// Add a selected row from a data table
  /// [group] is the validation group
  /// [key] is the widget key
  /// [rowPK] is row's primary key
  void addSelectedRow(int group, String key, String rowPK, JetsRow row) {
    assert(group < groupCount, "invalid group argument");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) {
      _selectedRows[group][key] = <String, JetsRow>{rowPK: row};
    } else {
      selectedRows[rowPK] = row;
    }
    // //*
    // print(
    //     "FormState.addSelectedRow called for group $group, key $key, rowIndex $rowPK, selected rows are now ${selectedRows?.keys}");
  }

  /// Remove a selected row from a data table
  /// [group] is the validation group
  /// [key] is the widget key
  /// [rowPK] is row's primary key
  void removeSelectedRow(int group, String key, String rowPK) {
    assert(group < groupCount, "invalid group");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) return;
    selectedRows.remove(rowPK);
    if (selectedRows.isEmpty) {
      _selectedRows[group].remove(key);
    }
    // //*
    // print(
    //     "FormState.removeSelectedRow called for group $group, key $key, rowIndex $rowPK, selected rows are now ${selectedRows.keys}");
  }

  /// Clear selected row from a data table
  /// [group] is the validation group
  /// [key] is the widget key
  void clearSelectedRow(int group, String key) {
    assert(group < groupCount, "invalid group");
    _selectedRows[group].remove(key);
    // //*
    // print(
    //     "FormState.clearSelectedRow called for group $group, key $key");
  }

  /// return an [Iterable] over the selected rows
  Iterable<JetsRow>? selectedRows(int group, String key) {
    assert(group < groupCount, "invalid group");
    SelectedRows? selectedRows = _selectedRows[group][key];
    if (selectedRows == null) return null;
    return selectedRows.values;
  }

  /// encode a validation group,
  /// return json string representing [ValidationGroup]
  String encodeState(int group, [String? indent]) {
    assert(group < groupCount, "invalid groupCount");
    return _encodeObject(getState(group), indent);
  }

  /// encode the full state,
  /// return json string representing [InternalFormState]
  String encodeFullState([String? indent]) {
    return _encodeObject(_state, indent);
  }

  String _encodeObject(Object state, String? indent) {
    final JsonEncoder encoder = JsonEncoder.withIndent(indent);
    return encoder.convert(state);
  }

  // Return value of find first occurence of [key] across groups
  dynamic findFirst(String key) {
    for (int i = 0; i < _state.length; i++) {
      if (_state[i].containsKey(key)) {
        return _state[i][key];
      }
    }
    return null;
  }
}

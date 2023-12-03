import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:jetsclient/models/data_table_model.dart';
import 'package:jetsclient/components/form.dart';

/// Jets Form validator
/// The last argument is either String? or List<String?>
typedef ValidatorDelegate = String? Function(
    JetsFormState formState, int, String, dynamic);

/// Do nothing Form Validator, ie Always Valid
String? alwaysValidForm(formState, p2, p3, p4) => null;

typedef AnonymousCallback = void Function();

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

typedef InternalCache = Map<String, dynamic>;

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
  JetsFormState({required initialGroupCount, this.parentFormState})
      : groupCount = initialGroupCount > 0 ? initialGroupCount : 1,
        _state = InternalFormState.generate(
            initialGroupCount > 0 ? initialGroupCount : 1,
            (index) => <String, dynamic>{},
            growable: true),
        _selectedRows = InternalSelectedRow.generate(
            initialGroupCount > 0 ? initialGroupCount : 1,
            (index) => <String, SelectedRows>{},
            growable: true),
        _updatedKeys = InternalUpdatedKeys.generate(
            initialGroupCount > 0 ? initialGroupCount : 1,
            (index) => <String>{},
            growable: true),
        _invalidKeys = InternalUpdatedKeys.generate(
            initialGroupCount > 0 ? initialGroupCount : 1,
            (index) => <String>{},
            growable: true),
        _callbacks = [];

  /// number of validation groups in the form state
  int groupCount;

  /// Indicates if the form associated with this state is a dialog
  bool isDialog = false;

  /// The associated formKey (used in screen delegate actions to trigger form validation)
  GlobalKey<FormState>? formKey;

  /// Active [JetsForm] instance associated with this [JetsFormState]
  /// Usefull when need to modify the list of input fields such as for
  /// dialog having a [FormConfig] with [formWithDynamicRows] set to [true]
  JetsFormWidgetState? activeFormWidgetState;

  /// Applicable to form state for dialogs;
  /// To have access to the form state of the parent form who
  /// created this state for a dialog form.
  /// Typically used to mark keys as dirty to refresh table having
  /// a where clause.
  final JetsFormState? parentFormState;

  /// Applicable to form state for ScreenWithMultiForms;
  /// To have access to all peer form state
  List<JetsFormState>? peersFormState;

  /// The actual state of the form, keyed by validation group (list item)
  ///  and widget key
  final InternalFormState _state;

  /// A cache mainly for form builders to access metadata information
  final InternalCache _cache = {};

  /// Keep track of selected rows for data table form widgets
  ///  using the same keying as [_state] does (group and widget key).
  /// The selected rows are in a map keyed by the row's primary key
  final InternalSelectedRow _selectedRows;

  /// Keep track of keys that have value that changed
  final InternalUpdatedKeys _updatedKeys;

  /// Keep track of keys that have value that are invalid based on
  /// form validation. This is used when form fields are setup
  /// to autovalidate (typically used with form builder, see [JetsForm] class)
  final InternalUpdatedKeys _invalidKeys;

  /// List of callback function using in data table action
  final List<AnonymousCallback> _callbacks;

  void addCallback(AnonymousCallback cb) {
    // print("+++ addCallback called");
    _callbacks.add(cb);
  }

  void removeCallback(AnonymousCallback cb) {
    // print("--- removeCallback called");
    _callbacks.remove(cb);
  }

  void invokeCallbacks() {
    // print("~~~ invokeCallback called");
    for (var cb in _callbacks) {
      cb();
    }
  }

  void resizeFormState(int newGroupCount) {
    // print("Resizing formState from $groupCount to $newGroupCount");
    var n = newGroupCount - groupCount;
    if (n > 0) {
      _state.addAll(
          InternalFormState.generate(n, (index) => <String, dynamic>{}));
      groupCount = _state.length;
      _selectedRows.addAll(
          InternalSelectedRow.generate(n, (index) => <String, SelectedRows>{}));
      _updatedKeys
          .addAll(InternalUpdatedKeys.generate(n, (index) => <String>{}));
      _invalidKeys
          .addAll(InternalUpdatedKeys.generate(n, (index) => <String>{}));
    }
  }

  void removeValidationGroup(int group) {
    assert(group < groupCount, "invalid group");
    _state.removeAt(group);
    _selectedRows.removeAt(group);
    _updatedKeys.removeAt(group);
    _invalidKeys.removeAt(group);
    groupCount = _state.length;
  }

  void resetUpdatedKeys(int group) {
    assert(group < groupCount, "invalid group");
    // print("resetUpdatedKeys clearing out group $group, keys ${_updatedKeys[group]}");
    _updatedKeys[group].clear();
  }

  /// [group] is validation group
  /// [key] is widget key
  void markKeyAsUpdated(int group, String key) {
    assert(group < groupCount, "invalid group");
    // print("markKeyAsUpdated add group $group, key $key");
    _updatedKeys[group].add(key);
  }

  /// [group] is validation group
  /// [key] is widget key
  bool isKeyUpdated(int group, String key) {
    assert(group < groupCount, "invalid group");
    return _updatedKeys[group].contains(key);
  }

  Set<String> getUpdatedKeys(int group) {
    assert(group < groupCount, "invalid group");
    return _updatedKeys[group];
  }

  /// Check for keys marked as invalid, if any are found then the form does not
  /// pass validation
  bool isFormValid() {
    for (var keys in _invalidKeys) {
      if (keys.isNotEmpty) return false;
    }
    return true;
  }

  /// Mark form element identified by [key] as not passing form validation
  void markFormKeyAsInvalid(int group, String key) {
    assert(group < groupCount, "invalid group");
    _invalidKeys[group].add(key);
    notifyListeners();
  }

  /// Mark form element identified by [key] as not passing form validation
  void markFormKeyAsValid(int group, String key) {
    assert(group < groupCount, "invalid group");
    _invalidKeys[group].remove(key);
    notifyListeners();
  }

  /// Set a form state [value] for widget [key]
  /// for validation [group]
  void setValue(int group, String key, dynamic value) {
    // print(
    //     "setValue: group $group, key $key, value $value :: groupCount $groupCount");
    assert(group < groupCount,
        "setValue: invalid group: $group, key is $key, value $value");
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

  /// Set a form state [value] for widget [key]
  /// for validation [group] and notify listeners
  /// of the change
  void setValueAndNotify(int group, String key, dynamic value) {
    resetUpdatedKeys(group);
    setValue(group, key, value);
    notifyListeners();
  }

  /// return the full model (aka state) as a list of ValidationGroup
  InternalFormState getInternalState() {
    return _state;
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
    if (!(group < groupCount)) {
      print(
          "Error in getValue with group $group while groupCount is $groupCount and _state length is ${_state.length}");
      return;
    }
    final value = _state[group][key];
    return value;
  }

  void addCacheValue(String key, dynamic value) {
    _cache[key] = value;
  }

  dynamic getCacheValue(String key) {
    return _cache[key];
  }

  /// Add a selected row from a data table
  /// [group] is the validation group
  /// [key] is the widget key
  /// [rowPK] is row's primary key
  void addSelectedRow(int group, String key, String rowPK, JetsRow row) {
    if (!(group < groupCount)) {
      print(
          "Error in addSelectedRow with group $group while groupCount is $groupCount and _selectedRows length is ${_selectedRows.length}");
      return;
    }

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
    final JsonEncoder encoder = JsonEncoder.withIndent(indent, (_) => '');
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

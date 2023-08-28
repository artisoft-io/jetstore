import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';

class JetsDropdownButtonFormField extends StatefulWidget {
  const JetsDropdownButtonFormField({
    required super.key,
    required this.screenPath,
    required this.formFieldConfig,
    required this.onChanged,
    required this.formValidator,
    required this.formState,
  });
  final JetsRouteData screenPath;
  final FormDropdownFieldConfig formFieldConfig;
  final void Function(String?) onChanged;
  final JetsFormState formState;

  /// Note: Validator is required as this control needs to be part of a form
  ///       so to have formFieldConfig.
  /// (Future requirements) We need to externalize the widget
  /// config (as done for data table) to to be able to use the widget
  /// without a form. Same applies to input text from.
  final JetsFormFieldValidator formValidator;

  @override
  State<JetsDropdownButtonFormField> createState() =>
      _JetsDropdownButtonFormFieldState();
}

class _JetsDropdownButtonFormFieldState
    extends State<JetsDropdownButtonFormField> {
  late final FormDropdownFieldConfig _config;
  String? predicatePreviousValue;
  String? selectedValue;
  List<DropdownItemConfig> items = [];

  @override
  void initState() {
    super.initState();
    _config = widget.formFieldConfig;
    // Check if there is a selection made in the form state
    // (case we are editing existing record versus add where there would be no
    //  existing value)
    selectedValue = widget.formState.getValue(_config.group, _config.key);
    // print("*** JetsDropdownButtonFormFieldState.initState() called");

    if (_config.dropdownItemsQuery != null) {
      if (_config.stateKeyPredicates.isNotEmpty ||
          _config.whereStateContains.isNotEmpty) {
        widget.formState.addListener(stateListener);
      }
      queryDropdownItems();
    } else {
      items.addAll(_config.items);
      if (items.isNotEmpty) {
        selectedValue = selectedValue ?? items[_config.defaultItemPos].value;
        widget.formState.setValue(_config.group, _config.key, selectedValue);
      }
    }
  }

  void stateListener() async {
    queryDropdownItems();
  }

  @override
  void dispose() {
    // print("*** DropDown dispose called");
    if (_config.dropdownItemsQuery != null) {
      if (_config.stateKeyPredicates.isNotEmpty ||
          _config.whereStateContains.isNotEmpty) {
        widget.formState.removeListener(stateListener);
      }
    }
    super.dispose();
  }

  void setDropdownItems(List<dynamic> rows) {
    final model = rows.map((e) => (e as List).cast<String?>()).toList();
    items = [];
    items.addAll(_config.items);
    items.addAll(
        model.map((e) => DropdownItemConfig(label: e[0]!, value: e[0]!)));
    if (_config.returnedModelCacheKey != null) {
      widget.formState.addCacheValue(_config.returnedModelCacheKey!, model);
    }
    var gotit = false;
    if (selectedValue != null) {
      // make sure selectedValue is in the returned list, otherwise set it to null
      for (var items in model) {
        for (var item in items) {
          if (selectedValue == item) {
            gotit = true;
            break;
          }
        }
        if (gotit) break;
      }
    }
    if (!gotit) selectedValue = null;
    setState(() {
      if (items.isNotEmpty) {
        selectedValue = selectedValue ?? items[_config.defaultItemPos].value;
        widget.formState.setValue(_config.group, _config.key, selectedValue);
      }
    });
  }

  void queryDropdownItems() async {
    // Check if user is logged in
    if (!JetsRouterDelegate().user.isAuthenticated) {
      // print("*** queryDropdownItems CANCELLED for ${_config.key} not auth");
      return;
    }

    // Check if we have predicate on formState
    var query = _config.dropdownItemsQuery;
    if (query == null) return;

    // check if the notification came from this widget
    // if so ignore it otherwise we'll overite the user's
    // choice in the formState
    if (widget.formState.isKeyUpdated(_config.group, _config.key)) {
      // print("*** queryDropdownItems: bailing out - self update");
      return;
    }

    // Check if has precondition
    var whereMatch = true;
    if (_config.whereStateContains.isNotEmpty) {
      _config.whereStateContains.forEach((key, value) {
        var stateValue = widget.formState.getValue(_config.group, key);
        if (stateValue is List<String>) {
          if (value != stateValue[0]) {
            whereMatch = false;
            return;
          }
        } else {
          if (value != stateValue) {
            whereMatch = false;
            return;
          }
        }
      });
    }
    if (!whereMatch) {
      // Clear the items
      if (items.isNotEmpty) {
        setState(() {
          predicatePreviousValue = null;
          items = [];
        });
      }
      return;
    }

    String valueStr = '';
    if (_config.stateKeyPredicates.isNotEmpty) {
      for (var key in _config.stateKeyPredicates) {
        var value = widget.formState.getValue(_config.group, key);
        if (value == null) {
          // Clear the items
          if (items.isNotEmpty) {
            setState(() {
              predicatePreviousValue = null;
              items = [];
            });
          }
          return;
        }
        assert((value is String) || (value is List<String>),
            "Error: unexpected type in dropdown formState");
        if (value is String) {
          valueStr += value;
          query = query!.replaceAll(RegExp('{$key}'), value);
        } else {
          valueStr += value[0];
          query = query!.replaceAll(RegExp('{$key}'), value[0]);
        }
      }
    }

    // check if predicate has not changed, if so no need to query again
    if (predicatePreviousValue != null && predicatePreviousValue == valueStr) {
      // print("*** queryDropdownItems: bailing out - nothing changed");
      return;
    }
    predicatePreviousValue = valueStr;

    if (_config.returnedModelCacheKey != null) {
      final rows =
          widget.formState.getCacheValue(_config.returnedModelCacheKey!);
      if (rows != null) {
        setDropdownItems(rows);
        return;
      }
    }

    // print("*** queryDropdownItems: preparing raw query");
    var msg = <String, dynamic>{
      'action': 'raw_query',
    };
    msg['query'] = query;
    var encodedMsg = json.encode(msg);
    var result = await HttpClientSingleton().sendRequest(
        path: "/dataTable",
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: encodedMsg);
    if (!mounted) return;
    if (result.statusCode == 200) {
      _config.dropdownItemLoaded = true;
      final rows = result.body['rows'] as List;
      setDropdownItems(rows);
    } else if (result.statusCode == 401) {
      // const snackBar = SnackBar(
      //   content: Text('Session Expired, please login'),
      // );
      // ScaffoldMessenger.of(context).showSnackBar(snackBar);
    } else {
      const snackBar = SnackBar(
        content: Text('Error reading dropdown list items'),
      );
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(snackBar);
      }
    }
    // print("*** queryDropdownItems: DONE!");
  }

  @override
  Widget build(BuildContext context) {
    return Expanded(
      flex: widget.formFieldConfig.flex,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16.0, 0.0, 16.0, 0.0),
        child: DropdownButtonFormField<String>(
            value: selectedValue,
            onChanged: (String? newValue) {
              setState(() {
                selectedValue = newValue;
              });
              widget.onChanged(newValue);
            },
            autovalidateMode: _config.autovalidateMode,
            validator: (p0) =>
                widget.formValidator(_config.group, _config.key, p0),
            items: items
                .map((e) => DropdownMenuItem<String>(
                    value: e.value, child: Text(e.label)))
                .toList()),
      ),
    );
  }
}

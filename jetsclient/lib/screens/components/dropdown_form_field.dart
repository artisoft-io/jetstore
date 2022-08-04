import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:provider/provider.dart';

class JetsDropdownButtonFormField extends StatefulWidget {
  const JetsDropdownButtonFormField({
    required super.key,
    required this.screenPath,
    required this.formFieldConfig,
    required this.onChanged,
    required this.formValidator,
    this.flex = 1,
  });
  final JetsRouteData screenPath;
  final FormDropdownFieldConfig formFieldConfig;
  final void Function(String?) onChanged;

  /// Note: (Future requirements) validator is require as this control needs to be part of a form
  ///       so to have formFieldConfig. We need to externalize the widget
  ///       config (as done for data table) to to be able to use the widget
  ///       without a form. Same applies to input text from.
  final JetsFormFieldValidator formValidator;
  final int flex;

  @override
  State<JetsDropdownButtonFormField> createState() =>
      _JetsDropdownButtonFormFieldState();
}

class _JetsDropdownButtonFormFieldState
    extends State<JetsDropdownButtonFormField> {
  late final FormDropdownFieldConfig _config;
  late final HttpClient httpClient;
  String? selectedValue;

  @override
  void initState() {
    super.initState();
    httpClient = Provider.of<HttpClient>(context, listen: false);
    _config = widget.formFieldConfig;
    if (_config.dropdownItemsQuery != null) {
      if (widget.screenPath.path == homePath) {
        // Get the first batch of data when navigated to screenPath
        JetsRouterDelegate().addListener(navListener);
      } else {
        queryDropdownItems();
      }
    } else if (_config.items.isNotEmpty) {
      selectedValue = _config.items[_config.defaultItemPos].value;
    }
  }

  void navListener() async {
    if (JetsRouterDelegate().currentConfiguration?.path == homePath) {
      queryDropdownItems();
    }
  }

  void queryDropdownItems() async {
    if (_config.dropdownItemLoaded) return;
    var msg = <String, dynamic>{
      'action': 'raw_query',
      'nbrColumns': 1,
    };
    msg['query'] = _config.dropdownItemsQuery;
    var encodedMsg = json.encode(msg);
    var result = await httpClient.sendRequest(
        path: "/dataTable",
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: encodedMsg);
    if (!mounted) return;
    if (result.statusCode == 200) {
      _config.dropdownItemLoaded = true;
      final rows = result.body['rows'] as List;
      final model = rows.map((e) => (e as List).cast<String?>()).toList();
      _config.items.addAll(
          model.map((e) => DropdownItemConfig(label: e[0]!, value: e[0]!)));
      setState(() {
        if (_config.items.isNotEmpty) {
          selectedValue = _config.items[_config.defaultItemPos].value;
        }
      });
    } else if (result.statusCode == 401) {
      const snackBar = SnackBar(
        content: Text('Session Expired, please login'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
    } else {
      const snackBar = SnackBar(
        content: Text('Error reading dropdown list items'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Expanded(
      flex: widget.flex,
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
            validator: (p0) =>
                widget.formValidator(_config.group, _config.key, p0),
            items: _config.items
                .map((e) => DropdownMenuItem<String>(
                    value: e.value, child: Text(e.label)))
                .toList()),
      ),
    );
  }
}

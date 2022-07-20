import 'package:flutter/material.dart';

import 'package:jetsclient/utils/form_config.dart';

class JetsDropdownButtonFormField extends StatefulWidget {
  const JetsDropdownButtonFormField({
    required super.key,
    required this.formFieldConfig,
    required this.onChanged,
    required this.validator,
    this.flex = 1});
  final FormDropdownFieldConfig formFieldConfig;
  final void Function(String?) onChanged;
  // Note: validator is require as this control needs to be part of a form
  //       so to have formFieldConfig. We need to externalize the widget
  //       config (as done for data table) to to be able to use the widget
  //       without a form. Same applies to input text from.
  final FormFieldValidator<String> validator;
  final int flex;

  @override
  State<JetsDropdownButtonFormField> createState() =>
      _JetsDropdownButtonFormFieldState();
}

class _JetsDropdownButtonFormFieldState
    extends State<JetsDropdownButtonFormField> {
  late final FormDropdownFieldConfig _config;
  String? selectedValue;

  @override
  void initState() {
    super.initState();
    _config = widget.formFieldConfig;
    if (_config.items.isNotEmpty) {
      selectedValue = _config.items[_config.defaultItemPos].value;
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
            validator: widget.validator,
            items: _config.items
                .map((e) => DropdownMenuItem<String>(
                    value: e.value, child: Text(e.label)))
                .toList()),
      ),
    );
  }
}

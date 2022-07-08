import 'package:flutter/material.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsDropdownButtonFormField extends StatefulWidget {
  const JetsDropdownButtonFormField(
      {Key? key,
      required this.inputFieldConfig,
      required this.onChanged,
      required this.validatorDelegate})
      : super(key: key);
  final FormDropdownFieldConfig inputFieldConfig;
  final void Function(String?) onChanged;
  final ValidatorDelegate validatorDelegate;

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
    _config = widget.inputFieldConfig;
  }

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16.0, 0.0, 16.0, 0.0),
        child: DropdownButtonFormField<String>(
            validator: (value) => widget.validatorDelegate(_config.group, _config.key, value),
            value: selectedValue,
            onChanged: (String? newValue) {
              setState(() {
                selectedValue = newValue!;
              });
              widget.onChanged(newValue);
            },
            items: _config.items
                .map((e) => DropdownMenuItem<String>(
                    value: e.value,
                    child: Text(e.label)))
                .toList()),
      ),
    );
  }
}

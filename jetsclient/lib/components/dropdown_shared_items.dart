import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/routes/export_routes.dart';

class JetsDropdownWithSharedItemsFormField extends StatelessWidget {
  const JetsDropdownWithSharedItemsFormField({
    required super.key,
    required this.screenPath,
    required this.formFieldConfig,
    required this.onChanged,
    required this.formValidator,
    required this.formState,
    this.selectedValue,
  });
  final JetsRouteData screenPath;
  final FormDropdownWithSharedItemsFieldConfig formFieldConfig;
  final void Function(String?) onChanged;
  final JetsFormState formState;
  final String? selectedValue;
  final JetsFormFieldValidator formValidator;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      flex: formFieldConfig.flex,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16.0, 0.0, 16.0, 0.0),
        child: DropdownButtonFormField<String>(
            value: selectedValue,
            onChanged: (String? newValue) {
              formState.setValue(formFieldConfig.group, formFieldConfig.key, newValue);
              onChanged(newValue);
            },
            autovalidateMode: formFieldConfig.autovalidateMode,
            validator: (p0) =>
                formValidator(formFieldConfig.group, formFieldConfig.key, p0),
            items: formState.getCacheValue(formFieldConfig.dropdownMenuItemCacheKey),
      ),
    ));
  }
}

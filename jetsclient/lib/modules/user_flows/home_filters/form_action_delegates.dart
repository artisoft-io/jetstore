import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/user_flows/home_filters/form_action_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? homeFiltersFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "homeFiltersFormValidator has unexpected data type");
  switch (key) {
    // optional keys
    case FSK.processName:
    case FSK.status:
      return null;
    // required keys
    case DTKeys.hfFileKeyFilterTypeTableUF:
      if (v != null) {
        String value = v;
        if (value.isNotEmpty) {
          return null;
        }
      }
      return "Select an option";

    default:
      print(
          'Oops homeFiltersFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Home Filters UF Form Actions - set on UserFlowState
Future<String?> homeFiltersFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    case ActionKeys.hfSelectProcessUF:
    case ActionKeys.hfSelectStatusUF:
    case ActionKeys.hfSelectFileKeyFilterUF:
    case ActionKeys.hfSelectTimeWindowUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      updateHomeFilters(context, formState);
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for File Mapping UF State: $actionKey');
  }
  return null;
}

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? fileMappingFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "fileMappingFormValidator has unexpected data type");
  switch (key) {
    case DTKeys.inputSourceMapping:
      if (v != null) {
        return null;
      }
      return "A file configuration must be selected.";
    case DTKeys.processMappingTable:
      return null;

    default:
      print(
          'Oops fileMappingFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> fileMappingFormActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Prepopulate the type of file from current record
    case ActionKeys.fmSelectSourceConfigUF:
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.org] = unpack(state[FSK.org]);
      state[FSK.tableName] = unpack(state[FSK.tableName]);
      state[FSK.objectType] = unpack(state[FSK.objectType]);
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  return null;
}

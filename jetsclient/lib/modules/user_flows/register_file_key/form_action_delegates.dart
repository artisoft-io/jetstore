import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Load Files UF
String? registerFileKeyFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "registerFileKeyFormValidator has unexpected data type");
  switch (key) {
    case FSK.fileKey:
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return "Please provide a file key";
      }
      return null;
    case FSK.schemaEventJson:
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return "Please provide a Schema Event json";
      }
      // Validate that value is valid json
      try {
        jsonDecode(value);
      } catch (e) {
        return "Schema Event is not a valid json: ${e.toString()}";
      }
      return null;

    default:
      print(
          'Oops registerFileKeyFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Load Files UF Form Actions - set on UserFlowState
Future<String?> registerFileKeyFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Submit schema event
    case ActionKeys.rfkSubmitSchemaEventUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'put_schema_event_to_s3',
        'data': [
          {
            'file_key': unpack(state[FSK.fileKey]),
            'event': unpack(state[FSK.schemaEventJson]),
          }
        ],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.registerFileKeyEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
        Navigator.of(context).pop();
      }
      break;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    default:
      print(
          'Oops unknown ActionKey for Register File Key UF ActionKey: $actionKey');
  }
  return null;
}

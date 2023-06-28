import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';

/// This file contains
/// Validation and Actions delegates for the Source Config forms
/// Validation and Actions delegates for the Client & Org admin forms

/// Validation and Actions delegates for Load ALL Files
String? loadAllFilesValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Load ALL Files Form has unexpected data type");
  switch (key) {
    case FSK.sourcePeriodKey:
      if (v != null) {
        return null;
      }
      return "Source period must be selected.";

    default:
      print(
          'Oops Load ALL Files Form has no validator configured for form field $key');
  }
  return null;
}

/// Load ALL Files Form Actions
Future<String?> loadAllFilesActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Load All Files
    case ActionKeys.loadAllFilesOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      state[FSK.sourcePeriodKey] = state[FSK.sourcePeriodKey][0];
      state[FSK.dayPeriod] = state[FSK.dayPeriod][0];
      state['user_email'] = JetsRouterDelegate().user.email;
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'load_all_files',
        'data': [state],
      }, toEncodable: (_) => '');

      JetsSpinnerOverlay.of(context).show();
      return postInsertRows(context, formState, encodedJsonBody, serverEndPoint: ServerEPs.registerFileKeyEP);

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Load All Files Form: $actionKey');
  }
  return null;
}

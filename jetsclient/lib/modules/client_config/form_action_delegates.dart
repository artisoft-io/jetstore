

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? clientConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "ClientConfigFormValidator has unexpected data type");
  switch (key) {
    // case FSK.wsFileEditorContent:
    //   return null;

    default:
      print(
          'Oops Workspace Home Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> clientConfigFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Start UF
    case ActionKeys.crStartUF:
      // Init state with user email
      final state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
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

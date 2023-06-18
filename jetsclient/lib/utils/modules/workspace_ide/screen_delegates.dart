import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';

/// Validation and Actions delegates for the workspaceIDE forms
String? workspaceIDEFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "workspaceIDE Form has unexpected data type");
  switch (key) {
    case FSK.wsName:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Workspace name is too short.";
      }
      return "Workspace name must be provided.";

    case FSK.wsURI:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Workspace URI is too short.";
      }
      return "Workspace URI must be provided.";

    case FSK.description:
      return null;

    default:
      print(
          'Oops workspaceIDE Form has no validator configured for form field $key');
  }
  return null;
}

/// workspaceIDE Form Actions
Future<String?> workspaceIDEFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {

    // Add/Update Workspace Entry
    case ActionKeys.addWorkspaceOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      // print('Add/Update Workspace state: $state');
      var table = 'workspace_registry'; // case add
      if (formState.getValue(0, FSK.key) != null) {
        table = 'update/workspace_registry';
      }

      state['user_email'] = JetsRouterDelegate().user.email;
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': table}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);

    case ActionKeys.compileWorkspace:
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      print('Compiling Workspace state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'compile_workspace'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for workspaceIDE Form: $actionKey');
  }
  return null;
}

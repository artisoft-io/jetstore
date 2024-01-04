import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> pullWorkspaceFormActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Pull Workspace - prepare for confirmation
    case ActionKeys.wpPullWorkspaceConfirmUF:
      state[DTKeys.wpPullWorkspaceConfirmOptions] =
          state[DTKeys.otherWorkspaceActionOptions];
      // print('Workspace Pull Action: $actionKey, state: $state');
      break;

    // Pull Workspace changes and perform workspace action
    case ActionKeys.wpPullWorkspaceOkUF:
      // print('Workspace Pull Action: $actionKey, state: $state');
      state['user_email'] = JetsRouterDelegate().user.email;
      state[FSK.key] = unpack(state[FSK.key]);
      state[FSK.wsName] = unpack(state[FSK.wsName]);
      state[FSK.wsBranch] = unpack(state[FSK.wsBranch]);
      state[FSK.wsFeatureBranch] = unpack(state[FSK.wsFeatureBranch]);
      state[FSK.wsURI] = unpack(state[FSK.wsURI]);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'pull_workspace'}
        ],
        'workspaceName': state[FSK.wsName],
        'workspaceBranch': state[FSK.wsBranch],
        'featureBranch': state[FSK.wsFeatureBranch],
        'data': [state],
      }, toEncodable: (_) => '');
      final statusCode = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (statusCode == 200) return null;
      if (context.mounted) {
        showAlertDialog(context, "Server error, please try again.");
      }
      return "Error while pulling workspace";

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Pull Workspace UF State: $actionKey');
  }
  return null;
}

/// Validation delegate for Load Config UF
String? loadConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "loadConfigFormValidator has unexpected data type");

  switch (key) {
    case FSK.wpClientList:
      if (unpack(v) != null) return null;
      return "Select Client to load their configuration";

    case FSK.wsName:
    case FSK.wsURI:
      return null;

    default:
      print(
          'Oops loadConfigFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

Future<int> loadConfigInternal(
  BuildContext context,
  GlobalKey<FormState> formKey,
  JetsFormState formState,
  String actionKey,
  group) async {
    
  final state = Map<String, dynamic>.from(formState.getState(group));
  state['user_email'] = JetsRouterDelegate().user.email;
  state[FSK.updateDbClients] = unpackToList(state[FSK.wpClientList])?.join(',');
  state[FSK.wsName] = unpack(state[FSK.wsName]);
  // print('Load Config Action: $actionKey, state: $state');
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'workspace_insert_rows',
    'fromClauses': [
      <String, String>{'table': 'load_workspace_config'}
    ],
    'workspaceName': state[FSK.wsName],
    'data': [state],
  }, toEncodable: (_) => '');
  int result = 0;
  if (context.mounted) {
    JetsSpinnerOverlay.of(context).show();
    result = await postSimpleAction(
        context, formState, ServerEPs.dataTableEP, encodedJsonBody);
  }
  if (context.mounted) {
    JetsSpinnerOverlay.of(context).hide();
  }
  return result;
}

Future<String?> loadConfigFormActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Load Config - prepare for confirmation
    case ActionKeys.wpLoadConfigConfirmUF:
      final state = formState.getState(group);
      state[FSK.wpClientListRO] = state[FSK.wpClientList];
      // print('Load Client Config Action: $actionKey, state: $state');
      break;

    // Load Selected Client Config
    case ActionKeys.wpLoadAllClientConfigUF:
      formState.getState(group).remove(FSK.wpClientList);
      final result = await loadConfigInternal(
          context, formKey, formState, actionKey, group);
      if (result != 200)
        print("OOps, loading all client config http result is $result");
      if(context.mounted) {
        Navigator.of(context).pop();
      }
      break;
    case ActionKeys.wpLoadConfigOkUF:
      final result = await loadConfigInternal(context, formKey, formState, actionKey, group);
      if (result != 200)
        print("OOps, loading specific client config http result is $result");
      break;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    default:
      print(
          'Oops unknown ActionKey for Load Client Config UF State: $actionKey');
  }
  return null;
}

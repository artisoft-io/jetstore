import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
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
      print('Workspace Pull Action: $actionKey, state: $state');
      break;

    // Pull Workspace changes and perform workspace action
    case ActionKeys.wpPullWorkspaceOkUF:
      print('Workspace Pull Action: $actionKey, state: $state');
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
      if(context.mounted) {
        showAlertDialog(context, "Server error, please try again.");
      }
      return "Error while pulling workspace";

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  return null;
}

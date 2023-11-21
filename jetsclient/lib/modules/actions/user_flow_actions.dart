import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// User Flow State Actions
Future<String?> userFlowStateActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Start User Flow
    case ActionKeys.ufStartFlow:
      // Prepare the FormState, get state from server
      // Keep the list of visited page for supporting previous and next buttons
      formState.setValue(group, FSK.ufCurrentPage, 0);
      formState.setValue(group, FSK.ufVisitedPages, <String>[]);

      // Set the next page to display
      final nextStateKey = userFlowScreenState.currentUserFlowState
          .next(group: group, formState: formState);
      userFlowScreenState.currentUserFlowState =
          userFlowScreenState.userFlowConfig.states[nextStateKey!]!;
      userFlowScreenState.formConfig =
          userFlowScreenState.currentUserFlowState.formConfig;
      break;

    case ActionKeys.ufNext:
      // Validate current form
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Do the Action of the UserFlowState
      final userFlowState = userFlowScreenState.currentUserFlowState;
      if (userFlowState.stateAction != null) {
        final err = await userFlowState.formConfig.formActionsDelegate(
            context, formKey, formState, userFlowState.stateAction!, group: group);
        if(err != null) {
          print("ERROR while doing userFlowState Action");
        }
      }
      // Move to next page
      final visitedPages =
          formState.getValue(group, FSK.ufVisitedPages) as List<String>;
      final nextStateKey =
          userFlowState.next(group: group, formState: formState);
      if (nextStateKey == null) {
        print("ERROR nextStateKey is null");
        return "ERROR nextStateKey is null";
      }
      visitedPages.add(nextStateKey);
      formState.setValue(group, FSK.ufCurrentPage, visitedPages.length - 1);
      userFlowScreenState.currentUserFlowState =
          userFlowScreenState.userFlowConfig.states[nextStateKey]!;
      userFlowScreenState.formConfig =
          userFlowScreenState.currentUserFlowState.formConfig;
      break;

    case ActionKeys.ufPrevious:
      // Move to previous page
      final visitedPages =
          formState.getValue(group, FSK.ufVisitedPages) as List<String>;
      if (visitedPages.isEmpty) {
        print("ERROR visitedPages is empty, cannot do previous");
        return "ERROR visitedPages is empty, cannot do previous";
      }
      final nextStateKey = visitedPages.removeLast();
      visitedPages.add(nextStateKey);
      formState.setValue(group, FSK.ufCurrentPage, visitedPages.length - 1);
      userFlowScreenState.currentUserFlowState =
          userFlowScreenState.userFlowConfig.states[nextStateKey!]!;
      userFlowScreenState.formConfig =
          userFlowScreenState.currentUserFlowState.formConfig;
      break;

    // Cancel / Continue Later
    case ActionKeys.ufContinueLater:
      // Save state with associated session id
      Navigator.of(context).pop();
      break;

    // User Flow Completed
    case ActionKeys.ufCompleted:
      // Save state with associated session id
      Navigator.of(context)
          .pushNamed(userFlowScreenState.userFlowConfig.exitScreenPath);
      break;
    default:
      print('Oops unknown ActionKey for workspaceIDE Form: $actionKey');
  }
  return null;
}

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
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
  // print("*** userFlowStateActions called with actionKey $actionKey");
  switch (actionKey) {
    // Start User Flow
    case ActionKeys.ufStartFlow:
      //* TODO Prepare the FormState, get state from server

      // Do the Action of the UserFlowState (associated with ufStartFlow)
      final userFlowState = userFlowScreenState.currentUserFlowState;
      if (userFlowState.stateAction != null) {
        final err = await userFlowState.actionDelegate(userFlowScreenState,
            context, formKey, formState, userFlowState.stateAction!,
            group: group);
        if (err != null) {
          print("ERROR while doing userFlowState Action");
        }
      } else {
        // print(
        //     "*** userFlowState.stateAction is null in userFlowStateActions with ActionKey $actionKey");
      }

      // Set the next page to display
      final nextStateKey =
          userFlowState.next(group: group, formState: formState);
      // print("^^^ nextStateKey is $nextStateKey");
      final visitedPages =
          formState.getValue(group, FSK.ufVisitedPages) as List<String>;
      visitedPages.add(nextStateKey!);
      // print("*** ActionKeys.ufStartFlow visitedPages is: $visitedPages");
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      break;

    case ActionKeys.ufNext:
      // Validate current form
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Do the Action of the UserFlowState (associated with ufNext)
      final userFlowState = userFlowScreenState.currentUserFlowState;
      if (userFlowState.stateAction != null) {
        final err = await userFlowState.actionDelegate(userFlowScreenState,
            context, formKey, formState, userFlowState.stateAction!,
            group: group);
        if (err != null) {
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
      if (visitedPages.contains(nextStateKey)) {
        // Visiting an already visited page, unwind the stack
        while (visitedPages.last != nextStateKey) {
          visitedPages.removeLast();
        }
      } else {
        visitedPages.add(nextStateKey);
      }
      // print("*** ActionKeys.ufNext visitedPages is now: $visitedPages");
      formState.setValue(group, FSK.ufCurrentPage, visitedPages.length - 1);
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      break;

    case ActionKeys.ufPrevious:
      // Move to previous page
      final visitedPages =
          formState.getValue(group, FSK.ufVisitedPages) as List<String>;
      // print("*** ActionKeys.ufPrevious visitedPages is: $visitedPages");
      if (visitedPages.length < 2) {
        print("ERROR visitedPages.length < 2, cannot do previous");
        return "ERROR visitedPages.length < 2, cannot do previous";
      }
      final page = visitedPages.removeLast();
      // print("*** ActionKeys.ufPrevious removed page: $page");
      final nextStateKey = visitedPages.last;
      // print("*** ActionKeys.ufPrevious going to page: $nextStateKey");
      formState.setValue(group, FSK.ufCurrentPage, visitedPages.length - 1);
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      break;

    // Cancel / Continue Later
    // Same as User Flow Completed except does not validate the form
    case ActionKeys.ufContinueLater:
      // Do the Action of the current UserFlowState
      final userFlowState = userFlowScreenState.currentUserFlowState;
      if (userFlowState.stateAction != null) {
        final err = await userFlowState.actionDelegate(userFlowScreenState,
            context, formKey, formState, userFlowState.stateAction!,
            group: group);
        if (err != null) {
          print("ERROR while doing userFlowState Action: $err");
        }
      }
      if (context.mounted) {
        final p = userFlowScreenState.userFlowConfig.exitScreenPath;
        if (p != null) {
          JetsRouterDelegate()(JetsRouteData(p));
        } else {
          Navigator.of(context).pop();
        }
      }
      break;

    // User Flow Completed
    case ActionKeys.ufCompleted:
      // Validate current form
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Save state with associated session id

      // Do the Action of the UserFlowState (associated with ufCompleted)
      final userFlowState = userFlowScreenState.currentUserFlowState;
      if (userFlowState.stateAction != null) {
        final err = await userFlowState.actionDelegate(userFlowScreenState,
            context, formKey, formState, userFlowState.stateAction!,
            group: group);
        if (err != null) {
          print("ERROR while doing userFlowState Action: $err");
        }
      }
      if (context.mounted) {
        final p = userFlowScreenState.userFlowConfig.exitScreenPath;
        if (p != null) {
          JetsRouterDelegate()(JetsRouteData(p));
        } else {
          Navigator.of(context).pop();
        }
      }
      break;

    // Cancel UF - bailing out w/o calling State's Action Delegate
    case ActionKeys.ufCancel:
      if (context.mounted) {
        final p = userFlowScreenState.userFlowConfig.exitScreenPath;
        if (p != null) {
          JetsRouterDelegate()(JetsRouteData(p));
        } else {
          Navigator.of(context).pop();
        }
      }
      break;

    default:
      // Delegate to the UserFlowState Action
      // print("@default userFlowStateAction for ActionKey $actionKey");
      final userFlowState = userFlowScreenState.currentUserFlowState;
      final err = await userFlowState.actionDelegate(
          userFlowScreenState, context, formKey, formState, actionKey,
          group: group);
      if (err != null) {
        print("ERROR while doing userFlowState Action");
      }
      return err;
  }
  return null;
}

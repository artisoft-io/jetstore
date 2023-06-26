import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';
import 'package:jetsclient/utils/screen_config.dart';

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

// Utility function to create MenuEntry recursively
List<MenuEntry> mapMenuEntry(List<dynamic> data) {
  final v = data.map((e) => MenuEntry(
      key: e!["key"],
      label: e!["label"],
      routePath: e!["route_path"],
      children: e!["children"] != null ? mapMenuEntry(e!["children"]) : [],
  ));
  return v.toList();
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

    case ActionKeys.openWorkspace:
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      state[FSK.key] = state[FSK.key][0];
      state[FSK.wsName] = state[FSK.wsName][0];
      state[FSK.wsURI] = state[FSK.wsURI][0];
      final encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_query_structure',
        'fromClauses': [
          <String, String>{'table': 'workspace_file_structure'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      final httpResponse =
          await postRawAction(context, ServerEPs.dataTableEP, encodedJsonBody);
      if (httpResponse.statusCode != 200) {
        showAlertDialog(context, "Something went wrong. Please try again.");
        return null;
      }
      // print("OK, WE GOT ${httpResponse.body}");
      final resultType = httpResponse.body["result_type"];
      if (resultType != null && resultType == "workspace_file_structure") {
        // Setup MenuEntry as the workspace file structure
        // Correspond to List<MenuEntry>
        final l = httpResponse.body["result_data"] as List;
        JetsRouterDelegate().workspaceMenuState = mapMenuEntry(l);
      } else {
        showAlertDialog(context, "Oops, nothing here, working on it!");
        return null;
      }

      // //* DO SAMPLE RETURN OF MENU ITEMS
      // JetsRouterDelegate().workspaceMenuState = [
      //   MenuEntry(key: "m1", label: "Classes", children: [
      //     MenuEntry(
      //         key: "m1.1",
      //         label: "jets:Entity",
      //         children: [MenuEntry(key: "m1.1.1", label: "wrs_c:RuleConfig")]),
      //     MenuEntry(key: "m2.1", label: "wrs:WalrusBase", children: [
      //       MenuEntry(key: "m2.1.1", label: "wrs:BaseClaim", children: [
      //         MenuEntry(key: "m2.1.1.1", label: "wrs:CorePharmacy", children: [
      //           MenuEntry(key: "m2m1m1m1m1", label: "wrs:PharmacyClaim")
      //         ])
      //       ])
      //     ]),
      //     MenuEntry(
      //       key: "m3.1",
      //       label: "wrs:OpenFields",
      //     ),
      //     MenuEntry(
      //       key: "m5.1",
      //       label: "wrs:CommonClaim",
      //     ),
      //     MenuEntry(
      //       key: "m4.1",
      //       label: "tmp:MappingVariables",
      //     ),
      //   ]),
      // ];
      // //* DO SAMPLE RETURN OF MENU ITEMS
      // Navigate to workspace home page
      Map<String, dynamic> params = {
        "ws_name": state[FSK.wsName],
      };
      print("NAVIGATING to $workspaceHomePath, with $params");
      JetsRouterDelegate()(JetsRouteData(workspaceHomePath, params: params));

      JetsSpinnerOverlay.of(context).hide();
      return null;

    case ActionKeys.compileWorkspace:
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      state[FSK.key] = state[FSK.key][0];
      state[FSK.wsName] = state[FSK.wsName][0];
      state[FSK.wsURI] = state[FSK.wsURI][0];
      // print('Compiling Workspace state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'compile_workspace'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      JetsSpinnerOverlay.of(context).hide();
      return null;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for workspaceIDE Form: $actionKey');
  }
  return null;
}

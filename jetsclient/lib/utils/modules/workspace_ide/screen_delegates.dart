import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';
import 'package:jetsclient/utils/modules/workspace_ide/screen_delegates_helpers.dart';

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

    case FSK.wsBranch:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Workspace branch is too short.";
      }
      return "Workspace branch must be provided.";

    case FSK.wsFeatureBranch:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Feature branch is too short.";
      }
      return "Feature branch must be provided.";

    case FSK.wsURI:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Workspace URI is too short.";
      }
      return "Workspace URI must be provided.";

    case FSK.gitCommitMessage:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Commit message must be provided.";

    case FSK.client:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        return null;
      }
      return "Client must be selected.";

    case FSK.wsDbSourceFileName:
      String? value = v;
      final wsSection = formState.getValue(group, FSK.wsSection) as String?;
      if (wsSection == null || wsSection.isEmpty) {
        return "Invalid configuration -- wsSection is null";
      }
      if (value != null &&
          value.characters.length > wsSection.characters.length) {
        if (value.startsWith(wsSection)) {
          return null;
        }
        return "File name must preserve the given directory prefix";
      }
      return "File name must be entered, preserving the directory prefix.";

    case FSK.description:
    case FSK.wsFileEditorContent:
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
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': table}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    case ActionKeys.openWorkspace:
      return openWorkspaceActions(context, formState);

    case ActionKeys.compileWorkspace:
      final state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      state[FSK.lastGitLog] = 'redacted';
      // print('Compiling Workspace state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'compile_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return null;

    case ActionKeys.loadWorkspaceConfig:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to load workspace configuration, some client configuration may be overriten?');
      if (uc != 'OK') return null;
      final state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      state[FSK.lastGitLog] = 'redacted';
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'load_workspace_config'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();
        await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return null;

    // Add workspace file
    case ActionKeys.addWorkspaceFilesOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final state = formState.getState(0);
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      state['user_email'] = JetsRouterDelegate().user.email;
      // print('File Editor::Save File state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'add_workspace_file',
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      final httpResponse =
          await postRawAction(context, ServerEPs.dataTableEP, encodedJsonBody);
      if (httpResponse.statusCode == 401) return null;
      if (httpResponse.statusCode != 200) {
        if (context.mounted) {
          showAlertDialog(context, "Something went wrong. Please try again.");
        }
        return null;
      }
      final resultType = httpResponse.body["result_type"];
      if (resultType != null && resultType == "workspace_file_structure") {
        // Setup MenuEntry as the workspace file structure
        // Correspond to List<MenuEntry>
        final l = httpResponse.body["result_data"] as List;
        JetsRouterDelegate().workspaceMenuState = mapMenuEntry(l);
      } else {
        if (context.mounted) {
          showAlertDialog(context, "Oops, nothing here, working on it!");
        }
        return null;
      }

      // Navigate to workspace home page
      Map<String, dynamic> params = {
        "workspace_name": wsName,
      };
      // print(
      //     "Action.openWorkspace: NAVIGATING to $workspaceHomePath, with $params");
      JetsRouterDelegate()(JetsRouteData(workspaceHomePath, params: params));

      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
        // All good, let's the table know to refresh
        Navigator.of(context).pop(DTActionResult.okDataTableDirty);
      }
      return null;

    // delete workspace files
    case ActionKeys.deleteWorkspaceFiles:
      // Get confirmation
      var uc = await showConfirmationDialog(
          context, 'Are you sure you want to delete the selected file(s)?');
      if (uc != 'OK') return null;
      final state = formState.getState(0);
      // This is a multi select table, convert the multi-select
      // that is column-oriented into a request that is row-oriented
      final wsName = state[FSK.wsName] as String?;
      final fnames = state[FSK.wsDbSourceFileName] as List<String>?;
      if (wsName == null || fnames == null) {
        print('Delete Workspace Files: unexpected null, state is $state');
        return 'Delete Workspace Files: unexpected null';
      }
      List<dynamic> requestData = [];
      for (var i = 0; i < fnames.length; i++) {
        requestData.add(<String, dynamic>{
          FSK.wsName: wsName,
          FSK.wsDbSourceFileName: fnames[i],
          FSK.userEmail: JetsRouterDelegate().user.email,
        });
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();
      }
      // print('WorkspaceHome::Delete Changes requestData: $requestData');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'delete_workspace_files',
        'workspaceName': wsName,
        'data': requestData,
      }, toEncodable: (_) => '');
      if (context.mounted) {
        final httpResponse = await postRawAction(
            context, ServerEPs.dataTableEP, encodedJsonBody);
        if (httpResponse.statusCode == 401) return null;
        if (httpResponse.statusCode != 200) {
          if (context.mounted) {
            showAlertDialog(context, "Something went wrong. Please try again.");
          }
          return null;
        }
        final resultType = httpResponse.body["result_type"];
        if (resultType != null && resultType == "workspace_file_structure") {
          // Setup MenuEntry as the workspace file structure
          // Correspond to List<MenuEntry>
          final l = httpResponse.body["result_data"] as List;
          JetsRouterDelegate().workspaceMenuState = mapMenuEntry(l);
        } else {
          if (context.mounted) {
            showAlertDialog(context, "Oops, nothing here, working on it!");
          }
          return null;
        }

        // Navigate to workspace home page
        Map<String, dynamic> params = {
          "workspace_name": wsName,
        };
        // print(
        //     "Action.openWorkspace: NAVIGATING to $workspaceHomePath, with $params");
        JetsRouterDelegate()(JetsRouteData(workspaceHomePath, params: params));
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      formState.invokeCallbacks();
      return null;

    // Commit & Push Workspace Changes to Repository
    case ActionKeys.commitWorkspaceOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      // print('Add/Update Workspace state: $state');
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'commit_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    // Git Command in Workspace
    case ActionKeys.doGitCommandWorkspaceOk:
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'git_command_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    // Git Status of Workspace
    case ActionKeys.doGitStatusWorkspaceOk:
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'git_status_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    // Push Only Workspace Changes to Repository
    case ActionKeys.pushOnlyWorkspaceOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      // print('Add/Update Workspace state: $state');
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'push_only_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    // Pull Workspace Changes from Repository
    case ActionKeys.pullWorkspaceOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      // print('Add/Update Workspace state: $state');
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'pull_workspace'}
        ],
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    // Export client config from db to workspace
    case ActionKeys.exportClientConfigOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsBranch] is List<String>) {
        state[FSK.wsBranch] = state[FSK.wsBranch][0];
      }
      final wsBranch = state[FSK.wsBranch];
      if (state[FSK.wsFeatureBranch] is List<String>) {
        state[FSK.wsFeatureBranch] = state[FSK.wsFeatureBranch][0];
      }
      final wsFeatureBranch = state[FSK.wsFeatureBranch];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'save_workspace_client_config',
        'workspaceName': wsName,
        'workspaceBranch': wsBranch,
        'featureBranch': wsFeatureBranch,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      var result = await postInsertRows(context, formState, encodedJsonBody,
          errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return result;

    case ActionKeys.deleteWorkspace:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected local workspace?');
      if (uc != 'OK') return null;
      var state = formState.getState(0);
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      final wsName = state[FSK.wsName];
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
      state['user_email'] = JetsRouterDelegate().user.email;
      state[FSK.lastGitLog] = 'redacted';
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'workspace_insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete_workspace'}
        ],
        'workspaceName': wsName,
        'data': [state],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      }
      break;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for workspaceIDE Form: $actionKey');
  }
  return null;
}

/// Workspace File Editor
///
/// Validation delegate for the Workspace Home
String? workspaceHomeFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Workspace Home Form has unexpected data type");
  switch (key) {
    case FSK.wsFileEditorContent:
      return null;

    default:
      print(
          'Oops Workspace Home Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Workspace Home Form Actions
Future<String?> workspaceHomeFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // File Editor - Save
    case ActionKeys.wsSaveFileOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final state = formState.getState(0);
      final wsName = state[FSK.wsName];
      state['user_email'] = JetsRouterDelegate().user.email;
      // print('File Editor::Save File state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'save_workspace_file_content',
        'workspaceName': wsName,
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      final result =
          await postRawAction(context, ServerEPs.dataTableEP, encodedJsonBody);

      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      if (result.statusCode != 200 && result.statusCode != 401) {
        print('Something went wrong while saving file: $result');
        if (context.mounted) {
          showAlertDialog(context, "Something went wrong. Please try again.");
        }
      } else {
        //* Would be nice to close the active file tab
      }
      return null;

    // Delete Workspace Changes (multi select)
    case ActionKeys.deleteWorkspaceChanges:
      // Get confirmation
      var uc = await showConfirmationDialog(
          context, 'Are you sure you want to revert the selected change(s)?');
      if (uc != 'OK') return null;
      final state = formState.getState(0);
      // This is a multi select table, convert the multi-select
      // that is column-oriented into a request that is row-oriented
      final wsName = state[FSK.wsName] as String?;
      final keys = state[FSK.key] as List<String>?;
      final oids = state[FSK.wsOid] as List<String>?;
      final fnames = state[FSK.wsFileName] as List<String>?;
      if (wsName == null || keys == null || oids == null || fnames == null) {
        print('Delete Workspace Changes: unexpected null, state is $state');
        return 'Delete Workspace Changes: unexpected null';
      }
      List<dynamic> requestData = [];
      for (var i = 0; i < keys.length; i++) {
        requestData.add(<String, dynamic>{
          FSK.key: keys[i],
          FSK.wsOid: oids[i],
          FSK.wsName: wsName,
          FSK.wsFileName: fnames[i],
          FSK.userEmail: JetsRouterDelegate().user.email,
        });
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();
      }
      // print('WorkspaceHome::Delete Changes requestData: $requestData');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'delete_workspace_changes',
        'workspaceName': wsName,
        'data': requestData,
      }, toEncodable: (_) => '');
      // await postInsertRows(context, formState, encodedJsonBody,
      //   errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        final result = await postRawAction(
            context, ServerEPs.dataTableEP, encodedJsonBody);
        if (context.mounted) {
          JetsSpinnerOverlay.of(context).hide();
        }
        if (result.statusCode != 200 && result.statusCode != 401) {
          print('Something went wrong while reverting changes: $result');
          if (context.mounted) {
            showAlertDialog(context, "Something went wrong. Please try again.");
          }
        }
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      formState.invokeCallbacks();
      return null;

    // Delete ALL Workspace Changes
    case ActionKeys.deleteAllWorkspaceChanges:
      // Get confirmation
      var uc = await showConfirmationDialog(
          context, 'Are you sure you want to revert the all changes?');
      if (uc != 'OK') return null;
      final state = formState.getState(0);
      final wsName = state[FSK.wsName] as String?;
      if (wsName == null) {
        print('Delete All Workspace Changes: unexpected null workspace_name');
        return 'Delete All Workspace Changes: unexpected null workspace_name';
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'delete_all_workspace_changes',
        'workspaceName': wsName,
        'data': [state],
      }, toEncodable: (_) => '');
      // await postInsertRows(context, formState, encodedJsonBody,
      //   errorReturnStatus: DTActionResult.statusErrorRefreshTable);
      if (context.mounted) {
        final result = await postRawAction(
            context, ServerEPs.dataTableEP, encodedJsonBody);
        if (context.mounted) {
          JetsSpinnerOverlay.of(context).hide();
        }
        if (result.statusCode != 200 && result.statusCode != 401) {
          print('Something went wrong while reverting all changes: $result');
          if (context.mounted) {
            showAlertDialog(context, "Something went wrong. Please try again.");
          }
        }
      }
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      formState.invokeCallbacks();
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Workspace Home Form: $actionKey');
  }
  return null;
}

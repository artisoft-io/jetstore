import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/components/base_screen.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/jets_tab_controller.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';
import 'package:jetsclient/utils/form_config_impl.dart';
import 'package:jetsclient/utils/screen_config.dart';
import 'package:jetsclient/utils/form_config.dart';

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
    case FSK.wsFileEditorContent:
      return null;

    default:
      print(
          'Oops workspaceIDE Form has no validator configured for form field $key');
  }
  return null;
}

// Utility function to create MenuEntry recursively
// Note: Cap file size to 120K (120000), larger than that we don't bring them in ui
// Note: MenuEntry.formConfigKey is constructed from e['type'] and e['key']
// (file, data_model, jet_rules, lookups)
// It is used in initializeWorkspaceFileEditor to get the formConfig to use.
List<MenuEntry> mapMenuEntry(List<dynamic> data) {
  final v = data.map((e) {
    final etype = e!['type'] as String;
    final pageMatchKey = e![FSK.pageMatchKey] ?? '';
    final routePath = e!['route_path'] as String;
    final size = e!['size'] as double;
    String? formConfigKey;
    var onPageStyle = ActionStyle.primary;
    var otherPageStyle = ActionStyle.secondary;
    switch (etype) {
      case 'dir':
        break;
      case 'file':
        formConfigKey = FormKeys.workspaceFileEditor;
        onPageStyle = ActionStyle.menuSelected;
        otherPageStyle = ActionStyle.menuAlternate;
        break;
      case 'section':
        formConfigKey = "workspace.$pageMatchKey.form";
        onPageStyle = ActionStyle.menuSelected;
        otherPageStyle = ActionStyle.menuAlternate;
        break;
      default:
        print("ERROR in mapMenuEntry: unknown menuEntry type: $etype");
    }
    return MenuEntry(
      key: pageMatchKey ?? '',
      label: e!["label"] ?? '',
      routePath:
          size < 120000 ? (routePath.isNotEmpty ? routePath : null) : null,
      pageMatchKey: pageMatchKey,
      routeParams: e!["route_params"],
      menuAction: size < 120000 ? initializeWorkspaceFileEditor : null,
      formConfigKey: formConfigKey,
      onPageStyle: onPageStyle,
      otherPageStyle: otherPageStyle,
      children: e!["children"] != null ? mapMenuEntry(e!["children"]) : [],
    );
  });
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
      if (state[FSK.key] is List<String>) {
        state[FSK.key] = state[FSK.key][0];
      }
      if (state[FSK.wsName] is List<String>) {
        state[FSK.wsName] = state[FSK.wsName][0];
      }
      if (state[FSK.wsURI] is List<String>) {
        state[FSK.wsURI] = state[FSK.wsURI][0];
      }
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
        "workspace_name": state[FSK.wsName],
      };
      // print(
      //     "Action.openWorkspace: NAVIGATING to $workspaceHomePath, with $params");
      JetsRouterDelegate()(JetsRouteData(workspaceHomePath, params: params));

      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return null;

    case ActionKeys.compileWorkspace:
      final state = formState.getState(0);
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
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      return null;

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
      state['user_email'] = JetsRouterDelegate().user.email;
      // print('File Editor::Save File state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'save_workspace_file_content',
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
      }
      return null;

    // Delete Workspace Changes (multi select)
    case ActionKeys.deleteWorkspaceChanges:
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
      // print('WorkspaceHome::Delete Changes requestData: $requestData');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'delete_workspace_changes',
        'data': requestData,
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      final result =
          await postRawAction(context, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      if (result.statusCode != 200 && result.statusCode != 401) {
        print('Something went wrong while deleting file changes: $result');
        return 'Something went wrong while deleting file changes: $result';
      }
      // Mark the formState as modified to trigger a table refresh
      // Note, the key 'state_modified' is not used
      formState.setValueAndNotify(
          0, 'state_modified', "${DateTime.now().millisecondsSinceEpoch}");
      return null;

    // Delete ALL Workspace Changes
    case ActionKeys.deleteAllWorkspaceChanges:
      final state = formState.getState(0);
      final wsName = state[FSK.wsName] as String?;
      if (wsName == null) {
        print('Delete All Workspace Changes: unexpected null workspace_name');
        return 'Delete All Workspace Changes: unexpected null workspace_name';
      }
      JetsSpinnerOverlay.of(context).show();
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'delete_all_workspace_changes',
        'data': [state],
      }, toEncodable: (_) => '');
      final result =
          await postRawAction(context, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      if (result.statusCode == 401) return "Not authorized";
      if (result.statusCode != 200) {
        print('Something went wrong while deleting all file changes: $result');
        return 'Something went wrong while deleting all file changes: $result';
      }
      // Mark the formState as modified to trigger a table refresh
      // Note, the key 'state_modified' is registered in tableConfig
      formState.setValueAndNotify(
          0, 'state_modified', "${DateTime.now().millisecondsSinceEpoch}");
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

/// Initialization Delegate for File Editor Screen
Future<int> initializeWorkspaceFileEditor(
    BuildContext context, MenuEntry menuEntry, State<StatefulWidget> s) async {
  assert(menuEntry.pageMatchKey != null,
      'menuEntry ${menuEntry.label} as null pageMatchKey');
  if (menuEntry.routeParams == null) return 200;
  final state = s as BaseScreenState;
  final tabIndex = menuEntry.routeParams!['tab.index'] as int?;

  // Put the pageMatchKey in current route config so the menu gets highlighted
  JetsRouterDelegate().currentConfiguration!.params[FSK.pageMatchKey] =
      menuEntry.pageMatchKey;

  if (tabIndex != null) {
    state.tabController.animateTo(tabIndex);
    return 200;
  }
  FormConfig? formConfig;
  if (menuEntry.formConfigKey != null) {
    formConfig = getFormConfig(menuEntry.formConfigKey!);
  }
  if (formConfig == null) return 200;

  final formState = JetsFormState(initialGroupCount: 1);
  formState.setValue(0, FSK.wsName, menuEntry.routeParams![FSK.wsName]);
  formState.setValue(0, FSK.wsFileName, menuEntry.routeParams![FSK.wsFileName]);

  // based on MenuEntry.formConfigKey fetch info from server (if file editor)
  // and get the formConfig
  if (menuEntry.formConfigKey == FormKeys.workspaceFileEditor) {
    // Need to get file_content from apiserver
    // JetsSpinnerOverlay.of(context).show();
    final encodedJsonBody = jsonEncode(<String, dynamic>{
      'action': 'get_workspace_file_content',
      'data': [menuEntry.routeParams],
    }, toEncodable: (_) => '');

    final result = await HttpClientSingleton().sendRequest(
        path: ServerEPs.dataTableEP,
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: encodedJsonBody);

    // JetsSpinnerOverlay.of(context).hide();

    if (result.statusCode == 200) {
      formState.setValue(
          0, FSK.wsFileEditorContent, result.body[FSK.wsFileEditorContent]);
    } else {
      print("Oops, Something went wrong. Could not get the file content");
      return result.statusCode;
    }
  }

  // Create the tab info for the tab manager
  state.tabsStateHelper.addTab(
      tabParams: JetsTabParams(
          workspaceName: menuEntry.routeParams![FSK.wsName] ?? '',
          label: menuEntry.label,
          pageMatchKey: menuEntry.pageMatchKey!,
          formConfig: getFormConfig(
              menuEntry.formConfigKey ?? FormKeys.workspaceFileEditor),
          formState: formState));

  // PUT TAB INDEX in menuEntry.routeParams for when clicking on menu again
  final l = state.tabsStateHelper.tabsParams.length;
  menuEntry.routeParams!['tab.index'] = l - 1;
  state.resetTabController(l - 1, l);

  return 200;
}

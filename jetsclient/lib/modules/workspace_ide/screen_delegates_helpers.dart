import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/base_screen.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/jets_tab_controller.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/models/screen_config.dart';
import 'package:jetsclient/models/form_config.dart';

Future<String?> openWorkspaceActions(
    BuildContext context, JetsFormState formState) async {
  var state = formState.getState(0);
  state['user_email'] = JetsRouterDelegate().user.email;
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
  state[FSK.lastGitLog] = 'redacted';
  final encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'workspace_query_structure',
    'fromClauses': [
      <String, String>{'table': 'workspace_file_structure'}
    ],
    'workspaceName': wsName,
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
  final wsName = menuEntry.routeParams![FSK.wsName];
  formState.setValue(0, FSK.wsName, wsName);
  formState.setValue(0, FSK.wsFileName, menuEntry.routeParams![FSK.wsFileName]);

  // based on MenuEntry.formConfigKey fetch info from server (if file editor)
  // and get the formConfig
  if (menuEntry.formConfigKey == FormKeys.workspaceFileEditor) {
    // Need to get file_content from apiserver
    // JetsSpinnerOverlay.of(context).show();
    final encodedJsonBody = jsonEncode(<String, dynamic>{
      'action': 'get_workspace_file_content',
      'workspaceName': wsName,
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

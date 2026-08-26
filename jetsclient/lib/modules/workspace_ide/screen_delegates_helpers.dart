import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:web/web.dart' as web;
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

/// Opens the CodeMirror Workspace IDE (jetsclient_ide) in a new tab.
///
/// That app is a separate bundle served by this same apiserver under /ide/, so
/// the url is resolved against the api origin rather than against the browser's,
/// which differ under `flutter run`.
///
/// It opens in a new tab rather than an iframe because it owns its own routing
/// and its own editor keybindings, both of which an embedded frame would fight.
/// One consequence worth knowing: it holds its token in memory and does not share
/// this app's session, so the user signs in once more there.
Future<int> openWorkspaceIdeApp(
    BuildContext context, MenuEntry menuEntry, State<StatefulWidget> state) async {
  // main() always sets serverAdd, but falling back to the relative path is the
  // right answer anyway: in production the IDE is served from this same origin.
  final base = HttpClientSingleton().serverAdd;
  final url = base == null ? '/ide/' : base.resolve('/ide/').toString();
  web.window.open(url, '_blank');
  return 200;
}

/// Files of this size or larger are not opened in the Workspace IDE.
///
/// The editor is a single Flutter text field, and Flutter lays a text field's
/// whole document out on every frame rather than virtualising it by line, so
/// the cost grows with the size of the file instead of with the part of it on
/// screen. Past roughly this size the screen stops being usable, so
/// [mapMenuEntry] drops both the route and the menu action below — which is
/// why an oversized file's menu entry does nothing at all when clicked rather
/// than opening slowly.
///
/// Raising this number on its own therefore trades a clear failure for an
/// unusable screen; it can only go up once the editor itself virtualises.
const workspaceFileEditorSizeLimit = 250000;

/// The form this client renders for each compiled view the apiserver declares.
///
/// **The apiserver decides whether a section *has* a compiled view; this map
/// decides whether *this client* renders one.** Those are different questions
/// and conflating them is what C.1 fixed. A section's heading is a view of
/// `workspace.db` — the compiled artifact — while the nodes below it are the
/// source files that go into the compiler; the server knows which sections have
/// a second side, because it knows what the compiler reads, and the client
/// cannot work it out from a directory name.
///
/// Until 2026-08-25 the client composed `"workspace.$pageMatchKey.form"` and
/// handed it to `getFormConfig`, **which throws for an unknown key**
/// (`getFormConfig`, `lib/modules/form_config_impl.dart:649`) — inside an async
/// menu delegate, so the symptom was a heading that did nothing. Six of the
/// eight sections took that path, and "no view was built", "no view can exist"
/// and "the registry lookup failed" were one event.
///
/// **`lookups` is deliberately absent, and it is now built elsewhere rather than
/// scheduled.** Its files do compile into `workspace.db`, so the server declares
/// `compiled_view: lookups`; the view was routed to React because track X deletes
/// this app and a view built here is discarded by construction (I-45, decided
/// 2026-08-23 by the user). **C.3a shipped it on 2026-08-25** —
/// `jetsclient_ide/src/screens/views/lookups.view.json` and its two table
/// documents — so the state this constant records is *built, and not in this
/// client*, which is what `viewsNotBuiltInFlutter` has always meant and stays
/// true until this app is deleted.
///
/// **This sentence said "scheduled as C.3a" and was corrected by C.3a itself**,
/// which is the only party a note naming a future trigger can rely on: nothing
/// re-reads a source comment on the day its trigger fires, so the task that fires
/// it has to. The repository `CLAUDE.md` records the same failure on the
/// `.pc.json` validator row, where nobody was holding it.
///
/// The state is asserted by `test/workspace_section_contract_test.dart`, which
/// fails if a section is added to either side without somebody deciding which
/// case it is.
const compiledViewForms = <String, String>{
  'data_model': FormKeys.wsDataModelForm,
  'jet_rules': FormKeys.wsJetRulesForm,
};

/// The form key for a section, or null when this client renders its sources.
///
/// **It is total, and that is the point.** Every key it can return is a key
/// `compiledViewForms` holds, so the composition that could miss is gone rather
/// than guarded; a section this client has no view for returns null and the menu
/// entry behaves as the group heading it is, with the source files beneath it.
String? compiledViewFormKey(Object? compiledView) =>
    compiledView is String ? compiledViewForms[compiledView] : null;

// Utility function to create MenuEntry recursively
// Note: files at or above [workspaceFileEditorSizeLimit] are listed but not
// openable — see that constant for why.
// Note: MenuEntry.formConfigKey is the file editor for a file node and, for a
// section node, the form named by [compiledViewForms] for the compiled view the
// server declared — null when there is none. It is used in
// initializeWorkspaceFileEditor to get the formConfig to use.
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
        formConfigKey = compiledViewFormKey(e!['compiled_view']);
        onPageStyle = ActionStyle.menuSelected;
        otherPageStyle = ActionStyle.menuAlternate;
        break;
      default:
        print("ERROR in mapMenuEntry: unknown menuEntry type: $etype");
    }
    return MenuEntry(
      key: pageMatchKey ?? '',
      label: '${e!["label"] ?? ''} (${size/1000}Kb)',
      routePath: size < workspaceFileEditorSizeLimit
          ? (routePath.isNotEmpty ? routePath : null)
          : null,
      pageMatchKey: pageMatchKey,
      routeParams: e!["route_params"],
      menuAction: size < workspaceFileEditorSizeLimit
          ? initializeWorkspaceFileEditor
          : null,
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

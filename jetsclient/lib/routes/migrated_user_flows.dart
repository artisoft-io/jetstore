/// Which user flows the React app now serves, and how to get there.
///
/// **Task S.8 of the UI refresh, repaired by task F.10.** While the port runs, a
/// flow lives in exactly one of the two apps, and a user who navigates here for
/// a migrated flow has to be handed over rather than shown a stale screen.
///
/// ## Why a handoff rather than a route the server decides
///
/// This app never calls `setPathUrlStrategy` — `url_strategy` is a dependency at
/// `pubspec.yaml:47` and nothing imports it — so Flutter web keeps its default
/// hash routing and every route here lives after the `#`. A fragment is never
/// sent to the server, which is why `jets/apiserver/server.go` has no catch-all
/// and needs none: the only path this app ever requests is `/`.
///
/// So the apiserver cannot dispatch on a Flutter route, because it never sees
/// one. The two apps' URL spaces cannot collide, and the handoff has to happen
/// here, on the client, as a full page load. That is correct rather than
/// unfortunate: the bundles share an origin and a token, not a heap.
///
/// ## The list, and why it is here rather than in the workspace
///
/// The React side decides ownership from the workspace — a flow is its own iff
/// `user_flows/<key>.uf.json` exists (`jetsclient_ide/src/userflow/routing.ts`).
/// This app cannot ask that question cheaply: it would be a request on every
/// navigation, in an app that is being replaced.
///
/// So this list is the one place the fact is duplicated, and it is **debt with a
/// short life**: it needs one line per flow as each is migrated, and the whole
/// file disappears when the Flutter app does. Recorded as I-30. The failure it
/// can produce is bounded and visible — a migrated flow that still opens the
/// Flutter version — rather than silent.
///
/// ## A route is not a flow key, and for four of eleven it never was
///
/// **The handoff used to look up `Uri.parse(path).pathSegments.first`**, on the
/// reasonable-looking assumption that a flow's route begins with its key. It
/// does for seven of the eleven flows and not for the other four
/// (`mapFileUF` → `fileMappingUF`, `homeFiltersUF` → `configureHomeFiltersUF`,
/// `workspacePullUF` → `pullWorkspaceUF`, `loadConfigUF` → **`workspaces`**,
/// which is also [workspaceRegistryPath]). Listing a key for one of those four
/// was either inert or destructive, and the destructive case would have handed
/// the workspace registry screen to a flow route. Found 2026-08-23 as the
/// ui_refresh project's **I-75**, before any key had been added; repaired by
/// task F.10, which is what [userFlowRoutes] below is.
///
/// **It survived because the shortcut is right for the seven flows anybody had
/// reason to check** — S.8 listed `loadFilesUF` and `registerFileKeyUF`, both of
/// which are their own leading segment, so the test agreed with the code and
/// both agreed with the world.
library;

import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';

/// One flow route of this app: which flow it opens, and what it carries.
class UserFlowRoute {
  const UserFlowRoute(this.flowKey, {this.parameters = const <String>[]});

  /// The [UserFlowKeys] value — the name the React app serves the flow under,
  /// and the string that goes into [migratedUserFlows].
  final String flowKey;

  /// The route template's `:`-prefixed segments, in declaration order.
  ///
  /// These are carried across as a query string rather than as path segments,
  /// because the names differ per flow and the React route is `/flow/:key` with
  /// nothing positional after it. `FlowRunner` seeds every query parameter into
  /// form-state group 0 by name before the flow loads
  /// (`jetsclient_ide/src/screens/FlowRunner.tsx`), so the names here are the
  /// names the flow's queries substitute.
  final List<String> parameters;
}

/// Every flow route this app has, keyed by the **route template**.
///
/// The key is a template — `/workspaces/loadConfigUF/:workspace_name`, not the
/// path a user is on. That is safe because [JetsRouteData.path] is always a
/// template taken verbatim from [jetsRoutesMap]: `jetsRoutesParser` either
/// matches one and returns it with the concrete values in `params`, or falls
/// through to [pageNotFoundPath]. So the lookup is an exact match on a key that
/// cannot be ambiguous, where the leading segment could be — `/fileMappingUF`
/// and `/fileMappingUF/mapping/:table_name/:object_type` are two different
/// flows, and `/workspaces` is not a flow at all.
///
/// **A route absent from this map is this app's**, which is the whole of the
/// answer for every non-flow route: [jetsRoutesMap] has 28 entries and 11 of
/// them are flows, so the other 17 — every screen, the login and register pages
/// and the 404 — are not listed and never hand off. Completeness is not a
/// promise made in this comment — `migrated_user_flows_test.dart` derives the
/// expected map from [jetsRoutesMap] itself, so a flow route added without a row
/// here fails the suite.
const userFlowRoutes = <String, UserFlowRoute>{
  ufClientRegistryPath:
      UserFlowRoute(UserFlowKeys.clientRegistryUF, parameters: ['startAtKey']),
  ufSourceConfigPath:
      UserFlowRoute(UserFlowKeys.sourceConfigUF, parameters: ['startAtKey']),
  ufFileMappingPath: UserFlowRoute(UserFlowKeys.fileMappingUF),
  ufMappingPath: UserFlowRoute(UserFlowKeys.mapFileUF,
      parameters: ['table_name', 'object_type']),
  ufPipelineConfigPath: UserFlowRoute(UserFlowKeys.pipelineConfigUF),
  ufLoadFilesPath: UserFlowRoute(UserFlowKeys.loadFilesUF),
  ufRegisterFileKeyPath: UserFlowRoute(UserFlowKeys.registerFileKeyUF),
  ufStartPipelinePath: UserFlowRoute(UserFlowKeys.startPipelineUF),
  ufHomeFiltersPath: UserFlowRoute(UserFlowKeys.homeFiltersUF),
  ufPullWorkspacePath: UserFlowRoute(UserFlowKeys.workspacePullUF, parameters: [
    'key',
    'workspace_name',
    'workspace_branch',
    'feature_branch',
    'workspace_uri',
  ]),
  ufLoadConfigPath: UserFlowRoute(UserFlowKeys.loadConfigUF,
      parameters: ['workspace_name']),
};

/// Flow keys the React app serves. One line per migrated flow.
///
/// Keep in step with the documents in the workspace: a key here whose documents
/// are absent sends users to a React screen that cannot load, which is the worse
/// of the two directions.
///
/// **Empty, and it was wrongly non-empty for five days.** `loadFilesUF` and
/// `registerFileKeyUF` were listed here from S.8 on the strength of their
/// documents existing, and the React app had no `/flow/:key` route to receive
/// either — so `handoffUrlFor` returned a URL that landed on `App.tsx`'s
/// `path="*"` and redirected to the Workspace IDE. The user asked for a flow and
/// got the editor. Found 2026-08-23 as the ui_refresh project's **I-50**;
/// emptied by task F.0a, which builds the runner.
///
/// **The comment above anticipated the opposite failure and that is the lesson.**
/// It warns about a key whose *documents* are absent. The documents were present;
/// what was absent was the route. S.8 tested this file's output —
/// `handoffUrlFor(loadFilesUF) == '/ide/flow/loadFilesUF'` — and nothing tested
/// that the other app accepts it, because the contract between two apps is a
/// string and each has its own suite. **A handoff tested from one end is tested
/// on the half that cannot fail.**
///
/// So a key goes back here only once the React runner serves that flow end to
/// end, which is the ordering rule the phase 3 plan records as F10: adding a key
/// is a *consequence* of migrating a flow, never a step in it.
///
/// **A key is now safe to add for any of the eleven**, which it was not before
/// F.10: the key is looked up through [userFlowRoutes] rather than guessed from
/// the path, so the four flows whose route disagrees with their key are reached
/// like the other seven and their parameters travel with them (I-75).
const migratedUserFlows = <String>{};

/// Where the React app is mounted. `/ide/` is a misnomer for a whole
/// application; see the UI refresh project's I-26.
const reactAppBase = '/ide';

/// The URL to hand off to, or null when this app keeps the route.
///
/// Null for three separate reasons, and they are all the same answer to the
/// user — the Flutter screen, which works:
///
///  1. **The route is not a flow route.** Every screen, dialog and error page.
///  2. **The flow is not migrated yet.** [migratedUserFlows] does not list it.
///  3. **A declared parameter is missing or empty.** `mapFileUF` without
///     `table_name` has no rows to draw and `loadConfigUF` without
///     `workspace_name` has no workspace, so the React runner would render an
///     empty worksheet where this app renders a working one. Falling through is
///     the bounded, visible failure — the user gets the old screen — where
///     handing off half a flow is the silent one. It can only happen from an
///     in-app navigation that built a [JetsRouteData] without its parameters,
///     since the parser fills every one it matched.
///
/// [migrated] is for tests: the production set is [migratedUserFlows] and is
/// empty, so nothing else could exercise the URL this builds.
String? handoffUrlFor(JetsRouteData route, {Set<String>? migrated}) {
  final flowRoute = userFlowRoutes[route.path];
  if (flowRoute == null) return null;
  if (!(migrated ?? migratedUserFlows).contains(flowRoute.flowKey)) return null;

  final query = <String>[];
  for (final name in flowRoute.parameters) {
    final value = route.params[name]?.toString() ?? '';
    if (value.isEmpty) return null;
    query.add('${Uri.encodeQueryComponent(name)}='
        '${Uri.encodeQueryComponent(value)}');
  }
  final suffix = query.isEmpty ? '' : '?${query.join('&')}';
  return '$reactAppBase/flow/${flowRoute.flowKey}$suffix';
}

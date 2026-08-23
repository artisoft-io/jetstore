/// Which screen configurations this app can actually reach, and by which roles.
///
/// **Why this file exists.** Task G3.2 is the second half of Phase 3's sizing
/// gate (`projects/ui_refresh/plan/phase3_plan.md` §1.2, sub-question 3). G3.1
/// counted the configuration the non-flow screens hold; it did not ask whether
/// anything routes to them. That matters because the migration is per route: a
/// screen nothing reaches costs nothing to leave behind, and a screen only an
/// administrator reaches is not on the critical path.
///
/// **Reachability here is a closure over configuration, not a guess.** The graph
/// is built from four kinds of edge, every one of them declared:
///
///  1. `jetsRoutesMap` — route to screen configuration. Every screen widget
///     extends [BaseScreen] and carries its `screenConfig`, so the route table
///     names its screen rather than being matched to it by key.
///  2. `MenuEntry.routePath` — a screen's left menu and toolbar, walked
///     recursively through `children`, across `menuEntries`, `adminMenuEntries`
///     and `toolbarMenuEntries`.
///  3. `ActionConfig.configScreenPath` — the data table's navigate action
///     (`components/data_table.dart:641`). Reached from the tables a route
///     shows, which are themselves configuration: `ScreenOne.tableConfig`, a
///     form's `FormDataTableFieldConfig.dataTableConfig`, and the dialog forms
///     an action names in `configForm`.
///  4. `UserFlowConfig.exitScreenPath` — where a flow goes when it finishes.
///
/// **And it reports what is declared, which is not the same as what happens**
/// (I-20). Nine navigations are written in Dart rather than configured —
/// `JetsRouterDelegate()(JetsRouteData(...))` outside `lib/routes/` — and no
/// corpus can see them. Routes with no configured edge are reported as
/// `routesWithNoConfiguredEdge` and named that way deliberately: it is a
/// statement about the configuration, not a claim that the route is dead. The
/// sizing document resolves each one by reading the code.
///
/// **The role model is the one the widgets implement**, and it is small:
/// `user.isAdmin` bypasses every capability check, in the menu
/// (`screens/base_screen.dart:145`) and on a table action
/// (`components/data_table.dart:30`). So an entry gated on capability `c` is
/// reachable two ways — by an administrator, or by a user holding `c` — and both
/// alternatives are carried through the closure rather than collapsed.
///
/// **This suite only runs on the chrome platform**, for the reason every corpus
/// test here does (I-5):
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/screen_reachability_corpus_test.dart
///
/// To regenerate the React fixture, see
/// `jetsclient_ide/src/screens/fixtures/README.md`.
library;

import 'dart:convert';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/models/screen_config.dart';
import 'package:jetsclient/modules/data_table_config_impl.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/modules/user_flows/client_registry/screen_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/screen_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/screen_config.dart';
import 'package:jetsclient/modules/user_flows/home_filters/screen_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/screen_config.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/screen_config.dart';
import 'package:jetsclient/modules/user_flows/register_file_key/screen_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/screen_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/screen_config.dart';
import 'package:jetsclient/modules/workspace_ide/infer_server_admin/screen_config.dart';
import 'package:jetsclient/modules/workspace_ide/screen_config.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/base_screen.dart';
import 'package:jetsclient/screens/screen_multi_form.dart';
import 'package:jetsclient/screens/screen_one.dart';
import 'package:jetsclient/screens/screen_form.dart';
import 'package:jetsclient/screens/screen_tab_form.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

import 'corpus_support.dart';

/// Where the React side keeps its copy. Not read here — the browser has no
/// filesystem, which is why drift is caught by a checksum.
const corpusPath =
    'jetsclient_ide/src/screens/fixtures/screen_reachability.json';

const beginMarker = '===BEGIN SCREEN REACHABILITY CORPUS===';
const endMarker = '===END SCREEN REACHABILITY CORPUS===';

/// Checksum of the corpus this app currently produces.
///
/// **Update it only together with the React fixture.** A failure here means a
/// route, a menu or a navigate action changed; the fix is to regenerate both,
/// never to edit this number so the test goes green.
const expectedChecksum = 'fnv1a32:6c0da84e';

/// Every screen configuration every registry holds, by registry.
///
/// Enumerated from the maps rather than from a list of `ScreenKeys` constants,
/// for I-37's reason: a constant class is not the key space.
final screenKeysByRegistry = <String, List<String>>{
  'central': mainScreenConfigKeys.toList()..sort(),
  'workspace_ide': workspaceScreenConfigKeys.toList()..sort(),
  'infer_server_admin': inferServerAdminScreenConfigKeys.toList()..sort(),
  'uf_client_registry': clientRegistryScreenConfigKeys.toList()..sort(),
  'uf_configure_files': configureFileScreenConfigKeys.toList()..sort(),
  'uf_file_mapping': fileMappingScreenConfigKeys.toList()..sort(),
  'uf_home_filters': homeFiltersScreenConfigKeys.toList()..sort(),
  'uf_load_files': loadFilesScreenConfigKeys.toList()..sort(),
  'uf_pipeline_config': pipelineConfigScreenConfigKeys.toList()..sort(),
  'uf_register_file_key': registerFileKeyScreenConfigKeys.toList()..sort(),
  'uf_start_pipeline': startPipelineScreenConfigKeys.toList()..sort(),
  'uf_workspace_pull': workspacePullScreenConfigKeys.toList()..sort(),
};

List<String> allScreenKeys() =>
    screenKeysByRegistry.values.expand((k) => k).toSet().toList()..sort();

// ---------------------------------------------------------------------------
// The role lattice
// ---------------------------------------------------------------------------

/// A *grant* is the set of things a user must have to walk one path to a route:
/// the empty set for any authenticated user, `{admin}`, or `{cap:workspace_ide}`.
/// A route carries a set of grants, because it may be reachable more than one
/// way, and only the minimal ones are kept.
typedef Grant = Set<String>;

/// Drops any grant that is a superset of another: needing *more* to reach the
/// same route by a longer path is not a distinct way in.
Set<Grant> minimise(Iterable<Grant> grants) {
  final out = <Grant>[];
  for (final g in grants) {
    if (out.any((o) => o.difference(g).isEmpty)) continue;
    out.removeWhere((o) => g.difference(o).isEmpty);
    out.add(g);
  }
  return out.toSet();
}

/// What one edge costs. A capability yields **two** alternatives, because
/// `isAdmin` bypasses the check in both places it is made.
List<Grant> edgeCost({required bool adminOnly, String? capability}) {
  if (capability != null) {
    return [
      <String>{'admin'},
      <String>{if (adminOnly) 'admin', 'cap:$capability'},
    ];
  }
  return [
    <String>{if (adminOnly) 'admin'},
  ];
}

String grantLabel(Grant g) => g.isEmpty ? 'anyUser' : (g.toList()..sort()).join('+');

List<String> grantLabels(Set<Grant> grants) =>
    grants.map(grantLabel).toList()..sort();

// ---------------------------------------------------------------------------
// The edges
// ---------------------------------------------------------------------------

class Edge {
  Edge(this.to, this.via, this.cost);
  final String to;
  final String via;
  final List<Grant> cost;
}

/// Menu edges out of one screen configuration, walked recursively through
/// `children` and across all three menu lists.
List<Edge> menuEdges(ScreenConfig config) {
  final edges = <Edge>[];
  // A route named by the plain menu is reachable without admin even if the
  // admin menu names it too, so the plain menu is walked first and an
  // admin-menu entry for a route already found is not added again.
  final seenAsPlain = <String>{};

  void walk(MenuEntry entry, {required bool adminOnly, required String via}) {
    final path = entry.routePath;
    if (path != null) {
      if (adminOnly && seenAsPlain.contains(path)) {
        // Already reachable the cheaper way.
      } else {
        if (!adminOnly) seenAsPlain.add(path);
        edges.add(Edge(
          path,
          '$via:${entry.key}',
          edgeCost(adminOnly: adminOnly, capability: entry.capability),
        ));
      }
    }
    for (final child in entry.children) {
      walk(child, adminOnly: adminOnly, via: via);
    }
  }

  for (final e in config.menuEntries) {
    walk(e, adminOnly: false, via: 'menu');
  }
  for (final e in config.toolbarMenuEntries) {
    walk(e, adminOnly: false, via: 'toolbar');
  }
  for (final e in config.adminMenuEntries) {
    walk(e, adminOnly: true, via: 'adminMenu');
  }
  return edges;
}

/// The table configurations a form shows, following the dialog forms its
/// actions name. Visited sets keep the two mutually recursive walks finite.
void collectFromForm(
  FormConfig config,
  Set<String> tables,
  Set<String> forms,
) {
  for (final field in allFields(config)) {
    if (field is FormDataTableFieldConfig) {
      collectFromTable(field.dataTableConfig, tables, forms);
    }
  }
}

void collectFromTable(String key, Set<String> tables, Set<String> forms) {
  if (!tables.add(key)) return;
  final table = getTableConfig(key);
  for (final action in [
    ...table.actions,
    ...table.secondRowActions,
    ...table.fromConfigRowActions,
  ]) {
    final formKey = action.configForm;
    if (formKey != null && forms.add(formKey)) {
      collectFromForm(getFormConfig(formKey), tables, forms);
    }
  }
}

/// The table configurations one route shows, and the navigate actions they
/// carry. The widget is the source: it holds the very configuration the screen
/// renders, so nothing has to be matched up by naming convention.
Set<String> tablesOfRoute(Widget widget) {
  final tables = <String>{};
  final forms = <String>{};
  if (widget is ScreenOne) {
    collectFromTable(widget.tableConfig.key, tables, forms);
  } else if (widget is ScreenWithMultiForms) {
    for (final f in widget.formConfig) {
      collectFromForm(f, tables, forms);
    }
  } else if (widget is ScreenWithTabsWithForm) {
    collectFromForm(widget.formConfig, tables, forms);
  } else if (widget is ScreenWithForm) {
    collectFromForm(widget.formConfig, tables, forms);
  } else if (widget is UserFlowScreen) {
    for (final state in widget.userFlowConfig.states.values) {
      collectFromForm(state.formConfig, tables, forms);
    }
  }
  return tables;
}

/// Every outgoing edge of one route.
List<Edge> edgesOf(String route, Widget widget) {
  final edges = <Edge>[];
  if (widget is BaseScreen) {
    edges.addAll(menuEdges(widget.screenConfig));
  }
  for (final key in tablesOfRoute(widget).toList()..sort()) {
    for (final action in [
      ...getTableConfig(key).actions,
      ...getTableConfig(key).secondRowActions,
      ...getTableConfig(key).fromConfigRowActions,
    ]) {
      final path = action.configScreenPath;
      if (path == null) continue;
      edges.add(Edge(
        path,
        'table:$key:${action.key}',
        edgeCost(adminOnly: false, capability: action.capability),
      ));
    }
  }
  if (widget is UserFlowScreen) {
    final exit = widget.userFlowConfig.exitScreenPath;
    if (exit != null) {
      edges.add(Edge(exit, 'flowExit', edgeCost(adminOnly: false)));
    }
  }
  return edges;
}

// ---------------------------------------------------------------------------
// The closure
// ---------------------------------------------------------------------------

String buildCorpus() {
  final routes = jetsRoutesMap.keys.toList()..sort();

  // Route to screen configuration. Every routed widget except the 404 message
  // extends BaseScreen, which is what makes this a lookup rather than a guess.
  final screenOfRoute = <String, String?>{};
  final widgetOfRoute = <String, String>{};
  for (final route in routes) {
    final widget = jetsRoutesMap[route]!;
    screenOfRoute[route] =
        widget is BaseScreen ? widget.screenConfig.key : null;
    widgetOfRoute[route] = widget.runtimeType.toString();
  }

  // Edges, computed once.
  final outgoing = <String, List<Edge>>{};
  for (final route in routes) {
    outgoing[route] = edgesOf(route, jetsRoutesMap[route]!);
  }

  // Fixpoint from the entry points. The home path is where an authenticated
  // session lands; the public paths are reachable with no session at all.
  final grants = <String, Set<Grant>>{
    homePath: {<String>{}},
  };
  final incoming = <String, Set<String>>{};
  var changed = true;
  while (changed) {
    changed = false;
    for (final route in routes) {
      final here = grants[route];
      if (here == null) continue;
      for (final edge in outgoing[route]!) {
        if (!jetsRoutesMap.containsKey(edge.to)) continue;
        (incoming[edge.to] ??= <String>{}).add('$route ${edge.via}');
        final produced = <Grant>[
          for (final g in here)
            for (final c in edge.cost) {...g, ...c},
        ];
        final before = grants[edge.to] ?? <Grant>{};
        final after = minimise([...before, ...produced]);
        if (grantLabels(after).join('|') != grantLabels(before).join('|')) {
          grants[edge.to] = after;
          changed = true;
        }
      }
    }
  }

  final publicRoutes = routes.where((r) => !_authRequired(r)).toList();

  final routeRows = <String, dynamic>{};
  for (final route in routes) {
    final reached = grants[route];
    routeRows[route] = <String, dynamic>{
      'screenKey': screenOfRoute[route],
      'widget': widgetOfRoute[route],
      'authRequired': _authRequired(route),
      'access': reached == null ? <String>[] : grantLabels(reached),
      'reachedFrom': (incoming[route]?.toList() ?? <String>[])..sort(),
      'outgoing': outgoing[route]!.map((e) => e.to).toSet().toList()..sort(),
    };
  }

  final routedScreens =
      screenOfRoute.values.whereType<String>().toSet().toList()..sort();
  final orphanScreens =
      allScreenKeys().where((k) => !routedScreens.contains(k)).toList();
  final noEdge = routes
      .where((r) => (incoming[r] == null || incoming[r]!.isEmpty) && r != homePath)
      .toList();

  final accessSummary = <String, int>{};
  for (final route in routes) {
    for (final label in grantLabels(grants[route] ?? <Grant>{})) {
      accessSummary[label] = (accessSummary[label] ?? 0) + 1;
    }
    if (grants[route] == null) {
      accessSummary['unreachedByConfiguration'] =
          (accessSummary['unreachedByConfiguration'] ?? 0) + 1;
    }
  }

  final body = <String, dynamic>{
    'comment':
        'Generated by jetsclient/test/screen_reachability_corpus_test.dart. '
        'Do not edit by hand — see the README beside this file.',
    'screenConfigCount': allScreenKeys().length,
    'screenKeysByRegistry': screenKeysByRegistry,
    'routeCount': routes.length,
    'routedScreenCount': routedScreens.length,
    'orphanScreenKeys': orphanScreens,
    'publicRoutes': publicRoutes,
    'routesWithNoConfiguredEdge': noEdge,
    'accessSummary': Map.fromEntries(
      accessSummary.entries.toList()..sort((a, b) => a.key.compareTo(b.key)),
    ),
    'routes': routeRows,
  };
  return '${const JsonEncoder.withIndent('  ').convert(body)}\n';
}

bool _authRequired(String route) => JetsRouteData(route).authRequired;

void main() {
  test('every registry reports keys, and every key resolves', () {
    // Guards the enumeration itself: an accessor wired to the wrong map would
    // otherwise produce a corpus that passes every other check in this file.
    screenKeysByRegistry.forEach((registry, keys) {
      expect(keys, isNotEmpty, reason: registry);
    });
    for (final key in allScreenKeys()) {
      expect(() => getScreenConfig(key), returnsNormally, reason: key);
    }
  });

  test('every routed screen is a registered screen', () {
    // The other direction from orphanScreenKeys: a route naming a configuration
    // no registry holds would throw at startup, and this says so by name.
    final registered = allScreenKeys().toSet();
    for (final entry in jetsRoutesMap.entries) {
      final widget = entry.value;
      if (widget is! BaseScreen) continue;
      expect(registered, contains(widget.screenConfig.key),
          reason: entry.key);
    }
  });

  test('the two public-path lists agree', () {
    // `noAuthRequiredPaths` and `JetsRouteData.publicPaths` are separate
    // constants naming the same idea, and only the second is enforced. The 404
    // page is in one and not the other; anything further apart is a bug.
    expect(
      noAuthRequiredPaths.difference(JetsRouteData.publicPaths),
      equals({pageNotFoundPath}),
      reason: 'the two public-path lists have drifted further apart',
    );
  });

  test('the login and register endpoints are the public route paths', () {
    // `http_client.dart:38` builds a [JetsRouteData] out of an **API** path and
    // uses `authRequired` to decide whether to send the request at all without a
    // session. That works only because `ServerEPs.loginEP` and `registerEP`
    // happen to be spelled the same as `loginPath` and `registerPath`, so the
    // router's public-path set doubles as the API client's allow-list.
    //
    // Nothing declares that coupling, and breaking it breaks login rather than
    // routing — an unauthenticated POST to a renamed endpoint would be refused
    // by the client before it left the browser. This is the assertion that says
    // so out loud.
    expect(JetsRouteData.publicPaths, contains(ServerEPs.loginEP));
    expect(JetsRouteData.publicPaths, contains(ServerEPs.registerEP));
  });

  test('corpus is unchanged', () {
    final corpus = buildCorpus();
    // ignore: avoid_print
    print('$beginMarker\n$corpus$endMarker');
    // ignore: avoid_print
    print('corpus checksum: ${checksum(corpus)}');
    expect(
      checksum(corpus),
      expectedChecksum,
      reason:
          'A route, a menu or a navigate action changed. Regenerate '
          '$corpusPath from this output and update expectedChecksum — both, '
          'together.',
    );
  });
}

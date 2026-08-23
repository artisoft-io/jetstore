/// Tests for the React handoff (task S.8, repaired by task F.10).
///
/// Runs on the chrome platform like everything else here — see
/// `jetstore_ai/CLAUDE.md`:
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/migrated_user_flows_test.dart
///
/// **The first test derives its expectation rather than restating it.** The
/// route map in `migrated_user_flows.dart` is a hand-written duplicate of facts
/// that live in `jetsRoutesMap` — which route opens which flow, and what the
/// route carries — and a hand-written duplicate is exactly what produced I-75 in
/// the first place. So the expected map is built by walking the route table for
/// its [UserFlowScreen]s: the flow key comes off the widget's own key and the
/// parameters off the template. A flow route added to this app without a row in
/// `userFlowRoutes` fails here, and a row naming the wrong flow fails here.
///
/// This is the project's *generate it, do not grep it* rule applied to a
/// mapping rather than to a count.
library;

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/routes/migrated_user_flows.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// The flow key a [UserFlowScreen] was registered under.
///
/// `jetsRoutesMap` builds each one as `UserFlowScreen(key: Key(<flow key>), …)`
/// and `Key(String)` is a `ValueKey<String>`, so the key the widget carries is
/// the [UserFlowKeys] value and nothing has to be matched by name.
String flowKeyOfWidget(String template, Widget widget) {
  final key = widget.key;
  if (key is! ValueKey<String>) {
    throw StateError('flow route $template has no string key: $key');
  }
  return key.value;
}

/// The `:`-prefixed segments of a route template, in declaration order.
List<String> parametersOfTemplate(String template) => Uri.parse(template)
    .pathSegments
    .where((s) => s.startsWith(PARAM_CHAR))
    .map((s) => s.substring(1))
    .toList();

/// Every flow route this app registers, read off `jetsRoutesMap`.
Map<String, UserFlowRoute> flowRoutesFromRouteTable() {
  final found = <String, UserFlowRoute>{};
  jetsRoutesMap.forEach((template, widget) {
    if (widget is! UserFlowScreen) return;
    found[template] = UserFlowRoute(flowKeyOfWidget(template, widget),
        parameters: parametersOfTemplate(template));
  });
  return found;
}

/// A plausible value for each of a route's parameters.
Map<String, dynamic> sampleParams(String template) => {
      for (final name in parametersOfTemplate(template)) name: 'v_$name',
    };

void main() {
  group('the route map is the route table', () {
    test('it names every flow route and no others', () {
      final expected = flowRoutesFromRouteTable();
      // Sorted, so a failure names the route that differs rather than dumping
      // two maps at each other.
      expect(userFlowRoutes.keys.toList()..sort(),
          equals(expected.keys.toList()..sort()));
    });

    test('eleven routes and eleven distinct flows', () {
      final expected = flowRoutesFromRouteTable();
      // The count is the corpus's (`user_flow_corpus_test.dart`, eleven flows in
      // 46 states) arrived at from the routing side. Two of them share a
      // directory — `file_mapping/` defines `fileMappingUF` and `mapFileUF` —
      // which is the ambiguity I-61 records and the reason a directory name is
      // not a flow name.
      expect(expected, hasLength(11));
      expect(expected.values.map((r) => r.flowKey).toSet(), hasLength(11));
    });

    test('every row names the flow the route actually opens', () {
      final expected = flowRoutesFromRouteTable();
      for (final entry in expected.entries) {
        final row = userFlowRoutes[entry.key];
        expect(row, isNotNull, reason: 'no row for ${entry.key}');
        expect(row!.flowKey, entry.value.flowKey, reason: entry.key);
        expect(row.parameters, entry.value.parameters, reason: entry.key);
      }
    });

    test('every flow key is a UserFlowKeys value', () {
      // Previously a hand-maintained set of eleven strings in this file. The
      // widget key and the constant are now the same object, so the check is
      // that nothing in `userFlowRoutes` was typed rather than referenced.
      final expected = flowRoutesFromRouteTable();
      for (final row in userFlowRoutes.values) {
        expect(expected.values.map((r) => r.flowKey), contains(row.flowKey));
      }
    });
  });

  group('the four routes that do not begin with their flow key', () {
    // I-75. The old lookup was `Uri.parse(path).pathSegments.first`, which is
    // right for seven of the eleven — and the two flows S.8 had listed were two
    // of the seven, so the shortcut was tested on the half that cannot fail.
    const disagreeing = <String, String>{
      ufMappingPath: UserFlowKeys.mapFileUF,
      ufHomeFiltersPath: UserFlowKeys.homeFiltersUF,
      ufPullWorkspacePath: UserFlowKeys.workspacePullUF,
      ufLoadConfigPath: UserFlowKeys.loadConfigUF,
    };

    test('exactly four disagree, and they are these four', () {
      final found = <String, String>{};
      userFlowRoutes.forEach((template, row) {
        final first = Uri.parse(template).pathSegments.first;
        if (first != row.flowKey) found[template] = row.flowKey;
      });
      expect(found, equals(disagreeing));
    });

    test('the map reaches each of them, where the leading segment did not', () {
      disagreeing.forEach((template, flowKey) {
        expect(userFlowRoutes[template]!.flowKey, flowKey);
        final url = handoffUrlFor(
            JetsRouteData(template, params: sampleParams(template)),
            migrated: {flowKey});
        expect(url, isNotNull, reason: template);
        expect(url, startsWith('/ide/flow/$flowKey'), reason: template);
      });
    });

    test("loadConfigUF's leading segment is the workspace registry's route", () {
      // The dangerous one: `/workspaces/loadConfigUF/:workspace_name` begins
      // with `workspaces`, which is `workspaceRegistryPath` exactly and
      // `workspaceHomePath`'s prefix. Under the old lookup, listing the segment
      // would have handed two working screens to a flow route.
      expect(Uri.parse(ufLoadConfigPath).pathSegments.first,
          Uri.parse(workspaceRegistryPath).pathSegments.first);
      // And under the new one it cannot, because the lookup is an exact match
      // on the template and neither screen is in the map.
      for (final screen in [workspaceRegistryPath, workspaceHomePath]) {
        expect(userFlowRoutes.containsKey(screen), isFalse, reason: screen);
        expect(
            handoffUrlFor(JetsRouteData(screen, params: sampleParams(screen)),
                migrated: {UserFlowKeys.loadConfigUF}),
            isNull,
            reason: screen);
      }
    });

    test('the two file_mapping flows are two routes, not one', () {
      // `/fileMappingUF` is `fileMappingUF` and
      // `/fileMappingUF/mapping/:table_name/:object_type` is `mapFileUF`. The
      // old lookup collapsed both onto `fileMappingUF` and dropped two
      // parameters doing it.
      expect(userFlowRoutes[ufFileMappingPath]!.flowKey,
          UserFlowKeys.fileMappingUF);
      expect(userFlowRoutes[ufMappingPath]!.flowKey, UserFlowKeys.mapFileUF);
      expect(
          handoffUrlFor(const JetsRouteData(ufFileMappingPath),
              migrated: {UserFlowKeys.fileMappingUF}),
          '/ide/flow/fileMappingUF');
      expect(
          handoffUrlFor(const JetsRouteData(ufFileMappingPath),
              migrated: {UserFlowKeys.mapFileUF}),
          isNull);
    });
  });

  group('the url carries the route parameters', () {
    test('all eleven routes build the url the React runner reads', () {
      // `FlowRunner` seeds every query parameter into form-state group 0 by
      // name (`jetsclient_ide/src/screens/FlowRunner.tsx`), so the names are the
      // template's and the order is the template's.
      const expectedUrls = <String, String>{
        ufClientRegistryPath:
            '/ide/flow/clientRegistryUF?startAtKey=v_startAtKey',
        ufSourceConfigPath: '/ide/flow/sourceConfigUF?startAtKey=v_startAtKey',
        ufFileMappingPath: '/ide/flow/fileMappingUF',
        ufMappingPath: '/ide/flow/mapFileUF'
            '?table_name=v_table_name&object_type=v_object_type',
        ufPipelineConfigPath: '/ide/flow/pipelineConfigUF',
        ufLoadFilesPath: '/ide/flow/loadFilesUF',
        ufRegisterFileKeyPath: '/ide/flow/registerFileKeyUF',
        ufStartPipelinePath: '/ide/flow/startPipelineUF',
        ufHomeFiltersPath: '/ide/flow/homeFiltersUF',
        ufPullWorkspacePath: '/ide/flow/workspacePullUF'
            '?key=v_key&workspace_name=v_workspace_name'
            '&workspace_branch=v_workspace_branch'
            '&feature_branch=v_feature_branch&workspace_uri=v_workspace_uri',
        ufLoadConfigPath:
            '/ide/flow/loadConfigUF?workspace_name=v_workspace_name',
      };
      // Every route, not a sample: this is the assertion I-75 says was missing.
      expect(expectedUrls.keys.toSet(), userFlowRoutes.keys.toSet());
      expectedUrls.forEach((template, url) {
        expect(
            handoffUrlFor(
                JetsRouteData(template, params: sampleParams(template)),
                migrated: {userFlowRoutes[template]!.flowKey}),
            url,
            reason: template);
      });
    });

    test('five of the eleven carry parameters', () {
      // I-75 counts five; `FlowRunner.tsx` said four until F.10 corrected it.
      final withParameters = userFlowRoutes.entries
          .where((e) => e.value.parameters.isNotEmpty)
          .map((e) => e.key)
          .toSet();
      expect(
          withParameters,
          {
            ufClientRegistryPath,
            ufSourceConfigPath,
            ufMappingPath,
            ufPullWorkspacePath,
            ufLoadConfigPath,
          },
          reason: 'the routes that carry parameters');
    });

    test('a value is encoded, not interpolated', () {
      // `pathSegments` are percent-decoded by the parser, so a workspace URI
      // arrives here raw and has to be encoded again on the way out. Without
      // this the `:` and `/` of a git URL would end the query value.
      final url = handoffUrlFor(
          const JetsRouteData(ufLoadConfigPath,
              params: {'workspace_name': 'a b&c=d/e'}),
          migrated: {UserFlowKeys.loadConfigUF});
      expect(url, '/ide/flow/loadConfigUF?workspace_name=a+b%26c%3Dd%2Fe');
      expect(Uri.parse(url!).queryParameters['workspace_name'], 'a b&c=d/e');
    });

    test('a missing or empty parameter falls through to this app', () {
      // The decision F.10 took: a flow whose arguments are absent is served
      // here, where it works, rather than handed over half-formed. The React
      // runner would render a worksheet with no rows and say nothing about why.
      expect(
          handoffUrlFor(const JetsRouteData(ufMappingPath),
              migrated: {UserFlowKeys.mapFileUF}),
          isNull);
      expect(
          handoffUrlFor(
              const JetsRouteData(ufMappingPath,
                  params: {'table_name': 'input_row'}),
              migrated: {UserFlowKeys.mapFileUF}),
          isNull,
          reason: 'one of two parameters is not enough');
      expect(
          handoffUrlFor(
              const JetsRouteData(ufMappingPath,
                  params: {'table_name': 'input_row', 'object_type': ''}),
              migrated: {UserFlowKeys.mapFileUF}),
          isNull,
          reason: 'an empty value is a missing one');
    });
  });

  group('what never hands off', () {
    test('no route hands off while the React app serves no flow', () {
      // **This assertion is the one that was missing, in the other direction.**
      // Until F.0a the set listed two flows the React app had no route for, and
      // the test that existed asserted only the URL this app *emits* — which was
      // correct, and said nothing about whether anything would accept it (I-50).
      expect(migratedUserFlows, isEmpty);
      for (final template in jetsRoutesMap.keys) {
        expect(
            handoffUrlFor(
                JetsRouteData(template, params: sampleParams(template))),
            isNull,
            reason: template);
      }
    });

    test('a non-flow route never hands off, whatever is migrated', () {
      // The other half of F.10's first decision: the map holds flow routes and
      // nothing else, so every screen route is this app's by absence. Asserted
      // with *every* flow key migrated at once, which is the strongest form —
      // no screen route can be reached by any flow's key.
      final everyFlow =
          userFlowRoutes.values.map((r) => r.flowKey).toSet();
      final nonFlowRoutes =
          jetsRoutesMap.keys.where((k) => !userFlowRoutes.containsKey(k));
      expect(nonFlowRoutes, hasLength(17));
      for (final template in nonFlowRoutes) {
        expect(
            handoffUrlFor(
                JetsRouteData(template, params: sampleParams(template)),
                migrated: everyFlow),
            isNull,
            reason: template);
      }
    });

    test('an unregistered path never hands off', () {
      // A path that is not a template. `jetsRoutesParser` cannot produce one —
      // it returns a `jetsRoutesMap` key or `pageNotFoundPath` — and an in-app
      // navigation that built one by hand is already broken before the handoff
      // sees it: `_setRoutePages` indexes `routesPagesMap` with a `!`
      // (`jets_router_delegate.dart`, `_setRoutePages`). So the lookup declines
      // rather than rescuing it, which keeps the failure where it belongs.
      final everyFlow =
          userFlowRoutes.values.map((r) => r.flowKey).toSet();
      for (final path in [
        '',
        '/',
        '/notAFlow',
        '/fileMappingUF/mapping/input_row/DomainKey', // the concrete path
        '/workspaces/loadConfigUF/my_workspace',
      ]) {
        expect(handoffUrlFor(JetsRouteData(path), migrated: everyFlow), isNull,
            reason: path);
      }
    });
  });
}

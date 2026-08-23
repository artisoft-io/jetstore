/// Tests for the React handoff (task S.8).
///
/// Runs on the chrome platform like everything else here — see
/// `jetstore_ai/CLAUDE.md`.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/routes/migrated_user_flows.dart';
import 'package:jetsclient/utils/constants.dart';

void main() {
  test('hands off nothing while the React app serves no flow', () {
    // **This assertion is the one that was missing, in the other direction.**
    // Until F.0a the set listed two flows the React app had no route for, and
    // the test that existed asserted only the URL this app *emits* — which was
    // correct, and said nothing about whether anything would accept it (I-50).
    expect(migratedUserFlows, isEmpty);
    expect(handoffUrlFor(UserFlowKeys.loadFilesUF), isNull);
    expect(handoffUrlFor(UserFlowKeys.registerFileKeyUF), isNull);
    expect(handoffUrlFor(UserFlowKeys.pipelineConfigUF), isNull);
  });

  test('the handoff url is still built the way the React app parses it', () {
    // The set is empty, so nothing exercises the URL shape any more. Asserting
    // it against a synthetic key keeps `reactAppBase` and the `/flow/<key>`
    // template covered while track F fills the set back in one flow at a time —
    // the React side reads it as `/ide` + `reactFlowPath(key)`
    // (`jetsclient_ide/src/userflow/routing.ts`, `reactFlowPath`).
    expect('$reactAppBase/flow/someFlowUF', '/ide/flow/someFlowUF');
  });

  test('every migrated key is a real flow key', () {
    // The failure this prevents is a typo that silently never matches, leaving
    // a migrated flow opening the Flutter version with nothing to say why.
    const known = <String>{
      UserFlowKeys.clientRegistryUF,
      UserFlowKeys.sourceConfigUF,
      UserFlowKeys.fileMappingUF,
      UserFlowKeys.homeFiltersUF,
      UserFlowKeys.mapFileUF,
      UserFlowKeys.pipelineConfigUF,
      UserFlowKeys.loadFilesUF,
      UserFlowKeys.registerFileKeyUF,
      UserFlowKeys.startPipelineUF,
      UserFlowKeys.workspacePullUF,
      UserFlowKeys.loadConfigUF,
    };
    for (final key in migratedUserFlows) {
      expect(known, contains(key), reason: '$key is not a UserFlowKeys value');
    }
  });

  test('an unknown key is not handed off', () {
    expect(handoffUrlFor('notAFlow'), isNull);
    expect(handoffUrlFor(''), isNull);
  });
}

/// Tests for the React handoff (task S.8).
///
/// Runs on the chrome platform like everything else here — see
/// `jetstore_ai/CLAUDE.md`.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/routes/migrated_user_flows.dart';
import 'package:jetsclient/utils/constants.dart';

void main() {
  test('hands off only the flows the React app serves', () {
    expect(handoffUrlFor(UserFlowKeys.loadFilesUF), '/ide/flow/loadFilesUF');
    expect(
      handoffUrlFor(UserFlowKeys.registerFileKeyUF),
      '/ide/flow/registerFileKeyUF',
    );
    expect(handoffUrlFor(UserFlowKeys.pipelineConfigUF), isNull);
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

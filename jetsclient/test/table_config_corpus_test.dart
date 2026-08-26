/// Serialises the nine user flows' table configurations and locks them against
/// the JSON corpus the React port consumes.
///
/// **Why this lives in the Flutter app's tests.** Phase 2 of the UI port
/// reimplements the data-table widget in React (task A.4). Its query builder has
/// to produce byte-for-byte the same `/dataTable` payload this app produces, and
/// the only way to check that without hand-transcribing 2,700 lines of Dart
/// configuration is to emit the configurations as data and let both sides read
/// the same file.
///
/// Running it as a *test* rather than a script is the point: if someone edits a
/// `TableConfig` in `lib/modules/user_flows/`, the checksum below stops matching
/// and the React corpus is known to be stale rather than silently diverging.
///
/// **This suite only runs on the chrome platform.** `flutter test` defaults to
/// the Dart VM, where `package:web` 1.1.1 — a direct dependency at
/// `pubspec.yaml:52`, and web-only by construction — fails to compile, taking
/// every test in this package with it. That is pre-existing and not caused by
/// this file; `test/form_button_test.dart` fails to load the same way. So:
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/table_config_corpus_test.dart
///
/// The browser has no filesystem, which is why the corpus is *printed* between
/// sentinels rather than written, and why drift is caught by a checksum rather
/// than by comparing files. To regenerate the React fixture, see
/// `jetsclient_ide/src/datatable/fixtures/README.md`.
///
/// **What cannot be serialised is itself the finding.** `cellFilter`,
/// `isEnabledFnc`, `modelStateHandler` and `actionDelegate` are Dart closures, so
/// they are emitted as booleans naming their presence. That is the assessment's
/// §3.2 — configuration is not data, it embeds closures — made countable: any
/// `true` below is a place the React port needs an answer that this file cannot
/// give it.
library;

import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/modules/data_table_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

import 'corpus_support.dart';

/// Where the React side keeps its copy. Not read here — see the header.
const corpusPath = 'jetsclient_ide/src/datatable/fixtures/table_configs.json';

/// Printed around the corpus so a regeneration can extract it from test output.
const beginMarker = '===BEGIN TABLE CONFIG CORPUS===';
const endMarker = '===END TABLE CONFIG CORPUS===';

/// Checksum of the corpus this app currently produces.
///
/// **Update it only together with the React fixture.** A failure here means a
/// `TableConfig` changed; the fix is to regenerate both, never to edit this
/// number so the test goes green.
///
/// **That pairing is enforced as of 2026-08-25**, and was not before:
/// `jetsclient_ide/src/corpusFixtures.test.ts` hashes the fixture on disk and
/// asserts it against this constant, so a fixture left behind by a bump here
/// fails on the React side under `npm test`. It caught nothing when it was
/// written; it was written because C.0 bumped two of these constants and left
/// both fixtures stale, and this whole suite stayed green.
const expectedChecksum = 'fnv1a32:190c0fed';

/// The 37 table configurations defined by the nine user flows, in the order
/// `sort` gives them, so a regeneration produces a stable diff.
///
/// Screen tables — the Workspace IDE's and the ones in `data_table_config_impl`
/// — are deliberately absent: the port reaches them in Phase 3, and including
/// them here would imply Phase 2 owes them a query builder.
const userFlowTableKeys = <String>[
  DTKeys.fmFileMappingTableUF,
  DTKeys.fmInputSourceMappingUF,
  DTKeys.hfFileKeyFilterTypeTableUF,
  DTKeys.hfProcessTableUF,
  DTKeys.hfStatusTableUF,
  DTKeys.injectedProcessInputTable,
  DTKeys.inputRegistryTable,
  DTKeys.lfFileKeyStagingTable,
  DTKeys.lfSourceConfigTable,
  DTKeys.mainProcessInputTable,
  DTKeys.mergeProcessInputTable,
  DTKeys.otherWorkspaceActionOptions,
  DTKeys.pcInjectedProcessInputKeys,
  DTKeys.pcMainProcessInputKey,
  DTKeys.pcMergedProcessInputKeys,
  DTKeys.pcPipelineConfigTable,
  DTKeys.pcProcessInputRegistry,
  DTKeys.pcProcessInputRegistry4MI,
  DTKeys.pcSummaryProcessInputs,
  DTKeys.pcViewInjectedProcessInputKeys,
  DTKeys.pcViewMergedProcessInputKeys,
  DTKeys.spInjectedProcessInput,
  DTKeys.spSummaryDataSources,
  DTKeys.wpPullWorkspaceConfirmOptions,
  FSK.client,
  FSK.mainInputRegistryKey,
  FSK.mergedInputRegistryKeys,
  FSK.org,
  FSK.pcAddOrEditPipelineConfigOption,
  FSK.pipelineConfigKey,
  FSK.scAddOrEditSourceConfigOption,
  FSK.scFileTypeOption,
  FSK.scSingleOrMultiPartFileOption,
  FSK.scSourceConfigKey,
  FSK.ufClientOrVendorOption,
  FSK.wpClientList,
  FSK.wpClientListRO,
];

String buildCorpus() {
  final tables = <String, dynamic>{};
  for (final key in userFlowTableKeys) {
    tables[key] = tableToJson(getTableConfig(key));
  }
  final body = <String, dynamic>{
    'comment': 'Generated by jetsclient/test/table_config_corpus_test.dart. '
        'Do not edit by hand — see the README beside this file.',
    'tableCount': userFlowTableKeys.length,
    'tables': tables,
  };
  return '${const JsonEncoder.withIndent('  ').convert(body)}\n';
}

void main() {
  test('the user flows define 37 table configurations', () {
    // Guards the list above against a flow adding a table without anyone
    // noticing here; the count is the assessment's §8 figure.
    expect(userFlowTableKeys.length, 37);
    expect(userFlowTableKeys.toSet().length, 37, reason: 'keys must be unique');
  });

  test('every user flow table configuration resolves', () {
    for (final key in userFlowTableKeys) {
      expect(() => getTableConfig(key), returnsNormally, reason: key);
    }
  });

  test('the corpus the React port reads has not drifted', () {
    final corpus = buildCorpus();
    // ignore: avoid_print
    print('$beginMarker\n$corpus$endMarker');
    // ignore: avoid_print
    print('corpus checksum: ${checksum(corpus)}');

    expect(checksum(corpus), expectedChecksum,
        reason: 'a TableConfig under lib/modules/user_flows/ has changed, so '
            '$corpusPath is stale. Regenerate it and update expectedChecksum '
            'together — see the README beside the fixture.');
  });
}

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
import 'package:jetsclient/models/data_table_config.dart';
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

Map<String, dynamic> columnToJson(ColumnConfig c) => <String, dynamic>{
      'index': c.index,
      if (c.table != null) 'table': c.table,
      'name': c.name,
      if (c.calculatedAs != null) 'calculatedAs': c.calculatedAs,
      'label': c.label,
      'tooltips': c.tooltips,
      'isNumeric': c.isNumeric,
      'isHidden': c.isHidden,
      'maxLines': c.maxLines,
      'columnWidth': c.columnWidth,
      // A closure. See the library comment.
      'hasCellFilter': c.cellFilter != null,
    };

Map<String, dynamic> fromClauseToJson(FromClause f) => <String, dynamic>{
      'schemaName': f.schemaName,
      'tableName': f.tableName,
      'asTableName': f.asTableName,
    };

Map<String, dynamic> withClauseToJson(WithClause w) => <String, dynamic>{
      'withName': w.withName,
      'asStatement': w.asStatement,
      'stateVariables': w.stateVariables,
    };

Map<String, dynamic> whereClauseToJson(WhereClause w) => <String, dynamic>{
      if (w.table != null) 'table': w.table,
      'column': w.column,
      if (w.formStateKey != null) 'formStateKey': w.formStateKey,
      'defaultValue': w.defaultValue,
      if (w.joinWith != null) 'joinWith': w.joinWith,
      if (w.predicate != null)
        'predicate': <String, dynamic>{
          'formStateKey': w.predicate!.formStateKey,
          'expectedValue': w.predicate!.expectedValue,
        },
      'lookupColumnInFormState': w.lookupColumnInFormState,
      if (w.like != null) 'like': w.like,
      if (w.ge != null) 'ge': w.ge,
      if (w.le != null) 'le': w.le,
      if (w.orWith != null) 'orWith': whereClauseToJson(w.orWith!),
    };

Map<String, dynamic> formStateToJson(DataTableFormStateConfig f) =>
    <String, dynamic>{
      'keyColumnIdx': f.keyColumnIdx,
      'otherColumns': f.otherColumns
          .map((o) => <String, dynamic>{
                'stateKey': o.stateKey,
                'columnIdx': o.columnIdx,
              })
          .toList(),
    };

Map<String, dynamic> criteriaToJson(ActionEnableCriteria c) => <String, dynamic>{
      'columnPos': c.columnPos,
      'criteriaType': c.criteriaType.name,
      'value': c.value,
    };

Map<String, dynamic> actionToJson(ActionConfig a) => <String, dynamic>{
      'actionType': a.actionType.name,
      'key': a.key,
      'label': a.label,
      'style': a.style.name,
      if (a.isVisibleWhenCheckboxVisible != null)
        'isVisibleWhenCheckboxVisible': a.isVisibleWhenCheckboxVisible,
      if (a.isEnabledWhenHavingSelectedRows != null)
        'isEnabledWhenHavingSelectedRows': a.isEnabledWhenHavingSelectedRows,
      if (a.isEnabledWhenWhereClauseSatisfied != null)
        'isEnabledWhenWhereClauseSatisfied': a.isEnabledWhenWhereClauseSatisfied,
      if (a.isEnabledWhenStateHasKeys != null)
        'isEnabledWhenStateHasKeys': a.isEnabledWhenStateHasKeys,
      if (a.navigationParams != null) 'navigationParams': a.navigationParams,
      if (a.stateFormNavigationParams != null)
        'stateFormNavigationParams': a.stateFormNavigationParams,
      if (a.configForm != null) 'configForm': a.configForm,
      if (a.configScreenPath != null) 'configScreenPath': a.configScreenPath,
      if (a.actionName != null) 'actionName': a.actionName,
      if (a.capability != null) 'capability': a.capability,
      'stateGroup': a.stateGroup,
      if (a.actionEnableCriterias != null)
        'actionEnableCriterias': a.actionEnableCriterias!
            .map((conj) => conj.map(criteriaToJson).toList())
            .toList(),
      // Closures. See the library comment.
      'hasIsEnabledFnc': a.isEnabledFnc != null,
      'hasActionDelegate': a.actionDelegate != null,
    };

Map<String, dynamic> tableToJson(TableConfig t) => <String, dynamic>{
      'key': t.key,
      'label': t.label,
      'apiPath': t.apiPath,
      'apiAction': t.apiAction,
      if (t.modelStateFormKey != null) 'modelStateFormKey': t.modelStateFormKey,
      if (t.staticTableModel != null) 'staticTableModel': t.staticTableModel,
      'isCheckboxVisible': t.isCheckboxVisible,
      'isCheckboxSingleSelect': t.isCheckboxSingleSelect,
      'isReadOnly': t.isReadOnly,
      'showSelectedOnly': t.showSelectedOnly,
      'actions': t.actions.map(actionToJson).toList(),
      'secondRowActions': t.secondRowActions.map(actionToJson).toList(),
      'fromConfigRowActions': t.fromConfigRowActions.map(actionToJson).toList(),
      'columns': t.columns.map(columnToJson).toList(),
      'defaultToAllRows': t.defaultToAllRows,
      if (t.sqlQuery != null)
        'sqlQuery': <String, dynamic>{
          'sqlQuery': t.sqlQuery!.sqlQuery,
          'stateVariables': t.sqlQuery!.stateVariables,
        },
      'requestColumnDef': t.requestColumnDef,
      'withClauses': t.withClauses.map(withClauseToJson).toList(),
      'fromClauses': t.fromClauses.map(fromClauseToJson).toList(),
      'whereClauses': t.whereClauses.map(whereClauseToJson).toList(),
      'distinctOnClauses': t.distinctOnClauses,
      'refreshOnKeyUpdateEvent': t.refreshOnKeyUpdateEvent,
      if (t.formStateConfig != null)
        'formStateConfig': formStateToJson(t.formStateConfig!),
      'sortColumnName': t.sortColumnName,
      'sortColumnTableName': t.sortColumnTableName,
      'sortAscending': t.sortAscending,
      'rowsPerPage': t.rowsPerPage,
      'noFooter': t.noFooter,
      if (t.dataRowMinHeight != null) 'dataRowMinHeight': t.dataRowMinHeight,
      if (t.dataRowMaxHeight != null) 'dataRowMaxHeight': t.dataRowMaxHeight,
      if (t.noCopy2Clipboard != null) 'noCopy2Clipboard': t.noCopy2Clipboard,
      // A closure. See the library comment.
      'hasModelStateHandler': t.modelStateHandler != null,
    };


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

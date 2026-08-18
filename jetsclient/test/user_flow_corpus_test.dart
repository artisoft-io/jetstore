/// Serialises every `UserFlowConfig` the app defines, for task S.1.
///
/// The third corpus, and the same argument as its two siblings
/// (`table_config_corpus_test.dart`, `form_field_corpus_test.dart`): S.1 writes
/// a JSON schema for `UserFlowConfig`, and a schema is a claim about what
/// configurations exist. Grepping the nine `user_flows/` directories answers a
/// question about *source text*; the schema needs an answer about the
/// constructed object graph. The two have already differed three times in this
/// phase — see I-3 and I-12.
///
/// **Chrome only**, like its siblings — see either file's header, and
/// `jetstore_ai/CLAUDE.md`:
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/user_flow_corpus_test.dart
///
/// **What cannot be serialised is the finding, again.** `actionDelegate` and
/// `formStateInitializer` are Dart closures. They are emitted as booleans naming
/// their presence, and each `true` is a place the schema needs an answer this
/// file cannot give it — which for S.1 means the named-`escape` mechanism S.2
/// builds, reached from the configuration side.
library;

import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flow_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

import 'corpus_support.dart';

const corpusPath = 'jetsclient_ide/src/userflow/fixtures/user_flows.json';
const beginMarker = '===BEGIN USER FLOW CORPUS===';
const endMarker = '===END USER FLOW CORPUS===';

/// Update only together with the fixture. See the README beside it.
const expectedChecksum = 'fnv1a32:1c9be3a3';

/// Every flow the app registers.
///
/// **Eleven, not the nine the documents say** — `UserFlowKeys`
/// (`utils/constants.dart:762`) declares eleven and `jets_routes_app.dart`
/// mounts eleven. "Nine" counts the *directories* under `lib/modules/user_flows/`:
/// `workspace_pull/` defines two flows and `file_mapping/` defines two.
const userFlowKeys = <String>[
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
];

/// Maps a `FormConfig` object back to the registry key it is served under.
///
/// **A flow state holds the resolved object, not the key it asked for**, so the
/// key has to be recovered by identity. It is worth recovering because the two
/// notions of a form's identity disagree: `FormConfig` also carries a `key`
/// field, and for two of the fifty registrations it names a *different* form.
/// The registry key is the one the app dispatches on; the field is read nowhere
/// outside a commented-out print (`components/form.dart:89`).
final Map<FormConfig, String> registryKeyOf = {
  for (final k in userFlowFormKeys) getFormConfig(k): k,
};

/// Forms whose own `key` field disagrees with the key they are registered under.
final formKeyMismatches = <String, String>{};

/// Counters filled in during serialisation, so the aggregate figures the schema
/// is argued from come from the same walk as the corpus rather than a second one.
final choiceTypeCounts = <String, int>{};
final operatorCounts = <String, int>{};
final rhsKindCounts = <String, int>{};
var nestedChoiceCount = 0;
var emptyNestedNextStateCount = 0;

Map<String, dynamic> choiceToJson(UserFlowChoice c, {bool nested = false}) {
  final type = c.runtimeType.toString();
  choiceTypeCounts[type] = (choiceTypeCounts[type] ?? 0) + 1;
  if (nested) {
    nestedChoiceCount++;
    // `nextState` is required by the abstract base, so a nested sub-expression
    // has to supply one and it is never read. Counting the empty ones is how the
    // schema justifies dropping the field from nested positions.
    if (c.nextState.isEmpty) emptyNestedNextStateCount++;
  }
  final json = <String, dynamic>{'type': type, 'nextState': c.nextState};
  if (c is Expression) {
    operatorCounts[c.op.name] = (operatorCounts[c.op.name] ?? 0) + 1;
    final kind = c.isRhsStateKey ? 'stateKey' : 'literal';
    rhsKindCounts[kind] = (rhsKindCounts[kind] ?? 0) + 1;
    json.addAll(<String, dynamic>{
      'lhsStateKey': c.lhsStateKey,
      'op': c.op.name,
      'rhsValue': c.rhsValue,
      'isRhsStateKey': c.isRhsStateKey,
    });
  } else if (c is IsNullExpression) {
    json['lhsStateKey'] = c.lhsStateKey;
  } else if (c is IsNullOrEmptyExpression) {
    json['lhsStateKey'] = c.lhsStateKey;
  } else if (c is IsNotExpression) {
    json['expression'] = choiceToJson(c.expression, nested: true);
  } else if (c is BooleanExpression) {
    json['isConjunction'] = c.isConjunction;
    json['items'] = c.items.map((i) => choiceToJson(i, nested: true)).toList();
  }
  return json;
}

Map<String, dynamic> stateToJson(UserFlowState s) {
  final registryKey = registryKeyOf[s.formConfig];
  if (registryKey != null && registryKey != s.formConfig.key) {
    formKeyMismatches[registryKey] = s.formConfig.key;
  }
  return <String, dynamic>{
    'key': s.key,
    'description': s.description,
    // The registry key — what `getFormConfig` was called with, and what a
    // `.uf.json` will name. `formConfigSelfKey` is emitted only when the
    // form's own `key` field disagrees, which is the whole point of emitting
    // it: it is evidence the field is unmaintained, not a second identity.
    'formConfig': registryKey ?? '!!UNRESOLVED:${s.formConfig.key}',
    if (registryKey != null && registryKey != s.formConfig.key)
      'formConfigSelfKey': s.formConfig.key,
    if (s.stateAction != null) 'stateAction': s.stateAction,
    if (s.isEnd) 'isEnd': true,
    if (s.defaultNextState != null) 'defaultNextState': s.defaultNextState,
    if (s.choices.isNotEmpty)
      'choices': s.choices.map((c) => choiceToJson(c)).toList(),
  };
}

String buildCorpus() {
  choiceTypeCounts.clear();
  operatorCounts.clear();
  rhsKindCounts.clear();
  nestedChoiceCount = 0;
  emptyNestedNextStateCount = 0;

  formKeyMismatches.clear();
  final flows = <String, dynamic>{};
  final formKeys = <String>{};
  var stateCount = 0;

  for (final key in userFlowKeys) {
    final config = getUserFlowConfig(key);
    // Keys are emitted in the map's own order, which is the source order of the
    // `states` literal — the reading order of the flow, not alphabetical.
    final states = <String, dynamic>{};
    config.states.forEach((k, s) {
      states[k] = stateToJson(s);
      formKeys.add(registryKeyOf[s.formConfig] ?? '!!UNRESOLVED');
      stateCount++;
    });
    flows[key] = <String, dynamic>{
      'startAtKey': config.startAtKey,
      if (config.exitScreenPath != null)
        'exitScreenPath': config.exitScreenPath,
      if (config.formStateInitializer != null) 'hasFormStateInitializer': true,
      'stateCount': states.length,
      'validationErrors': config.validateConfiguration(),
      'states': states,
    };
  }

  final body = <String, dynamic>{
    'comment':
        'Generated by jetsclient/test/user_flow_corpus_test.dart. '
        'Do not edit by hand — see the README beside this file.',
    'flowCount': userFlowKeys.length,
    'stateCount': stateCount,
    'distinctFormKeys': (formKeys.toList()..sort()).length,
    'choiceTypeCounts': Map.fromEntries(
      choiceTypeCounts.entries.toList()
        ..sort((a, b) => b.value.compareTo(a.value)),
    ),
    'operatorCounts': operatorCounts,
    'rhsKindCounts': rhsKindCounts,
    'nestedChoiceCount': nestedChoiceCount,
    'emptyNestedNextStateCount': emptyNestedNextStateCount,
    'formKeyMismatches': Map.fromEntries(
      formKeyMismatches.entries.toList()
        ..sort((a, b) => a.key.compareTo(b.key)),
    ),
    'unreferencedFormKeys':
        userFlowFormKeys.where((k) => !formKeys.contains(k)).toList()..sort(),
    'formKeys': formKeys.toList()..sort(),
    'flows': flows,
  };
  return '${const JsonEncoder.withIndent('  ').convert(body)}\n';
}

void main() {
  test('the app registers eleven user flows, and every one resolves', () {
    expect(userFlowKeys.length, 11);
    expect(
      userFlowKeys.toSet().length,
      userFlowKeys.length,
      reason: 'keys must be unique',
    );
    for (final key in userFlowKeys) {
      expect(() => getUserFlowConfig(key), returnsNormally, reason: key);
    }
  });

  test('every flow passes the validator the port has to reimplement', () {
    // `validateConfiguration()` (`user_flow_config.dart:61`) is 19 lines and S.4
    // ports it to Go. If the shipping corpus failed it, the port would be
    // reproducing a rule the app does not actually keep.
    for (final key in userFlowKeys) {
      expect(
        getUserFlowConfig(key).validateConfiguration(),
        isEmpty,
        reason: key,
      );
    }
  });

  test(
    'every state resolves to a registry key, so nothing is unaccounted for',
    () {
      // The cross-check I-12 says a generated corpus needs. `registryKeyOf` is
      // built from a hand-written list of fifty keys; if a flow references a form
      // outside it, the corpus would silently emit the form's own `key` field
      // instead and the schema would be written against a name nothing serves.
      buildCorpus();
      expect(
        registryKeyOf.length,
        50,
        reason: 'two registry keys serving one object would collapse the map',
      );
      for (final key in userFlowKeys) {
        getUserFlowConfig(key).states.forEach((k, s) {
          expect(
            registryKeyOf[s.formConfig],
            isNotNull,
            reason: '\$key.\$k names a form outside userFlowFormKeys',
          );
        });
      }
    },
  );

  test('the user flow corpus the React port reads has not drifted', () {
    final corpus = buildCorpus();
    // ignore: avoid_print
    print('$beginMarker\n$corpus$endMarker');
    // ignore: avoid_print
    print('corpus checksum: ${checksum(corpus)}');

    expect(
      checksum(corpus),
      expectedChecksum,
      reason:
          'a user flow configuration under lib/modules/user_flows/ has '
          'changed, so $corpusPath is stale. Regenerate it and update '
          'expectedChecksum together — see the README beside the fixture.',
    );
  });
}

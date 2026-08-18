/// Serialises the field inventory of the nine user flows' forms.
///
/// The sibling of `table_config_corpus_test.dart`, and for the same reason: task
/// A.3 builds the text input and dropdown widgets, and the only trustworthy way
/// to know *which options of those widgets are actually used* is to ask the
/// running app rather than to grep the source. The table sizing was wrong three
/// times over before that lesson took — configurations share objects by
/// reference and dead code counts as live — and this is the same shape of
/// question.
///
/// **Only the field inventory, not the whole `FormConfig`.** A form also carries
/// validators, action delegates and layout, all of which are Dart closures; the
/// closures are the thing the port replaces rather than reads, so emitting them
/// is neither possible nor useful. What is emitted is what a widget has to
/// support: the field type, its key, and the options set to something other than
/// their default.
///
/// **Chrome only**, like its sibling — see that file's header, and
/// `jetstore_ai/CLAUDE.md`:
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/form_field_corpus_test.dart
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

import 'corpus_support.dart';

const corpusPath = 'jetsclient_ide/src/datatable/fixtures/form_fields.json';
const beginMarker = '===BEGIN FORM FIELD CORPUS===';
const endMarker = '===END FORM FIELD CORPUS===';

/// Update only together with the fixture. See the README beside it.
const expectedChecksum = 'fnv1a32:a87baa83';

/// The 50 form configurations the nine user flows define.
const userFlowFormKeys = <String>[
  FormKeys.fmFileMappingUF,
  FormKeys.fmMappingFormUF,
  FormKeys.fmSelectSourceConfigUF,
  FormKeys.hfSelectFileKeyFilterUF,
  FormKeys.hfSelectProcessUF,
  FormKeys.hfSelectStatusUF,
  FormKeys.hfSelectTimeWindowUF,
  FormKeys.hfViewStatusTableUF,
  FormKeys.lfSelectFileKeysUF,
  FormKeys.lfSelectSourceConfigUF,
  FormKeys.loadRawRows,
  FormKeys.pcAddInjectedProcessInputsUF,
  FormKeys.pcAddMergeProcessInputsUF,
  FormKeys.pcAddOrEditPipelineConfigUF,
  FormKeys.pcAddPipelineConfigUF,
  FormKeys.pcAutomationUF,
  FormKeys.pcNewProcessInputDialog,
  FormKeys.pcNewProcessInputDialog4MI,
  FormKeys.pcSelectMainProcessInputUF,
  FormKeys.pcSelectPipelineConfigUF,
  FormKeys.pcSummaryUF,
  FormKeys.pcViewInjectedProcessInputsUF,
  FormKeys.pcViewMergeProcessInputsUF,
  FormKeys.rfkSubmitSchemaEvent,
  FormKeys.scAddOrEditSourceConfigUF,
  FormKeys.scAddSchemaProviderJsonUF,
  FormKeys.scAddSourceConfigUF,
  FormKeys.scEditCodeValueMappingUF,
  FormKeys.scEditDomainKeysUF,
  FormKeys.scEditFileHeadersUF,
  FormKeys.scEditFixedWidthLayoutUF,
  FormKeys.scEditXlsxOptionsUF,
  FormKeys.scSelectSingleOrMultiPartFileUF,
  FormKeys.scSelectSourceConfigUF,
  FormKeys.scSourceConfigTypeUF,
  FormKeys.scSummaryUF,
  FormKeys.spSelectMainDataSourceUF,
  FormKeys.spSelectMergedDataSourcesUF,
  FormKeys.spSelectPipelineConfigUF,
  FormKeys.spSummaryUF,
  FormKeys.ufCreateClient,
  FormKeys.ufSelectClient,
  FormKeys.ufSelectClientOrVendor,
  FormKeys.ufShowVendor,
  FormKeys.ufVendor,
  FormKeys.wpConfirmLoadConfigUF,
  FormKeys.wpConfirmPullWorkspaceUF,
  FormKeys.wpLoadConfigUF,
  FormKeys.wpPullWorkspaceUF,
  FormKeys.wpSelectClientsgUF,
];

/// Every field a form declares, across all three containers it may use.
///
/// **A `FormConfig` holds fields in three places** — `inputFields` for classic
/// forms, `inputFieldsV2` for the row-flex variant, and `formTabsConfig` for
/// tabbed ones — and a form uses whichever suits it. Walking only `inputFields`
/// found 29 of the 37 `FormDataTableFieldConfig` sites the flows declare; the
/// missing eight were not dead code, they were in the other two containers.
///
/// A fourth path is deliberately not walked: `inputFieldRowBuilder` is a closure
/// that builds rows per record at run time, so its fields do not exist until a
/// form is driven. Forms that use it are counted with the fields they declare
/// statically, and `hasRowBuilder` marks them.
List<FormFieldConfig> allFields(FormConfig config) => [
      ...config.inputFields.expand((row) => row),
      ...config.inputFieldsV2.expand((row) => row.rowConfig),
      ...config.formTabsConfig.map((tab) => tab.inputField),
    ];

Map<String, dynamic> fieldToJson(FormFieldConfig f) {
  final json = <String, dynamic>{
    'type': f.runtimeType.toString(),
    'key': f.key,
    'group': f.group,
    'flex': f.flex,
  };

  // Only options set away from their default are emitted. The question A.3 asks
  // is which of them a widget must support, and a field that leaves an option
  // alone is evidence that it need not.
  if (f is FormInputFieldConfig) {
    json.addAll(<String, dynamic>{
      'label': f.label,
      'hint': f.hint,
      if (f.autofocus) 'autofocus': true,
      if (f.obscureText) 'obscureText': true,
      if (f.isReadOnly) 'isReadOnly': true,
      if (f.isReadOnlyEval != null) 'hasIsReadOnlyEval': true,
      'textRestriction': f.textRestriction.name,
      if (f.maxLines != 1) 'maxLines': f.maxLines,
      'maxLength': f.maxLength,
      if (f.defaultValue != null) 'defaultValue': f.defaultValue,
      if (f.autofillHints != null) 'autofillHints': f.autofillHints,
      if (f.useDefaultFont) 'useDefaultFont': true,
      if (f.syncWithFormState) 'syncWithFormState': true,
      if (f.showCopyToClipboard) 'showCopyToClipboard': true,
    });
  } else if (f is FormDropdownFieldConfig) {
    json.addAll(<String, dynamic>{
      'itemCount': f.items.length,
      'items': f.items.map((i) => i.label).toList(),
      if (f.defaultItemPos != 0) 'defaultItemPos': f.defaultItemPos,
      if (f.dropdownItemsQuery != null) 'hasDropdownItemsQuery': true,
      if (f.returnedModelCacheKey != null) 'returnedModelCacheKey': f.returnedModelCacheKey,
      if (f.stateKeyPredicates.isNotEmpty) 'stateKeyPredicates': f.stateKeyPredicates,
      if (f.whereStateContains.isNotEmpty) 'whereStateContains': f.whereStateContains,
      if (f.isReadOnly) 'isReadOnly': true,
      if (f.makeReadOnlyWhenHasSelectedValue) 'makeReadOnlyWhenHasSelectedValue': true,
    });
  } else if (f is FormDataTableFieldConfig) {
    json['dataTableConfig'] = f.dataTableConfig;
  }
  return json;
}

String buildCorpus() {
  final forms = <String, dynamic>{};
  final typeCounts = <String, int>{};

  for (final key in userFlowFormKeys) {
    final config = getFormConfig(key);
    final fields = allFields(config).map((f) {
      typeCounts[f.runtimeType.toString()] =
          (typeCounts[f.runtimeType.toString()] ?? 0) + 1;
      return fieldToJson(f);
    }).toList();
    forms[key] = <String, dynamic>{
      'key': config.key,
      'title': config.title,
      'fieldCount': fields.length,
      if (config.inputFieldRowBuilder != null) 'hasRowBuilder': true,
      'fields': fields,
    };
  }

  final body = <String, dynamic>{
    'comment': 'Generated by jetsclient/test/form_field_corpus_test.dart. '
        'Do not edit by hand — see the README beside this file.',
    'formCount': userFlowFormKeys.length,
    'fieldTypeCounts': Map.fromEntries(
        typeCounts.entries.toList()..sort((a, b) => b.value.compareTo(a.value))),
    'forms': forms,
  };
  return '${const JsonEncoder.withIndent('  ').convert(body)}\n';
}

void main() {
  test('the nine user flows define 50 form configurations', () {
    expect(userFlowFormKeys.length, 50);
    expect(userFlowFormKeys.toSet().length, userFlowFormKeys.length,
        reason: 'keys must be unique');
  });

  test('every form configuration resolves', () {
    for (final key in userFlowFormKeys) {
      expect(() => getFormConfig(key), returnsNormally, reason: key);
    }
  });

  test('the field corpus the React port reads has not drifted', () {
    final corpus = buildCorpus();
    // ignore: avoid_print
    print('$beginMarker\n$corpus$endMarker');
    // ignore: avoid_print
    print('corpus checksum: ${checksum(corpus)}');

    expect(checksum(corpus), expectedChecksum,
        reason: 'a form configuration under lib/modules/user_flows/ has changed, '
            'so $corpusPath is stale. Regenerate it and update expectedChecksum '
            'together — see the README beside the fixture.');
  });
}

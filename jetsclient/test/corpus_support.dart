/// Shared helpers for the three corpus tests.
///
/// `table_config_corpus_test.dart`, `form_field_corpus_test.dart` and
/// `user_flow_corpus_test.dart` each serialise part of the Flutter app's
/// configuration for the React port to read, and all three detect drift with a
/// checksum rather than by comparing files — because these tests run in a
/// browser, where there is no filesystem. See any of those files' headers, and
/// `jetstore_ai/CLAUDE.md`.
library;

import 'dart:convert';

import 'package:jetsclient/utils/constants.dart';

/// FNV-1a, 32-bit, over the UTF-8 bytes of the corpus.
///
/// Hand-rolled because `package:crypto` is only a transitive dependency here and
/// this is drift detection, not a security boundary: the failure it has to catch
/// is a developer editing a `TableConfig` and not knowing the React side reads
/// it too.
String checksum(String s) {
  var hash = 0x811c9dc5;
  for (final byte in utf8.encode(s)) {
    hash ^= byte;
    // Multiply by the FNV prime, 16777619, in 32-bit arithmetic. Written as
    // shifts because the product overflows JavaScript's 53-bit integers, and
    // these tests run in a browser.
    hash =
        (hash +
            ((hash << 1) & 0xffffffff) +
            ((hash << 4) & 0xffffffff) +
            ((hash << 7) & 0xffffffff) +
            ((hash << 8) & 0xffffffff) +
            ((hash << 24) & 0xffffffff)) &
        0xffffffff;
  }
  return 'fnv1a32:${hash.toRadixString(16).padLeft(8, '0')}';
}

/// The 50 form configurations the user flows' screens define.
///
/// **These are *registry* keys** — the keys `getFormConfig` is called with — and
/// that distinction is load-bearing rather than pedantic: a `FormConfig` also
/// carries a `key` field of its own, and for two of the fifty the two disagree.
/// See `user_flow_corpus_test.dart`, which reports the mismatches.
///
/// Shared by `form_field_corpus_test.dart`, which walks the forms' fields, and
/// `user_flow_corpus_test.dart`, which needs the list to recover which registry
/// key each flow state's form was looked up by.
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

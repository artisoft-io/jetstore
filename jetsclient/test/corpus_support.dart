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

import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/models/form_config.dart';
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

/// **Moved here from `form_field_corpus_test.dart` on 2026-08-23**, when
/// `screen_config_corpus_test.dart` needed the same traversal. Two traversals
/// would be free to disagree, and the note below is the record of how expensive
/// getting this one right was.
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
///
/// **A fifth was not walked and should have been: `FormConfig.actions`.** It is
/// not a field container — its buttons render in the form's action bar rather
/// than in the layout — which is why this function is right to leave it out and
/// why nothing noticed that *nothing else* picked it up either. See
/// [formActionsOf] and the note on [formEnvelope]. Found 2026-08-25.
List<FormFieldConfig> allFields(FormConfig config) => [
  ...config.inputFields.expand((row) => row),
  ...config.inputFieldsV2.expand((row) => row.rowConfig),
  ...config.formTabsConfig.map((tab) => tab.inputField),
];

/// The buttons a form declares in its action bar, as opposed to among its
/// fields.
///
/// **These were in no corpus until 2026-08-25**, and the way they were missed is
/// worth more than the count. [allFields] walks the three *field* containers and
/// is correct to stop there; `actions` is a fourth container of a different kind,
/// and both form corpora built their output from [allFields] alone. So the
/// omission was not a bug in either place — it was a container nobody's traversal
/// owned.
///
/// The consequence was not uniform, which is what kept it invisible. Most forms
/// put every button in `actions` and reported none; `inferServerAdminForm` puts
/// seven among its fields and one — **Submit** — in `actions`, so it was the one
/// form whose buttons the corpus appeared to see. A reader checking "does the
/// corpus report buttons?" against that form got yes.
List<FormActionConfig> formActionsOf(FormConfig config) => config.actions;

/// The row structure of a `inputFieldsV2` form, which [allFields] flattens away.
///
/// **`FormFieldRowConfig.flex` is the entire reason a form chooses V2 over
/// `inputFields`** — `inferServerAdminForm`'s own doc comment says so: every row
/// of `inputFields` gets the same flex, which would make its button row as tall
/// as its text areas. [allFields] does `expand((row) => row.rowConfig)` and
/// discards the row, so the corpus reported that form's rows at the fields' own
/// flex of 1 while the request and response rows are 2 and 3.
///
/// Emitted beside `fields` rather than folded into them, because a field's flex
/// and its row's flex are different numbers and merging them would give the
/// React port one value where the Flutter widget reads two.
List<Map<String, dynamic>> rowFlexesOf(FormConfig config) => config
    .inputFieldsV2
    .map(
      (row) => <String, dynamic>{
        'flex': row.flex,
        'keys': row.rowConfig.map((f) => f.key).toList(),
      },
    )
    .toList();

/// The JSON envelope both form corpora emit for one form.
///
/// **Shared rather than written twice**, for the reason the table serialisers
/// below give: emitting one shape from two places is how two corpora of the same
/// thing stop being comparable. That it was *not* shared is part of how the
/// `actions` gap survived — each corpus built its own map, so fixing one would
/// not have prompted the other.
Map<String, dynamic> formEnvelope(FormConfig config, List<dynamic> fields) {
  final actions = formActionsOf(config);
  final rows = rowFlexesOf(config);
  return <String, dynamic>{
    'key': config.key,
    'title': config.title,
    'fieldCount': fields.length,
    // Counted separately from `fieldCount` on purpose. A button among the fields
    // and a button in the action bar are laid out by different widgets and are
    // two facts about a form, not one; a single total would answer "how many
    // buttons" and lose "how many does the layout have to place".
    'actionCount': actions.length,
    if (config.inputFieldRowBuilder != null) 'hasRowBuilder': true,
    if (config.onLoadActionKey != null) 'onLoadActionKey': config.onLoadActionKey,
    'fields': fields,
    'actions': actions.map(formActionToJson).toList(),
    if (rows.isNotEmpty) 'rows': rows,
  };
}

/// A `FormActionConfig`'s own properties.
///
/// **`fieldToJson` had no branch for this type**, so the seven buttons that did
/// reach a corpus — all of them `inferServerAdminForm`'s — arrived as bare
/// `type`/`key`/`group`/`flex` with their `capability`, their style and their
/// enablement dropped. Every one of those seven carries
/// `capability: "infer_server_admin"`, and the corpus said nothing about it: a
/// capability is exactly the kind of claim a port must not infer.
Map<String, dynamic> formActionToJson(FormActionConfig a) => <String, dynamic>{
  'key': a.key,
  'group': a.group,
  'flex': a.flex,
  'label': a.label,
  if (a.labelByStyle.isNotEmpty)
    'labelByStyle': a.labelByStyle.map((k, v) => MapEntry(k.name, v)),
  'buttonStyle': a.buttonStyle.name,
  if (a.enableOnlyWhenFormValid) 'enableOnlyWhenFormValid': true,
  if (a.enableOnlyWhenFormNotValid) 'enableOnlyWhenFormNotValid': true,
  if (a.capability != null) 'capability': a.capability,
  // A closure. See the library comment on the table serialisers below: any
  // `true` here is a place the React port needs an answer the configuration
  // cannot give it.
  'hasIsEnabledEval': a.isEnabledEval != null,
  if (a.leftMargin != 0.0) 'leftMargin': a.leftMargin,
  if (a.topMargin != 0.0) 'topMargin': a.topMargin,
  if (a.rightMargin != 0.0) 'rightMargin': a.rightMargin,
  if (a.bottomMargin != 0.0) 'bottomMargin': a.bottomMargin,
};

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
      if (f.returnedModelCacheKey != null)
        'returnedModelCacheKey': f.returnedModelCacheKey,
      if (f.stateKeyPredicates.isNotEmpty)
        'stateKeyPredicates': f.stateKeyPredicates,
      if (f.whereStateContains.isNotEmpty)
        'whereStateContains': f.whereStateContains,
      if (f.isReadOnly) 'isReadOnly': true,
      if (f.makeReadOnlyWhenHasSelectedValue)
        'makeReadOnlyWhenHasSelectedValue': true,
    });
  } else if (f is FormDataTableFieldConfig) {
    json['dataTableConfig'] = f.dataTableConfig;
  } else if (f is FormActionConfig) {
    // **There was no branch here until 2026-08-25**, so the ten buttons that do
    // sit among the fields — seven of `inferServerAdminForm`'s and three in the
    // flows — reached both corpora as bare `type`/`key`/`group`/`flex`, with
    // their capability, their style and their enablement dropped. `capability`
    // is the one that matters: it is not a property a port may infer.
    //
    // The same serialiser as the action bar's, because the Dart is one class —
    // `FormActionConfig` appears in `actions` and among the fields alike — and
    // `src/userflow/form.ts` already models it as one shape for that reason.
    final action = formActionToJson(f);
    json.addAll(<String, dynamic>{
      for (final e in action.entries)
        if (e.key != 'key' && e.key != 'group' && e.key != 'flex')
          e.key: e.value,
    });
  }
  return json;
}

/// The table-configuration serialisers, **moved here from
/// `table_config_corpus_test.dart` on 2026-08-23** when
/// `screen_config_corpus_test.dart` needed the same shapes. Emitting one shape
/// from two places is how two corpora of the same thing stop being comparable.
///
/// Closures are emitted as booleans naming their presence, which is the point
/// rather than a limitation: any `true` is a place the React port needs an
/// answer the configuration cannot give it.

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

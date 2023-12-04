import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Start Pipeline UF
String? startPipelineFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "startPipelineFormValidator has unexpected data type");

  switch (key) {
    case FSK.pipelineConfigKey:
    case FSK.mainInputRegistryKey:
    case FSK.client:
    case FSK.processName:
    case FSK.mergedInputRegistryKeys:
      if (v != null) return null;
      return "Select an option";

    case FSK.mainTableName:
    case FSK.description:
    case DTKeys.mainProcessInputTable:
    case DTKeys.mergeProcessInputTable:
    case DTKeys.injectedProcessInputTable:
    case FSK.mergedProcessInputKeys:
    case FSK.sourcePeriodKey:
    case DTKeys.spSummaryDataSources:
    case DTKeys.spInjectedProcessInput:
      return null;

    default:
      print(
          'Oops startPipelineFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

String mkStartPipelinePayload(
    Map<String, dynamic> state, String action, String table) {
  if (state[FSK.mergedInputRegistryKeys] == null) {
    state[FSK.mergedInputRegistryKeys] = '{}';
  } else {
    state[FSK.mergedInputRegistryKeys] =
        '{${(state[FSK.mergedInputRegistryKeys] as List<String>).join(',')}}';
  }
  if (state[FSK.pipelineConfigKey] is List<String>) {
    state[FSK.pipelineConfigKey] = state[FSK.pipelineConfigKey][0];
  }
  state[FSK.mainInputRegistryKey] = unpack(state[FSK.mainInputRegistryKey]);
  state[FSK.mainInputFileKey] = unpack(state[FSK.mainInputFileKey]);
  state[FSK.client] = unpack(state[FSK.client]);
  state[FSK.processName] = unpack(state[FSK.processName]);
  state[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
  state[FSK.sourcePeriodKey] = unpack(state[FSK.sourcePeriodKey]);

  state['status'] = StatusKeys.submitted;
  state['user_email'] = JetsRouterDelegate().user.email;
  state['session_id'] = "${DateTime.now().millisecondsSinceEpoch}";
  state[FSK.objectType] = state[FSK.mainObjectType];
  state[FSK.fileKey] = state[FSK.mainInputFileKey];

  return jsonEncode(<String, dynamic>{
    'action': action,
    'fromClauses': [
      <String, String>{'table': table}
    ],
    'workspaceName': state[FSK.wsName] ?? '',
    'data': [state],
  }, toEncodable: (_) => '');
}

/// Load Files UF Form Actions - set on UserFlowState
Future<String?> startPipelineFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  // print("=== Start Pipeline UF ActionKey: $actionKey");

  switch (actionKey) {
    // Pipeline Seclected, unwrap lists
    case ActionKeys.spPipelineSelected:
      state[FSK.mergedProcessInputKeys] =
          unpackToList(unpack(state[FSK.mergedProcessInputKeys]));
      state[FSK.injectedProcessInputKeys] =
          unpackToList(unpack(state[FSK.injectedProcessInputKeys]));
      break;

    // Prepare Start Pipeline
    case ActionKeys.spPrepareStartPipeline:
      final main = unpackToList(state[FSK.mainInputRegistryKey]) ?? [];
      final merged = unpackToList(state[FSK.mergedInputRegistryKeys]) ?? [];
      state[FSK.spAllDataSourceKeys] = [merged, main].expand((x) => x).toList();
      state[FSK.description] = unpack(state[FSK.description]);
      break;

    // Start Pipeline
    case ActionKeys.spStartPipelineUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final encodedJsonBody = mkStartPipelinePayload(
          state, 'insert_rows', 'pipeline_execution_status');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      break;

    // Test Pipeline
    case ActionKeys.spTestPipelineUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final encodedJsonBody =
          mkStartPipelinePayload(state, 'test_pipeline', 'unit_test');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
        Navigator.of(context).pop();
      }
      break;

    default:
      print(
          'Oops unknown ActionKey for Start Pipeline UF ActionKey: $actionKey');
  }
  // print("=== Form State: $state");
  return null;
}

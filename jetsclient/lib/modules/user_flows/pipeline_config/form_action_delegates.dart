import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Pipeline Config UF
String? pipelineConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "configureFilesFormValidator has unexpected data type");
  switch (key) {
    case FSK.pcAddOrEditPipelineConfigOption:
    case DTKeys.pcPipelineConfigTable:
    case FSK.mainProcessInputKey:
    case DTKeys.pcMergedProcessInputKeys:
    case DTKeys.pcInjectedProcessInputKeys:
    case FSK.client:
    case FSK.processName:
    case FSK.sourcePeriodType:
    case FSK.automated:
      if (v != null) {
        return null;
      }
      return "Please select an option.";

    case FSK.description:
      return null;

    default:
      print(
          'Oops configureFilesFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Pipeline Config UF Form Actions - set on UserFlowState
Future<String?> pipelineConfigFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);

  switch (actionKey) {
    case ActionKeys.pcAddPipelineConfigUF:
      // Initialize some state for the pipeline config
      formState.setValue(group, FSK.maxReteSessionSaved, 0);
      formState.setValue(group, FSK.mergedProcessInputKeys, <String?>[]);
      formState.setValue(group, FSK.injectedProcessInputKeys, <String?>[]);
      break;

    case ActionKeys.pcSelectPipelineConfigUF:
      state[FSK.automated] = unpack(state[FSK.automated]);
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.description] = unpack(state[FSK.description]);
      state[FSK.key] = unpack(state[FSK.key]);
      state[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
      state[FSK.mainProcessInputKey] = unpack(state[FSK.mainProcessInputKey]);
      state[FSK.mainSourceType] = unpack(state[FSK.mainSourceType]);
      state[FSK.maxReteSessionSaved] = unpack(state[FSK.maxReteSessionSaved]);
      state[FSK.processConfigKey] = unpack(state[FSK.processConfigKey]);
      state[FSK.processName] = unpack(state[FSK.processName]);
      state[FSK.ruleConfigJson] = unpack(state[FSK.ruleConfigJson]);
      state[FSK.sourcePeriodType] = unpack(state[FSK.sourcePeriodType]);
      return null;

    case ActionKeys.deletePipelineConfig:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected Pipeline Configuration?');
      if (uc != 'OK') return null;
      var state = formState.getState(0);
      state[FSK.key] = state[FSK.key][0];
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/pipeline_config'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        state.remove(FSK.key);
        if (statusCode == 200) return null;
        return "Error while deleting pipeline config";
      }
      break;

    case ActionKeys.pcGotToAddMergeProcessInputUF:
      // Got to State: add_merge_process_inputs - updating list of visited page
      final visitedPages = state[FSK.ufVisitedPages] as List<String>;
      const nextStateKey = 'add_merge_process_inputs';
      visitedPages.add(nextStateKey);
      // print("*** ActionKeys.pcGotToAddMergeProcessInputUF visitedPages is now: $visitedPages");
      state[FSK.ufCurrentPage] = visitedPages.length - 1;
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      return null;

    case ActionKeys.pcGotToAddInjectedProcessInputUF:
      // Got to State: add_merge_process_inputs - updating list of visited page
      final visitedPages = state[FSK.ufVisitedPages] as List<String>;
      const nextStateKey = 'add_injected_process_inputs';
      visitedPages.add(nextStateKey);
      // print("*** ActionKeys.pcGotToAddInjectedProcessInputUF visitedPages is now: $visitedPages");
      state[FSK.ufCurrentPage] = visitedPages.length - 1;
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      return null;

    case ActionKeys.pcAddMergeProcessInputUF:
      // Add selected Merge Process Input to FSK.mergedProcessInputKeys
      final key = unpack(state[DTKeys.pcMergedProcessInputKeys]);
      final mergedKeys = state[FSK.mergedProcessInputKeys] as List<String?>?;
      if (key != null && mergedKeys != null) {
        mergedKeys.add(key);
      } else {
        print('ERROR key ($key) or mergedKeys ($mergedKeys) is null');
      }

      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  return null;
}

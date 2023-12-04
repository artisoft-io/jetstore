import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/modules/actions/utils/get_process_info.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Pipeline Config UF
String? pipelineConfigFormValidatorUF(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "pipelineConfigFormValidator has unexpected data type");
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
    case DTKeys.pcMainProcessInputKey:
    case DTKeys.pcProcessInputRegistry4MI:
      if (v != null) {
        return null;
      }
      return "Please select an option.";

    case FSK.ruleConfigJson:
      if (v != null) {
        return null;
      }
      return "Please provide a value.";

    case FSK.description:
    case DTKeys.pcViewInjectedProcessInputKeys:
    case DTKeys.pcViewMergedProcessInputKeys:
    case DTKeys.pcSummaryProcessInputs:
      return null;

    default:
      print(
          'Oops pipelineConfigFormValidator Form Validator has no validator configured for form field $key');
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

  // print("=== pipelineConfigFormActionsUF DO ActionKey: $actionKey ===");
  switch (actionKey) {
    case ActionKeys.pcAddPipelineConfigUF:
      // Initialize some state for the pipeline config
      state[FSK.maxReteSessionSaved] = 0;
      state[FSK.mergedProcessInputKeys] = <String?>[];
      state[FSK.injectedProcessInputKeys] = <String?>[];
      // Get input_rdf_types and key as process_config_key from process_config table by process_name
      // returned value as entity_rdf_type
      final key = unpack(state[FSK.processName]);
      if (key == null) {
        print("Error: null process_name in formState");
        return "Error: null process_name in formState";
      }
      final processInfo =
          await getProcessInputRdfTypes(context, formState, key);
      if (processInfo == null) {
        return "No rows returned";
      }
      state[FSK.processConfigKey] = processInfo[FSK.processConfigKey];
      state[FSK.entityRdfType] = processInfo[FSK.entityRdfType];
      break;

    case ActionKeys.pcSelectPipelineConfigUF:
      state[FSK.automated] = unpack(state[FSK.automated]);
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.description] = unpack(state[FSK.description]);
      state[DTKeys.pcPipelineConfigTable] =
          unpack(state[DTKeys.pcPipelineConfigTable]);
      state[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
      state[FSK.mainProcessInputKey] = unpack(state[FSK.mainProcessInputKey]);
      state[FSK.mergedProcessInputKeys] =
          unpackToList(unpack(state[FSK.mergedProcessInputKeys]));
      state[FSK.injectedProcessInputKeys] =
          unpackToList(unpack(state[FSK.injectedProcessInputKeys]));
      state[FSK.mainSourceType] = unpack(state[FSK.mainSourceType]);
      state[FSK.maxReteSessionSaved] = unpack(state[FSK.maxReteSessionSaved]);
      state[FSK.processConfigKey] = unpack(state[FSK.processConfigKey]);
      state[FSK.processName] = unpack(state[FSK.processName]);
      state[FSK.ruleConfigJson] = unpack(state[FSK.ruleConfigJson]);
      state[FSK.sourcePeriodType] = unpack(state[FSK.sourcePeriodType]);

      // To have the data table rows selected
      state[DTKeys.pcMainProcessInputKey] =
          unpack(state[FSK.mainProcessInputKey]);
      break;

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

    case ActionKeys.pcRemoveMergedProcessInput:
      // Remove Process Input from Merge Process Input set
      final piKey = unpack(state[DTKeys.pcViewMergedProcessInputKeys]);
      // print("pcRemoveMergedProcessInput REMOVING $piKey");
      final l = state[FSK.mergedProcessInputKeys];
      if (l == null) {
        print("Error state does not have merged_process_input_keys");
        print("The Form State is $state");
        return "Error state does not have merged_process_input_keys";
      }
      l.remove(piKey);
      state[DTKeys.pcViewMergedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcViewMergedProcessInputKeys);
      formState.setValueAndNotify(group, FSK.mergedProcessInputKeys, l);
      break;

    case ActionKeys.pcRemoveInjectedProcessInput:
      // Remove Process Input from Injected Process Input set
      final piKey = unpack(state[DTKeys.pcViewInjectedProcessInputKeys]);
      // print("pcRemoveInjectedProcessInput REMOVING $piKey");
      final l = state[FSK.injectedProcessInputKeys];
      if (l == null) {
        print("Error state does not have injected_process_input_keys");
        print("The Form State is $state");
        return "Error state does not have injected_process_input_keys";
      }
      l.remove(piKey);
      state[DTKeys.pcViewInjectedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcViewInjectedProcessInputKeys);
      formState.setValueAndNotify(group, FSK.injectedProcessInputKeys, l);
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
      // Make sure the we don't have selected row from previous visit
      state[DTKeys.pcMergedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcMergedProcessInputKeys);
      break;

    case ActionKeys.pcGotToAddInjectedProcessInputUF:
      // Got to State: add_injected_process_inputs - updating list of visited page
      final visitedPages = state[FSK.ufVisitedPages] as List<String>;
      const nextStateKey = 'add_injected_process_inputs';
      visitedPages.add(nextStateKey);
      // print("*** ActionKeys.pcGotToAddInjectedProcessInputUF visitedPages is now: $visitedPages");
      state[FSK.ufCurrentPage] = visitedPages.length - 1;
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      // Make sure the we don't have selected row from previous visit
      state[DTKeys.pcInjectedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcInjectedProcessInputKeys);
      break;

    case ActionKeys.pcSelectMainProcessInputUF:
      // Set the selected Main Process Input to FSK.mainProcessInputKey
      state[FSK.mainProcessInputKey] =
          unpack(state[DTKeys.pcMainProcessInputKey]);
      break;

    case ActionKeys.pcAddMergeProcessInputUF:
      // Add selected Merge Process Input to FSK.mergedProcessInputKeys
      final key = unpack(state[DTKeys.pcMergedProcessInputKeys]);
      final mergedKeys = state[FSK.mergedProcessInputKeys] as List<String?>?;
      if (key != null && mergedKeys != null) {
        // Add if not present to avoid duplicated keys
        if (!mergedKeys.contains(key)) {
          mergedKeys.add(key);
        }
      } else {
        print('ERROR key ($key) or mergedKeys ($mergedKeys) is null');
      }
      // Remove the key so it's not selected again by default
      state[DTKeys.pcViewMergedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcViewMergedProcessInputKeys);
      break;

    case ActionKeys.pcAddInjectedProcessInputUF:
      // Add selected Injected Process Input to FSK.injectedProcessInputKeys
      final key = unpack(state[DTKeys.pcInjectedProcessInputKeys]);
      final injectedKeys =
          state[FSK.injectedProcessInputKeys] as List<String?>?;
      if (key != null && injectedKeys != null) {
        if (!injectedKeys.contains(key)) {
          injectedKeys.add(key);
        }
      } else {
        print('ERROR key ($key) or injectedKeys ($injectedKeys) is null');
      }
      // Remove the key so it's not selected again by default
      state[DTKeys.pcViewInjectedProcessInputKeys] = null;
      formState.clearSelectedRow(group, DTKeys.pcViewInjectedProcessInputKeys);
      break;

    // Prepare for the summary page
    case ActionKeys.pcPrepareSummaryUF:
      final main = unpackToList(state[FSK.mainProcessInputKey]);
      final merged = unpackToList(state[FSK.mergedProcessInputKeys]);
      final injected = unpackToList(state[FSK.injectedProcessInputKeys]);
      if (main == null || merged == null || injected == null) {
        print("### ufAllProcessInputKeys: $main + $merged + $injected");
        print("Error got a null list!");
        return "Unexpected error";
      }
      state[FSK.ufAllProcessInputKeys] =
          [injected, merged, main].expand((x) => x).toList();
      break;

    // Add/Update Pipeline Config
    case ActionKeys.pcSavePipelineConfigUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Get the Pipeline Config key (case update)
      // if no key is present then it's an add
      var updateState = <String, dynamic>{};
      var updateKey = unpack(state[DTKeys.pcPipelineConfigTable]);
      var query = 'pipeline_config'; // case add
      if (updateKey != null) {
        query = 'update/pipeline_config';
        updateState[FSK.key] = updateKey;
      }
      var processName = unpack(state[FSK.processName]);
      updateState[FSK.processName] = processName;
      updateState[FSK.processConfigKey] = unpack(state[FSK.processConfigKey]);
      updateState[FSK.client] = unpack(state[FSK.client]);
      updateState[FSK.maxReteSessionSaved] =
          unpack(state[FSK.maxReteSessionSaved]);
      updateState[FSK.ruleConfigJson] = unpack(state[FSK.ruleConfigJson]);
      updateState[FSK.sourcePeriodType] = unpack(state[FSK.sourcePeriodType]);
      updateState[FSK.mainProcessInputKey] =
          unpack(state[FSK.mainProcessInputKey]);
      updateState[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
      updateState[FSK.mainSourceType] = unpack(state[FSK.mainSourceType]);
      updateState[FSK.automated] = unpack(state[FSK.automated]);
      updateState[FSK.description] = unpack(state[FSK.description]);
      updateState[FSK.userEmail] = JetsRouterDelegate().user.email;

      // merged_process_input_keys and injected_process_input_keys:
      // They are as List<String?>, need to encode them as a string
      updateState[FSK.mergedProcessInputKeys] =
          makePgArray(state[FSK.mergedProcessInputKeys]);
      updateState[FSK.injectedProcessInputKeys] =
          makePgArray(state[FSK.injectedProcessInputKeys]);

      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [updateState],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        if (statusCode == 200) return null;
        if(statusCode == 409 && context.mounted) {
          showAlertDialog(context, "Record already exist, please verify.");
        }
        if(context.mounted) {
          showAlertDialog(context, "Server error, please try again.");
        }
        return "Error while saving pipeline config";
      }
      break;

    // Set the process_input_registry key
    case ActionKeys.pcSetProcessInputRegistryKey:
      // process_name || object_type || table_name || source_type AS key,
      var processName = unpack(state[FSK.processName]);
      var objectType = unpack(state[FSK.objectType]);
      var tableName = unpack(state[FSK.tableName]);
      var sourceType = unpack(state[FSK.sourceType]);
      if (processName == null ||
          objectType == null ||
          tableName == null ||
          sourceType == null) {
        // print(
        //     "Something is null: processName:$processName, objectType:$objectType, tableName:$tableName, sourceType:$sourceType");
        state.remove(DTKeys.pcProcessInputRegistry);
        state.remove(DTKeys.pcProcessInputRegistry4MI);
        return null;
      }
      state[DTKeys.pcProcessInputRegistry] =
          "$processName$objectType$tableName$sourceType";
      state[DTKeys.pcProcessInputRegistry4MI] =
          "$processName$objectType$tableName$sourceType";
      break;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  // print(
  //     "*** pipelineConfigFormActionsUF Action: $actionKey\nFromState: ${formState.getState(group)}");
  return null;
}

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/button_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';

/// Validation and Actions delegates for the source to pipeline config forms
/// Home Forms Validator
String? homeFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Home Form has unexpected data type");
  switch (key) {
    case FSK.client:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Client name must be selected.";
    case FSK.objectType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Object Type name must be selected.";
    case FSK.fileKey:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "File Key name must be selected.";

    // Start Pipeline Dialog
    case FSK.pipelineConfigKey:
      if (v != null) return null;
      return "Pipeline configuration row must be selected";
    case FSK.mainInputRegistryKey:
      if (v != null) return null;
      return "Main input source row must be selected";

    case FSK.mainTableName:
    case DTKeys.mainProcessInputTable:
    case DTKeys.mergeProcessInputTable:
    case DTKeys.injectedProcessInputTable:
    case FSK.mergedInputRegistryKeys:
    case FSK.mergedProcessInputKeys:
    case FSK.sourcePeriodKey:
      return null;

    default:
      print('Oops home form has no validator configured for form field $key');
  }
  return null;
}

/// Source Configuration Form Actions
Future<String?> resubmitPipeline(
    BuildContext context, JetsFormState formState) async {
  var state = formState.getState(0);
  state[FSK.sessionId] = unpack(state[FSK.sessionId]);
  // print('state contains $state');
  print('resubmiting: session_id is ${state[FSK.sessionId]}');
  // Send the pipeline resubmit insert
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'resubmit_pipeline',
    'data': [state],
  }, toEncodable: (_) => '');
  await postSimpleAction(
      context, formState, ServerEPs.dataTableEP, encodedJsonBody);
  return null;
}

Future<String?> homeFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Start Pipeline Dialogs
    case ActionKeys.startPipelineOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      if (state[FSK.mergedInputRegistryKeys] == null) {
        state[FSK.mergedInputRegistryKeys] = '{}';
      } else {
        state[FSK.mergedInputRegistryKeys] =
            '{${(state[FSK.mergedInputRegistryKeys] as List<String>).join(',')}}';
      }
      state[FSK.pipelineConfigKey] = unpack(state[FSK.pipelineConfigKey]);
      state[FSK.mainInputRegistryKey] = unpack(state[FSK.mainInputRegistryKey]);
      state[FSK.mainInputFileKey] = unpack(state[FSK.mainInputFileKey]);
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.processName] = unpack(state[FSK.processName]);
      state[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
      state[FSK.sourcePeriodKey] = unpack(state[FSK.sourcePeriodKey]);
      state[FSK.wsName] = unpack(state[FSK.wsName]);
      state['status'] = StatusKeys.submitted;
      state['user_email'] = JetsRouterDelegate().user.email;
      state['session_id'] = "${DateTime.now().millisecondsSinceEpoch}";
      state[FSK.objectType] = state[FSK.mainObjectType];
      state[FSK.fileKey] = state[FSK.mainInputFileKey];
      final action = state[FSK.dataTableAction];
      final table = state[FSK.dataTableFromTable];

      // Send the pipeline start insert
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': action,
        'fromClauses': [
          <String, String>{'table': table}
        ],
        'workspaceName': state[FSK.wsName] ?? '',
        'data': [state],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      return postInsertRows(context, formState, encodedJsonBody);

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    case ActionKeys.resubmitPipeline:
      return resubmitPipeline(context, formState);

    default:
      // Delegate to AppConfig ButtonConfig Actions
      return AppConfig()
          .buttonConfigActions(context, formKey, formState, actionKey);
  }
  return null;
}

/// Process Input and Mapping Form Actions
Future<String?> processInputFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Supporting Process Config UF as well as expert mode
    case ActionKeys.addProcessInputOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      return addProcessInput(context, formState);

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for process input form: $actionKey');
  }
  return null;
}

/// Rule Configv2 Form / Dialog Validator
String? ruleConfigv2FormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  // print(
  //     "ruleConfigv2 Validator Called for $group, $key, $v, state is ${formState.getValue(group, key)}");
  assert((v is String?) || (v is List<String>?),
      "Rule Configv2 Form has unexpected data type");
  switch (key) {
    // Rule Configv2 Dialog Validation
    case FSK.processName:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        return null;
      }
      return "Process name must be selected.";

    case FSK.client:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        return null;
      }
      return "Client must be selected.";

    case FSK.ruleConfigJson:
      String? value = v;
      if (value == null || value.isEmpty) {
        return "Rule config must contain at least an empty array";
      }
      // Check if it's a json array, if not return
      if (!value.startsWith('[')) return null;
      // Validate that value is valid json
      try {
        final jv = jsonDecode(value);
        if (jv is List) {
          return null;
        }
        return "Rule config must be a list of objects";
      } catch (e) {
        return "Rule config is not a valid json: ${e.toString()}";
      }

    default:
      print(
          'Oops rule configv2 form has no validator configured for form field $key');
  }
  return null;
}

/// Rule Configv2 Form Actions
Future<String?> ruleConfigv2FormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Rule Configv2 Dialog
    case ActionKeys.ruleConfigv2Ok:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Get the Rule Configv2 key (case update)
      // if no key is present then it's an add
      var updateState = <String, dynamic>{};
      var updateKey = formState.getValue(0, FSK.key);
      var query = 'rule_configv2'; // case add
      if (updateKey != null) {
        query = 'update/rule_configv2';
        updateState[FSK.key] = updateKey;
      }
      final processKey = formState.getValue(0, FSK.processConfigKey);
      updateState[FSK.processConfigKey] = processKey;

      final processName = formState.getValue(0, FSK.processName);
      updateState[FSK.processName] = processName;

      final client = formState.getValue(0, FSK.client);
      updateState[FSK.client] = client;

      updateState[FSK.ruleConfigJson] =
          formState.getValue(0, FSK.ruleConfigJson);

      // add process_config_key based on process_name
      final processConfigCache =
          formState.getCacheValue(FSK.processConfigCache) as List?;
      if (processConfigCache == null) {
        print("ruleConfigv2FormActions error: processConfigCache is null");
        return "ruleConfigv2FormActions error: processConfigCache is null";
      }
      final row = processConfigCache.firstWhere((e) => e[0] == processName);
      if (row == null) {
        print(
            "ruleConfigv2FormActions error: can't find process_name in ruleConfigv2Cache");
        return "ruleConfigv2FormActions error: can't find process_name in ruleConfigv2Cache";
      }
      updateState[FSK.processConfigKey] = row[1];
      updateState[FSK.userEmail] = JetsRouterDelegate().user.email;

      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [updateState],
      }, toEncodable: (_) => '');

      final result = await postRawAction(
          context, ServerEPs.dataTableEP, encodedJsonBody,
          successMessage: 'Rule Configuration successfully saved',
          failureMessage: 'Failed to save Rule Configuration');
      if (result.statusCode == 401) return "Not Authorized";
      if (result.statusCode == 200) {
        if (context.mounted) {
          Navigator.of(context).pop(DTActionResult.okDataTableDirty);
        }
      } else {
        // There was an error, just pop back to the page
        if (result.statusCode == 409) {
          formState.setValue(group, FSK.serverError,
              "A record already exist for $client on process $processName, please edit that record.");
        } else {
          formState.setValue(group, FSK.serverError, result.body['error']);
        }
        if (context.mounted) {
          Navigator.of(context).pop(DTActionResult.statusError);
        }
      }
      return null;

    case ActionKeys.deleteRuleConfigv2:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected Rule Configuration?');
      if (uc != 'OK') return null;
      var state = formState.getState(0);
      state[FSK.key] = state[FSK.key][0];
      state['user_email'] = JetsRouterDelegate().user.email;
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/rule_configv2'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        formState.clearSelectedRow(group, DTKeys.ruleConfigv2Table);
        state.remove(DTKeys.ruleConfigv2Table);
        await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      }
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop(DTActionResult.canceled);
      break;
    default:
      print('Oops unknown ActionKey for rule configv2 form: $actionKey');
  }
  return null;
}

/// Pipeline Config Form / Dialog Validator
String? pipelineConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  print(
      "Validator Called for $group, $key, $v, state is ${formState.getValue(group, key)}");
  assert((v is String?) || (v is List<String>?),
      "Pipeline Config Form has unexpected data type");
  switch (key) {
    // Pipeline Config Dialog Validation
    case FSK.processName:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        return null;
      }
      return "Process name must be provided.";

    case FSK.client:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        return null;
      }
      return "Client must be provided.";

    case FSK.mainProcessInputKey:
      // Somehow v still have old value when client drop down is nullified
      final vv = formState.getValue(group, key);
      if (vv != null && vv.isNotEmpty) {
        return null;
      }
      return "Main process input must be selected.";

    case FSK.automated:
      if (v != null && v.isNotEmpty) {
        return null;
      }
      return "Pipeline automation status must be selected.";

    case FSK.maxReteSessionSaved:
      // print("maxReteSessionSaved v is $v");
      if (v != null && v.isNotEmpty) {
        return null;
      }
      return "Max number of rete sessions saved must be provided.";

    case FSK.sourcePeriodType:
      if (v != null && v.isNotEmpty) {
        return null;
      }
      return "Execution frequency must be selected.";

    case FSK.ruleConfigJson:
      String? value = v;
      if (value == null || value.isEmpty) {
        return "Rule config must contain at least an empty array";
      }
      // Check if it's a json array, if not return
      if (!value.startsWith('[')) return null;
      // Validate that value is valid json
      try {
        final jv = jsonDecode(value);
        if (jv is List) {
          return null;
        }
        return "Rule config must be a list of objects";
      } catch (e) {
        return "Rule config is not a valid json: ${e.toString()}";
      }

    case FSK.description:
    case FSK.mergedProcessInputKeys:
    case FSK.injectedProcessInputKeys:
      return null;

    default:
      print(
          'Oops pipeline config form has no validator configured for form field $key');
  }
  return null;
}

/// Pipeline Config Form Actions
Future<String?> pipelineConfigFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Pipeline Config Dialog
    case ActionKeys.pipelineConfigOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Get the Pipeline Config key (case update)
      // if no key is present then it's an add
      var updateState = <String, dynamic>{};
      var updateKey = formState.getValue(0, FSK.key);
      var query = 'pipeline_config'; // case add
      if (updateKey != null) {
        query = 'update/pipeline_config';
        updateState[FSK.key] = updateKey;
      }
      var processName = formState.getValue(0, FSK.processName);
      updateState[FSK.processName] = processName;
      updateState[FSK.client] = formState.getValue(0, FSK.client);
      updateState[FSK.maxReteSessionSaved] =
          formState.getValue(0, FSK.maxReteSessionSaved);
      updateState[FSK.ruleConfigJson] =
          formState.getValue(0, FSK.ruleConfigJson);
      updateState[FSK.sourcePeriodType] =
          formState.getValue(0, FSK.sourcePeriodType);

      // add process_config_key based on process_name
      var processConfigCache =
          formState.getCacheValue(FSK.processConfigCache) as List?;
      if (processConfigCache == null) {
        print("pipelineConfigFormActions error: processConfigCache is null");
        return "pipelineConfigFormActions error: processConfigCache is null";
      }
      var row = processConfigCache.firstWhere((e) => e[0] == processName);
      if (row == null) {
        print(
            "pipelineConfigFormActions error: can't find process_name in processConfigCache");
        return "pipelineConfigFormActions error: can't find process_name in processConfigCache";
      }
      updateState[FSK.processConfigKey] = row[1];

      // mainProcessInputKey, mainObjectType, and mainSourceType are either
      // pre-populated as String from the data table action
      // from the selected row to update or is a List<String?> if user have selected
      // a row from the data table
      updateState[FSK.mainProcessInputKey] =
          unpack(state[FSK.mainProcessInputKey]);
      if (updateState[FSK.mainProcessInputKey] == null) {
        print("UNEXPECTED null for mainProcessInputKey\nForm State is $state");
        updateState[FSK.mainProcessInputKey] = const [];
      }
      updateState[FSK.mainObjectType] = unpack(state[FSK.mainObjectType]);
      if (updateState[FSK.mainObjectType] == null) {
        print("UNEXPECTED null for mainObjectType\nForm State is $state");
        return 'Unexpeced null for mainObjectType';
      }

      var mainSourceType = formState.getValue(0, FSK.mainSourceType);
      assert(mainSourceType != null, "unexpected null value");
      if (mainSourceType is List<String?>) {
        updateState[FSK.mainSourceType] = mainSourceType[0];
      } else {
        updateState[FSK.mainSourceType] = mainSourceType;
      }
      // same pattern for merged_process_input_keys
      var mergedProcessInputKeys =
          formState.getValue(0, FSK.mergedProcessInputKeys);
      updateState[FSK.mergedProcessInputKeys] =
          makePgArray(mergedProcessInputKeys);
      // same pattern for injected_process_input_keys
      var injectedProcessInputKeys =
          formState.getValue(0, FSK.injectedProcessInputKeys);
      updateState[FSK.injectedProcessInputKeys] =
          makePgArray(injectedProcessInputKeys);
      updateState[FSK.automated] = formState.getValue(0, FSK.automated);
      updateState[FSK.description] = formState.getValue(0, FSK.description);
      updateState[FSK.userEmail] = JetsRouterDelegate().user.email;

      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [updateState],
      }, toEncodable: (_) => '');
      // return postInsertRows(context, formState, encodedJsonBody);
      final res = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (res == 401) return "Not Authorized";
      if (res == 200) {
        if (context.mounted) {
          Navigator.of(context).pop();
        }
        // JetsRouterDelegate()(
        //     JetsRouteData(pipelineConfigPath, params: {'x': 'x'}));
      } else {
        // There was an error, just pop back to the page
        if (context.mounted) {
          Navigator.of(context).pop();
        }
      }
      return null;
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      // JetsRouterDelegate()(
      //     JetsRouteData(pipelineConfigPath, params: {'x': 'x'}));
      break;
    default:
      print('Oops unknown ActionKey for pipeline config form: $actionKey');
  }
  return null;
}

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:provider/provider.dart';

/// postInsertRows - main function to post for inserting rows into db
Future<String?> postInsertRows(BuildContext context, JetsFormState formState,
    String encodedJsonBody) async {
  var navigator = Navigator.of(context);
  var messenger = ScaffoldMessenger.of(context);
  var result = await context.read<HttpClient>().sendRequest(
      path: ServerEPs.dataTableEP,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: encodedJsonBody);

  // print("Got reply with status code ${result.statusCode}");
  if (result.statusCode == 200) {
    // Inform the user and transition
    const snackBar = SnackBar(content: Text('Record(s) successfully inserted'));
    messenger.showSnackBar(snackBar);
    // All good, let's the table know to refresh
    navigator.pop(DTActionResult.okDataTableDirty);
    return null;
  } else if (result.statusCode == 400 ||
      result.statusCode == 406 ||
      result.statusCode == 422) {
    // http Bad Request / Not Acceptable / Unprocessable
    formState.setValue(
        0, FSK.serverError, "Something went wrong. Please try again.");
    navigator.pop(DTActionResult.statusError);
    return "Something went wrong. Please try again.";
  } else if (result.statusCode == 409) {
    // http Conflict
    const snackBar = SnackBar(
      content: Text("Duplicate Record."),
    );
    messenger.showSnackBar(snackBar);
    formState.setValue(0, FSK.serverError, "Duplicate record. Please verify.");
    navigator.pop();
    return "Duplicate record. Please verify.";
  } else {
    formState.setValue(
        0, FSK.serverError, "Got a server error. Please try again.");
    navigator.pop(DTActionResult.statusError);
    return "Got a server error. Please try again.";
  }
}

/// postSimpleAction - post action that does not require navigation
void postSimpleAction(BuildContext context, JetsFormState formState,
    String serverEndPoint, String encodedJsonBody) async {
  var messenger = ScaffoldMessenger.of(context);
  var result = await context.read<HttpClient>().sendRequest(
      path: serverEndPoint,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: encodedJsonBody);

  // print("Got reply with status code ${result.statusCode}");
  if (result.statusCode == 200) {
    // Inform the user and transition
    const snackBar = SnackBar(content: Text('Request successfully completed'));
    messenger.showSnackBar(snackBar);
    formState.invokeCallbacks();
  } else {
    showAlertDialog(context, "Something went wrong. Please try again.");
  }
}

String makeTableName(Map<String, dynamic> state) {
  if (state['org'].toString().isNotEmpty) {
    return state[FSK.client] +
        '_' +
        state[FSK.org] +
        '_' +
        state[FSK.objectType];
  } else {
    return state[FSK.client] + '_' + state[FSK.objectType];
  }
}

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

    case DTKeys.fileKeyStagingForPipelineMainProcessInput:
      if (v != null) return null;
      return "File Key row must be selected";

    // case DTKeys.fileKeyStagingForPipelineMergeProcessInput:
    //   return null;

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
Future<String?> homeFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Start Pipeline Dialogs
    // Load & Start Pipeline Dialogs
    case ActionKeys.startPipelineOk:
    case ActionKeys.loadAndStartPipelineOk:
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
      state[FSK.pipelineConfigKey] = state[FSK.pipelineConfigKey][0];
      var w = state[FSK.mainInputRegistryKey];
      if (w != null) state[FSK.mainInputRegistryKey] = w[0];
      w = state[FSK.mainInputFileKey];
      if (w != null) state[FSK.mainInputFileKey] = w[0];
      state[FSK.client] = state[FSK.client][0];
      state[FSK.processName] = state[FSK.processName][0];
      state[FSK.mainObjectType] = state[FSK.mainObjectType][0];
      state['status'] = StatusKeys.submitted;
      state['user_email'] = JetsRouterDelegate().user.email;
      state['session_id'] = "${DateTime.now().millisecondsSinceEpoch}";
      state[FSK.objectType] = state[FSK.mainObjectType];
      state[FSK.fileKey] = state[FSK.mainInputFileKey];
      state[FSK.sourcePeriodKey] = state[FSK.sourcePeriodKey][0];
      if (actionKey == ActionKeys.loadAndStartPipelineOk) {
        state[FSK.org] = state[FSK.org][0];
        state['load_and_start'] = 'true';
        state['input_session_id'] = state['session_id'];
        state['table_name'] = makeTableName(state);
      } else {
        state['load_and_start'] = 'false';
      }
      var navigator = Navigator.of(context);
      if (actionKey == ActionKeys.loadAndStartPipelineOk) {
        // Send the load insert
        var encodedJsonBody = jsonEncode(<String, dynamic>{
          'action': 'insert_rows',
          'fromClauses': [
            <String, String>{'table': 'input_loader_status'}
          ],
          'data': [state],
        }, toEncodable: (_) => '');
        var loadResult = await context.read<HttpClient>().sendRequest(
            path: ServerEPs.dataTableEP,
            token: JetsRouterDelegate().user.token,
            encodedJsonBody: encodedJsonBody);
        if (loadResult.statusCode != 200) {
          formState.setValue(
              0, FSK.serverError, "Something went wrong. Please try again.");
          navigator.pop(DTActionResult.statusError);
          return "Something went wrong. Please try again.";
        }
        formState.parentFormState
            ?.setValueAndNotify(group, FSK.key, state['session_id']);
      }
      // Send the pipeline start insert
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'pipeline_execution_status'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for home form: $actionKey');
  }
}

/// Validation and Actions delegates for the Source Config forms
/// Validation and Actions delegates for the Client & Org admin forms
String? sourceConfigValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Source Config Form has unexpected data type");
  switch (key) {
    case FSK.client:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Client name is too short.";
      }
      return "Client name must be selected.";
    case FSK.org:
      String? value = v;
      if (value != null) {
        return null;
      }
      return "Organization name must be selected.";
    case FSK.details:
      // optional
      return null;

    case FSK.objectType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Object Type name must be selected.";
    case FSK.domainKeysJson:
      String? value = v;
      if (value == null || value.isEmpty) {
        return null; // this field is nullable
      }
      // Validate that value is valid json
      try {
        jsonDecode(value);
      } catch (e) {
        return "Domain keys is not a valid json: ${e.toString()}";
      }
      return null;

    case FSK.sourcePeriodKey:
      if (v != null) {
        return null;
      }
      return "Execution frequency choice must be selected.";

    case FSK.automated:
      if (v != null) {
        return null;
      }
      return "Automation choice must be selected.";

    default:
      print(
          'Oops Source Config Form has no validator configured for form field $key');
  }
  return null;
}

/// Source Configuration Form Actions
/// Cient and Org Admin Form Actions
Future<String?> sourceConfigActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Add Client
    case ActionKeys.clientOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'client_registry'}
        ],
        'data': [formState.getState(0)],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    // Add Org
    case ActionKeys.orgOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'client_org_registry'}
        ],
        'data': [formState.getState(0)],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    case ActionKeys.deleteClient:
      // Get confirmation
      var uc = await showConfirmationDialog(
          context, 'Are you sure you want to delete the selected client?');
      if (uc != 'OK') return null;
      var state = formState.getState(0);
      state[FSK.client] = state[FSK.client][0];
      state['user_email'] = JetsRouterDelegate().user.email;
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/client'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      break;

    case ActionKeys.deleteOrg:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected organization?');
      if (uc != 'OK') return null;
      var state = formState.getState(0);
      state[FSK.client] = state[FSK.client][0];
      state[FSK.org] = state[FSK.org][0];
      state['user_email'] = JetsRouterDelegate().user.email;
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/org'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      break;

    // Add/Update Source Config
    case ActionKeys.addSourceConfigOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      var state = formState.getState(0);
      // print('Add Source Config state: $state');
      var query = 'source_config'; // case add
      if (formState.getValue(0, FSK.key) != null) {
        query = 'update/source_config';
      }

      state['user_email'] = JetsRouterDelegate().user.email;
      state['table_name'] = makeTableName(state);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    // Start loader
    case ActionKeys.loaderOk:
      // No form validation since does not use widgets
      // var valid = formKey.currentState!.validate();
      // if (!valid) {
      //   return;
      // }
      var state = formState.getState(0);
      // Fields comming from table selected row will be in array, unpack the value
      state[FSK.client] = state[FSK.client][0];
      state[FSK.org] = state[FSK.org][0];
      state[FSK.objectType] = state[FSK.objectType][0];
      state[FSK.tableName] = state[FSK.tableName][0];
      state[FSK.fileKey] = state[FSK.fileKey][0];
      state[FSK.sourcePeriodKey] = state[FSK.sourcePeriodKey][0];
      state['status'] = StatusKeys.submitted;
      state['user_email'] = JetsRouterDelegate().user.email;
      state['session_id'] = "${DateTime.now().millisecondsSinceEpoch}";
      state['load_and_start'] = 'false';
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'input_loader_status'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    // Sync File Keys with web storage (s3)
    case ActionKeys.syncFileKey:
      // No form validation since does not use widgets
      // var valid = formKey.currentState!.validate();
      // if (!valid) {
      //   return;
      // }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'sync_file_keys',
        'table': '',
        'data': [],
      }, toEncodable: (_) => '');
      postSimpleAction(
          context, formState, ServerEPs.registerFileKeyEP, encodedJsonBody);
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Source Config Form: $actionKey');
  }
}

/// Process Input Form / Dialog Validator
String? processInputFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Process Input Form has unexpected data type");
  var isRequired = formState.getValue(group, FSK.isRequiredFlag);
  // print(
  //     "Validator Called for $group ($isRequired), $key, $v, state is ${formState.getValue(group, key)}");
  // Check if we have client, object_type, and source_type to populate table_name
  // add entity_rdf_type based on object_type
  var objectTypeRegistry =
      formState.getCacheValue(FSK.objectTypeRegistryCache) as List?;
  var objectType = formState.getValue(0, FSK.objectType);
  var client = formState.getValue(0, FSK.client);
  var org = formState.getValue(0, FSK.org);
  var sourceType = formState.getValue(0, FSK.sourceType);
  // print("GOT $client, $objectType, $sourceType");

  if (objectTypeRegistry != null &&
      objectType != null &&
      client != null &&
      org != null &&
      sourceType != null) {
    var row = objectTypeRegistry.firstWhere((e) => e[0] == objectType);
    if (row == null) {
      print(
          "processInputFormActions error: can't find object_type in objectTypeRegistry");
    } else {
      formState.setValue(0, FSK.entityRdfType, row[1]);
      // add table_name to form state based on source_type
      String tableName = makeTableName(formState.getState(0));
      if (formState.getValue(0, FSK.tableName) != tableName) {
        // print("SET AND NOTIFY TABLENAME $tableName");
        formState.setValueAndNotify(0, FSK.tableName, tableName);
      }
    }
  }

  switch (key) {
    // Add/Update Process Input Dialog Validations
    case FSK.client:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Client name must be provided.";
    case FSK.org:
      if (v != null) {
        return null;
      }
      return "Organization must be selected.";

    case FSK.objectType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Object Type name must be selected.";
    case FSK.sourceType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Source Type name must be selected.";
    case FSK.lookbackPeriods:
      String? value = v;
      if (value != null && value.characters.isNotEmpty) {
        return null;
      }
      return "Lookback period must be provided.";

    // Process Mapping Dialog Validation
    case FSK.inputColumn:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (isRequired == null || isRequired == false) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var defaultValue =
          formState.getValue(group, FSK.mappingDefaultValue) as String?;
      if (defaultValue != null && defaultValue.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var errorMsg =
          formState.getValue(group, FSK.mappingErrorMessage) as String?;
      if (errorMsg != null && errorMsg.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Input Column must be selected or either a default or an error message must be provided.";

    case FSK.functionName:
      return null;

    case FSK.functionArgument:
      String? value = v;
      var functionName = formState.getValue(group, FSK.functionName) as String?;
      // print("Validating argument '$value' for function $functionName");
      if (functionName == null || functionName.isEmpty) {
        if (value != null && value.isNotEmpty) {
          formState.markFormKeyAsInvalid(group, key);
          return "Remove the argument when no function is selected";
        } else {
          formState.markFormKeyAsValid(group, key);
          return null;
        }
      }
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var mappingFunctionDetails =
          formState.getCacheValue(FSK.mappingFunctionDetailsCache) as List?;
      assert(mappingFunctionDetails != null,
          "processInputFormActions error: mappingFunctionDetails is null");
      if (mappingFunctionDetails == null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var row = mappingFunctionDetails.firstWhere(
        (e) => e[0] == functionName,
      );
      // check if function argument is required
      if (row[1] != "1") {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Cleansing function argument is required";

    case FSK.mappingDefaultValue:
      String? value = v;
      if (value != null && value.isEmpty) {
        value = null;
      }
      var errorMsg =
          formState.getValue(group, FSK.mappingErrorMessage) as String?;
      if (errorMsg != null && errorMsg.isEmpty) {
        errorMsg = null;
      }
      if (value != null && errorMsg == null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (value == null && errorMsg != null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (value != null && errorMsg != null) {
        formState.markFormKeyAsInvalid(group, key);
        return "Cannot specify both a default value and an error message";
      }
      formState.markFormKeyAsValid(group, key);
      return null;

    case FSK.mappingErrorMessage:
      return null;
    default:
      print(
          'Oops process input form has no validator configured for form field $key');
  }
  return null;
}

/// Process and Rules Config Form / Dialog Validator
String? processConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Process Config Form has unexpected data type");
  // print(
  //     "Validator Called for $group, $key, $v, state is ${formState.getValue(group, key)}");
  switch (key) {
    // Rule Config Dialog Validation
    case FSK.subject:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Rule Config Subject must be provided.";

    case FSK.predicate:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Rule Config Predicate must be provided.";

    case FSK.object:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Rule Config Object must be provided.";

    case FSK.rdfType:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Rule Config Object rdf type must be provided.";

    default:
      print(
          'Oops process / rules config form has no validator configured for form field $key');
  }
  return null;
}

/// Process Input and Mapping Form Actions
Future<String?> processInputFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    case ActionKeys.addProcessInputOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }

      formState.setValue(0, FSK.userEmail, JetsRouterDelegate().user.email);
      var query = 'process_input'; // case add
      if (formState.getValue(0, FSK.key) != null) {
        query = 'update2/process_input';
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [formState.getState(0)],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    // Process Mapping Dialog
    case ActionKeys.mapperOk:
    case ActionKeys.mapperDraft:
      var processInputStatus = 'saved as draft';
      if (actionKey == ActionKeys.mapperOk) {
        processInputStatus = 'configured';
        if (!formState.isFormValid()) {
          return null;
        }
      }
      // Insert rows to process_mapping, if successful update process_input.status
      var tableName = formState.getValue(0, FSK.tableName);
      var processInputKey = formState.getValue(0, FSK.processInputKey);
      if (tableName == null || processInputKey == null) {
        print(
            "processInputFormActions error: save mapping - table_name or process_input.key is null");
        return "processInputFormActions error: save mapping - table_name or process_input.key is null";
      }
      for (var i = 0; i < formState.groupCount; i++) {
        formState.setValue(i, FSK.tableName, tableName);
        formState.setValue(i, FSK.userEmail, JetsRouterDelegate().user.email);
      }
      var navigator = Navigator.of(context);
      // first issue a delete statement to make sure we replace all mappings
      var deleteJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/process_mapping'}
        ],
        'data': [
          {
            FSK.tableName: tableName,
            FSK.userEmail: JetsRouterDelegate().user.email,
          }
        ],
      }, toEncodable: (_) => '');
      var deleteResult = await context.read<HttpClient>().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: deleteJsonBody);

      if (deleteResult.statusCode != 200) {
        formState.setValue(
            0, FSK.serverError, "Something went wrong. Please try again.");
        navigator.pop(DTActionResult.statusError);
        return "Something went wrong. Please try again.";
      }
      // delete successfull, update process_mapping
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'process_mapping'}
        ],
        'data': formState.getInternalState(),
      }, toEncodable: (_) => '');
      // Insert rows to process_mapping
      var result = await context.read<HttpClient>().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: encodedJsonBody);

      if (result.statusCode == 200) {
        // insert successfull, update process_input status
        var encodedJsonBody = jsonEncode(<String, dynamic>{
          'action': 'insert_rows',
          'fromClauses': [
            <String, String>{'table': 'update/process_input'}
          ],
          'data': [
            {
              'key': processInputKey,
              'user_email': JetsRouterDelegate().user.email,
              'status': processInputStatus
            }
          ],
        }, toEncodable: (_) => '');
        postInsertRows(context, formState, encodedJsonBody);
        // trigger a refresh of the process_mapping table
        formState.parentFormState?.setValue(0, FSK.tableName, null);
        formState.parentFormState
            ?.setValueAndNotify(0, FSK.tableName, tableName);
      } else if (result.statusCode == 400 ||
          result.statusCode == 406 ||
          result.statusCode == 422) {
        // http Bad Request / Not Acceptable / Unprocessable
        formState.setValue(
            0, FSK.serverError, "Something went wrong. Please try again.");
        navigator.pop(DTActionResult.statusError);
      } else {
        formState.setValue(
            0, FSK.serverError, "Got a server error. Please try again.");
        navigator.pop(DTActionResult.statusError);
      }
      break;
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for process input form: $actionKey');
  }
}

/// Process and Rules Config Form Actions
Future<String?> processConfigFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {required int group}) async {
  switch (actionKey) {
    // Rule Config Dialog
    case ActionKeys.ruleConfigOk:
      if (!formState.isFormValid()) {
        return null;
      }
      // Insert rows to rule_config table
      var processConfigKey = formState.getValue(0, FSK.processConfigKey);
      var processName = formState.getValue(0, FSK.processName);
      var client = formState.getValue(0, FSK.client);
      if (processName == null || client == null) {
        print("processConfigFormActions error: save rule config");
        return null;
      }
      for (var i = 0; i < formState.groupCount; i++) {
        formState.setValue(i, FSK.client, client);
        formState.setValue(i, FSK.processConfigKey, processConfigKey);
        formState.setValue(i, FSK.processName, processName);
        formState.setValue(i, FSK.userEmail, JetsRouterDelegate().user.email);
      }
      var stateList = formState.getInternalState();

      var deleteJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/rule_config'}
        ],
        'data': [
          {
            FSK.client: client,
            FSK.processConfigKey: processConfigKey,
            FSK.processName: processName,
            FSK.userEmail: JetsRouterDelegate().user.email,
          }
        ],
      }, toEncodable: (_) => '');
      var navigator = Navigator.of(context);
      // First delete existing rule config triples
      var deleteResult = await context.read<HttpClient>().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: deleteJsonBody);

      if (deleteResult.statusCode == 200) {
        // now insert the new triples
        var insertJsonBody = jsonEncode(<String, dynamic>{
          'action': 'insert_rows',
          'fromClauses': [
            <String, String>{'table': 'rule_config'}
          ],
          'data': stateList.getRange(0, stateList.length - 1).toList(),
        }, toEncodable: (_) => '');
        String? err = await postInsertRows(context, formState, insertJsonBody);
        // insert successfull
        // trigger a refresh of the rule_config table
        formState.parentFormState?.setValue(0, FSK.processName, null);
        formState.parentFormState
            ?.setValueAndNotify(0, FSK.processName, processName);
        return err;
      } else if (deleteResult.statusCode == 400 ||
          deleteResult.statusCode == 406 ||
          deleteResult.statusCode == 422) {
        // http Bad Request / Not Acceptable / Unprocessable
        formState.setValue(
            0, FSK.serverError, "Something went wrong. Please try again.");
        navigator.pop(DTActionResult.statusError);
        return "Something went wrong. Please try again.";
      } else {
        formState.setValue(
            0, FSK.serverError, "Got a server error. Please try again.");
        navigator.pop(DTActionResult.statusError);
        return "Something went wrong. Please try again.";
      }
      break;

    // delete rule config triple
    case ActionKeys.ruleConfigDelete:
      var style = formState.getValue(group, ActionKeys.ruleConfigDelete);
      assert(style is ActionStyle?,
          'error: invalid formState value for ActionKeys.ruleConfigDelete');
      if (style == ActionStyle.alternate) {
        formState.setValueAndNotify(
            group, ActionKeys.ruleConfigDelete, ActionStyle.danger);
      } else {
        var altInputFields =
            formState.activeFormWidgetState?.alternateInputFields;
        assert(altInputFields != null);
        if (altInputFields == null) return null;
        altInputFields.removeAt(group);
        for (var i = group; i < altInputFields.length; i++) {
          for (var j = 0; j < altInputFields[i].length; j++) {
            altInputFields[i][j].group = i;
          }
        }
        //* PROBLEM - Need to stop using group 0 as a special group with validation keys
        //  since removing group 0 creates a problem
        if (group == 0) {
          // Need to carry over context keys
          var processConfigKey = formState.getValue(0, FSK.processConfigKey);
          var processName = formState.getValue(0, FSK.processName);
          var client = formState.getValue(0, FSK.client);
          formState.setValue(1, FSK.client, client);
          formState.setValue(1, FSK.processConfigKey, processConfigKey);
          formState.setValue(1, FSK.processName, processName);
          formState.setValue(1, FSK.userEmail, JetsRouterDelegate().user.email);
        }
        formState.removeValidationGroup(group);
        formState.activeFormWidgetState?.markAsDirty();
        print("OK row with index $group should be deleted");
      }
      break;

    case ActionKeys.ruleConfigAdd:
      var index = formState.groupCount - 1;
      formState.resizeFormState(formState.groupCount + 1);
      formState.activeFormWidgetState?.alternateInputFields.insert(index, [
        FormInputFieldConfig(
            key: FSK.subject,
            label: 'Subject',
            hint: 'Rule config subject',
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            textRestriction: TextRestriction.none,
            maxLength: 512),
        FormInputFieldConfig(
            key: FSK.predicate,
            label: 'Predicate',
            hint: 'Rule config predicate',
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            textRestriction: TextRestriction.none,
            maxLength: 512),
        FormInputFieldConfig(
            key: FSK.object,
            label: 'Object',
            hint: 'Rule config object',
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            textRestriction: TextRestriction.none,
            maxLength: 512),
        FormDropdownFieldConfig(
            key: FSK.rdfType,
            group: index,
            flex: 1,
            autovalidateMode: AutovalidateMode.always,
            items: FormDropdownFieldConfig.rdfDropdownItems,
            defaultItemPos: 0),
        FormActionConfig(
            key: ActionKeys.ruleConfigDelete,
            group: index,
            flex: 1,
            label: '',
            labelByStyle: {
              ActionStyle.alternate: 'Delete',
              ActionStyle.danger: 'Confirm',
            },
            buttonStyle: ActionStyle.alternate,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
      ]);
      formState.activeFormWidgetState?.markAsDirty();
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    default:
      print(
          'Oops unknown ActionKey for process and rules config form: $actionKey');
  }
}

/// Pipeline Config Form / Dialog Validator
String? pipelineConfigFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  // print(
  //     "Validator Called for $group, $key, $v, state is ${formState.getValue(group, key)}");
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
      if (v != null && v.isNotEmpty) {
        return null;
      }
      return "Main process input must be selected.";

    case FSK.automated:
      if (v != null && v.isNotEmpty) {
        return null;
      }
      return "Pipeline automation status must be selected.";

    case FSK.description:
    case FSK.mergedProcessInputKeys:
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
      var mainProcessInputKey = formState.getValue(0, FSK.mainProcessInputKey);
      assert(mainProcessInputKey != null, "unexpected null value");
      if (mainProcessInputKey is List<String?>) {
        updateState[FSK.mainProcessInputKey] = mainProcessInputKey[0];
      } else {
        updateState[FSK.mainProcessInputKey] = mainProcessInputKey;
      }
      var mainObjectType = formState.getValue(0, FSK.mainObjectType);
      assert(mainObjectType != null, "unexpected null value");
      if (mainObjectType is List<String?>) {
        updateState[FSK.mainObjectType] = mainObjectType[0];
      } else {
        updateState[FSK.mainObjectType] = mainObjectType;
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
      if (mergedProcessInputKeys != null) {
        if (mergedProcessInputKeys is List<String>) {
          final buf = StringBuffer();
          buf.write("{");
          buf.writeAll(mergedProcessInputKeys, ",");
          buf.write("}");
          updateState[FSK.mergedProcessInputKeys] = buf.toString();
        } else {
          updateState[FSK.mergedProcessInputKeys] = mergedProcessInputKeys;
        }
      } else {
        updateState[FSK.mergedProcessInputKeys] = '{}';
      }
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
      return postInsertRows(context, formState, encodedJsonBody);
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for pipeline config form: $actionKey');
  }
}

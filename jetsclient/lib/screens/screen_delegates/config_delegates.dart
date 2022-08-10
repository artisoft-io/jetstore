import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:provider/provider.dart';

/// Validation and Actions delegates for the source to pipeline config forms
/// Login Form Validator
String? homeFormValidator(BuildContext context, JetsFormState formState,
    int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "sourceConfig Form has unexpected data type");
  switch (key) {
    case FSK.client:
      String? value = v;
      print('Validating client $value');
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length == 1) {
        return "Client name is too short.";
      }
      return "Client name must be provided.";
    case FSK.objectType:
      String? value = v;
      print('Validating object type $value');
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
    case FSK.details:
      // always good
      return null;
    default:
      print('Oops login form has no validator configured for form field $key');
  }
  return null;
}

void postInsertRows(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String encodedJsonBody) async {
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
  } else if (result.statusCode == 400 ||
      result.statusCode == 406 ||
      result.statusCode == 422) {
    // http Bad Request / Not Acceptable / Unprocessable
    formState.setValue(
        0, FSK.serverError, "Something went wrong. Please try again.");
    navigator.pop(DTActionResult.statusError);
  } else if (result.statusCode == 409) {
    // http Conflict
    const snackBar = SnackBar(
      content: Text("Looks like the record(s) already existed, that's ok."),
    );
    messenger.showSnackBar(snackBar);
    navigator.pop();
  } else {
    formState.setValue(
        0, FSK.serverError, "Got a server error. Please try again.");
    navigator.pop(DTActionResult.statusError);
  }
}

/// Source Configuration Form Actions
void homeFormActions(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey) async {
  switch (actionKey) {
    case ActionKeys.clientOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return;
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'table': 'client_registry',
        'data': [formState.getState(0)],
      });
      postInsertRows(context, formKey, formState, encodedJsonBody);
      break;
    case ActionKeys.loaderOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return;
      }
      var state = formState.getState(0);
      state['status'] = StatusKeys.submitted;
      state['user_email'] = JetsRouterDelegate().user.email;
      state['session_id'] = "${DateTime.now().millisecondsSinceEpoch}";
      state['table_name'] = state[FSK.client] + '_' + state[FSK.objectType];
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'table': 'input_loader_status',
        'data': [state],
      });
      postInsertRows(context, formKey, formState, encodedJsonBody);
      break;
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for home form: $actionKey');
      Navigator.of(context).pop();
  }
}

/// Process Input Form / Dialog Validator
String? processInputFormValidator(BuildContext context, JetsFormState formState,
    int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Process Input Form has unexpected data type");
  var isRequired = formState.getValue(group, FSK.isRequiredFlag);
  switch (key) {
    // Add Process Input Dialog Validations
    case FSK.client:
      String? value = v;
      print('Validating client $value');
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Client name must be provided.";
    case FSK.objectType:
      String? value = v;
      print('Validating object type $value');
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
    case FSK.groupingColumn:
      // always good
      return null;

    // Process Mapping Dialog Validation
    case FSK.inputColumn:
      if (v != null) return null;
      if (isRequired == null) return null;
      var defaultValue = formState.getValue(group, FSK.mappingDefaultValue);
      if (defaultValue != null) return null;
      return "Source Input Column must be selected or a default must be provided.";
    case FSK.functionName:
      return null;
    case FSK.functionArgument:
      if (v != null) return null;
      var functionName = formState.getValue(group, FSK.functionName);
      var mappingFunctionDetails =
          formState.getCacheValue(FSK.mappingFunctionDetailsCache) as List?;
      if (mappingFunctionDetails == null) {
        print("processInputFormActions error: mappingFunctionDetails is null");
        return "processInputFormActions error: mappingFunctionDetails is null";
      }
      var row = mappingFunctionDetails.firstWhere((e) => e[0] == functionName);
      if (row == null) {
        print(
            "processInputFormActions error: can't find object_type in objectTypeRegistry");
        return "processInputFormActions error: can't find function_name in mappingFunctionDetails";
      }
      // check if function argument is required
      if (row[1] != "1") return null;
      return "Cleansing function argument is required";
    case FSK.mappingDefaultValue:
      var errorMsg = formState.getValue(group, FSK.mappingErrorMessage);
      if (v != null && errorMsg == null) return null;
      if (v == null && errorMsg != null) return null;
      if (v != null && errorMsg != null) {
        return "Cannot specify both a default value and an error message";
      }
      return null;
    case FSK.mappingErrorMessage:
      return null;
    default:
      print(
          'Oops process input form has no validator configured for form field $key');
  }
  return null;
}

/// Source Configuration Form Actions
void processInputFormActions(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey) async {
  switch (actionKey) {
    case ActionKeys.addProcessInputOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return;
      }

      // add entity_rdf_type based on object_type
      var objectTypeRegistry =
          formState.getCacheValue(FSK.objectTypeRegistryCache) as List?;
      String objectType = formState.getValue(0, FSK.objectType) as String;
      if (objectTypeRegistry == null) {
        print("processInputFormActions error: objectTypeRegistry is null");
        return;
      }
      var row = objectTypeRegistry.firstWhere((e) => e[0] == objectType);
      if (row == null) {
        print(
            "processInputFormActions error: can't find object_type in objectTypeRegistry");
        return;
      }
      formState.setValue(0, FSK.entityRdfType, row[1]);

      // add table_name to form state based on source_type
      if (formState.getValue(0, FSK.sourceType) == 'file') {
        String client = formState.getValue(0, FSK.client) as String;
        formState.setValue(0, FSK.tableName, "${client}_$objectType");
      } else {
        // entity_rdf_type is the table_name for domain_table sources
        formState.setValue(0, FSK.tableName, row[1]);
      }

      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'table': 'process_input',
        'data': [formState.getState(0)],
      });
      postInsertRows(context, formKey, formState, encodedJsonBody);
      break;
    // Process Mapping Dialog
    case ActionKeys.mapperOk:
    case ActionKeys.mapperDraft:
      var processInputStatus = 'saved as draft';
      if (actionKey == ActionKeys.mapperOk) {
        processInputStatus = 'configured';
        var valid = formKey.currentState!.validate();
        if (!valid) {
          return;
        }
      }
      // Insert rows to process_mapping, if successful update process_input.status
      var tableName = formState.getValue(0, FSK.tableName);
      var processInputKey = formState.getValue(0, FSK.processInputKey);
      if (tableName == null || processInputKey == null) {
        print(
            "processInputFormActions error: save mapping - table_name or process_input.key is null");
        return;
      }
      for (var i = 0; i < formState.groupCount; i++) {
        formState.setValue(i, FSK.tableName, tableName);
        formState.setValue(i, FSK.userEmail, JetsRouterDelegate().user.email);
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'table': 'process_mapping',
        'data': formState.getInternalState(),
      });
      // Insert rows to process_mapping
      var navigator = Navigator.of(context);
      var result = await context.read<HttpClient>().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: encodedJsonBody);

      if (result.statusCode == 200) {
        // insert successfull, update process_input status
        var encodedJsonBody = jsonEncode(<String, dynamic>{
          'action': 'insert_rows',
          'table': 'update/process_input',
          'data': [
            {'key': processInputKey, 'status': processInputStatus}
          ],
        });
        postInsertRows(context, formKey, formState, encodedJsonBody);
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
      Navigator.of(context).pop();
  }
}

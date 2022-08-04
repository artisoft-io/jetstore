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
String? sourceConfigFormValidator(BuildContext context, JetsFormState formState,
    int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "sourceConfig Form has unexpected data type");
  switch (key) {
    case FSK.client:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length == 1) {
        return "Client name is too short.";
      }
      return "Client name must be provided.";
    case FSK.details:
      // always good
      return null;
    default:
      showAlertDialog(context,
          'Oops login form has no validator configured for form field $key');
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
void sourceConfigFormActions(BuildContext context, GlobalKey<FormState> formKey,
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
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for sourceConfig form: $actionKey');
      Navigator.of(context).pop();
  }
}

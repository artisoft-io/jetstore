import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/data_table_model.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:provider/provider.dart';

/// unpack an array to it's first element
String? unpack(dynamic elm) {
  if (elm == null) {
    return null;
  }
  if (elm is String) {
    return elm;
  }
  if (elm is List<String>) {
    return elm[0];
  }
  return null;
}

/// postInsertRows - main function to post for inserting rows into db
/// returns error message (if status code != 200) or null
/// NOTE: does navigator.pop with DTActionResult enum
/// NOTE: ignore error status code == 409 (http confict) //* TODO change this?
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
/// returns the http status code
Future<int> postSimpleAction(BuildContext context, JetsFormState formState,
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
  return result.statusCode;
}

// Action: raw_query
// returns list of rows
// returns JetsDataModel? = List<List<String?>>?
Future<JetsDataModel?> queryJetsDataModel(
    BuildContext context,
    JetsFormState formState,
    String serverEndPoint,
    String encodedJsonBody) async {
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
    final rows = result.body['rows'] as List;
    final model = rows.map((e) => (e as List).cast<String?>()).toList();
    return model;
  } else {
    showAlertDialog(context, "Something went wrong. Please try again.[1]");
  }
  return null;
}

// Action: raw_query_map
// return Map[queryKey, list of rows]
// return Map[String, JetsDataModel?]
Future<Map<String, JetsDataModel?>?> queryMapJetsDataModel(
    BuildContext context,
    JetsFormState formState,
    String serverEndPoint,
    String encodedJsonBody) async {
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
    final data = result.body['result_map'] as Map<String, dynamic>?;
    if (data == null) {
      return null;
    }
    final model = <String, JetsDataModel?>{};
    for (var item in data.entries) {
      model[item.key] =
          (item.value as List).map((e) => (e as List).cast<String?>()).toList();
    }
    return model;
  } else {
    showAlertDialog(context, "Something went wrong. Please try again.[2]");
  }
  return null;
}

String makeTableName(String client, String org, String objectType) {
  if (org.isNotEmpty) {
    return '${client}_${org}_$objectType';
  } else {
    return '${client}_$objectType';
  }
}

String makeTableNameFromState(Map<String, dynamic> state) {
  return makeTableName(
    state[FSK.client],
    state[FSK.org],
    state[FSK.objectType],
  );
}

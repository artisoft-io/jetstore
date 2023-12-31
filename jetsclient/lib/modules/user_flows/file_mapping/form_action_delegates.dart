import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? fileMappingFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "fileMappingFormValidator has unexpected data type");
  switch (key) {
    // Load Raw Rows
    case FSK.rawRows:
      if (v != null) {
        String value = v;
        if (value.isNotEmpty) {
          return null;
        }
      }
      return "Raw rows must be provided.";

    case DTKeys.fmInputSourceMappingUF:
      if (unpack(v) != null) {
        return null;
      }
      return "A file configuration must be selected.";

    case DTKeys.fmFileMappingTableUF:
      return null;

    default:
      print(
          'Oops fileMappingFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// File Mapping Form Action - aka mapperOk
Future<String?> fileMappingFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // loadRawRows
    case ActionKeys.loadRawRowsOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      return loadRawRows(context, formState);

    // Process Mapping Dialog
    case ActionKeys.mapperOk:
    case ActionKeys.mapperDraft:
      if (!formState.isFormValid() && actionKey == ActionKeys.mapperOk) {
        return null;
      }
      // Insert rows to process_mapping
      var tableName = formState.getValue(group, FSK.tableName);
      if (tableName == null) {
        print(
            "processInputFormActions error: save mapping - table_name is null");
        return "processInputFormActions error: save mapping - table_name is null";
      }
      for (var i = 0; i < formState.groupCount; i++) {
        formState.setValue(i, FSK.tableName, tableName);
        formState.setValue(i, FSK.userEmail, JetsRouterDelegate().user.email);
      }
      // var navigator = Navigator.of(context);
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
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();
      }

      var deleteResult = await HttpClientSingleton().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: deleteJsonBody);

      if (deleteResult.statusCode == 401) return "Not Authorized";
      if (deleteResult.statusCode != 200) {
        if (context.mounted) {
          showAlertDialog(context, "Something went wrong. Please try again.");
          Navigator.of(context).pop();
        }
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
      var result = await HttpClientSingleton().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: encodedJsonBody);

      if (result.statusCode == 401) return "Not Authorized";
      if (result.statusCode == 200) {
        // trigger a refresh of the process_mapping table
        formState.parentFormState?.setValue(0, FSK.tableName, null);
        formState.parentFormState
            ?.setValueAndNotify(0, FSK.tableName, tableName);
        const snackBar = SnackBar(
          content: Text('Mapping Updated Sucessfully'),
        );
        if (context.mounted) {
          JetsSpinnerOverlay.of(context).hide();
          ScaffoldMessenger.of(context).showSnackBar(snackBar);
          Navigator.of(context).pop();
        }
      } else {
        if (context.mounted) {
          showAlertDialog(context, "Something went wrong. Please try again.");
        }
        return "Something went wrong. Please try again.";
      }
      break;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for File Mapping Action: $actionKey');
  }
  return null;
}

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> fileMappingFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Download process mapping rows
    case ActionKeys.downloadMapping:
      return downloadMapping(context, formState);

    // Prepopulate the type of file from current record
    case ActionKeys.fmSelectSourceConfigUF:
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.org] = unpack(state[FSK.org]);
      state[FSK.tableName] = unpack(state[FSK.tableName]);
      state[FSK.objectType] = unpack(state[FSK.objectType]);
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for File Mapping UF State: $actionKey');
  }
  return null;
}

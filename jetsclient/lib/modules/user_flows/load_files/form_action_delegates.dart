import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/spinner_overlay.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Load Files UF
String? loadFilesFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "loadFilesFormValidator has unexpected data type");
  switch (key) {
    case DTKeys.lfSourceConfigTable:
      if (v != null) {
        return null;
      }
      return "Please select a file data source configuration.";

    case DTKeys.lfFileKeyStagingTable:
      if (v != null) {
        return null;
      }
      return "Please select a file(s) to load.";

    default:
      print(
          'Oops loadFilesFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Load Files UF Form Actions - set on UserFlowState
Future<String?> loadFilesFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Start loader
    case ActionKeys.lfLoadFilesUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Fields comming from table selected row will be in array, unpack the value
      // This is a multi select table, convert column array to multiple rows
      List<dynamic> requestData = [];
      for (var i = 0; i < state[FSK.fileKey].length; i++) {
        requestData.add(<String, dynamic>{
          FSK.client: state[FSK.client][0],
          FSK.org: state[FSK.org][0],
          FSK.objectType: state[FSK.objectType][0],
          FSK.tableName: state[FSK.tableName][0],
          FSK.fileKey: state[FSK.fileKey][i],
          FSK.sourcePeriodKey: state[FSK.sourcePeriodKey][i],
          FSK.status: StatusKeys.submitted,
          FSK.userEmail: JetsRouterDelegate().user.email,
          FSK.sessionId: "${DateTime.now().millisecondsSinceEpoch + i}",
        });
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'input_loader_status'}
        ],
        'data': requestData,
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      break;

    // Sync File Keys with web storage (s3)
    case ActionKeys.lfSyncFileKey:
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'sync_file_keys',
        'data': [],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      break;

    // Drop staging table
    case ActionKeys.lfDropTable:
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'drop_table',
        'data': [
          {
            'schemaName': 'public',
            'tableName': unpack(state[FSK.tableName]),
          }
        ],
      }, toEncodable: (_) => '');
      JetsSpinnerOverlay.of(context).show();
      await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (context.mounted) {
        JetsSpinnerOverlay.of(context).hide();
      }
      break;

    default:
      print('Oops unknown ActionKey for Load Files UF ActionKey: $actionKey');
  }
  return null;
}

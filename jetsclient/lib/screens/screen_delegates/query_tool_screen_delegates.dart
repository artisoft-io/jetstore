import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation and Actions delegates for the workspaceIDE forms
String? queryToolFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Query Tool Form has unexpected data type");
  switch (key) {
    case FSK.rawQuery:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length <= 1) {
        return "Query too short.";
      }
      return "Query must be provided.";

    case DTKeys.queryToolResultSetTable:
      return null;

    default:
      print(
          'Oops Query Tool Form has no validator configured for form field $key');
  }
  return null;
}

/// QueryTool Form Actions
Future<String?> queryToolFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Submit query
    case ActionKeys.queryToolDdlOk:
    case ActionKeys.queryToolOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final query = formState.getValue(group, FSK.rawQuery);
      final viewFormState = formState.peersFormState?[1];
      if (viewFormState == null) {
        print(
            'ERROR: queryToolFormActions does not have access to data table to refresh it');
        return 'ERROR: queryToolFormActions is not properly configured';
      }
      if (actionKey == ActionKeys.queryToolOk) {
        viewFormState.setValue(group, FSK.rawQueryReady, query);
      } else {
        viewFormState.setValue(group, FSK.rawDdlQueryReady, query);
      }
      viewFormState.setValueAndNotify(group, FSK.queryReady, query);
      // JetsSpinnerOverlay.of(context).show();
            // // hide the spinner if showing
            // if(JetsSpinnerOverlay.of(context).isLoading) {
            //   JetsSpinnerOverlay.of(context).hide();
            // }
      return null;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for workspaceIDE Form: $actionKey');
  }
  return null;
}

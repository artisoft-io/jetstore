import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? configureFilesFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "configureFilesFormValidator has unexpected data type");
  switch (key) {
    case FSK.scAddOrEditSourceConfigOption:
    case FSK.scCsvOrFixedOption:
      if (v != null) {
        return null;
      }
      return "An option must be selected.";

    case FSK.client:
      String? value = unpack(v);
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Client name must be selected.";
    case FSK.org:
      String? value = unpack(v);
      if (value != null) {
        return null;
      }
      return "Organization name must be selected.";

    case FSK.objectType:
      String? value = unpack(v);
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Object Type name must be selected.";
    case FSK.domainKeysJson:
      String? value = unpack(v);
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
    case FSK.codeValuesMappingJson:
      //* codeValuesMappingJson can be json or csv, not validating csv so not validating json here
      // String? value = v;
      // if (value == null || value.isEmpty) {
      //   return null; // this field is nullable
      // }
      // // Validate that value is valid json
      // try {
      //   jsonDecode(value);
      // } catch (e) {
      //   return "Code values mapping is not a valid json: ${e.toString()}";
      // }
      return null;
    case FSK.inputColumnsJson:
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return null; // this field is nullable
      }
      // Validate that FSK.inputColumnsJson and FSK.inputColumnsPositionsCsv are exclusive
      final otherv = formState.getValue(0, FSK.inputColumnsPositionsCsv);
      if (otherv != null) {
        return "Cannot specify both input columns names (headerless file) and input columns names and positions (fixed-width file).";
      }
      // Validate that value is valid json
      try {
        jsonDecode(value);
      } catch (e) {
        return "Input column names is not a valid json: ${e.toString()}";
      }
      return null;

    case FSK.inputColumnsPositionsCsv:
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return null; // this field is nullable
      }
      // Validate that FSK.inputColumnsJson and FSK.inputColumnsPositionsCsv are exclusive
      final otherv = formState.getValue(0, FSK.inputColumnsJson);
      if (otherv != null) {
        return "Cannot specify both input columns names (headerless file) and input columns names and positions (fixed-width file).";
      }
      return null;

    case FSK.automated:
      if (v != null) {
        return null;
      }
      return "Automation choice must be selected.";

    case FSK.scSourceConfigKey:
      if (v != null) {
        return null;
      }
      return "A file configuration must be selected.";

    default:
      print(
          'Oops configureFilesFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> configureFilesFormActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    // Start UF
    case ActionKeys.scStartUF:
      return null;

    // Prepopulate the type of file from current record
    case ActionKeys.scSelectSourceConfigUF:
      state[FSK.key] = unpack(state[FSK.key]);
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.org] = unpack(state[FSK.org]);
      state[FSK.tableName] = unpack(state[FSK.tableName]);
      state[FSK.objectType] = unpack(state[FSK.objectType]);
      state[FSK.inputColumnsJson] = unpack(state[FSK.inputColumnsJson]);
      state[FSK.inputColumnsPositionsCsv] = unpack(state[FSK.inputColumnsPositionsCsv]);
      state[FSK.domainKeysJson] = unpack(state[FSK.domainKeysJson]);
      state[FSK.codeValuesMappingJson] = unpack(state[FSK.codeValuesMappingJson]);
      state[FSK.automated] = unpack(state[FSK.automated]);
      if (state[FSK.inputColumnsJson] != null) {
        formState.setValue(group, FSK.scCsvOrFixedOption, FSK.scHeaderlessCsvOption);
      } else if(state[FSK.inputColumnsPositionsCsv] != null) {
        formState.setValue(group, FSK.scCsvOrFixedOption, FSK.scFixedWidthOption);
      } else {
        formState.setValue(group, FSK.scCsvOrFixedOption, FSK.scCsvOption);
      }
      return null;

    // Add/Update Source Config
    case ActionKeys.addSourceConfigOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // print('Add Source Config state: $state');
      var query = 'source_config'; // case add
      if (formState.getValue(0, FSK.key) != null) {
        query = 'update/source_config';
      }

      state['table_name'] = makeTableNameFromState(state);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      final statusCode = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (statusCode == 200) return null;
      if(statusCode == 409 && context.mounted) {
        showAlertDialog(context, "Record already exist, please verify.");
      }
      if(context.mounted) {
        showAlertDialog(context, "Server error, please try again.");
      }
      return "Error while saving file configuration";

    case ActionKeys.deleteSourceConfig:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected File Configuration?');
      if (uc != 'OK') return null;
      state[FSK.key] = unpack(state[FSK.key]);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/source_config'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        if (statusCode != 200) return "Error while deleting file configuration";
        state.remove(FSK.key);
        state.remove(FSK.client);
        state.remove(FSK.org);
        state.remove(FSK.objectType);
        state.remove(FSK.scCsvOrFixedOption);
        state.remove(FSK.inputColumnsJson);
        state.remove(FSK.inputColumnsPositionsCsv);
        state.remove(FSK.domainKeysJson);
        state.remove(FSK.codeValuesMappingJson);
      }
      return null;

    // Drop staging table
    case ActionKeys.dropTable:
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'drop_table',
        'data': [
          {
            'schemaName': 'public',
            'tableName': unpack(state[FSK.tableName]),
          }
        ],
      }, toEncodable: (_) => '');
      final statusCode = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (statusCode != 200) return "Error while droping staging table";
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

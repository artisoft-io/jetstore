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

  v = formState.getValue(group, key);
  final fileType = unpack(formState.getValue(0, FSK.scFileTypeOption));
  switch (key) {
    case FSK.scAddOrEditSourceConfigOption:
    case FSK.scSingleOrMultiPartFileOption:
    case FSK.scFileTypeOption:
      if (unpack(v) != null) {
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
      if (value != null && value.characters.isNotEmpty) {
        return null;
      }
      return "Object Type name must be selected.";

    case FSK.scCurrentSheet:
      String? value = unpack(v);
      if (value != null && value.characters.isNotEmpty) {
        return null;
      }
      return "Specify the sheet position.";

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
    case FSK.computePipesJson:
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
      // this field is nullable unless FSK.scFileTypeOption is Headerless CSV or Parquet Select
      if ((fileType != FSK.scHeaderlessCsvOption) &&
          (fileType != FSK.scParquetSelectOption)) return null;
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return "Input column names must be provided";
      }
      // Validate that value is valid json
      try {
        jsonDecode(value);
      } catch (e) {
        return "Input column names is not a valid json: ${e.toString()}";
      }
      return null;

    case FSK.inputColumnsPositionsCsv:
      if (fileType != FSK.scFixedWidthOption) return null;
      String? value = unpack(v);
      if (value == null || value.isEmpty) {
        return "Input columns names and positions must be provided using csv";
      }
      // // Validate that FSK.inputColumnsJson and FSK.inputColumnsPositionsCsv are exclusive
      // final otherv = formState.getValue(0, FSK.inputColumnsJson);
      // if (otherv != null) {
      //   return "Cannot specify both input columns names (headerless file) and input columns names and positions (fixed-width file).";
      // }
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

    case FSK.tableName:
      if (v != null) {
        return null;
      }
      return "Error, a table name should be specified automatically.";

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

    // Edit Xlsx Option: specify sheet name or position
    case ActionKeys.scEditXlsxOptionsUF:
      state[FSK.scInputFormatDataJson] =
          '{"currentSheet": "${unpack(state[FSK.scCurrentSheet])}"}';
      return null;

    // Prepopulate the type of file from current record
    case ActionKeys.scSelectSourceConfigUF:
      state[FSK.key] = unpack(state[FSK.key]);
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.org] = unpack(state[FSK.org]);
      state[FSK.tableName] = unpack(state[FSK.tableName]);
      state[FSK.objectType] = unpack(state[FSK.objectType]);
      state[FSK.inputColumnsJson] = unpack(state[FSK.inputColumnsJson]);
      state[FSK.inputColumnsPositionsCsv] =
          unpack(state[FSK.inputColumnsPositionsCsv]);
      state[FSK.domainKeysJson] = unpack(state[FSK.domainKeysJson]);
      state[FSK.codeValuesMappingJson] =
          unpack(state[FSK.codeValuesMappingJson]);
      state[FSK.computePipesJson] =
          unpack(state[FSK.computePipesJson]);
      state[FSK.automated] = unpack(state[FSK.automated]);
      state[FSK.scFileTypeOption] = unpack(state[FSK.scFileTypeOption]);
      // Map part file indicator
      if (unpack(state['is_part_files']) == '1') {
        state[FSK.scSingleOrMultiPartFileOption] = FSK.scMultiPartFileOption;
      } else if (unpack(state['is_part_files']) == '0') {
        state[FSK.scSingleOrMultiPartFileOption] = FSK.scSingleFileOption;
      } else {
        print(
            "*** ERROR Invalid value for 'is_part_files': ${unpack(state['is_part_files'])}");
      }
      // Backward compatibility on input_type
      final fileType = state[FSK.scFileTypeOption];
      if (fileType == '') {
        if (state[FSK.inputColumnsJson] != null) {
          formState.setValue(
              group, FSK.scFileTypeOption, FSK.scHeaderlessCsvOption);
        } else if (state[FSK.inputColumnsPositionsCsv] != null) {
          formState.setValue(
              group, FSK.scFileTypeOption, FSK.scFixedWidthOption);
        } else {
          formState.setValue(group, FSK.scFileTypeOption, FSK.scCsvOption);
        }
      }
      // input file options
      final scOptions = unpack(state[FSK.scInputFormatDataJson]);
      state[FSK.scInputFormatDataJson] = scOptions;
      if (fileType == FSK.scHeaderlessXlsxOption ||
          fileType == FSK.scXlsxOption) {
        if (scOptions != null && scOptions.isNotEmpty) {
          try {
            final xlsxOptions = jsonDecode(scOptions);
            state[FSK.scCurrentSheet] = xlsxOptions[FSK.scCurrentSheet];
          } catch (e) {
            return "Input column names is not a valid json: ${e.toString()}";
          }
        }
      }
      return null;

    // Add Source Config
    case ActionKeys.scAddSourceConfigUF:
      state['table_name'] = makeTableNameFromState(state);
      break;

    // Post Add/Update Source Config to server
    case ActionKeys.addSourceConfigOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }

      final stateCopy = Map<String, dynamic>.from(state);
      var query = 'source_config'; // case add
      if (stateCopy[FSK.key] != null) {
        query = 'update/source_config';
      }
      switch (unpack(stateCopy[FSK.scFileTypeOption])) {
        case FSK.scXlsxOption:
        case FSK.scHeaderlessXlsxOption:
          stateCopy[FSK.inputColumnsJson] = null;
          stateCopy[FSK.inputColumnsPositionsCsv] = null;
          break;
        case FSK.scCsvOption:
        case FSK.scParquetOption:
          stateCopy[FSK.inputColumnsJson] = null;
          stateCopy[FSK.inputColumnsPositionsCsv] = null;
          stateCopy[FSK.scInputFormatDataJson] = '';
          break;
        case FSK.scHeaderlessCsvOption:
        case FSK.scParquetSelectOption:
          stateCopy[FSK.inputColumnsPositionsCsv] = null;
          stateCopy[FSK.scInputFormatDataJson] = '';
          break;
        case FSK.scFixedWidthOption:
          stateCopy[FSK.inputColumnsJson] = null;
          stateCopy[FSK.scInputFormatDataJson] = '';
          break;
        default:
          print(
              "ERROR: unknown FSK.scFileTypeOption in state: ${unpack(stateCopy[FSK.scFileTypeOption])}");
          return "error";
      }
      switch (unpack(stateCopy[FSK.scSingleOrMultiPartFileOption])) {
        case FSK.scSingleFileOption:
          stateCopy['is_part_files'] = 0;
          break;
        case FSK.scMultiPartFileOption:
          stateCopy['is_part_files'] = 1;
          break;
        default:
          print(
              "ERROR: missing/invalid FSK.scSingleOrMultiPartFileOption selection in state!");
          return "error";
      }
      // print('*** Add Source Config state: $stateCopy');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': query}
        ],
        'data': [stateCopy],
      }, toEncodable: (_) => '');
      final statusCode = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      if (statusCode == 200) return null;
      if (statusCode == 409 && context.mounted) {
        showAlertDialog(context, "Record already exist, please verify.");
      }
      if (context.mounted) {
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
      // print("*** Clear Selected Rows Called, pre post");
      formState.clearSelectedRow(group, FSK.scSourceConfigKey);
      state.remove(FSK.scSourceConfigKey);
      state.remove(FSK.key);
      state.remove(FSK.client);
      state.remove(FSK.org);
      state.remove(FSK.objectType);
      state.remove(FSK.scFileTypeOption);
      state.remove(FSK.scSingleOrMultiPartFileOption);
      state.remove('is_part_files');
      state.remove(FSK.inputColumnsJson);
      state.remove(FSK.inputColumnsPositionsCsv);
      state.remove(FSK.domainKeysJson);
      state.remove(FSK.codeValuesMappingJson);
      state.remove(FSK.computePipesJson);
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        if (statusCode != 200) return "Error while deleting file configuration";
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
      print('Oops unknown ActionKey for Client Config UF: $actionKey');
  }
  return null;
}

import 'package:flutter/material.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_delegates.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_mapping_validator.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for File Mapping UF Module

final Map<String, FormConfig> _formConfigurations = {
  // File Mapping - Select Source
  FormKeys.fmSelectSourceConfigUF: FormConfig(
    key: FormKeys.fmSelectSourceConfigUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.fmInputSourceMappingUF,
            dataTableConfig: DTKeys.fmInputSourceMappingUF,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: fileMappingFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),

  // File Mapping - List mapping definition & Actions
  FormKeys.fmFileMappingUF: FormConfig(
    key: FormKeys.fmFileMappingUF,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.fmFileMappingTableUF,
            dataTableConfig: DTKeys.fmFileMappingTableUF,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: fileMappingFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // loadRawRows - Dialog to load / replace process mapping
  FormKeys.loadRawRows: FormConfig(
    key: FormKeys.loadRawRows,
    title: "File Mapping Intake",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.loadRawRowsOk,
          capability: "client_config",
          label: "Save",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.dialogCancel,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        // Instruction
        TextFieldConfig(
            label: "Enter the File Mapping Definition as csv/tsv-encoded text.",
            maxLines: 3,
            topMargin: defaultPadding,
            bottomMargin: defaultPadding)
      ],
      [
        FormInputFieldConfig(
            key: FSK.rawRows,
            label: "File Mapping (csv/tsv)",
            hint: "Paste from spreadsheet using JetStore template",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 20,
            maxLength: 51200),
      ],
    ],
    formValidatorDelegate: fileMappingFormValidator,
    formActionsDelegate: fileMappingFormActions,
  ),

  // Mapping Form - mapping of intake file structure to canonical model
  FormKeys.fmMappingFormUF: FormConfig(
    key: FormKeys.processMapping,
    title: "File Mapping Worksheet",
    actions: [
      FormActionConfig(
          key: ActionKeys.mapperOk,
          capability: "client_config",
          label: "Save",
          enableOnlyWhenFormValid: true,
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.mapperDraft,
          capability: "client_config",
          label: "Save as Draft",
          enableOnlyWhenFormNotValid: true,
          buttonStyle: ActionStyle.dialogCancel,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.dialogCancel,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    queries: {
      "inputFieldsQuery":
          "SELECT md.data_property, md.is_required, pm.input_column, pm.function_name, pm.argument, pm.default_value, pm.error_message, md.default_column_value FROM jetsapi.object_type_mapping_details md LEFT JOIN (SELECT * FROM jetsapi.process_mapping WHERE table_name = '{table_name}') pm ON md.data_property = pm.data_property WHERE md.object_type = '{object_type}' ORDER BY md.data_property ASC LIMIT 1000",
      "inputColumnsQuery":
          "SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '{table_name}' AND column_name NOT IN ('file_key','last_update','session_id','shard_id') ORDER BY column_name",
      "mappingFunctionsQuery":
          "SELECT function_name, is_argument_required FROM jetsapi.mapping_function_registry ORDER BY function_name ASC LIMIT 500",
    },
    inputFieldsQuery: "inputFieldsQuery",
    savedStateQuery: "inputFieldsQuery",
    dropdownItemsQueries: {
      FSK.mappingFunctionsDropdownItemsCache: "mappingFunctionsQuery",
    },
    typeaheadItemsQueries: {
      FSK.inputColumnsDropdownItemsCache: "inputColumnsQuery",
    },
    metadataQueries: {
      FSK.mappingFunctionDetailsCache: "mappingFunctionsQuery",
      FSK.inputColumnsCache: "inputColumnsQuery",
    },
    stateKeyPredicates: [FSK.objectType, FSK.tableName],
    inputFieldRowBuilder: (index, inputFieldRow, formState) {
      assert(inputFieldRow != null, 'error inputFieldRow should not be null');
      if (inputFieldRow == null) {
        return [];
      }
      // savedState is List<String?>? with values as per savedStateQuery
      final savedState = formState.getCacheValue(FSK.savedStateCache) as List?;
      final isRequired = inputFieldRow[1]! == '1';
      final isRequiredIndicator = isRequired ? '*' : '';
      final savedInputColumn = savedState?[index][2];
      final inputColumnList =
          (formState.getCacheValue(FSK.inputColumnsCache) as List)
              .map((e) => e[0])
              .toList();
      final inputColumnDefault = inputFieldRow[7] ??
          (inputColumnList.contains(inputFieldRow[0])
              ? inputFieldRow[0]
              : null);
      if (isRequired) formState.setValue(index, FSK.isRequiredFlag, "1");
      // set the default values to the formState
      formState.setValue(index, FSK.dataProperty, inputFieldRow[0]);
      formState.setValue(
          index, FSK.inputColumn, savedInputColumn ?? inputColumnDefault);
      formState.setValue(index, FSK.functionName, savedState?[index][3]);
      formState.setValue(index, FSK.functionArgument, savedState?[index][4]);
      formState.setValue(index, FSK.mappingDefaultValue, savedState?[index][5]);
      formState.setValue(index, FSK.mappingErrorMessage, savedState?[index][6]);
      // print("Form BUILDER savedState row ${savedState![index]}");
      return [
        [
          // data_property
          TextFieldConfig(
              // label: "$index: ${inputFieldRow[0]}$isRequiredIndicator",
              label: "${inputFieldRow[0]}$isRequiredIndicator",
              group: index,
              flex: 1,
              topMargin: 20.0)
        ],
        [
          // input_column
          FormTypeaheadFieldConfig(
              key: FSK.inputColumn,
              group: index,
              flex: 3,
              autovalidateMode: AutovalidateMode.always,
              typeaheadMenuItemCacheKey: FSK.inputColumnsDropdownItemsCache,
              priorityTargetKey: FSK.dataProperty,
              defaultItem: savedInputColumn ?? inputColumnDefault,
              inputFieldConfig: FormInputFieldConfig(
                key: FSK.inputColumn,
                group: index,
                label: 'Input Column',
                hint: 'Input File Column Name',
                autofocus: false,
                autovalidateMode: AutovalidateMode.always,
                textRestriction: TextRestriction.none,
                defaultValue: savedInputColumn ?? inputColumnDefault,
                maxLength: 120,
              )),
          // FormDropdownWithSharedItemsFieldConfig(
          //   key: FSK.inputColumn,
          //   group: index,
          //   flex: 2,
          //   autovalidateMode: AutovalidateMode.always,
          //   dropdownMenuItemCacheKey: FSK.inputColumnsDropdownItemsCache,
          //   defaultItem: savedInputColumn ?? inputColumnDefault,
          // ),
          // function_name
          FormDropdownWithSharedItemsFieldConfig(
            key: FSK.functionName,
            group: index,
            flex: 2,
            dropdownMenuItemCacheKey: FSK.mappingFunctionsDropdownItemsCache,
            defaultItem: savedState?[index][3],
          ),
          // argument
          FormInputFieldConfig(
            key: FSK.functionArgument,
            label: "Function Argument",
            hint:
                "Cleansing function argument, it is either required or ignored",
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
          ),
          // default_value
          FormInputFieldConfig(
            key: FSK.mappingDefaultValue,
            label: "Default Value",
            hint:
                "Default value to use if input value is not provided or cleansing function returns null",
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
          ),
          // error_message
          FormInputFieldConfig(
            key: FSK.mappingErrorMessage,
            label: "Error Message",
            hint:
                "Error message to raise if input value is not provided or cleansing function returns null and there is no default value",
            group: index,
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 125,
          ),
        ],
      ];
    },
    formValidatorDelegate: mappingFormValidator,
    formActionsDelegate: fileMappingFormActions,
  ),
};

FormConfig? getFileMappingFormConfig(String key) {
  return _formConfigurations[key];
}

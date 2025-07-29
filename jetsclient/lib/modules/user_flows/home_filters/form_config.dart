import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/home_filters/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for File Mapping UF Module

final Map<String, FormConfig> _formConfigurations = {
  // Select Process Form
  FormKeys.hfSelectProcessUF: FormConfig(
    key: FormKeys.hfSelectProcessUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.hfProcessTableUF,
            dataTableConfig: DTKeys.hfProcessTableUF,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: homeFiltersFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // Select Status Form
  FormKeys.hfSelectStatusUF: FormConfig(
    key: FormKeys.hfSelectStatusUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.hfStatusTableUF,
            dataTableConfig: DTKeys.hfStatusTableUF,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: homeFiltersFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // Select File Key filter Form
  FormKeys.hfSelectFileKeyFilterUF: FormConfig(
    key: FormKeys.hfSelectFileKeyFilterUF,
    actions: standardActions,
    inputFieldsV2: [
      FormFieldRowConfig(flex: 3, rowConfig: [
        FormDataTableFieldConfig(
            key: DTKeys.hfFileKeyFilterTypeTableUF,
            dataTableConfig: DTKeys.hfFileKeyFilterTypeTableUF,
            tableHeight: 400),
      ]),
      FormFieldRowConfig(flex: 0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.hfFileKeySubstring,
            label: "File Key Extract",
            hint: "Enter portion of File Key to retain",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ]),
    ],
    formValidatorDelegate: homeFiltersFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // Select Time Window filter Form
  FormKeys.hfSelectTimeWindowUF: FormConfig(
    key: FormKeys.hfSelectTimeWindowUF,
    actions: standardActions,
    inputFieldsV2: [
      FormFieldRowConfig(flex: 0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.hfStartTime,
            label: "Start Time",
            hint:
                "Start time (format 2025-07-03T15:00:00Z), leave blank for current time",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 25,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.hfStartOffset,
            label: "Start Offset Duration",
            hint:
                "Offset as duration (example: 12 hours, 3 days), or leave blank",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 25,
            useDefaultFont: true),
      ]),
      FormFieldRowConfig(flex: 0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.hfEndTime,
            label: "End Time",
            hint:
                "End time (format 2025-07-03T15:00:00Z), leave blank for current time",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 25,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.hfEndOffset,
            label: "End Offset Duration",
            hint:
                "Offset as duration (example: 12 hours, 3 days), or leave blank",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 25,
            useDefaultFont: true),
      ]),
    ],
    formValidatorDelegate: homeFiltersFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // Show Pipeline Execution Status Table Form
  FormKeys.hfViewStatusTableUF: FormConfig(
    key: FormKeys.hfViewStatusTableUF,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Save & Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pipelineExecStatusTable,
            dataTableConfig: DTKeys.pipelineExecStatusTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: homeFiltersFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getHomeFiltersFormConfig(String key) {
  return _formConfigurations[key];
}

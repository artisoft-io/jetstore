import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for File Mapping UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.fmSelectSourceConfigUF: FormConfig(
    key: FormKeys.fmSelectSourceConfigUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.inputSourceMapping,
            dataTableConfig: DTKeys.inputSourceMapping,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: fileMappingFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
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
            key: DTKeys.processMappingTable,
            dataTableConfig: DTKeys.processMappingTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: fileMappingFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
};

FormConfig? getFileMappingFormConfig(String key) {
  return _formConfigurations[key];
}

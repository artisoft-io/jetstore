import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Load Files UF Module

final Map<String, FormConfig> _formConfigurations = {

  FormKeys.lfSelectSourceConfigUF: FormConfig(
    key: FormKeys.lfSelectSourceConfigUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.lfSourceConfigTable,
            dataTableConfig: DTKeys.lfSourceConfigTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: loadFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.lfSelectFileKeysUF: FormConfig(
    key: FormKeys.lfSelectFileKeysUF,
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
          label: "Load Files & Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.lfFileKeyStagingTable,
            dataTableConfig: DTKeys.lfFileKeyStagingTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: loadFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getLoadFilesFormConfig(String key) {
  return _formConfigurations[key];
}

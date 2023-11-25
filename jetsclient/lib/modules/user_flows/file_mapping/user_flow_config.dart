import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.sourceConfigUF: UserFlowConfig(startAtKey: "startUF", states: {
    "startUF": UserFlowState(
        key: "startUF",
        description: 'Start File Mapping User Flow',
        formConfig: getFormConfig(FormKeys.fmStartFileMappingUF),
        actionDelegate: fileMappingFormActions,
        stateAction: ActionKeys.fmStartUF,
        defaultNextState: "select_source_config"),
    "select_source_config": UserFlowState(
        key: "select_source_config",
        description: 'Select an existing Source Config for mapping',
        formConfig: getFormConfig(FormKeys.fmSelectSourceConfigUF),
        actionDelegate: fileMappingFormActions,
        stateAction: ActionKeys.fmSelectSourceConfigUF,
        defaultNextState: "file_mapping"),
    "file_mapping": UserFlowState(
        key: "file_mapping",
        description: 'File Mapping',
        formConfig: getFormConfig(FormKeys.fmFileMappingUF),
        actionDelegate: fileMappingFormActions,
        stateAction: ActionKeys.mapperOk,
        defaultNextState: "doneUF"),
    "doneUF": UserFlowState(
        key: "doneUF",
        description: 'User Flow Completed',
        formConfig: getFormConfig(FormKeys.fmDoneFileMappingUF),
        actionDelegate: fileMappingFormActions,
        isEnd: true),
  })
};

UserFlowConfig? getConfigureFilesUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

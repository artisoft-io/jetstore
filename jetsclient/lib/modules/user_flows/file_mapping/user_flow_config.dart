import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/actions/config_delegates.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.fileMappingUF:
      UserFlowConfig(startAtKey: "select_source_config", states: {
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
        actionDelegate: (_, v1, v2, v3, v4, {group}) =>
            processInputFormActions(v1, v2, v3, v4, group: group),
        stateAction: ActionKeys.mapperOk,
        isEnd: true),
  })
};

UserFlowConfig? getFileMappingUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

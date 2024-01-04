import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  // Base UF to tee up the file mapping
  UserFlowKeys.fileMappingUF:
      UserFlowConfig(startAtKey: "select_source_config", states: {
    "select_source_config": UserFlowState(
        key: "select_source_config",
        description: 'Select an existing Source Config for mapping',
        formConfig: getFormConfig(FormKeys.fmSelectSourceConfigUF),
        actionDelegate: fileMappingFormActionsUF,
        stateAction: ActionKeys.fmSelectSourceConfigUF,
        defaultNextState: "file_mapping"),
    "file_mapping": UserFlowState(
        key: "file_mapping",
        description: 'File Mapping',
        formConfig: getFormConfig(FormKeys.fmFileMappingUF),
        actionDelegate: fileMappingFormActionsUF,
        isEnd: true),
  }),
  // File mapping screen
  UserFlowKeys.mapFileUF: UserFlowConfig(startAtKey: "map_file", states: {
    "map_file": UserFlowState(
        key: "map_file",
        description: 'Map file screen/dialog',
        formConfig: getFormConfig(FormKeys.fmMappingFormUF),
        actionDelegate: (p1, p2, p3, p4, p5, {group}) => fileMappingFormActions(p2, p3, p4, p5, group:group),
        isEnd: true),
  }),
};

UserFlowConfig? getFileMappingUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

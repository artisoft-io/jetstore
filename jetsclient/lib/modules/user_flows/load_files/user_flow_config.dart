import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/load_files/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.loadFilesUF:
      UserFlowConfig(startAtKey: "select_source_config", states: {
    "select_source_config": UserFlowState(
        key: "select_source_config",
        description: 'Select a file data source configuration',
        formConfig: getFormConfig(FormKeys.lfSelectSourceConfigUF),
        actionDelegate: loadFilesFormActionsUF,
        defaultNextState: "select_file_keys"),
    "select_file_keys": UserFlowState(
        key: "select_file_keys",
        description: 'Select file keys to load',
        formConfig: getFormConfig(FormKeys.lfSelectFileKeysUF),
        actionDelegate: loadFilesFormActionsUF,
        stateAction: ActionKeys.lfLoadFilesUF,
        isEnd: true),
  })
};

UserFlowConfig? getLoadFilesUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

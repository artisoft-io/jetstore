import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/form_action_delegates.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  // Pull Workspace
  UserFlowKeys.workspacePullUF: UserFlowConfig(
      startAtKey: "pull_workspace",
      exitScreenPath: workspaceRegistryPath,
      states: {
        "pull_workspace": UserFlowState(
            key: "pull_workspace",
            description: 'Pull workspace form',
            formConfig: getFormConfig(FormKeys.wpPullWorkspaceUF),
            actionDelegate: pullWorkspaceFormActions,
            stateAction: ActionKeys.wpPullWorkspaceConfirmUF,
            defaultNextState: "confirm"),
        "confirm": UserFlowState(
            key: "confirm",
            description: 'Confirm pull workspace',
            formConfig: getFormConfig(FormKeys.wpConfirmPullWorkspaceUF),
            actionDelegate: pullWorkspaceFormActions,
            stateAction: ActionKeys.wpPullWorkspaceOkUF,
            isEnd: true),
      }),
  // Load Client Config
  UserFlowKeys.loadConfigUF: UserFlowConfig(
      startAtKey: "load_config",
      exitScreenPath: workspaceRegistryPath,
      states: {
        "load_config": UserFlowState(
            key: "load_config",
            description: 'Load Client Config form',
            formConfig: getFormConfig(FormKeys.wpLoadConfigUF),
            actionDelegate: loadConfigFormActions,
            stateAction: ActionKeys.wpLoadConfigConfirmUF,
            defaultNextState: "confirm"),
        "confirm": UserFlowState(
            key: "confirm",
            description: 'Confirm Load Config',
            formConfig: getFormConfig(FormKeys.wpConfirmLoadConfigUF),
            actionDelegate: loadConfigFormActions,
            stateAction: ActionKeys.wpLoadConfigOkUF,
            isEnd: true),
      }),
};

UserFlowConfig? getWorkspacePullUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

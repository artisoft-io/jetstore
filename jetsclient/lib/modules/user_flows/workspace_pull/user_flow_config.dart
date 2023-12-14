import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/form_action_delegates.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
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
      })
};

UserFlowConfig? getWorkspacePullUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

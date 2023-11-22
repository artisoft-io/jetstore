import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/client_config/form_action_delegates.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  // 
  UserFlowKeys.clientRegistryUF: UserFlowConfig(
      startAtKey: "startUF",
      states: {
        "startUF": UserFlowState(
          key: "startUF",
          description: 'Start Client Registry User Flow',
          formConfig: getFormConfig(FormKeys.ufStartClientRegistry),
          actionDelegate: clientConfigFormActions,
          defaultNextState: "client"),
        "client": UserFlowState(
          key: "client",
          description: 'Create client',
          formConfig: getFormConfig(FormKeys.ufClient),
          actionDelegate: clientConfigFormActions,
          defaultNextState: "client_org"),
        "client_org": UserFlowState(
          key: "client_org",
          description: 'Create org/vendor of client',
          formConfig: getFormConfig(FormKeys.ufVendor),
          actionDelegate: clientConfigFormActions,
          defaultNextState: "doneUF"),
        "doneUF": UserFlowState(
          key: "doneUF",
          description: 'User Flow Completed',
          formConfig: getFormConfig(FormKeys.ufDoneClientRegistry),
          actionDelegate: clientConfigFormActions,
          isEnd: true),
      })
};

UserFlowConfig? getScreenConfigUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}

import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/client_config/user_flow_delegates.dart';
import 'package:jetsclient/modules/actions/config_delegates.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  // 
  UserFlowKeys.clientRegistryUF: UserFlowConfig(
      startAtKey: "",
      states: {
        "client": UserFlowState(
          key: "client",
          description: 'Create client',
          formConfig: getFormConfig(FormKeys.ufClient),
          defaultNextState: "client_org"),
        "client_org": UserFlowState(
          key: "client_org",
          description: 'Create org/vendor of client',
          formConfig: getFormConfig(FormKeys.ufVendor),
          isEnd: true),
      },
      validatorDelegate: sourceConfigValidator,
      actionDelegate: clientUFAction)
};

UserFlowConfig getUserFlowConfig(String key) {
  var config = _userFlowConfigurations[key];
  if (config == null) {
    // config = getWorkspaceUserFlowConfig(key);
    if (config == null) {
      throw Exception(
          'ERROR: Invalid program configuration: user flow configuration $key not found');
    }
  }
  return config;
}

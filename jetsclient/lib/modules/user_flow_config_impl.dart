import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/client_config/user_flow_config.dart';

UserFlowConfig getUserFlowConfig(String key) {
  var config = getScreenConfigUserFlowConfig(key);
  if (config != null) return config;
  // Add UF from other modules here
  throw Exception(
      'ERROR: Invalid program configuration: user flow configuration $key not found');
}

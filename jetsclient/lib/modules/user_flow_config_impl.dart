import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/client_registry/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/home_filters/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/register_file_key/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/user_flow_config.dart';

UserFlowConfig getUserFlowConfig(String key) {
  var config = getScreenConfigUserFlowConfig(key);
  if (config != null) return config;
  config = getConfigureFilesUserFlowConfig(key);
  if (config != null) return config;
  config = getFileMappingUserFlowConfig(key);
  if (config != null) return config;
  config = getHomeFiltersUserFlowConfig(key);
  if (config != null) return config;
  config = getPipelineConfigUserFlowConfig(key);
  if (config != null) return config;
  config = getLoadFilesUserFlowConfig(key);
  if (config != null) return config;
  config = getRegisterFileKeyUserFlowConfig(key);
  if (config != null) return config;
  config = getStartPipelineUserFlowConfig(key);
  if (config != null) return config;
  config = getWorkspacePullUserFlowConfig(key);
  if (config != null) return config;
  // Add UF from other modules here
  throw Exception(
      'ERROR: Invalid program configuration: user flow configuration $key not found');
}

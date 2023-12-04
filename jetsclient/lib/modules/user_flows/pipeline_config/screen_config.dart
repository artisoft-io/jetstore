import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufPipelineConfig: ScreenConfig(
      key: ScreenKeys.ufPipelineConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Pipeline Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getPipelineConfigScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

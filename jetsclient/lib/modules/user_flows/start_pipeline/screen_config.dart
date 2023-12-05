import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Start Pipeline
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufStartPipeline: ScreenConfig(
      key: ScreenKeys.ufStartPipeline,
      appBarLabel: 'JetStore Workspace',
      title: 'Start Pipeline',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};

ScreenConfig? getStartPipelineScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

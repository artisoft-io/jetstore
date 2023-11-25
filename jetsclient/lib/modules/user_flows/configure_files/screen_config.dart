import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufSourceConfig: ScreenConfig(
      key: ScreenKeys.ufSourceConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Client File Configuration User Flow',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getConfigureFileScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

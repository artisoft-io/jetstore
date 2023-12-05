import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufLoadFiles: ScreenConfig(
      key: ScreenKeys.ufLoadFiles,
      appBarLabel: 'JetStore Workspace',
      title: 'Load Files',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getLoadFilesScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufRegisterFileKey: ScreenConfig(
      key: ScreenKeys.ufRegisterFileKey,
      appBarLabel: 'JetStore Workspace',
      title: 'Submit Schema Event (Register File Key)',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getRegisterFileKeyScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufPullWorkspace: ScreenConfig(
      key: ScreenKeys.ufPullWorkspace,
      appBarLabel: 'JetStore Workspace',
      title: 'Pull Workspace Changes',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getWorkspacePullScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

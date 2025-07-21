import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufHomeFilters: ScreenConfig(
      key: ScreenKeys.ufHomeFilters,
      appBarLabel: 'JetStore Workspace',
      title: 'File Mapping Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};

ScreenConfig? getHomeFiltersScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

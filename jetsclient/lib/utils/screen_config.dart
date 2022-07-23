import 'package:jetsclient/utils/constants.dart';

import '../routes/jets_routes_app.dart';
import 'data_table_config.dart';

class ScreenConfig {
  ScreenConfig(
      {required this.key,
      required this.appBarLabel,
      required this.title,
      required this.showLogout,
      required this.leftBarLogo,
      required this.menuEntries});
  final String key;
  final String appBarLabel;
  final String title;
  final bool showLogout;
  final String leftBarLogo;
  final List<MenuEntry> menuEntries;
}

class MenuEntry {
  MenuEntry({required this.key, required this.label, this.routePath});
  final String key;
  final String label;
  final String? routePath;
}

final defaultMenuEntries = [
  MenuEntry(
      key: 'inputFiles', label: 'Input Files', routePath: fileRegistryPath),
  MenuEntry(
      key: 'mappingConfig',
      label: 'Mapping Configurations',
      routePath: mappingConfigPath),
  MenuEntry(
      key: 'processConfig', label: 'Process Configurations'),
  MenuEntry(key: 'jobList', label: 'Data Pipeline', routePath: pipelinePath),
];

final Map<String, ScreenConfig> _screenConfigurations = {
  "testScreen": ScreenConfig(
      key: "testScreen",
      appBarLabel: 'JetStore Workspace',
      title: 'Test Screen',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
  ScreenKeys.login: ScreenConfig(
      key: ScreenKeys.login,
      appBarLabel: 'JetStore Workspace',
      title: 'Please login',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: []),
  ScreenKeys.register: ScreenConfig(
      key: ScreenKeys.register,
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome, please register',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: []),
  ScreenKeys.pipelines: ScreenConfig(
      key: ScreenKeys.pipelines,
      appBarLabel: 'JetStore Workspace',
      title: 'Data Pipelines',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
  ScreenKeys.fileRegistry: ScreenConfig(
      key: ScreenKeys.fileRegistry,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File Registry',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
  ScreenKeys.fileRegistryTable: ScreenConfig(
      key: ScreenKeys.fileRegistryTable,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File as Table',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
  ScreenKeys.home: ScreenConfig(
      key: ScreenKeys.home,
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome to JetStore Workspace!',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
};

ScreenConfig getScreenConfig(String key) {
  var config = _screenConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: screen configuration $key not found');
  }
  return config;
}

import 'package:jetsclient/utils/constants.dart';

import '../routes/jets_routes_app.dart';

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
      key: 'sourceConfig', label: 'Source Config', routePath: sourceConfigPath),
  MenuEntry(
      key: 'processInput',
      label: 'Process Input Config',
      routePath: processInputPath),
  MenuEntry(
      key: 'processConfig',
      label: 'Process Configurations',
      routePath: processConfigPath),
  MenuEntry(
      key: 'pipelineConfig',
      label: 'Data Pipeline Config',
      routePath: pipelineConfigPath),
];

final Map<String, ScreenConfig> _screenConfigurations = {
  // Home Screen
  ScreenKeys.home: ScreenConfig(
      key: ScreenKeys.home,
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome to JetStore Workspace!',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Source Config Screen
  ScreenKeys.sourceConfig: ScreenConfig(
      key: ScreenKeys.sourceConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'File Source Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Domain Table Viewer Screen
  ScreenKeys.domainTableViewer: ScreenConfig(
      key: ScreenKeys.domainTableViewer,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File Staging Table',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Process Input & Mapping Screen
  ScreenKeys.processInput: ScreenConfig(
      key: ScreenKeys.processInput,
      appBarLabel: 'JetStore Workspace',
      title: 'Process Input and Mapping',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Process Config Screen
  ScreenKeys.processConfig: ScreenConfig(
      key: ScreenKeys.processConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Process and Client Rule Config',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Login Screen
  ScreenKeys.login: ScreenConfig(
      key: ScreenKeys.login,
      appBarLabel: 'JetStore Workspace',
      title: 'Please login',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: []),

  // Registration Screen
  ScreenKeys.register: ScreenConfig(
      key: ScreenKeys.register,
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome, please register',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: []),

  //* DEMOS
  "testScreen": ScreenConfig(
      key: "testScreen",
      appBarLabel: 'JetStore Workspace',
      title: 'Test Screen',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),
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
};

ScreenConfig getScreenConfig(String key) {
  var config = _screenConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: screen configuration $key not found');
  }
  return config;
}

import 'package:flutter/material.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/components/menu_delegates/menu_delegates.dart';

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

/// MenuActionDelegate is action function used by menu items
/// that does not require to navigate to a new form but perform the action
/// "in place" on the screen having the menu item
/// The functions are defined in menu_delegates folder
typedef MenuActionDelegate = void Function(BuildContext context);

class MenuEntry {
  MenuEntry({
    this.style = ActionStyle.primary,
    required this.key,
    required this.label,
    this.routePath,
    this.menuAction,
  });
  final ActionStyle style;
  final String key;
  final String label;
  final String? routePath;
  final MenuActionDelegate? menuAction;
}

final defaultMenuEntries = [
  MenuEntry(
      key: 'sourceConfig',
      label: 'File Staging Area',
      routePath: sourceConfigPath),
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
  MenuEntry(
      style: ActionStyle.danger,
      key: 'dataPurge',
      label: 'Purge Client Data',
      menuAction: purgeDataAction),
  MenuEntry(
      style: ActionStyle.danger,
      key: 'runInitDb',
      label: 'Run Workspace Database Initialization',
      menuAction: rerunDbInitAction),
];

final adminMenuEntries = [
  MenuEntry(
      style: ActionStyle.primary,
      key: 'userAdmin',
      label: 'User Administration',
      routePath: userAdminPath),
  MenuEntry(
      style: ActionStyle.danger,
      key: 'dataPurge',
      label: 'Purge Client Data',
      menuAction: purgeDataAction),
  MenuEntry(
      style: ActionStyle.danger,
      key: 'runInitDb',
      label: 'Run Workspace Database Initialization',
      menuAction: rerunDbInitAction),
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
      title: 'File Staging Area',
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

  // Pipeline Config Screen
  ScreenKeys.pipelineConfig: ScreenConfig(
      key: ScreenKeys.pipelineConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Pipeline Config',
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

  // UserAdmin Screen
  ScreenKeys.userAdmin: ScreenConfig(
      key: ScreenKeys.userAdmin,
      appBarLabel: 'JetStore Workspace',
      title: 'User Administration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: adminMenuEntries),

  ScreenKeys.fileRegistryTable: ScreenConfig(
      key: ScreenKeys.fileRegistryTable,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File as Table',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  ScreenKeys.execStatusDetailsTable: ScreenConfig(
      key: ScreenKeys.execStatusDetailsTable,
      appBarLabel: 'JetStore Workspace',
      title: 'Pipeline Execution Details',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  ScreenKeys.processErrorsTable: ScreenConfig(
      key: ScreenKeys.processErrorsTable,
      appBarLabel: 'JetStore Workspace',
      title: 'Pipeline Execution Errors',
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

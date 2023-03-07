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
    this.onPageStyle = ActionStyle.primary,
    this.otherPageStyle = ActionStyle.secondary,
    required this.key,
    required this.label,
    this.routePath,
    this.menuAction,
  });
  final ActionStyle onPageStyle;
  final ActionStyle otherPageStyle;
  final String key;
  final String label;
  final String? routePath;
  final MenuActionDelegate? menuAction;
}

final defaultMenuEntries = [
  MenuEntry(
      key: 'clientOrgAdmin',
      label: 'Clients and Organizations',
      routePath: clientAdminPath),
  MenuEntry(
      key: 'sourceConfig',
      label: 'File Staging Area',
      routePath: sourceConfigPath),
  MenuEntry(
      key: 'inputSourceMapping',
      label: 'Input Source Mapping',
      routePath: inputSourceMappingPath),
  MenuEntry(
      key: 'processInput',
      label: 'Process Input Configuration',
      routePath: processInputPath),
  MenuEntry(
      key: 'processConfig',
      label: 'Client Rules Configurations',
      routePath: processConfigPath),
  MenuEntry(
      key: 'pipelineConfig',
      label: 'Pipelines Configuration',
      routePath: pipelineConfigPath),
  MenuEntry(
      otherPageStyle: ActionStyle.danger,
      key: 'dataPurge',
      label: 'Purge Client Data',
      menuAction: purgeDataAction),
  MenuEntry(
      otherPageStyle: ActionStyle.danger,
      key: 'runInitDb',
      label: 'Run Workspace Database Initialization',
      menuAction: rerunDbInitAction),
];

final adminMenuEntries = [
  MenuEntry(
      key: 'userAdmin',
      label: 'User Administration',
      routePath: userAdminPath),
  MenuEntry(
      otherPageStyle: ActionStyle.danger,
      key: 'dataPurge',
      label: 'Purge Client Data',
      menuAction: purgeDataAction),
  MenuEntry(
      otherPageStyle: ActionStyle.danger,
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

  // Client & Organization Admin Screen
  ScreenKeys.clientAdmin: ScreenConfig(
      key: ScreenKeys.clientAdmin,
      appBarLabel: 'JetStore Workspace',
      title: 'Clients and Organizations Administration',
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

  // Input Source Mapping Screen
  ScreenKeys.inputSourceMapping: ScreenConfig(
      key: ScreenKeys.inputSourceMapping,
      appBarLabel: 'JetStore Workspace',
      title: 'Input Source Mapping',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Process Input Configuration Screen
  ScreenKeys.processInput: ScreenConfig(
      key: ScreenKeys.processInput,
      appBarLabel: 'JetStore Workspace',
      title: 'Process Input Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Rules Config Screen
  ScreenKeys.processConfig: ScreenConfig(
      key: ScreenKeys.processConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Rules Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries),

  // Pipeline Config Screen
  ScreenKeys.pipelineConfig: ScreenConfig(
      key: ScreenKeys.pipelineConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Pipelines Configuration',
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

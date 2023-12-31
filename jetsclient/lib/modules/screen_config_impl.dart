import 'package:jetsclient/modules/user_flows/client_registry/screen_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/screen_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/screen_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/screen_config.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/screen_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/screen_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/screen_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/modules/actions/menu_delegates.dart';
import 'package:jetsclient/models/screen_config.dart';
import 'package:jetsclient/modules/workspace_ide/screen_config.dart';

import 'package:jetsclient/routes/jets_routes_app.dart';

final defaultMenuEntries = [
  MenuEntry(key: 'jetstoreHome', label: 'JetStore Home', routePath: homePath),
  MenuEntry(
      key: 'clientOrgAdmin',
      label: 'Clients and Organizations',
      routePath: ufClientRegistryPath,
      routeParams: {FSK.ufStartAtKey: 'select_client'}),
  MenuEntry(
      key: 'sourceConfigUF',
      label: 'Client Files',
      routePath: ufSourceConfigPath,
      routeParams: {FSK.ufStartAtKey: 'select_source_config'}),
  MenuEntry(
      key: 'inputSourceMapping',
      label: 'File Mapping',
      routePath: ufFileMappingPath),
  MenuEntry(
      key: 'processConfig',
      label: 'Rules Configurations',
      routePath: ruleConfigPath),
  MenuEntry(
      key: 'pipelineConfig',
      label: 'Pipelines Configuration',
      routePath: ufPipelineConfigPath,
      routeParams: {FSK.ufStartAtKey: 'select_pipeline_config'}),
    MenuEntry(
        key: 'workspaceIDEHome',
        capability: 'workspace_ide',
        label: 'Workspace IDE Home',
        routePath: workspaceRegistryPath),
];

final adminMenuEntries = [
  MenuEntry(key: 'jetstoreHome', label: 'JetStore Home', routePath: homePath),
  MenuEntry(
      key: 'clientOrgAdmin',
      label: 'Clients and Organizations',
      routePath: ufClientRegistryPath,
      routeParams: {FSK.ufStartAtKey: 'select_client'}),
  MenuEntry(
      key: 'sourceConfigUF',
      label: 'Client Files',
      routePath: ufSourceConfigPath,
      routeParams: {FSK.ufStartAtKey: 'select_source_config'}),
  MenuEntry(
      key: 'inputSourceMapping',
      label: 'File Mapping',
      routePath: ufFileMappingPath),
  MenuEntry(
      key: 'processConfig',
      label: 'Rules Configurations',
      routePath: ruleConfigPath),
  MenuEntry(
      key: 'pipelineConfig',
      label: 'Pipelines Configuration',
      routePath: ufPipelineConfigPath,
      routeParams: {FSK.ufStartAtKey: 'select_pipeline_config'}),
  MenuEntry(
      key: 'workspaceIDEHome',
      label: 'Workspace IDE Home',
      routePath: workspaceRegistryPath),
  MenuEntry(
      key: 'userAdmin', label: 'User Administration', routePath: userAdminPath),
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

final toolbarMenuEntries = [
  MenuEntry(
      key: 'clientRegistryUF',
      label: 'Client / Vendor',
      routePath: ufClientRegistryPath),
  MenuEntry(
      key: 'sourceConfigUF',
      label: 'File Configuration',
      routePath: ufSourceConfigPath),
  MenuEntry(
      key: 'fileMappingUF',
      label: 'File Mapping',
      routePath: ufFileMappingPath),
  MenuEntry(
      key: 'pipelineConfigUF',
      label: 'Pipeline Configuration',
      routePath: ufPipelineConfigPath),
  MenuEntry(
      key: 'Spacer01',
      label: ''),
  MenuEntry(
      key: 'loaderUF',
      label: 'Load Files',
      routePath: ufLoadFilesPath),
  MenuEntry(
      key: 'startPipelineUF',
      label: 'Start Pipeline',
      routePath: ufStartPipelinePath),
  // MenuEntry(
  //     key: 'Spacer02',
  //     label: ''),
  // MenuEntry(
  //     key: 'dataRegistryUF',
  //     label: 'View Data',
  //     routePath: ufSourceConfigPath),
];

final Map<String, ScreenConfig> _screenConfigurations = {
  // Home Screen
  ScreenKeys.home: ScreenConfig(
      key: ScreenKeys.home,
      appBarLabel: 'JetStore Workspace',
      title: '',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Source Config Screen
  ScreenKeys.sourceConfig: ScreenConfig(
      key: ScreenKeys.sourceConfig,
      appBarLabel: 'JetStore Workspace',
      title: 'Client Files',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Domain Table Viewer Screen
  ScreenKeys.domainTableViewer: ScreenConfig(
      key: ScreenKeys.domainTableViewer,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File Staging Table',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Query Tool Screen
  ScreenKeys.queryToolScreen: ScreenConfig(
      key: ScreenKeys.queryToolScreen,
      appBarLabel: 'JetStore Workspace',
      title: 'Query Tool',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: workspaceRegistryMenuEntries,
      adminMenuEntries: workspaceRegistryMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries,
      type: ScreenType.other),

  // Input Source Mapping Screen
  ScreenKeys.inputSourceMapping: ScreenConfig(
      key: ScreenKeys.inputSourceMapping,
      appBarLabel: 'JetStore Workspace',
      // title: 'Input Source Mapping',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Rules Config Screen
  ScreenKeys.processConfig: ScreenConfig(
      key: ScreenKeys.processConfig,
      appBarLabel: 'JetStore Workspace',
      // title: 'Rules Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Rule Configv2 Screen
  ScreenKeys.ruleConfigv2: ScreenConfig(
      key: ScreenKeys.ruleConfigv2,
      appBarLabel: 'JetStore Workspace',
      // title: 'Rules Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Pipeline Config Edit Screen
  ScreenKeys.pipelineConfigEdit: ScreenConfig(
      key: ScreenKeys.pipelineConfigEdit,
      appBarLabel: 'JetStore Workspace',
      title: 'Edit Pipelines Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // Login Screen
  ScreenKeys.login: ScreenConfig(
      key: ScreenKeys.login,
      type: ScreenType.other,
      appBarLabel: 'JetStore Workspace',
      title: 'Please login',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [],
      adminMenuEntries: []),

  // Registration Screen
  ScreenKeys.register: ScreenConfig(
      key: ScreenKeys.register,
      type: ScreenType.other,
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome, please register',
      showLogout: false,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [],
      adminMenuEntries: []),

  // Git User Profile Screen
  ScreenKeys.userGitProfile: ScreenConfig(
      key: ScreenKeys.userGitProfile,
      appBarLabel: 'JetStore Workspace',
      title: 'Edit Git Profile',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  // UserAdmin Screen
  ScreenKeys.userAdmin: ScreenConfig(
      key: ScreenKeys.userAdmin,
      type: ScreenType.other,
      appBarLabel: 'JetStore Workspace',
      title: 'User Administration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: adminMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  ScreenKeys.fileRegistryTable: ScreenConfig(
      key: ScreenKeys.fileRegistryTable,
      appBarLabel: 'JetStore Workspace',
      title: 'Staging Table or Domain Table View',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  ScreenKeys.filePreview: ScreenConfig(
      key: ScreenKeys.filePreview,
      appBarLabel: 'JetStore Workspace',
      title: 'Input File Preview',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  ScreenKeys.execStatusDetailsTable: ScreenConfig(
      key: ScreenKeys.execStatusDetailsTable,
      appBarLabel: 'JetStore Workspace',
      // title: 'Pipeline Execution Details',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),

  ScreenKeys.processErrorsTable: ScreenConfig(
      key: ScreenKeys.processErrorsTable,
      appBarLabel: 'JetStore Workspace',
      // title: 'Pipeline Execution Errors',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};

ScreenConfig getScreenConfig(String key) {
  var config = _screenConfigurations[key];
  if (config != null) return config;
  config = getWorkspaceScreenConfig(key);
  if (config != null) return config;
  config = getClientRegistryScreenConfig(key);
  if (config != null) return config;
  config = getConfigureFileScreenConfig(key);
  if (config != null) return config;
  config = getFileMappingScreenConfig(key);
  if (config != null) return config;
  config = getPipelineConfigScreenConfig(key);
  if (config != null) return config;
  config = getLoadFilesScreenConfig(key);
  if (config != null) return config;
  config = getStartPipelineScreenConfig(key);
  if (config != null) return config;
  config = getWorkspacePullScreenConfig(key);
  if (config != null) return config;

  throw Exception(
      'ERROR: Invalid program configuration: screen configuration $key not found');
}

import '../routes/jets_routes_app.dart';
import 'data_table_config.dart';

class ScreenConfig {
  ScreenConfig(
      {required this.key,
      required this.appBarLabel,
      required this.title,
      required this.showLogout,
      required this.leftBarLogo,
      required this.menuEntries,
      required this.tableConfig});
  final String key;
  final String appBarLabel;
  final String title;
  final bool showLogout;
  final String leftBarLogo;
  final List<MenuEntry> menuEntries;
  final TableConfig tableConfig;
}

class MenuEntry {
  MenuEntry({required this.key, required this.label, required this.routePath});
  final String key;
  final String label;
  final String routePath;
}

final Map<String, ScreenConfig> _screenConfigurations = {
  'jobListScreen': ScreenConfig(
      key: 'jobListScreen',
      appBarLabel: 'JetStore Workspace',
      title: 'Data Pipelines',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [
        MenuEntry(key: 'inputFiles', label: 'Input Files', routePath: fileRegistryPath),
        MenuEntry(
            key: 'mappingConfig',
            label: 'Mapping Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'processConfig',
            label: 'Process Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'jobList', label: 'Data Pipeline', routePath: jobListPath),
      ],
      tableConfig: getTableConfig('pipelineTable')),
  'fileRegistryScreen': ScreenConfig(
      key: 'fileRegistryScreen',
      appBarLabel: 'JetStore Workspace',
      title: 'Input File Registry',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [
        MenuEntry(key: 'inputFiles', label: 'Input Files', routePath: fileRegistryPath),
        MenuEntry(
            key: 'mappingConfig',
            label: 'Mapping Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'processConfig',
            label: 'Process Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'jobList', label: 'Data Pipeline', routePath: jobListPath),
      ],
      tableConfig: getTableConfig('registryTable')),
  'fileRegistryTableScreen': ScreenConfig(
      key: 'fileRegistryTableScreen',
      appBarLabel: 'JetStore Workspace',
      title: 'Input File as Table',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [
        MenuEntry(key: 'inputFiles', label: 'Input Files', routePath: fileRegistryPath),
        MenuEntry(
            key: 'mappingConfig',
            label: 'Mapping Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'processConfig',
            label: 'Process Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'jobList', label: 'Data Pipeline', routePath: jobListPath),
      ],
      tableConfig: getTableConfig('inputTable')),
  'homeScreen': ScreenConfig(
      key: 'homeScreen',
      appBarLabel: 'JetStore Workspace',
      title: 'Welcome to JetStore Workspace!',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [
        MenuEntry(key: 'inputFiles', label: 'Input Files', routePath: fileRegistryPath),
        MenuEntry(
            key: 'mappingConfig',
            label: 'Mapping Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'processConfig',
            label: 'Process Configurations',
            routePath: 'TODO'),
        MenuEntry(
            key: 'jobList', label: 'Data Pipelines', routePath: jobListPath),
      ],
      tableConfig: getTableConfig('userTable')),
};

ScreenConfig getScreenConfig(String key) {
  var config = _screenConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: screen configuration $key not found');
  }
  return config;
}

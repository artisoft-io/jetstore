import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

//*TODO Take path params from current Navigator provider (current page)
final List<MenuEntry> workspaceRegistryMenuEntries = [
  MenuEntry(
      key: 'jetstoreHome',
      label: 'JetStore Home',
      routePath: homePath),
  MenuEntry(
      key: 'workspaceIDEHome',
      label: 'Workspace IDE Home',
      routePath: workspaceRegistryPath),
  MenuEntry(
      key: 'queryTool',
      label: 'Query Tool',
      routePath: queryToolPath),
];

final Map<String, ScreenConfig> _screenConfigurations = {
  // workspaceRegistry Screen
  ScreenKeys.workspaceRegistry: ScreenConfig(
      key: ScreenKeys.workspaceRegistry,
      type: ScreenType.other,
      appBarLabel: 'JetStore Workspace IDE',
      title: '',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: workspaceRegistryMenuEntries,
      adminMenuEntries: workspaceRegistryMenuEntries),

  // Workspace IDE Home Screen
  ScreenKeys.workspaceHome: ScreenConfig(
      key: ScreenKeys.workspaceHome,
      type: ScreenType.workspace,
      appBarLabel: 'JetStore Workspace IDE',
      title: 'Workspace Home',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: [],
      adminMenuEntries: []),
};


ScreenConfig? getWorkspaceScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

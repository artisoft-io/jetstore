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
  // Disabled rather than hidden without the capability, as base_screen does for every
  // menu entry. The api endpoint enforces the same capability independently.
  MenuEntry(
      key: 'inferServerAdmin',
      label: 'Infer Server Admin',
      capability: 'infer_server_admin',
      routePath: inferServerAdminPath),
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

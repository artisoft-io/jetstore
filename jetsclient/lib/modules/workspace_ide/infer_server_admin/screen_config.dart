import 'package:jetsclient/models/screen_config.dart';
import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/modules/workspace_ide/screen_config.dart';
import 'package:jetsclient/utils/constants.dart';

/// Screen Config for the Infer Server Admin screen.
///
/// Carries the Workspace IDE menu, like the Query Tool screen does, so the IDE navigation
/// stays put while the screen is open.
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.inferServerAdminScreen: ScreenConfig(
      key: ScreenKeys.inferServerAdminScreen,
      appBarLabel: 'JetStore Workspace',
      title: 'Infer Server Admin',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: workspaceRegistryMenuEntries,
      adminMenuEntries: workspaceRegistryMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries,
      type: ScreenType.other),
};

ScreenConfig? getInferServerAdminScreenConfig(String key) {
  return _screenConfigurations[key];
}

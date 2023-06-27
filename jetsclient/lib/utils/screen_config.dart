import 'package:flutter/material.dart';
import 'package:jetsclient/utils/constants.dart';

/// Screen type, determine mainly the left panel content
enum ScreenType { home, workspace, other }

class ScreenConfig {
  ScreenConfig(
      {required this.key,
      this.type = ScreenType.home,
      required this.appBarLabel,
      required this.title,
      required this.showLogout,
      required this.leftBarLogo,
      required this.menuEntries,
      required this.adminMenuEntries});
  final ScreenType type;
  final String key;
  final String appBarLabel;
  final String title;
  final bool showLogout;
  final String leftBarLogo;
  final List<MenuEntry> menuEntries;
  final List<MenuEntry> adminMenuEntries;
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
    this.children = const [],
  });
  final ActionStyle onPageStyle;
  final ActionStyle otherPageStyle;
  final String key;
  final String label;
  final String? routePath;
  final MenuActionDelegate? menuAction;
  List<MenuEntry> children;
}

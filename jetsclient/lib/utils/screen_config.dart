import 'package:flutter/material.dart';
import 'package:jetsclient/utils/constants.dart';

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

import 'package:flutter/material.dart';
import 'package:jetsclient/utils/constants.dart';

/// Screen type, determine mainly the left panel content
enum ScreenType { home, workspace, other }

class ScreenConfig {
  ScreenConfig({
    required this.key,
    this.type = ScreenType.home,
    required this.appBarLabel,
    this.title,
    required this.showLogout,
    required this.leftBarLogo,
    required this.menuEntries,
    required this.adminMenuEntries,
    this.toolbarMenuEntries = const[],
  });
  final ScreenType type;
  final String key;
  final String appBarLabel;
  final String? title;
  final bool showLogout;
  final String leftBarLogo;
  final List<MenuEntry> menuEntries;
  final List<MenuEntry> adminMenuEntries;
  final List<MenuEntry> toolbarMenuEntries;
}

/// MenuActionDelegate is action function used by menu items
/// that does not require to navigate to a new form but perform the action
/// "in place" on the screen having the menu item
/// The functions are defined in menu_delegates folder
typedef MenuActionDelegate = Future<int> Function(
    BuildContext context, MenuEntry menuEntry, State<StatefulWidget> state);

/// MenuConfig
/// MenuConfig.formConfigKey is used by ScreenWithTabsWithForm
/// where each tab can have a different formConfig
/// and the routing is done within the same page using
/// the menuAction
class MenuEntry {
  MenuEntry({
    this.onPageStyle = ActionStyle.tbPrimary,
    this.otherPageStyle = ActionStyle.tbSecondary,
    required this.key,
    required this.label,
    this.routePath,
    this.pageMatchKey,
    this.routeParams,
    this.menuAction,
    this.formConfigKey,
    this.children = const [],
    this.capability,
  });
  final ActionStyle onPageStyle;
  final ActionStyle otherPageStyle;
  final String key;
  final String label;
  final String? routePath;
  // PageMatchKey is a value to match menuItem and page on screen
  // by matching value between menuItem and a value placed in current page route
  // parameters (this is used by screens having mutiple formConfig (virtual pages))
  final String? pageMatchKey;
  final Map<String, dynamic>? routeParams;
  final MenuActionDelegate? menuAction;
  final String? formConfigKey;
  List<MenuEntry> children;
  final String? capability;
}

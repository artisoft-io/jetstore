import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_tab_controller.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/screen_config.dart';
import 'package:split_view/split_view.dart';
import 'package:flutter_simple_treeview/flutter_simple_treeview.dart';

/// Signature for building the widget of main area of BaseScreen.
typedef ScreenWidgetBuilder = Widget Function(
    BuildContext context, BaseScreenState baseScreenState);

class BaseScreen extends StatefulWidget {
  const BaseScreen({
    super.key,
    required this.screenPath,
    required this.screenConfig,
    required this.builder,
  });

  final JetsRouteData screenPath;
  final ScreenConfig screenConfig;
  final ScreenWidgetBuilder builder;

  @override
  State<BaseScreen> createState() => BaseScreenState();
}

class BaseScreenState extends State<BaseScreen> with TickerProviderStateMixin {
  final tabsStateHelper = JetsTabsStateHelper();
  late var tabController = TabController(length: 0, vsync: this);

  @override
  void initState() {
    // print("*** base_screen initState called with key ${widget.key}");
    super.initState();
    JetsRouterDelegate().addListener(navListener);
    tabsStateHelper.addListener(navListener);
  }

  void navListener() async {
    // print("*** BaseScreenState navListener called...");
    if (JetsRouterDelegate().currentConfiguration?.path ==
            widget.screenPath.path &&
        mounted) {
      setState(() {});
    }
  }

  void syncMenuWithTab() {
    // print("*** BaseScreenState * syncMenuWithTab called with tabIndex ${tabController.index}");
    // Put the file name in current route config so the menu gets highlighted
    JetsRouterDelegate().currentConfiguration!.params[FSK.pageMatchKey] =
        tabsStateHelper.tabsParams[tabController.index].pageMatchKey;
    setState(() {});
  }

  void removeRecursive(MenuEntry menuEntry, int index) {
    final tabIndex = menuEntry.routeParams?['tab.index'] as int?;
    if (tabIndex != null && tabIndex >= index) {
      if (tabIndex == index) {
        // Got it, erase the tab.index
        menuEntry.routeParams!['tab.index'] = null;
      } else {
        // reduce tabIndex by 1
        menuEntry.routeParams!['tab.index'] = tabIndex - 1;
      }
    }
    for (var childEntry in menuEntry.children) {
      removeRecursive(childEntry, index);
    }
  }

  void removeTab(int index) {
    // print(
    //     "*** removeTab called @ $index with ${JetsRouterDelegate().workspaceMenuState.length}");
    for (var menuEntry in JetsRouterDelegate().workspaceMenuState) {
      removeRecursive(menuEntry, index);
    }
    tabsStateHelper.removeTab(index: index);
    resetTabController(
        index > 0 ? index - 1 : 0, tabsStateHelper.tabsParams.length);
    syncMenuWithTab();
  }

  void resetTabController(int initialIndex, int length) {
    tabController.removeListener(syncMenuWithTab);
    tabController.dispose();
    tabController =
        TabController(length: length, initialIndex: initialIndex, vsync: this);
    tabController.addListener(syncMenuWithTab);
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(navListener);
    tabController.removeListener(syncMenuWithTab);
    tabController.dispose();
    tabsStateHelper.removeListener(navListener);
    super.dispose();
  }

  ActionStyle getActionStyle(MenuEntry menuEntry) {
    final routeData = JetsRouterDelegate().currentConfiguration;
    if (routeData == null) return menuEntry.otherPageStyle;
    if (menuEntry.pageMatchKey == null) {
      return routeData.path == menuEntry.routePath
          ? menuEntry.onPageStyle
          : menuEntry.otherPageStyle;
    }
    return routeData.params[FSK.pageMatchKey] == menuEntry.pageMatchKey
        ? menuEntry.onPageStyle
        : menuEntry.otherPageStyle;
  }

  // Note: The menuAction may do the routing, hence doing menuAction first.
  //       If no menuAction, do routing only if defined, otherwise do nothing
  void doMenuOnPress(MenuEntry menuEntry) async {
    if (menuEntry.menuAction != null) {
      final statusCode = await menuEntry.menuAction!(context, menuEntry, this);
      if (statusCode != 200 && context.mounted) {
        showAlertDialog(context, 'Something went wrong. Please try again.');
      }
      navListener();
    } else if (menuEntry.routePath != null) {
      JetsRouterDelegate()(
          JetsRouteData(menuEntry.routePath!, params: menuEntry.routeParams));
    }
  }

  TreeNode _makeTreeNode(int level, BuildContext context, ThemeData themeData,
      MenuEntry menuEntry) {
    List<TreeNode>? childs = menuEntry.children.isNotEmpty
        ? menuEntry.children
            .map((e) => _makeTreeNode(level + 1, context, themeData, e))
            .toList()
        : null;
    // Note: The menuAction may do the routing, hence doing menuAction first.
    //       If no menuAction, do routing only if defined, otherwise do nothing
    final doIt = JetsRouterDelegate().user.isAdmin ||
        (menuEntry.capability == null ||
            JetsRouterDelegate().user.hasCapability(menuEntry.capability!));
    return TreeNode(
        content: (level == 0)
            ? Expanded(
                child: ElevatedButton(
                  style: buttonStyle(getActionStyle(menuEntry), themeData),
                  onPressed: doIt ? () => doMenuOnPress(menuEntry) : null,
                  child: Center(child: Text(menuEntry.label)),
                ),
              )
            : Expanded(
                child: TextButton(
                  style: buttonStyle(getActionStyle(menuEntry), themeData),
                  onPressed: doIt ? () => doMenuOnPress(menuEntry) : null,
                  child: Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        menuEntry.label,
                        maxLines: 3,
                        overflow: TextOverflow.ellipsis,
                      )),
                ),
              ),
        children: childs);
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData themeData = Theme.of(context);
    final dropdownItems = [DropdownItemConfig(label: 'Select client')];
    dropdownItems.addAll(JetsRouterDelegate().clients);
    List<MenuEntry> menuEntries = [];

    switch (widget.screenConfig.type) {
      case ScreenType.home:
        // Home screen and all (pipeline) config & operational pages
        menuEntries = JetsRouterDelegate().user.isAdmin
            ? widget.screenConfig.adminMenuEntries
            : widget.screenConfig.menuEntries;
        break;
      case ScreenType.other:
        menuEntries = JetsRouterDelegate().user.isAdmin
            ? widget.screenConfig.adminMenuEntries
            : widget.screenConfig.menuEntries;
        break;
      case ScreenType.workspace:
        // All workspace IDE screens
        // All screens with workspace content in left menu tree
        menuEntries = JetsRouterDelegate().workspaceMenuState;
        break;
      default:
        // unknown
        print(
            'Oops unknown widget.screenConfig.type: ${widget.screenConfig.type}');
    }
    // print("*** BUILDING BaseScreen");
    return Scaffold(
        appBar: appBar(
            context, widget.screenConfig.appBarLabel, widget.screenConfig,
            showLogout: widget.screenConfig.showLogout),
        body: SplitView(
          // SplitView: Left menu & client area
          viewMode: SplitViewMode.Horizontal,
          indicator: const SplitIndicator(viewMode: SplitViewMode.Horizontal),
          activeIndicator: const SplitIndicator(
            viewMode: SplitViewMode.Horizontal,
            isActive: true,
          ),
          controller: SplitViewController(
              weights: JetsRouterDelegate().splitViewControllerWeights ??
                  [0.2, 0.8]),
          onWeightChanged: (w) =>
              JetsRouterDelegate().splitViewControllerWeights = w,
          children: [
            // Left menu
            Column(children: [
              const SizedBox(height: defaultPadding),
              // JetStore logo as button to home screen
              Expanded(
                  flex: 3,
                  child: ConstrainedBox(
                      constraints: const BoxConstraints.expand(),
                      child: IconButton(
                          onPressed: () =>
                              JetsRouterDelegate()(JetsRouteData(homePath)),
                          padding: const EdgeInsets.all(0.0),
                          icon: Image.asset(widget.screenConfig.leftBarLogo)))),
              const SizedBox(height: defaultPadding),
              // Client filter drop down in left menu
              Expanded(
                flex: 1,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(40.0, 0.0, 0.0, 0.0),
                  child: DropdownButtonFormField<String>(
                      value: JetsRouterDelegate().selectedClient,
                      onChanged: (String? newValue) {
                        setState(() {
                          JetsRouterDelegate().selectedClient = newValue;
                        });
                      },
                      items: dropdownItems
                          .map((e) => DropdownMenuItem<String>(
                              value: e.value, child: Text(e.label)))
                          .toList()),
                ),
              ),
              // Left menu as TreeView
              Expanded(
                flex: 24,
                child: SingleChildScrollView(
                  child: TreeView(
                      nodes: menuEntries
                          .map((menuEntry) =>
                              _makeTreeNode(0, context, themeData, menuEntry))
                          .toList()),
                ),
              )
            ]),
            // Client area
            JetsSpinnerOverlay(child: widget.builder(context, this)),
          ],
        ));
  }
}

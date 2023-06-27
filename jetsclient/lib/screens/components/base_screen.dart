import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/screen_config.dart';
import 'package:jetsclient/utils/screen_config_impl.dart';
import 'package:split_view/split_view.dart';
import 'package:flutter_simple_treeview/flutter_simple_treeview.dart';

/// Signature for building the widget of main area of BaseScreen.
typedef ScreenWidgetBuilder = Widget Function(BaseScreenState baseScreenState);

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

class BaseScreenState extends State<BaseScreen> {
  @override
  void initState() {
    super.initState();
    JetsRouterDelegate().addListener(navListener);
  }

  void navListener() async {
    if (JetsRouterDelegate().currentConfiguration?.path == homePath &&
        mounted) {
      setState(() {});
    }
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(navListener);
    super.dispose();
  }

  TreeNode _makeTreeNode(int level, BuildContext context, ThemeData themeData,
      MenuEntry menuEntry) {
    List<TreeNode>? childs = menuEntry.children.isNotEmpty
        ? menuEntry.children
            .map((e) => _makeTreeNode(level + 1, context, themeData, e))
            .toList()
        : null;
    fn() => menuEntry.routePath != null
        ? JetsRouterDelegate()(JetsRouteData(menuEntry.routePath!))
        : menuEntry.menuAction != null
            ? menuEntry.menuAction!(context)
            : null;
    return TreeNode(
        content: (level == 0)
            ? Expanded(
                child: ElevatedButton(
                  style: buttonStyle(
                      JetsRouterDelegate().currentConfiguration?.path ==
                              menuEntry.routePath
                          ? menuEntry.onPageStyle
                          : menuEntry.otherPageStyle,
                      themeData),
                  onPressed: fn,
                  child: Center(child: Text(menuEntry.label)),
                ),
              )
            : Expanded(
                child: TextButton(
                  onPressed: fn,
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
    var dropdownItems = [DropdownItemConfig(label: 'Select client')];
    List<MenuEntry> menuEntries = [];

    switch (widget.screenConfig.type) {
      case ScreenType.home:
        dropdownItems.addAll(JetsRouterDelegate().clients);
        JetsRouterDelegate().workspaceMenuState = [];
        menuEntries = JetsRouterDelegate().user.isAdmin
            ? widget.screenConfig.adminMenuEntries
            : widget.screenConfig.menuEntries;
        break;
      case ScreenType.other:
        JetsRouterDelegate().selectedClient = null;
        JetsRouterDelegate().workspaceMenuState = [];
        menuEntries = JetsRouterDelegate().user.isAdmin
            ? widget.screenConfig.adminMenuEntries
            : widget.screenConfig.menuEntries;
        break;
      case ScreenType.workspace:
        JetsRouterDelegate().selectedClient = null;
        menuEntries = JetsRouterDelegate().workspaceMenuState;
        break;
      default:
        // unknown
        print(
            'Oops unknown widget.screenConfig.type: ${widget.screenConfig.type}');
    }

    return Scaffold(
        appBar: appBar(
            context, widget.screenConfig.appBarLabel, widget.screenConfig,
            showLogout: widget.screenConfig.showLogout),
        body: SplitView(
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
            Column(children: [
              const SizedBox(height: defaultPadding),
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
              if (widget.screenConfig.type == ScreenType.home)
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
            Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Flexible(
                flex: 1,
                fit: FlexFit.tight,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(
                      defaultPadding, 2 * defaultPadding, 0, 0),
                  child: Text(
                    widget.screenConfig.title,
                    style: Theme.of(context).textTheme.headlineMedium,
                  ),
                ),
              ),
              Flexible(
                flex: 8,
                fit: FlexFit.tight,
                child: widget.builder(this),
              ),
            ]),
          ],
        ));
  }
}

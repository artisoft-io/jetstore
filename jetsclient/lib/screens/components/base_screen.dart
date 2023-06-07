import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/screen_config.dart';
import 'package:jetsclient/utils/screen_config_impl.dart';

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

  @override
  Widget build(BuildContext context) {
    final ThemeData themeData = Theme.of(context);
    final menuEntries = JetsRouterDelegate().user.isAdmin
        ? adminMenuEntries
        : widget.screenConfig.menuEntries;
    return Scaffold(
      appBar: appBar(
          context, widget.screenConfig.appBarLabel, widget.screenConfig,
          showLogout: widget.screenConfig.showLogout),
      body: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Flexible(
            flex: 1,
            fit: FlexFit.tight,
            child: Column(children: [
              const SizedBox(height: defaultPadding),
              Expanded(
                  flex: 1,
                  child: ConstrainedBox(
                      constraints: const BoxConstraints.expand(),
                      child: IconButton(
                          onPressed: () =>
                              JetsRouterDelegate()(JetsRouteData(homePath)),
                          padding: const EdgeInsets.all(0.0),
                          icon: Image.asset(widget.screenConfig.leftBarLogo)))),
              const SizedBox(height: defaultPadding),
              Expanded(
                  flex: 8,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(defaultPadding),
                    itemCount: menuEntries.length,
                    itemBuilder: (BuildContext context, int index) {
                      final menuEntry = menuEntries[index];
                      return ElevatedButton(
                        style: buttonStyle(
                            JetsRouterDelegate().currentConfiguration?.path ==
                                    menuEntry.routePath
                                ? menuEntry.onPageStyle
                                : menuEntry.otherPageStyle,
                            themeData),
                        onPressed: () => menuEntry.routePath != null
                            ? JetsRouterDelegate()(
                                JetsRouteData(menuEntry.routePath!))
                            : menuEntry.menuAction != null
                                ? menuEntry.menuAction!(context)
                                : null,
                        child: Center(child: Text(menuEntry.label)),
                      );
                    },
                    separatorBuilder: (BuildContext context, int index) =>
                        const Divider(),
                  ))
            ]),
          ),
          Flexible(
            flex: 4,
            fit: FlexFit.tight,
            child:
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
          ),
        ],
      ),
    );
  }
}

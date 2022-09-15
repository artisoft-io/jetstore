import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/screen_config.dart';

AppBar appBar(BuildContext context, String title, ScreenConfig screenConfig,
    {bool showLogout = false}) {
  final appTitle = JetsRouterDelegate().devMode
      ? "$title (DEV MODE)"
      : JetsRouterDelegate().user.isAdmin
          ? "$title (ADMIN)"
          : title;
  final ThemeData themeData = Theme.of(context);
  return AppBar(
    automaticallyImplyLeading:
        screenConfig.key == ScreenKeys.login ? false : true,
    title: Text(appTitle),
    actions: <Widget>[
      ElevatedButton(
          style: JetsRouterDelegate().isDarkMode(context)
              ? buttonStyle(ActionStyle.primary, themeData)
              : null,
          onPressed: () {},
          child: Center(child: Text(JetsRouterDelegate().user.name ?? ''))),
      IconButton(
        icon: const Icon(Icons.dark_mode_sharp),
        tooltip: 'Toggle Theme',
        onPressed: () {
          AdaptiveTheme.of(context).toggleThemeMode();
        },
      ),
      if (showLogout)
        IconButton(
          icon: const Icon(Icons.logout_sharp),
          tooltip: 'Log Out',
          onPressed: () {
            var user = UserModel();
            user.name = "";
            user.email = "";
            user.password = "";
            user.token = "";
            user.isAdmin = false;
            JetsRouterDelegate().user = user;
            JetsRouterDelegate()(JetsRouteData(loginPath));
          },
        ),
    ],
  );
}

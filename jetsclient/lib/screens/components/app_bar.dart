import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';

AppBar appBar(BuildContext context, String title, {bool showLogout = false}) {
  return AppBar(
    automaticallyImplyLeading: false,
    title: Text(title),
    actions: <Widget>[
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
            JetsRouterDelegate().user = user;
            JetsRouterDelegate()(JetsRouteData(loginPath));
          },
        ),
    ],
  );
}

void showAlertDialog(BuildContext context, String message) {
  showDialog<void>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text(message),
      actions: [
        TextButton(
          child: const Text('OK'),
          onPressed: () => Navigator.of(context).pop(),
        ),
      ],
    ),
  );
}

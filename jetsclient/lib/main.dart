import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';

import 'package:jetsclient/routes/jets_route_information_parser.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';

final jetsRouteDelegate = JetsRouterDelegate();
final jetsRouteInformationParser = JetsRouteInformationParser();

void main() {
  runApp(const JetsClient(serverOrigin: 'http://localhost:8080'));
}

class JetsClient extends StatefulWidget {
  final String serverOrigin;
  const JetsClient({required this.serverOrigin, super.key});

  @override
  State<JetsClient> createState() => JetsClientState();
}

class JetsClientState extends State<JetsClient> {
  // This widget is the root of your application.
  @override
  Widget build(BuildContext context) {
    return AdaptiveTheme(
      light: ThemeData(
          brightness: Brightness.light,
          // colorSchemeSeed: const Color.fromRGBO(118, 219, 21, 1.0)),
          colorSchemeSeed: const Color.fromARGB(125, 11, 137, 215)),
          // colorSchemeSeed: const Color.fromARGB(255, 137, 28, 63)),
      dark: ThemeData(
          brightness: Brightness.dark,
          // colorSchemeSeed: const Color.fromARGB(255, 137, 28, 63)),
          colorSchemeSeed: const Color.fromRGBO(53, 69, 79, 1.0)),
          // colorSchemeSeed: const Color.fromRGBO(118, 219, 21, 1.0)),
      initial: AdaptiveThemeMode.light,
      builder: (theme, darkTheme) => MultiProvider(
        providers: [
          // http Client
          Provider(create: (context) => HttpClient(widget.serverOrigin)),
        ],
        child: MaterialApp.router(
          title: 'JetStore',
          theme: theme,
          routerDelegate: jetsRouteDelegate,
          routeInformationParser: jetsRouteInformationParser,
        ),
      ),
    );
  }
}

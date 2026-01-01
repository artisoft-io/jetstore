import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:flutter/foundation.dart' as foundation;
import 'package:jetsclient/http_client.dart';

import 'package:jetsclient/routes/jets_route_information_parser.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';

final jetsRouteDelegate = JetsRouterDelegate();
final jetsRouteInformationParser = JetsRouteInformationParser();

void main() {
  final uri = Uri.base;
  final protocol = '${uri.scheme}:';
  // Use a consistent String type for port to avoid type issues.
  final String port =
      foundation.kDebugMode ? '8080' : (uri.hasPort ? uri.port.toString() : '');
  final hostname = uri.host; // you probably need this one
  // print("url: $url");
  // print("protocol: $protocol");
  // print("hostname: $hostname");
  // print("port: $port");
  // Build origin correctly when the port is empty (default HTTP/HTTPS).
  final serverOrigin =
      port.isEmpty ? "$protocol//$hostname" : "$protocol//$hostname:$port";
  HttpClientSingleton().serverAdd = Uri.parse(serverOrigin);

  // FlutterError.onError = (details) {
  //   print("#### GOT ERROR $details");
  // };

  runApp(JetsClient(serverOrigin: serverOrigin));
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
          colorSchemeSeed: const Color.fromARGB(125, 11, 137, 215),
          fontFamily: 'Roboto'),
      dark: ThemeData(
          brightness: Brightness.dark,
          // colorSchemeSeed: const Color.fromARGB(255, 137, 28, 63)),
          colorSchemeSeed: const Color.fromRGBO(53, 69, 79, 1.0),
          fontFamily: 'Roboto'),
      // colorSchemeSeed: const Color.fromRGBO(118, 219, 21, 1.0)),
      initial: AdaptiveThemeMode.light,
      builder: (theme, darkTheme) => MaterialApp.router(
        title: 'JetStore',
        theme: theme,
        routerDelegate: jetsRouteDelegate,
        routeInformationParser: jetsRouteInformationParser,
      ),
    );
  }
}

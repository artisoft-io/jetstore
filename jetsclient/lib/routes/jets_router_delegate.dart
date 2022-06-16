import 'package:flutter/material.dart';
import 'dart:async';
import 'jets_route_information_parser.dart';
import 'jets_routes_app.dart';
import 'jets_route_data.dart';
import 'package:jetsclient/models/user.dart';

class JetsRouterDelegate extends RouterDelegate<JetsRouteData>
    with ChangeNotifier, PopNavigatorRouterDelegateMixin<JetsRouteData> {
  factory JetsRouterDelegate() => _singlton;
  static final JetsRouterDelegate _singlton = JetsRouterDelegate._();
  JetsRouterDelegate._() : _navigatorKey = GlobalKey<NavigatorState>() {
    _buildListPages();
  }

  final GlobalKey<NavigatorState> _navigatorKey;
  JetsRouteData routeData = JetsRouteData(homePath);

  var user = UserModel();
  List<MaterialPage> _pages = [];
  Map<String, List<MaterialPage>> routesPagesMap = {};
  Map<String, String> routePrevMap = {};

  @override
  GlobalKey<NavigatorState>? get navigatorKey => _navigatorKey;

  @override
  JetsRouteData? get currentConfiguration => routeData;

  void call(JetsRouteData appRoute) {
    routeData = appRoute;
    //*
    print(
        "call called authRequired is ${appRoute.authRequired} and token is ${user.token}");
    if (appRoute.authRequired && (user.token == null || user.token!.isEmpty)) {
      _pages = routesPagesMap["/login"]!;
    } else {
      _pages = routesPagesMap[appRoute.path]!;
    }

    notifyListeners();
  }

  @override
  Widget build(BuildContext context) => Navigator(
        key: navigatorKey,
        pages: _pages.isEmpty
            ? [MaterialPage(child: Container(color: Colors.blueGrey[900]))]
            : _pages,
        onPopPage: (route, result) {
          if (!route.didPop(result)) return false;

          _onpop();

          notifyListeners();
          return true;
        },
      );

  @override
  Future<void> setNewRoutePath(JetsRouteData configuration) async {
    routeData = configuration;
    print(
        "*** setNewRoutePath called with path ${configuration.path}, authRequired? ${configuration.authRequired}");
    _pages = routesPagesMap[configuration.path]!;
  }

  void _onpop() {
    var newPath = routePrevMap[routeData.path];

    List<String> pathList = Uri.parse(newPath!).pathSegments;

    var pathString = '';

    for (var ps in pathList) {
      if (ps[0] == PARAM_CHAR) {
        pathString += '/${routeData.params[ps.replaceFirst(PARAM_CHAR, '')]!}';
      } else {
        pathString += '/$ps';
      }
    }

    routeData = jetsRoutesParser(pathString);
    _pages = routesPagesMap[routeData.path]!;
  }

  void _buildListPages() {
    final myRoutes = jetsRoutesMap.keys.toList();

    for (var route in myRoutes) {
      List<MaterialPage> pagesList = [];
      List<String> routeList = [];

      var uri = Uri.parse(route).pathSegments;
      var tmp = '';

      pagesList.add(MaterialPage(
        key: const ValueKey(homePath),
        child: jetsRoutesMap[homePath]!,
      ));

      routeList.add(homePath);

      if (route == homePath) {
        tmp = homePath;
      } else {
        for (var ps in uri) {
          tmp += '/$ps';

          for (var i = 0; i < myRoutes.length; i++) {
            if (myRoutes[i] == tmp) {
              pagesList.add(MaterialPage(
                key: ValueKey(myRoutes[i] + i.toString()),
                child: jetsRoutesMap[tmp]!,
              ));
              routeList.add(tmp);
              break;
            }
          }
        }
      }

      if (tmp != homePath) routeList.removeLast();

      routePrevMap[tmp] = routeList.last;
      routesPagesMap[tmp] = pagesList;
    }
  }
}

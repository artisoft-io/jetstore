import 'package:flutter/material.dart';

import 'jets_routes_app.dart';
import 'jets_route_data.dart';

JetsRouteData jetsRoutesParser(String pathFromUrl) {
  final List<String> pathUriList = Uri.parse(pathFromUrl).pathSegments;

  if (pathUriList.isEmpty) {
    return JetsRouteData(homePath);
  }

  for (var route in jetsRoutesMap.keys) {
    List<String> routeUriList = Uri.parse(route).pathSegments;

    if (routeUriList.length == pathUriList.length) {
      var diff = 0;
      Map<String, dynamic> params = {};

      for (var i = 0; i < routeUriList.length; i++) {
        if (routeUriList[i][0] != PARAM_CHAR &&
            routeUriList[i] != pathUriList[i]) {
          diff++;
        } else if (routeUriList[i][0] == PARAM_CHAR) {
          params[routeUriList[i].replaceFirst(PARAM_CHAR, '')] = pathUriList[i];
        }
      }

      if (diff == 0) {
        return JetsRouteData(route, params: params);
      }
    }
  }

  return JetsRouteData(pageNotFoundPath);
}

String buildRouteLocation(JetsRouteData route) {
  if (!route.path.contains(PARAM_CHAR)) return route.path;

  var location = '';
  final pathList = Uri.parse(route.path).pathSegments;

  for (var ps in pathList) {
    if (ps[0] == PARAM_CHAR) {
      location +=
          '/${route.params[ps.replaceFirst(PARAM_CHAR, '')].toString()}';
    } else {
      location += '/$ps';
    }
  }

  return location;
}

class JetsRouteInformationParser extends RouteInformationParser<JetsRouteData> {
  @override
  Future<JetsRouteData> parseRouteInformation(
          RouteInformation routeInformation) async =>
      jetsRoutesParser(routeInformation.uri.path);

  @override
  RouteInformation? restoreRouteInformation(JetsRouteData configuration) =>
      RouteInformation(uri: Uri.parse(buildRouteLocation(configuration)));
}

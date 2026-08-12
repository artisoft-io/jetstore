import 'package:jetsclient/routes/jets_routes_app.dart';

class JetsRouteData {
  final String path;
  final Map<String, dynamic> params;

  /// The paths reachable without an authenticated session.
  ///
  /// An exact match, deliberately, rather than a substring test. [path] is
  /// always a route template taken from [jetsRoutesMap] — `jetsRoutesParser`
  /// either matches a template and returns it verbatim, or falls through to
  /// [pageNotFoundPath] — so a substring test buys nothing. What it cost was
  /// safety: the previous
  /// `path.contains('login') || path.contains('register')` would have made
  /// *any* future route public the moment its path happened to contain either
  /// word.
  static const publicPaths = <String>{loginPath, registerPath};

  bool get authRequired => !publicPaths.contains(path);

  const JetsRouteData(this.path, {Map<String, dynamic>? params})
      : params = params ?? const {};

  @override
  String toString() => 'path: $path | params: $params';
}

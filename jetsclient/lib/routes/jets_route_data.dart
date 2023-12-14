class JetsRouteData {
  final String path;
  final Map<String, dynamic> params;

  bool get authRequired =>
      !(path.contains('login') || path.contains('register'));

  const JetsRouteData(this.path, {Map<String, dynamic>? params})
      : params = params ?? const {};

  @override
  String toString() => 'path: $path | params: $params';
}

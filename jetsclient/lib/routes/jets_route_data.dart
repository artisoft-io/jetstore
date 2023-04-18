class JetsRouteData {
  final String path;
  final Map<String, dynamic> params;

  bool get authRequired =>
      !(path.contains('login') || path.contains('register'));

  JetsRouteData(this.path, {Map<String, dynamic>? m})
      : params = m ?? {};

  @override
  String toString() => 'path: $path | params: $params';
}

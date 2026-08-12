import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';

/// [JetsRouteData.authRequired] decides whether a route may be reached without
/// a session, so widening it by accident is a security bug rather than a
/// cosmetic one. It used to be a substring test —
/// `path.contains('login') || path.contains('register')` — which made any
/// route public as soon as its path happened to contain either word. These
/// tests pin the exact-match behaviour that replaced it.
void main() {
  group('authRequired', () {
    test('the two public routes are reachable without a session', () {
      expect(const JetsRouteData(loginPath).authRequired, isFalse);
      expect(const JetsRouteData(registerPath).authRequired, isFalse);
    });

    test('ordinary routes require a session', () {
      for (final path in [
        homePath,
        workspaceRegistryPath,
        userAdminPath,
        queryToolPath,
        inferServerAdminPath,
        pageNotFoundPath,
      ]) {
        expect(JetsRouteData(path).authRequired, isTrue,
            reason: '$path must not be public');
      }
    });

    test('a path merely containing "login" or "register" is not public', () {
      // These are the cases the old substring test got wrong. None is a real
      // route today; the point is that adding one later must not silently
      // open a hole.
      for (final path in [
        '/auditLoginHistory',
        '/loginAttempts',
        '/registerFileKeyUF',
        '/clientRegistry/register/details',
      ]) {
        expect(JetsRouteData(path).authRequired, isTrue,
            reason: '$path must not be public');
      }
    });

    test('the public set is exactly the login and register routes', () {
      expect(JetsRouteData.publicPaths, {loginPath, registerPath});
    });
  });
}

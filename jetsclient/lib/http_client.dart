import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/utils/constants.dart';

class HttpResponse {
  final int statusCode;
  final dynamic body;

  HttpResponse(this.statusCode, this.body);
}

class HttpClientSingleton {
  factory HttpClientSingleton() => _singlton;
  static final HttpClientSingleton _singlton = HttpClientSingleton._();
  HttpClientSingleton._();
  final http.Client httpClient = http.Client();
  Uri? serverAdd;

  void refreshToken() async {
    await sendRequest(
        path: ServerEPs.dataTableEP,
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: jsonEncode(
            <String, dynamic>{'action': 'refresh_token'},
            toEncodable: (_) => ''));
    print('*** refreshToken() called');
  }

  Future<HttpResponse> sendRequest(
      {required String path, String? token, String? encodedJsonBody}) async {
    try {
      if (serverAdd == null) {
        return HttpResponse(400, "serverAdd not set");
      }
      final routeData = JetsRouteData(path);
      if (routeData.authRequired && !JetsRouterDelegate().isAuthenticated()) {
        // print("*** User Not Authenticated - not sending request");
        return HttpResponse(401, '');
      }

      // print('*** Request $path (auth:${JetsRouterDelegate().isAuthenticated()}): ${encodedJsonBody==null?'':encodedJsonBody.substring(0, encodedJsonBody.length>50?50:encodedJsonBody.length)}');
      var h = <String, String>{'Content-Type': 'application/json'};
      if (token != null) {
        h['Authorization'] = 'token $token';
      }
      var response = await httpClient.post(serverAdd!.replace(path: path),
          headers: h, body: encodedJsonBody);
      // print('Response status: ${response.statusCode} body: ${response.body}');
      // print('Response headers: ${response.headers}');
      // print('---');
      if (response.statusCode == 401) {
        if (JetsRouterDelegate().isAuthenticated()) {
          // redirect to login page
          // print('Not authorized redirecting to login');
          JetsRouterDelegate().user.token = '';
          JetsRouterDelegate()(JetsRouteData(loginPath));
        }
        return HttpResponse(response.statusCode, '');
      }
      var data = jsonDecode(response.body) as Map<String, dynamic>;
      token = data['token'];
      if (token != null) {
        JetsRouterDelegate().user.token = token;
        JetsRouterDelegate().user.lastTokenRefresh = DateTime.now();
      }
      return HttpResponse(response.statusCode, data);
    } on Exception {
      // print('HTTP Exception details\n$e');
      return HttpResponse(400, "Exception while communicating");
    } catch (e) {
      // print('Unknown HTTP exception $e of type ${e.runtimeType}.');
      return HttpResponse(400, "Exception while communicating");
    }
  }
}

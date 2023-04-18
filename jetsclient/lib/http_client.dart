import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';

class HttpResponse {
  final int statusCode;
  final dynamic body;

  HttpResponse(this.statusCode, this.body);
}

class HttpClient {
  final http.Client httpClient = http.Client();
  final Uri serverAdd;

  HttpClient(String serverOrigin) : serverAdd = Uri.parse(serverOrigin);

  Future<HttpResponse> sendRequest(
      {required String path, String? token, String? encodedJsonBody}) async {
    try {
      // print('*** Request: $encodedJsonBody');
      var h = <String, String>{'Content-Type': 'application/json'};
      if (token != null) {
        h['Authorization'] = 'token $token';
      }
      var response = await httpClient.post(serverAdd.replace(path: path),
          headers: h, body: encodedJsonBody);
      // print('Response status: ${response.statusCode} body: ${response.body}');
      // print('Response headers: ${response.headers}');
      // print('---');
      if (response.statusCode == 401) {
        // redirect to login page
        JetsRouterDelegate()(JetsRouteData(loginPath));
      }
      var data = jsonDecode(response.body) as Map<String, dynamic>;
      token = data['token'];
      if (token != null) {
        JetsRouterDelegate().user.token = token;
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

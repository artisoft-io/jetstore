import 'dart:convert';
import 'package:http/http.dart' as http;

class HttpResponse {
  final int statusCode;
  final dynamic body;

  HttpResponse(this.statusCode, this.body);
}

class HttpClient {
  final http.Client httpClient = http.Client();
  final Uri serverAdd;

  HttpClient(String serverOrigin) : serverAdd = Uri.parse(serverOrigin);

  Future<HttpResponse> sendRequest(String path, String encodedJsonBody) async {
    try {
      var response = await httpClient.post(serverAdd.replace(path: path),
          headers: {'Content-Type': 'application/json'},
          body: encodedJsonBody);
      // print('Response status: ${result.statusCode}');
      // print('Response body: ${result.body}');
      if (response.statusCode >= 200 && response.statusCode < 207) {
        // user.token = jsonDecode(utf8.decode(result.bodyBytes)) as String;
        return HttpResponse(response.statusCode, jsonDecode(response.body));
      }
      return HttpResponse(response.statusCode, null);
    } on Exception catch (e) {
      print('HTTP Exception details\n$e');
      return HttpResponse(999, null);
    } catch (e) {
      print('Unknown HTTP exception $e of type ${e.runtimeType}.');
      return HttpResponse(999, null);
    }
  }
}

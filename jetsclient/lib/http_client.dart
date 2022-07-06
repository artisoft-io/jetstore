import 'dart:convert';
import 'package:flutter/material.dart';
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

  Future<HttpResponse> sendRequest(
      {required String path, String? token, String? encodedJsonBody}) async {
    try {
      var h = <String, String>{'Content-Type': 'application/json'};
      if (token != null) {
        h['Authorization'] = 'token $token';
      }
      var response = await httpClient.post(serverAdd.replace(path: path),
          headers: h, body: encodedJsonBody);
      // print('Response body: ${response.body}');
      // user.token = jsonDecode(utf8.decode(result.bodyBytes)) as String;
      return HttpResponse(response.statusCode, jsonDecode(response.body));
    } on Exception catch (e) {
      debugPrint('HTTP Exception details\n$e');
      return HttpResponse(999, null);
    } catch (e) {
      debugPrint('Unknown HTTP exception $e of type ${e.runtimeType}.');
      return HttpResponse(999, null);
    }
  }
}

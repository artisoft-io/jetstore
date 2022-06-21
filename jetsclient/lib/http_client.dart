import 'package:http/http.dart' as http;

class HttpClient {
  final http.Client httpClient = http.Client();
  final Uri serverAdd;

  HttpClient(String serverOrigin) : serverAdd = Uri.parse(serverOrigin);

  // Domain methods to crud on domain model
  // Some objects are cacheable obj like static config obj

}

import 'dart:convert';
import 'package:web/web.dart' as web;
void download(
  List<int> bytes, {
  String? downloadName,
}) {
  // Encode our file in base64
  final data = base64Encode(bytes);
  // Create the link with the file
  final anchor = web.HTMLAnchorElement()
    ..href = 'data:application/octet-stream;base64,$data'
    ..target = 'blank';
  // add the name
  if (downloadName != null) {
    anchor.download = downloadName;
  }
  // trigger download
  web.document.body?.append(anchor);
  anchor.click();
  anchor.remove();
  return;
}
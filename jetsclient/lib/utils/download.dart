import 'dart:convert';
// ignore: avoid_web_libraries_in_flutter
import 'dart:html';

void download(
  List<int> bytes, {
  String? downloadName,
}) {
  // Encode our file in base64
  final data = base64Encode(bytes);
  // Create the link with the file
  final anchor =
      AnchorElement(href: 'data:application/octet-stream;base64,$data')
      ..target = 'blank';
  // add the name
  if (downloadName != null) {
    anchor.download = downloadName;
  }
  // trigger download
  document.body?.append(anchor);
  anchor.click();
  anchor.remove();
  return;
}
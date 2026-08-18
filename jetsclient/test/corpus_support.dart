/// Shared helpers for the two corpus tests.
///
/// `table_config_corpus_test.dart` and `form_field_corpus_test.dart` both
/// serialise part of the Flutter app's configuration for the React port to read,
/// and both detect drift with a checksum rather than by comparing files —
/// because these tests run in a browser, where there is no filesystem. See
/// either file's header, and `jetstore_ai/CLAUDE.md`.
library;

import 'dart:convert';

/// FNV-1a, 32-bit, over the UTF-8 bytes of the corpus.
///
/// Hand-rolled because `package:crypto` is only a transitive dependency here and
/// this is drift detection, not a security boundary: the failure it has to catch
/// is a developer editing a `TableConfig` and not knowing the React side reads
/// it too.
String checksum(String s) {
  var hash = 0x811c9dc5;
  for (final byte in utf8.encode(s)) {
    hash ^= byte;
    // Multiply by the FNV prime, 16777619, in 32-bit arithmetic. Written as
    // shifts because the product overflows JavaScript's 53-bit integers, and
    // these tests run in a browser.
    hash = (hash +
            ((hash << 1) & 0xffffffff) +
            ((hash << 4) & 0xffffffff) +
            ((hash << 7) & 0xffffffff) +
            ((hash << 8) & 0xffffffff) +
            ((hash << 24) & 0xffffffff)) &
        0xffffffff;
  }
  return 'fnv1a32:${hash.toRadixString(16).padLeft(8, '0')}';
}

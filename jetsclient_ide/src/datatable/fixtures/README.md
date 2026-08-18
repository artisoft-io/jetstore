# `table_configs.json` — the user flows' table configurations, as data

Generated from the running Flutter app. **Do not edit it by hand.**

It holds the 37 `TableConfig` objects the nine user flows define — 28 that query
`/dataTable` and 9 static ones — serialised by
`jetsclient/test/table_config_corpus_test.dart`. The React query builder
(`../query.ts`) is tested against it, so the fixtures are the app's own
configuration rather than a transcription of it. That matters more than it
sounds: the configurations run to roughly 2,700 lines of Dart, and the counting
mistakes made while sizing this task all came from reading those files with
`grep` instead of from the constructed objects.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/table_config_corpus_test.dart
```

The test prints the corpus between `===BEGIN TABLE CONFIG CORPUS===` and
`===END TABLE CONFIG CORPUS===`, and prints its checksum. Copy the JSON between
the markers into this file, and put the new checksum into `expectedChecksum` in
that test. **Both, together** — the checksum is what makes a stale fixture fail
rather than pass quietly.

Two things about that command are not incidental:

- **`--platform chrome` is required.** `flutter test` defaults to the Dart VM,
  where `package:web` 1.1.1 — a direct dependency at `jetsclient/pubspec.yaml:52`
  — fails to compile and takes every test in the package with it. This predates
  the corpus test; `test/form_button_test.dart` fails to load the same way.
- **The browser has no filesystem**, which is why the corpus is printed and
  copied rather than written, and why drift is caught by a checksum rather than
  by the test comparing itself to this file.

## What is in it, and what cannot be

Each table carries its columns, from/where/with clauses, form-state bindings,
actions, sorting and paging — everything `makeQuery` reads.

What it cannot carry is the closures. `cellFilter`, `isEnabledFnc`,
`modelStateHandler` and `actionDelegate` are Dart function pointers, and they
appear here as `hasCellFilter`, `hasIsEnabledFnc` and so on. Every `true` is a
place the React port needs an answer this file cannot give it. Currently three
columns set a `cellFilter` (all of them `file_key`) and three actions set an
`isEnabledFnc`; no table sets `modelStateHandler` and no action sets
`actionDelegate`.

That is the UI assessment's §3.2 — configuration is not data, it embeds Dart
closures — reduced to a checklist of six.

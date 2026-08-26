Two files here, and only one of them had a section until 2026-08-25:
`table_configs.json` and `form_fields.json`.

---

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
that test. **Both, together.**

**~~the checksum is what makes a stale fixture fail rather than pass quietly~~ —
corrected 2026-08-25.** The Dart assertion compares the app's configuration
against the Dart constant, and the fixture is in neither operand; bumping the
constant while leaving the fixture alone was green on both sides, which is what
C.0 did to the two screen fixtures. It is enforced now, by
`jetsclient_ide/src/corpusFixtures.test.ts`, which hashes each of the five
fixtures and asserts it against the `expectedChecksum` its corpus test declares.

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

---

# `form_fields.json` — the user flows' forms, field by field

Generated from the running Flutter app by
`jetsclient/test/form_field_corpus_test.dart`. **Do not edit it by hand.**

**This section did not exist until 2026-08-25**, and the fixture had been in the
repository since the corpus was built. Its Dart test said "update only together
with the fixture — see the README beside it", and the README beside it described
only `table_configs.json`. Written now because the traversal repair below adds
data this file had never carried and there was nowhere to say what that data is.

It holds the 50 form configurations the eleven user flows define, their fields
across the three field containers, and — since 2026-08-25 — the buttons they
declare in `FormConfig.actions`.

## What it establishes

| Quantity | |
|---|---:|
| Form configurations | **50** |
| Buttons in `actions`, the form's action bar | **143** |
| Buttons among the fields | **3** |
| Buttons carrying a `capability` | **7** |
| Forms using `inputFieldsV2` row structure | **8** |

The three among the fields are `wpLoadConfigUF`'s "Load All Clients Config" and
`pcViewMergeProcessInputsUF` / `pcViewInjectedProcessInputsUF`'s two "add"
buttons; `src/userflow/form.ts` explains why each is a button in a row rather
than a layout preference.

**All 143 of the action-bar buttons were in no corpus at all until 2026-08-25.**
See the note below, which is the same repair on both form corpora.

## The traversal that never saw them

`allFields` (`jetsclient/test/corpus_support.dart`) walks the three field
containers a `FormConfig` may use — `inputFields`, `inputFieldsV2` and
`formTabsConfig`. **`FormConfig.actions` is a fourth container of a different
kind**, holding the buttons the action bar renders rather than the layout, and
both form corpora built their output from `allFields` alone. Neither traversal
was wrong about the question it was written for; the container simply belonged to
no traversal.

**Three losses, repaired together:**

1. **`actions` was not walked**, so 143 buttons here and 30 in
   `../../screens/fixtures/screen_configs.json` were absent — and with them 20 of
   the 27 `capability` claims the two surfaces make on a button.
2. **`fieldToJson` had no `FormActionConfig` branch**, so the ten buttons that
   *do* sit among the fields arrived as bare `type`/`key`/`group`/`flex`. Seven of
   those ten carry `capability: "infer_server_admin"` and two carry an
   `isEnabledEval` closure, and the corpus said nothing about either.
3. **`inputFieldsV2`'s rows were flattened.** `allFields` does
   `expand((row) => row.rowConfig)` and discards `FormFieldRowConfig`, whose
   `flex` is the entire reason a form chooses V2. The corpus now emits `rows`
   beside `fields` for the 9 forms that use it.

**Item 3 was argued rather than assumed, and the argument is worth keeping.** A
corpus answering *which widgets must the port support* is entitled to drop
layout, and on that reasoning item 3 could have been left. What settled it was
that a consumer had arrived: C.5 is porting `inferServerAdminForm`, whose own doc
comment says it uses V2 *specifically* so its rows can carry their own flex.
**The rule against building for no consumer is not a rule for declining one that
turned up.** The next traversal question will not have a consumer standing there;
this is the precedent for what to do then.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/form_field_corpus_test.dart
```

Copy the JSON printed between `===BEGIN FORM FIELD CORPUS===` and
`===END FORM FIELD CORPUS===` into this file, and put the printed checksum into
`expectedChecksum` in that test. **Both, together** — enforced by
`jetsclient_ide/src/corpusFixtures.test.ts`.

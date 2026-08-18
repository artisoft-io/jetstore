# `user_flows.json` — the eleven user flows, as data

Generated from the running Flutter app. **Do not edit it by hand.**

It holds every `UserFlowConfig` the app registers, serialised by
`jetsclient/test/user_flow_corpus_test.dart`. The schema in `../schema.ts` is
argued from it, and `../schema.test.ts` converts all eleven and asserts they
validate — so the schema is tested against the app's own configuration rather
than against a reading of it.

**Eleven flows, not the nine this project's documents said until 2026-08-18.**
Nine is the number of *directories* under `jetsclient/lib/modules/user_flows/`;
`workspace_pull/` defines two flows and `file_mapping/` defines two.
`UserFlowKeys` (`jetsclient/lib/utils/constants.dart:762`) declares eleven and
`jets_routes_app.dart` mounts eleven.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/user_flow_corpus_test.dart
```

The test prints the corpus between `===BEGIN USER FLOW CORPUS===` and
`===END USER FLOW CORPUS===`, and prints its checksum. Copy the JSON between the
markers into this file, and put the new checksum into `expectedChecksum` in that
test. **Both, together** — the checksum is what makes a stale fixture fail rather
than pass quietly.

`--platform chrome` is required and the browser has no filesystem; both
constraints are explained in `../../datatable/fixtures/README.md`, which the two
sibling corpora share.

## What is in it, and what cannot be

Each flow carries its start state, exit path, and states; each state its
description, form, action name, choices and default transition. Choices are
serialised recursively, so a nested expression appears nested.

Two things are Dart closures and appear as facts about their presence rather
than as themselves:

- **`actionDelegate`**, one per state. The schema does not carry it at all: what
  a state *does* is named by `stateAction`, and S.2's grammar is what the name
  resolves to.
- **`hasFormStateInitializer`**, set by one flow. A boolean cannot become a name,
  so `../translate.ts` holds the one name explicitly and refuses to convert a
  flow that grows a second one without being told what to call it.

## The three aggregate figures the schema rests on

- **`emptyNestedNextStateCount`: 1 of 1.** Every `UserFlowChoice` inherits a
  required `nextState`, including nested sub-expressions where it is never read.
  The corpus has one nested expression and its value is `""`. This is why the
  schema separates a condition from the transition it guards.
- **`rhsKindCounts`: 17 literals, 0 state keys.** `isRhsStateKey` has never been
  set in a shipping flow, which is why the schema replaces the flag with two
  named fields rather than carrying it forward.
- **`formKeyMismatches`: 2.** `FormConfig` carries a `key` field of its own that
  disagrees with the key it is registered under for `fmMappingFormUF` and
  `spSelectMergedDataSourcesUF`. The field is read nowhere outside a
  commented-out print, so the registry key is the identity and the schema uses
  it — and carries no self-key of its own, for the same reason.

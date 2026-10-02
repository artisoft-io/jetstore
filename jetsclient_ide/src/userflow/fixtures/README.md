# `user_flows.json` — the eleven user flows, as data

Generated from the running Flutter app. **Do not edit it by hand.**

**Except once, and it is recorded here: ten flows since 2026-10-01.** `registerFileKeyUF`
was retired by `jetstore_maintenance_02` (`Q-6`, task `AD.4`), and under that project's
`Q-5` a document leaves this fixture rather than being changed in it. Its `flows` entry
was deleted and the four aggregates it contributed to — `flowCount`, `stateCount`,
`distinctFormKeys` and `formKeys` — were reduced to match, so the file stays internally
consistent. Everything else is the app's, as generated, and the prose below describes
the eleven it was generated with.

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

## Three flows left on 2026-10-01

**`clientRegistryUF`, `sourceConfigUF` and `pipelineConfigUF` are no longer in
this file.** `jetstore_maintenance_02` Phase 1 (task `AF.1`, defect `D06`) makes
the three open on their table, and Michel's answer to that project's `Q-5` is
that a document this repository edits leaves the emitted set the first time it is
touched rather than being edited here. They are hand-authored in
`jets/workspace_assets/user_flows/` and listed in `../schema.test.ts` as
`HAND_AUTHORED`; `ui_refresh`'s **I-299** is the question this answers one flow at
a time.

**The aggregate figures at the top of the file were recomputed from the flows
that remain**, so the file stays internally consistent — 7 flows, 19 states, 19
form keys, after `registerFileKeyUF`'s retirement the same day — and they no longer describe the Flutter app as a whole. The
statements about them below are about the Dart corpus as measured and are kept as
history: `rhsKindCounts` was 17 literals there and is 1 here, and every `equals`
the fixture held belonged to one of the three.

**`fileMappingUF` followed them the same day** (`jetstore_maintenance_02` Phase 2),
so that its *Done* could return to its start table. The aggregates were recomputed
again — 6 flows, 17 states, 17 form keys. The flow has no choices, so no other
figure moved.

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

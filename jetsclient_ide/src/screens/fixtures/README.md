# `screen_configs.json` — the non-flow screens' configuration, as data

Generated from the running Flutter app. **Do not edit it by hand.**

It holds the configuration of the screens **outside** `lib/modules/user_flows/` —
28 `TableConfig` objects and 32 `FormConfig` objects with 80 fields between them
— serialised by `jetsclient/test/screen_config_corpus_test.dart`.

**It exists because the other three corpora do not cover this.** They are all
scoped to the user flows; the data-table corpus README says so in its first line.
Phase 3 of the UI port migrates mostly the screens they exclude, and its plan
opens with a gate that refuses to size that work from a `grep`. This file is that
gate's answer. See `projects/ui_refresh/plan/phase3_plan.md` §1 in the
`jetstore_agentic_ai` repository.

## It enumerates; the other three list

The earlier corpora carry hand-maintained key lists, which I-25 records as debt.
This one asks the registries for their keys, through five accessors added for the
purpose — `screenTableConfigKeys`, `workspaceTableConfigKeys`,
`screenFormConfigKeys`, `workspaceFormConfigKeys` and
`inferServerAdminFormConfigKeys`.

**That is not tidiness, and the reason is worth keeping.** A key list here could
not be checked for completeness, because the registry's key space is not one
constant class: **13 of the user flows' 37 table keys are `FSK` constants rather
than `DTKeys`**. Enumerating the declared constants of `DTKeys` would have missed
a third of them and reported a clean result. Asking the map is the only
enumeration that cannot be wrong about its own contents.

## What it establishes

| Quantity | Non-flow screens | User flows, for comparison |
|---|---:|---:|
| Table configurations | **28** | 37 |
| Form configurations | **32** | 50 |
| Fields | **80** | 101 |
| Input-field types in use | **3** | 3 |

`sharedTableKeysWithUserFlows` is empty — measured, not assumed. The two surfaces
define disjoint table configurations.

**The three input types are the same three the React port already built**:
`FormInputFieldConfig` (44), `FormDataTableFieldConfig` (24) and
`FormDropdownFieldConfig` (4). The remaining 8 of the 80 are `FormActionConfig`
(7 buttons) and `PaddingConfig` (1). **Neither exotic type appears** —
`FormDropdownWithSharedItemsFieldConfig` and `FormTypeaheadFieldConfig` are as
absent here as they are in the flows.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/screen_config_corpus_test.dart
```

The test prints the corpus between `===BEGIN SCREEN CONFIG CORPUS===` and
`===END SCREEN CONFIG CORPUS===`, and prints its checksum. Copy the JSON between
the markers into this file, and put the new checksum into `expectedChecksum` in
that test. **Both, together** — the checksum is what makes a stale fixture fail
rather than diverge quietly. The browser has no filesystem, which is why the
corpus is printed rather than written (I-5).

# The screens' fixtures

Two files, generated from the running Flutter app, both answering one half of
Phase 3's sizing gate: `screen_configs.json` counts what the non-flow screens
*hold*, and `screen_reachability.json` says what *reaches* them.

---

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

---

# `screen_reachability.json` — which screens anything routes to, and for whom

Generated from the running Flutter app by
`jetsclient/test/screen_reachability_corpus_test.dart`. **Do not edit it by
hand.**

`screen_configs.json` counts configuration; it does not ask whether a user can
get to it. **G3.2 asks that**, because the migration is per route: a screen
nothing reaches costs nothing to leave behind, and a screen only an administrator
reaches is not on the critical path. See
`projects/ui_refresh/plan/phase3_plan.md` §1.2 sub-question 3, and the sizing
document that reads this file.

## Reachability here is a closure over configuration

Four kinds of edge, every one of them declared somewhere:

| Edge | Where it is declared |
|---|---|
| route → screen configuration | `jetsRoutesMap`; every screen widget extends `BaseScreen` and carries its own `screenConfig`, so the route table *names* its screen |
| screen → route | `MenuEntry.routePath`, walked recursively through `children`, over `menuEntries`, `adminMenuEntries` and `toolbarMenuEntries` |
| table action → route | `ActionConfig.configScreenPath`, on the tables a route shows — `ScreenOne.tableConfig`, a form's `FormDataTableFieldConfig.dataTableConfig`, and the dialog forms an action names in `configForm` |
| flow → route | `UserFlowConfig.exitScreenPath` |

The closure runs from `/`, which is where an authenticated session lands.

**The role model is the one the widgets implement**, and it is small: `isAdmin`
bypasses every capability check, in the menu (`screens/base_screen.dart:145`) and
on a table action (`components/data_table.dart:30`). So an entry gated on
capability `c` is reachable two ways — by an administrator, or by a user holding
`c` — and `access` carries both alternatives rather than collapsing them.

## What it establishes

| Quantity | |
|---|---:|
| Screen configurations registered | **30** |
| Routes in `jetsRoutesMap` | **28** |
| Screen configurations a route names | **26** |
| Registered but unrouted — dead configuration | **4** |
| Routes reachable by any authenticated user | **16** |
| Routes needing `admin` or a capability | **6** |
| Routes with no configured edge | **6** |

`orphanScreenKeys` and `routesWithNoConfiguredEdge` are the two lists worth
reading first.

**Each route also carries its own inventory** — `tables` and `dialogForms`, the
configuration reachable from that screen. `screen_configs.json` counts the
non-flow surface in total; this says which route each piece of it belongs to,
which is what turns a total into an order of work.

**Eighteen of that file's 28 table configurations are reached from a non-flow
route by this closure. The other ten are not, and none of the three reasons is
"dead code" except one:**

| Tables | Why the closure does not reach them |
|---:|---|
| 8 | The Workspace IDE's tabbed sections. `MenuEntry.formConfigKey` is composed at run time as `"workspace.$pageMatchKey.form"` from the server's file-tree response (`workspace_ide/screen_delegates_helpers.dart:137`), so the edge is a *string template* over server data rather than a declaration. |
| 1 | `inputRecordsFromProcessErrorTable`, inside an `inputFieldRowBuilder` closure — the fourth field container `allFields` deliberately does not walk, because its fields do not exist until a form is driven. |
| 1 | `reteSessionTriplesTable`, whose only opener is a **commented-out** `configForm` (`data_table_config_impl.dart:671`). `viewReteTriplesV2` superseded it. This one really is dead. |

The first row is the one that matters: the Workspace IDE is first in the
migration order, and its screen inventory is data-driven from the apiserver
rather than enumerable here.

**"No configured edge" is a statement about the configuration, not a claim that
the route is dead** (I-20). Five of the six are reached from Dart:
`/login` and `/register` from `user_delegates.dart` and `http_client.dart`,
`/userGitProfile/…` from `app_bar.dart:43`, `/workspaces/:workspace_name/home`
from `screen_delegates.dart` and `screen_delegates_helpers.dart`, and `/404` from
the route parser's fallback. **The sixth, `/processConfig`, is reached from
nowhere at all** — the `processConfig` menu entry points at `ruleConfigPath`.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/screen_reachability_corpus_test.dart
```

Same procedure as above: copy the JSON printed between
`===BEGIN SCREEN REACHABILITY CORPUS===` and
`===END SCREEN REACHABILITY CORPUS===` into this file, and put the printed
checksum into `expectedChecksum` in that test. **Both, together.**

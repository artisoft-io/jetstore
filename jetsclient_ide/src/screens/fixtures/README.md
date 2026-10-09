# The screens' fixtures

Two files, generated from the running Flutter app, both answering one half of
Phase 3's sizing gate: `screen_configs.json` counts what the non-flow screens
*hold*, and `screen_reachability.json` says what *reaches* them.

**Both were stale for a day, and every number below moved when they were
repaired.** C.0 (jetstore#2018, 2026-08-25) deleted seven registry entries and
bumped both Dart `expectedChecksum` constants, and neither fixture was
regenerated with them; C.0a regenerated both from the same app the same day. The
counts here are as of that regeneration. **What made the gap possible is fixed
rather than noted** — see *Regenerating*, below, which is where the instruction
that was not followed lives.

---

# `screen_configs.json` — the non-flow screens' configuration, as data

Generated from the running Flutter app. **Do not edit it by hand.**

It holds the configuration of the screens **outside** `lib/modules/user_flows/` —
25 `TableConfig` objects and 28 `FormConfig` objects with 75 fields between them
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

| Quantity | Non-flow screens | Before C.0's deletions | User flows, for comparison |
|---|---:|---:|---:|
| Table configurations | **25** | 28 | 37 |
| Form configurations | **28** | 32 | 50 |
| Fields | **75** | 80 | 101 |
| Input-field types in use | **3** | 3 | 3 |
| Buttons in `actions`, the form's action bar | **30** | *uncounted* | 143 |
| Buttons among the fields | **7** | 7 | 3 |
| Buttons carrying a `capability` | **20** | *uncounted* | 7 |

The "before" column is kept for one revision only, because the sizing document
and the Phase 3 plan were both written against it and a reader arriving from
either needs to know which number they are holding.

**Two of its cells say *uncounted* rather than a number, and the distinction is
the whole of C.0a's second finding.** The other columns moved because
configuration was *deleted*; those two moved because a container was **never
walked**. See *The buttons no corpus could see*, below. A reviewer looking at two
repairs an hour apart that both move counts in this file is looking at two
different failures — one where data went stale after being seen, one where it was
never seen.

`sharedTableKeysWithUserFlows` is empty — measured, not assumed. The two surfaces
define disjoint table configurations.

**The three input types are the same three the React port already built**:
`FormInputFieldConfig` (42), `FormDataTableFieldConfig` (21) and
`FormDropdownFieldConfig` (4). The remaining 8 of the 75 are `FormActionConfig`
(7 buttons) and `PaddingConfig` (1). **Neither exotic type appears** —
`FormDropdownWithSharedItemsFieldConfig` and `FormTypeaheadFieldConfig` are as
absent here as they are in the flows.

C.0's five deletions took only `FormInputFieldConfig` and
`FormDataTableFieldConfig` fields — 2 and 3 of them — so the shape of this
surface is what it was and only its size moved.

## The buttons no corpus could see

**Added 2026-08-25, by C.0a, on the day C.5 was about to consume this file.**

`allFields` (`jetsclient/test/corpus_support.dart`) walks the three field
containers a `FormConfig` may use. **`FormConfig.actions` is a fourth container**
— the buttons the action bar renders rather than the layout — and no traversal
owned it, so **30 buttons on this surface and 143 in the flows were in no corpus
at all.**

**The consequence was measured rather than hypothesised, by the party holding the
consumer.** `inferServerAdminForm` — C.5's screen, and this file's only
`infer_server_admin` form — declares **eight** buttons: seven among its fields and
**Submit** in `actions`. The corpus reported seven. *Porting C.5 from this file
would have shipped a screen with no submit button.*

**It stayed invisible because the undercount was not uniform.** Most forms put
every button in `actions` and reported none; `inferServerAdminForm` is the one
form that puts most of them among its fields, so it was the one form whose buttons
the corpus appeared to see. A reader asking *does this corpus report buttons?*
against the `FormActionConfig` count of 7 got yes — and all seven were that one
form's.

**Three losses, repaired together** — the full account is in
`../../datatable/fixtures/README.md`, which carries the same repair from the flows'
side:

1. `actions` was not walked.
2. `fieldToJson` had no `FormActionConfig` branch, so the seven buttons that *did*
   reach this file arrived without their `capability` — all seven carry
   `infer_server_admin` — without their style, and without the `isEnabledEval`
   closure that gates Start and Stop on the reported server state.
3. `inputFieldsV2`'s rows were flattened, losing `FormFieldRowConfig.flex`. This
   file's one V2 form is `inferServerAdminForm`, whose doc comment says it chose
   V2 *precisely* for that flex — rows of 2 and 3 for the request and response
   boxes, reported at the fields' own flex of 1. The corpus now emits `rows`
   beside `fields`.

**Each of the three passes over this configuration was written for the question in
front of it, and each lost what the next question needed.** That is I-12's *"a
corpus is exactly as trustworthy as its traversal"* with a mechanism rather than a
moral, and it is the third time on this surface.

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/screen_config_corpus_test.dart
```

The test prints the corpus between `===BEGIN SCREEN CONFIG CORPUS===` and
`===END SCREEN CONFIG CORPUS===`, and prints its checksum. Copy the JSON between
the markers into this file, and put the new checksum into `expectedChecksum` in
that test. **Both, together.** The browser has no filesystem, which is why the
corpus is printed rather than written (I-5).

**~~The checksum is what makes a stale fixture fail rather than diverge
quietly.~~ It was not, and C.0 is the proof** — corrected 2026-08-25. The Dart
assertion compares the app's configuration against the Dart constant; the fixture
is in neither operand, so regenerating the app and bumping the constant while
leaving this file alone was green on both sides of the repository. *Both,
together* was a convention with no enforcement behind it.

**It has one now**, and it is on this side rather than that one:
`jetsclient_ide/src/corpusFixtures.test.ts` hashes each of the five fixtures and
asserts it against the `expectedChecksum` its corpus test declares. It runs under
`npm test`, needs no Flutter and no browser, and covers all five corpora rather
than these two. Restoring the pre-C.0a fixtures makes it fail on both, naming the
checksums C.0's commit replaced.

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
bypasses every capability check, in the menu (the `doIt` guard,
`screens/base_screen.dart:147`) and on a table action (`ac.isEnabled`'s condition,
`components/data_table.dart:29`). **Both line numbers moved by two while this
paragraph read as verified**, which is what an in-range citation does; they are
re-read and the identifiers named, 2026-08-25. So an entry gated on
capability `c` is reachable two ways — by an administrator, or by a user holding
`c` — and `access` carries both alternatives rather than collapsing them.

## What it establishes

| Quantity | | Before C.0's deletions |
|---|---:|---:|
| Screen configurations registered | **25** | 30 |
| Routes in `jetsRoutesMap` | **27** | 28 |
| Screen configurations a route names | **25** | 26 |
| Registered but unrouted — dead configuration | **0** | 4 |
| Routes reachable by any authenticated user | **16** | 16 |
| Routes needing `admin` or a capability | **6** | 6 |
| Routes with no configured edge | **5** | 6 |

**`orphanScreenKeys` is now empty, and that is a result rather than a default.**
The four it held are exactly the four C.0 deleted, so the list has gone to zero by
the work it existed to commission — which is the only way a dead-configuration
list should ever empty. `routesWithNoConfiguredEdge` is the one left worth reading
first.

**Each route also carries its own inventory** — `tables` and `dialogForms`, the
configuration reachable from that screen. `screen_configs.json` counts the
non-flow surface in total; this says which route each piece of it belongs to,
which is what turns a total into an order of work.

**Sixteen of that file's 25 table configurations are reached from a non-flow
route by this closure. The other nine are not, and neither reason is "dead
code":**

| Tables | Why the closure does not reach them |
|---:|---|
| 8 | The Workspace IDE's tabbed sections. `MenuEntry.formConfigKey` is decided at run time from the server's file-tree response, by `mapMenuEntry` (`workspace_ide/screen_delegates_helpers.dart:159`), so the edge is *server data* rather than a declaration this closure can walk. |
| 1 | `inputRecordsFromProcessErrorTable`, inside an `inputFieldRowBuilder` closure — the fourth field container `allFields` deliberately does not walk, because its fields do not exist until a form is driven. |

**The third row is gone rather than reclassified.** It held
`reteSessionTriplesTable`, whose only opener was a commented-out `configForm` and
which this table called the one that really was dead; C.0 deleted it. The count
fell by three and the reached count by two, because the other two deletions —
`clientsAndProcessesTableView` and `ruleConfigTable` — *were* reached, from the
`/processConfig` route that C.0 deleted alongside them.

**~~a *string template* over server data~~ — corrected 2026-08-25.** That row
described `formConfigKey` as composed as `"workspace.$pageMatchKey.form"`, which
it was when this file was written and has not been since C.1 (jetstore#2019):
`compiledViewFormKey` (`workspace_ide/screen_delegates_helpers.dart:149`) is now a
lookup into the two-entry `compiledViewForms` map, total by construction, and a
section this client has no view for gets `null` rather than a key that misses.
**What the row was saying is unchanged** — the closure cannot walk the edge either
way, because the input is a server response — but the mechanism it named is gone,
and a reader sent looking for that template would not find it.

The first row is the one that matters: the Workspace IDE is first in the
migration order, and its screen inventory is data-driven from the apiserver
rather than enumerable here.

**"No configured edge" is a statement about the configuration, not a claim that
the route is dead** (I-20). All five are reached from Dart:
`/login` and `/register` from `user_delegates.dart` and `http_client.dart`,
`/userGitProfile/…` from `userGitProfilePath` in `app_bar.dart:43`, `/workspaces/:workspace_name/home`
from `screen_delegates.dart` and `screen_delegates_helpers.dart`, and `/404` from
the route parser's fallback.

**There were six, and the sixth was `/processConfig`, reached from nowhere at
all** because both `processConfig` menu entries pointed at `ruleConfigPath`. C.0
deleted the route and its screen, so all five that remain are reached from Dart
and the distinction this paragraph draws no longer has a counterexample in the
corpus. It is kept because the distinction is what stops the next reader treating
"no configured edge" as "dead".

## Regenerating

```bash
cd jetsclient
CHROME_EXECUTABLE=/usr/bin/google-chrome \
  flutter test --platform chrome test/screen_reachability_corpus_test.dart
```

Same procedure as above: copy the JSON printed between
`===BEGIN SCREEN REACHABILITY CORPUS===` and
`===END SCREEN REACHABILITY CORPUS===` into this file, and put the printed
checksum into `expectedChecksum` in that test. **Both, together** — and here too
that is now enforced by `jetsclient_ide/src/corpusFixtures.test.ts` rather than
asked for.

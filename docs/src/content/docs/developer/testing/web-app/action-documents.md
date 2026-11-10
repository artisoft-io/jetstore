---
title: Validating action documents
description: What the checks on a user flow's action document guarantee, the ways a valid document can still be unfaithful to what it was derived from, and what changes when the documents are generated.
kind: explanation
sidebar:
  order: 3
---

`jetsclient_ide/src/actions/` holds the action grammar. The schema is `schema.ts`, which emits
`action.schema.json`; `interpret.ts` runs a step list against form state; `escapes.ts` and
`registry.ts` are the bodies a step cannot say and the names they are reachable under. The action
document each flow runs is a `.ua.json` under `jets/workspace_assets/user_flows/`, which has its own
note on the format (`actions.md`).

**This page is about the gap between *valid* and *faithful*.** It is written for anyone who authors
one of these documents by hand or emits one from a generator. The flows were first written by
transcribing the retired Flutter client's Dart, and the list below is what that transcription found.

## What the checks guarantee

| Check | Where | What it refuses |
|---|---|---|
| `ActionDocumentSchema` | `src/actions/schema.ts`, and `action.schema.json` for the Go side | a step verb, value form or field the grammar does not declare |
| `checkActions` | `src/userflow/documentSet.ts` | a state or a form button naming an action the document does not define |
| `validateTableActions` | `src/userflow/documentSet.ts` | a table's `doAction` naming an action the document does not define, and a `showDialog` naming a form the form document does not hold |
| `validateDocumentSet` | `src/userflow/documentSet.ts` | the above across a flow, form and action document together |

**The last two are callable with parsed documents and nothing else**, deliberately, and each function's
own docblock says so, because that is what lets a generator run them without a browser and without
fetching a `table_configs/` directory.

## What they do not guarantee

**A document that passes every check above can still not do what the thing it was derived from
does.** For the transcribed flows the thing derived from was a Dart delegate arm, a function body, so
nothing could serialise it and no corpus could be generated for it. Nine findings reduced to **five
distinct failures**:

| | The failure | Is a check possible? |
|---|---|---|
| **1** | **The arm is not in the document at all.** | **Yes, and it is already made.** The three checks above cover the three places a name can appear: a state, a form button, a table action. |
| **2** | **A step is missing from the arm.** | No. One tell is a statement that reads as a no-op: `state[k] = unpack(state[k])` is a conversion the server type-asserts on. |
| **3** | **A step is present and its value expression is wrong.** | No. Three tells: a nested call written as its outer half; the left-hand side supplying the right; a name-to-name map written as an identity where one end's names are decided somewhere else. |
| **4** | **The steps are in the source's reading order rather than its effect order.** | No. Every step present, every step right. The tell is a `post` with a mutation of a key the payload carries anywhere near it. |
| **5** | **The arm has fewer exits than the source has outcomes.** | Not by reading the arm. The check is arithmetic: an arm should have as many exits as the source has returns and reassignments of its post target, and the branch may be *outside* the arm, either because the grammar of the day could not express it or because it lives in the helper the arm calls. **When a step is a call, read the callee.** |

**A sixth is not about the document at all.** The five above are ways a *document* can be wrong while
every check passes. This one is a way the *test* can be wrong while it passes, and it belongs here
because the remedy for failures 2 to 5 is "write a test that drives the thing", so a test that goes
green without exercising what it names undoes the whole list.

| | The failure | Is a check possible? |
|---|---|---|
| **6** | **The test passes on something other than what it names.** | Not by a tool. Two shapes. **The harness omits what renders the assertion's subject**: a screen *raises* a banner and `AppShell` *renders* it, so a test that mounts the screen without the shell and then waits for `role="alert"` after a server refusal finds a *field* error and passes. **The fixture does not reach the path**: a "valid" form fixture with a three-character git handle against a `> 3` rule meant four tests that read as exercising submit never called it. |

The tells are cheap and worth the habit: **assert on text rather than on a role** when two things in
the tree can carry that role, and **make the happy-path fixture prove itself once**: one assertion
that the request was sent is what turns three silent tests into three real ones.

**Only the first has an oracle.** For the other four, fidelity was established by re-deriving each arm
from the Dart, and beyond that by running a flow against a live server and diffing the payload, which
was done for one flow, `lfLoadFilesUF`, against
`src/datatable/fixtures/load_files_flutter_audit.log`.

## The part of the transcription that was checked

**147 of 147 buttons matched the Dart**: the same key, label, style, capability and enablement, across
51 forms, measured on 2026-08-25. It is re-runnable: `src/buttonFidelity.test.ts` reads the two
generated corpora and the `.form.json` documents in its `FLOW_DOCUMENTS` and compares them button by
button, on every `npm test`.

**It is worth stating as a measurement, because it settled a real gap.** `form_fields.json`, the
corpus the form work was sized against, once reported **zero** of the flows' action-bar buttons,
because `FormConfig.actions` was a container no traversal walked. The flows were transcribed from the
Dart rather than from the corpus, which is why the transcription worked and also why nothing checked
it. **What was unverified was fidelity, not function.**

**The 7 capability claims were the part worth checking hardest, and all 7 are present.** A button that
lost its `capability` in transcription is a control offered to someone the Dart hides it from; the
server refuses either way, so the failure would have been a user pressing a button that cannot work,
**not** a privilege escalation.

Three things a later check of this kind has to get right:

- **Two corpora, not one.** `homeFiltersUF` carries `showFailureDetailsDialog`, whose Dart lives in
  `screen_configs.json` because it is a non-flow screen's dialog. Reading only `form_fields.json`
  reports it as a document form with no Dart counterpart.
- **A button can sit among the fields.** Three do (`FormActionConfig` in the Dart, `field: "button"`
  in the document), and the two containers have to be compared separately, because swapping one for
  the other is a real difference.
- **Three action names were deliberately renamed**, and none was forced by the schema: `Identifier`
  accepts a dot. `dialog.cancelAction` → `dialogCancel` because `DIALOG_CANCEL` is the dialog host's
  own constant; `mapper.ok` and `mapper.draft` → `mapperOk`, `mapperDraft` as a naming choice.

**This covers failure 1 of the five and none of the other four.** A button that is present, correctly
named and correctly gated can still run the wrong action, and nothing about a declaration shows it.

## If you are generating these documents rather than writing them

`cpipes-contract templates --project` emits a `.uf.json`, `.form.json` and `.ua.json` from a compute
pipes template, plus an `.apply.json` that is not a user-flow document, into
`jets/workspace_assets/user_flows/`, where the installer takes them (see
[Projections](/docs/developer/tools/cpipes-contract/projections/)). Those documents are transcriptions
in the same sense: the schema says they are well formed and says nothing about whether they say what
the template means.

**Three things change and two do not.**

- **Failure 1 stops needing a human and starts needing one call.** Run `validateDocumentSet` and
  `validateTableActions` over the emitted set. The one recorded set-level defect in the projected
  corpus is exactly this class: the projection emitted two documents where `FlowStore.load` requires
  three, every per-document layer passed, and it was found by a person at load time. It is checkable
  earlier than that, by the two functions above.
- **Failure 4 largely goes away.** A generator that emits steps in a fixed order from one traversal
  has no reading order to confuse with an effect order. A hand-transcriber reads down the page and a
  generator does not.
- **Failures 2, 3 and 5 recur in generated form**, and 5 is the one to watch. A hole the traversal
  never visits produces no state, no field and no substitution, and *every downstream layer agrees*,
  because every layer is downstream of the same traversal. A value bound from the wrong end of a
  mapping is failure 3 with the names supplied by a type rather than by a reading. And a hole whose
  value comes out of a helper carries the helper's branches, which is failure 5 exactly.

**The last point is the one worth arguing rather than listing.** Sharing one traversal between the
projection and the plan it emits is what stops the two drifting, and it is also what removes the
independent second reading that would catch a mistake *in the traversal*. That is a good trade and it
is not a free one. What restores the missing reading is a check on the **outcome** against something
the traversal did not produce: for `qc_metrics` that is
`tools/cpipes_contract/projections/qc_metrics.demonstrated.pc.json` validating against the full
compute pipes contract, which is a stronger fidelity test than a hand-transcribed arm ever had.

The same failures 2 to 5 apply to the screens ported from the same Dart (`screen_delegates.dart` and
`user_delegates.dart` were function bodies on the same terms the flow delegates were), so the first
consumer of this list is as likely to be a screen as a flow.

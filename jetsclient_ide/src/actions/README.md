# `src/actions/` — the action grammar, and what validating a document does not tell you

The schema is `schema.ts`, which emits `action.schema.json`; `interpret.ts` runs a
step list against form state; `escapes.ts` and `registry.ts` are the bodies a step
cannot say and the names they are reachable under. The eleven documents a migrated
flow runs are under `flows/`, with their own README.

**This file is about the gap between *valid* and *faithful*.** It is written for
anyone who authors one of these documents by hand or emits one from a generator,
and it is the surviving form of a list this project accumulated one flow at a time
between 2026-08-23 and 2026-08-24.

## What the checks do guarantee

| Check | Where | What it refuses |
|---|---|---|
| `ActionDocumentSchema` | `schema.ts`, and `action.schema.json` for the Go side | a step verb, value form or field the grammar does not declare |
| `checkActions` | `../userflow/documentSet.ts:139` | a state or a form button naming an action the document does not define |
| `validateTableActions` | `../userflow/documentSet.ts:255` | a table's `doAction` naming an action the document does not define, and a `showDialog` naming a form the form document does not hold |
| `validateDocumentSet` | `../userflow/documentSet.ts:336` | the above across a flow, form and action document together |

**The last two are callable with parsed documents and nothing else** — deliberately,
and each function's own docblock says so, because that is what lets a generator run
them without a browser and without fetching a `table_configs/` directory.

## What they do not guarantee, and it is the whole point of this file

**A document that passes every check above can still not do what the thing it was
derived from does.** For this corpus the thing derived from is a Dart delegate arm —
a function body, so nothing can serialise it and no corpus can be generated for it
(`sizing_action_grammar.md` §2). Nine re-partitions produced nine recorded findings.
They reduce to **five distinct failures**, and the reduction is worth more than the
list because the entries were named rather than numbered and read as nine unrelated
things:

| | The failure | Recorded as | Is a check possible? |
|---|---|---|---|
| **1** | **The arm is not in the document at all.** | I-120 | **Yes, and it is already made.** The three checks above cover the three places a name can appear — a state, a form button, a table action (I-88). |
| **2** | **A step is missing from the arm.** | I-84, I-100 | No. The tell for I-100's variant is a statement that reads as a no-op: `state[k] = unpack(state[k])` is a conversion the server type-asserts on. |
| **3** | **A step is present and its value expression is wrong.** | I-97, I-110, I-116 | No. Three tells: a nested call written as its outer half; the left-hand side supplying the right; a name-to-name map written as an identity where one end's names are decided somewhere else. |
| **4** | **The steps are in the source's reading order rather than its effect order.** | I-90 | No. Every step present, every step right. The tell is a `post` with a mutation of a key the payload carries anywhere near it. |
| **5** | **The arm has fewer exits than the source has outcomes.** | I-115, I-130 | Not by reading the arm. The check is arithmetic: an arm should have as many exits as the source has returns and reassignments of its post target — and the branch may be *outside* the arm, either because the grammar of the day could not express it (I-115) or because it lives in the helper the arm calls (I-130). **When a step is a call, read the callee.** |

**Only the first has an oracle.** For the other four, fidelity was established by
re-deriving each arm from the Dart, and beyond that by running a flow against a live
server and diffing the payload — which has been done for one flow, `lfLoadFilesUF`,
against `../datatable/fixtures/load_files_flutter_audit.log`.

## If you are generating these documents rather than writing them

`tools/cpipes_contract/projections/` emits a `.uf.json`, `.form.json` and `.ua.json`
from a cpipes template, plus an `.apply.json` that is not a UserFlow document. Those
documents are transcriptions in the same sense: the schema says they are well
formed and says nothing about whether they say what the template means.

**Three things change and two do not.**

- **Failure 1 stops needing a human and starts needing one call.** Run
  `validateDocumentSet` and `validateTableActions` over the emitted set. The one
  recorded set-level defect in the projected corpus is exactly this class — the
  projection emitted two documents where `FlowStore.load` requires three, every
  per-document layer passed, and it was found by a person at load time (the M.4 row
  in `projects/agentic_ai/plan/tracking/phase2_tasks_status.md`, whose own note says
  the set is *"only checkable at load, which is the one layer a generator does not
  run"*). It is checkable earlier than that, by the two functions above.
- **Failure 4 largely goes away.** A generator that emits steps in a fixed order
  from one traversal has no reading order to confuse with an effect order. A
  hand-transcriber reads down the page and a generator does not.
- **Failures 2, 3 and 5 recur in generated form**, and 5 is the one to watch. A hole
  the traversal never visits produces no state, no field and no substitution, and
  *every downstream layer agrees* — because every layer is downstream of the same
  traversal. A value bound from the wrong end of a mapping is failure 3 with the
  names supplied by a type rather than by a reading. And a hole whose value comes
  out of a helper carries the helper's branches, which is I-130 exactly.

**The last point is the one worth arguing rather than listing.** Sharing one
traversal between the projection and the plan it emits is what stops the two
drifting, and it is also what removes the independent second reading that would
catch a mistake *in the traversal*. That is a good trade and it is not a free one.
What restores the missing reading is a check on the **outcome** against something
the traversal did not produce — for `qc_metrics` that is
`projections/qc_metrics.demonstrated.pc.json` validating against the full cpipes
contract, which is a stronger fidelity test than this project ever had for a
hand-transcribed arm.

**Who this is addressed to, named as a task rather than as a condition:** the
`agentic_ai` project's **U.2** — putting a projected flow's document set where
`FlowStore` reads — and **U.3**, driving one in a browser end to end
(`projects/agentic_ai/plan/tracking/phase3_tasks_status.md`, track U). A note that
names a future *condition* goes silent when the condition passes; this repository
has that written up at length in its root `CLAUDE.md`.

**And it is not addressed only to them.** Track C authors table and form documents
for the non-flow screens from the same Dart, so failures 2 through 5 are as live
there as they were in track F.

*F.9, 2026-08-24. The list is I-84 and its eight successors in
`projects/ui_refresh/plan/tracking/phase3_tasks_issues_risks.md`; the decision to
reduce it and put it here is I-146.*

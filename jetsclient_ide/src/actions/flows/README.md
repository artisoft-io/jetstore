# `*.ua.json` — a flow's actions, authored as data

One document per flow, named for the flow it serves and sitting beside the
`.uf.json` in `../../userflow/flows/`. Validated against `../action.schema.json`,
which `../schema.ts` emits.

**Two flows so far, deliberately.** These are the proof flows the plan nominates
(`plan/phase2_plan.md` §2 item 6): `register_file_key`, 2 arms and no data table
at all, and `load_files`, 3 arms including the corpus's only fan-out. The other
nine flows' 53 arms are migrated per flow by S.5 and Phase 3 — the grammar has to
be complete because it is a schema, but the migration does not.

## Field order is the Dart's, and that is on purpose

Inside a `fields` map the order is the order the Dart constructs the row in, so a
`/dataTable` payload captured from one app diffs cleanly against the other. The
server unmarshals into a map and does not care; the *diff* is what this buys, and
it is how `lfLoadFilesUF` is checked against
`../../datatable/fixtures/load_files_flutter_audit.log` — the same capture that
closed I-4 for the read side.

There is no place to write that in the documents themselves: every object in the
schema is closed, so a `_comment` key is a validation error rather than a note.
That is the right trade and it is why this file exists.

## What is not here

**No action body that needs an `escape`.** Neither proof flow has one; four of the
58 arms across all flows do (I-19), and those name a compiled function through the
registry in `../escapes.ts`.

**No `formStateInitializer`.** One flow sets one — `homeFiltersUF` — and it is not
a proof flow.

# `*.ua.json` — a flow's actions, authored as data

One document per flow, named for the flow it serves and sitting beside the
`.uf.json` in `../../userflow/flows/`. Validated against `../action.schema.json`,
which `../schema.ts` emits.

**Nine flows as of F.6, and this paragraph said *two* until then.** It described
the two proof flows the plan nominates (`plan/phase2_plan.md` §2 item 6) —
`register_file_key`, 2 arms and no data table at all, and `load_files`, 3 arms
including the corpus's only fan-out — and every sentence under *What is not here*
was true of those two and of nothing since. Track F has landed seven more:
`mapFileUF`, `loadConfigUF`, `workspacePullUF`, `clientRegistryUF`,
`startPipelineUF`, `homeFiltersUF` and `pipelineConfigUF`. Two coverage documents
remain in `../coverage/`, which is `fileMappingUF` (F.8) and `sourceConfigUF`
(F.7).

**Nobody re-read this file for five tasks**, which is the shape the repository
`CLAUDE.md` describes: a standing claim with no owner between consumers, corrected
by the first task that had a reason to open it. F.6 had one — it is the ninth
document — and the numbers below are now the ones a check would produce.

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

~~**No action body that needs an `escape`.**~~ True of the two proof flows and
false since F.5. `homeFiltersUF` names `updateHomeFilters` and `clearHomeFilters`,
and `mapFileUF` names `downloadMapping`; the bodies are in `../homeFilters.ts` and
`../fileMapping.ts` and the registry is `../registry.ts`. The count is still an
upper bound rather than a target (I-74): two of the sizing's four turned into
grammar rather than into escapes.

~~**No `formStateInitializer`.**~~ `homeFiltersUF` sets one — `seedFromHomeFilters`
— and it is the only one in the corpus.

**No `.pc.json`, and this one is still true.** These documents describe user
flows. A pipeline configuration is another project's file type and another
project's validator row.

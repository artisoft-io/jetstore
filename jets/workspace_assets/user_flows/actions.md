# `*.ua.json` — a flow's actions, authored as data

**This file was `jetsclient_ide/src/actions/flows/README.md` and moved here with the
documents it describes, 2026-08-26.** Every `../` path in it was relative to
`jetsclient_ide/src/actions/`, so each has been rewritten to say where it is from the
repository root rather than left to resolve against a directory this file is no longer
in — which is the failure a moved document invites and the one nothing would have
reported. The rewrite was checked by assertion rather than by reading: the first pass
missed one and the check named it.

One document per flow, named for the flow it serves and sitting beside the
`.uf.json` in this directory. Validated against `jetsclient_ide/src/actions/action.schema.json`,
which `jetsclient_ide/src/actions/schema.ts` emits.

**Eleven flows as of F.7, which is all of them, and this paragraph said *two*
until F.6.** It described the two proof flows the plan nominates
(`plan/phase2_plan.md` §2 item 6) — `register_file_key`, 2 arms and no data table
at all, and `load_files`, 3 arms including the corpus's only fan-out — and every
sentence under *What is not here* was true of those two and of nothing since.
Track F has landed nine more: `mapFileUF`, `loadConfigUF`, `workspacePullUF`,
`clientRegistryUF`, `startPipelineUF`, `homeFiltersUF`, `pipelineConfigUF`,
`fileMappingUF` and `sourceConfigUF`.

**`jetsclient_ide/src/actions/coverage/` is gone.** It held one transcription per unmigrated flow, wired to
nothing; F.7 promoted the last of them and deleted the directory. So every document
in *this* directory is one a flow runs, and the distinction the two directories drew
no longer needs a place to live.

**What the transcriptions taught is in `jetsclient_ide/src/actions/README.md`** — the five ways a document
that validates is still not faithful to what it was derived from, and which one of
the five a check can find. F.9, 2026-08-24. `jetsclient_ide/src/actions/coverage.test.ts` is
`jetsclient_ide/src/actions/flowActions.test.ts` as of the same task, for the same reason the directory
went.

**Nobody re-read this file for five tasks**, which is the shape the repository
`CLAUDE.md` describes: a standing claim with no owner between consumers, corrected
by the first task that had a reason to open it. F.6 had one — it is the ninth
document — and the numbers below are now the ones a check would produce.

## Field order is the Dart's, and that is on purpose

Inside a `fields` map the order is the order the Dart constructs the row in, so a
`/dataTable` payload captured from one app diffs cleanly against the other. The
server unmarshals into a map and does not care; the *diff* is what this buys, and
it is how `lfLoadFilesUF` is checked against
`jetsclient_ide/src/datatable/fixtures/load_files_flutter_audit.log` — the same capture that
closed I-4 for the read side.

There is no place to write that in the documents themselves: every object in the
schema is closed, so a `_comment` key is a validation error rather than a note.
That is the right trade and it is why this file exists.

## What is not here

~~**No action body that needs an `escape`.**~~ True of the two proof flows and
false since F.5. `homeFiltersUF` names `updateHomeFilters` and `clearHomeFilters`,
and **`fileMappingUF`** names `downloadMapping` and `loadRawRows`; the bodies are
in `jetsclient_ide/src/actions/homeFilters.ts` and `jetsclient_ide/src/actions/fileMapping.ts` and the registry is
`jetsclient_ide/src/actions/registry.ts`. **This sentence said `mapFileUF` named `downloadMapping` and it
never did** — the two flows share the `file_mapping/` directory and nothing else
(I-61), and `mapFileUF`'s two bodies are a row seeder and a validator. Corrected
by F.8, whose flow it is.

The count is still an upper bound rather than a target (I-74): two of the sizing's
four turned into grammar rather than into escapes. **What F.8 adds is that it is
an upper bound on a narrower question than the one I-74 asked** — `loadRawRows`
is four lines the grammar can say exactly, and it is a body because S.7's
allowlist refuses its target rather than because the vocabulary is short (I-121).

**F.7 closes the question with a third answer and the count stayed at five, which
is the wrong thing to read off it.** `sourceConfigUF` names `readXlsxSheetOption`
and `saveSourceConfigForFileType`; the coverage document named
`loadSourceConfigWithFileTypeInference` and `saveSourceConfigForFileType`. The
first arm is now twenty-one steps and a four-line body, because F.2's `when` guard
says everything about it except `JSON.parse`. The second stays whole because
`wholeState`'s `normalise` and `omit` carry no guard, so the Dart's per-file-type
projection of a *copy* of the state has no guarded form — neither vocabulary nor
permission, but a gap in the payload grammar. See `jetsclient_ide/src/actions/sourceConfig.ts`.

~~**No `formStateInitializer`.**~~ `homeFiltersUF` sets one — `seedFromHomeFilters`
— and it is the only one in the corpus.

**No `.pc.json`, and this one is still true.** These documents describe user
flows. A pipeline configuration is another project's file type and another
project's validator row.

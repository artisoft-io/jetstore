---
title: "Flow actions"
description: "The .ua.json documents: a flow's actions authored as data, the clear-state lists of the add arms, field order, and what the documents leave to code."
kind: explanation
sidebar:
  order: 5
---

A `.ua.json` is a flow's actions, authored as data: one document per flow, named for the flow it
serves and sitting beside the `.uf.json` in `jets/workspace_assets/user_flows/`. Validated against
`jetsclient_ide/src/actions/action.schema.json`, which `jetsclient_ide/src/actions/schema.ts` emits.

**Ten since 2026-10-01**, when `register_file_key` (`registerFileKeyUF`) was retired; what follows
is the history it was written as.

**Eleven flows were ported.** The first two were the proof flows — `register_file_key`, 2 arms and no
data table at all, and `load_files`, 3 arms including the corpus's only fan-out — and nine more
followed: `mapFileUF`, `loadConfigUF`, `workspacePullUF`, `clientRegistryUF`, `startPipelineUF`,
`homeFiltersUF`, `pipelineConfigUF`, `fileMappingUF` and `sourceConfigUF`.

**`jetsclient_ide/src/actions/coverage/` is gone.** It held one transcription per unmigrated flow,
wired to nothing, and was deleted when the last of them was promoted. So every document in this
directory is one a flow runs, and the distinction the two directories drew no longer needs a place
to live.

**What the transcriptions taught is in the React client's actions notes**
([Action documents](/docs/developer/testing/web-app/action-documents/)) — the five ways a document that validates is still not
faithful to what it was derived from, and which one of the five a check can find (2026-08-24).
`jetsclient_ide/src/actions/coverage.test.ts` became `jetsclient_ide/src/actions/flowActions.test.ts`
at the same time, for the same reason the directory went.

## The two clear-state lists, and why each key is on them

**Added 2026-10-01.** *Source Configuration* and *Pipeline Configuration* now open on their table and
return to it after *Save*, for an edit as well as an add. Both wizards decide insert-or-update from
what is in form state — `saveSourceConfigForFileType` updates whenever `key` is set
(`jetsclient_ide/src/actions/sourceConfig.ts`), and `pcSavePipelineConfigUF` updates whenever
`pcPipelineConfigTable` is — and a flow run holds one form state. So *edit → back to the table →
+ Add → Save* would update the record just edited. **The *+ Add* arm removes every key the edit path
can set**, and these lists were enumerated from the documents before the arms were written, so that
none is missed. The documents cannot carry the reasoning — every object is closed — so it is here.

**`scGoToAddSourceConfig`** clears the selection of three tables and removes ~~seventeen~~ eighteen
keys (`is_cumulative`, Release 4.0.4, 2026-10-15):

| Key | Set on the edit path by |
|---|---|
| `scSourceConfigKey` (and its selection) | the table's own widget key |
| `key` | the table's `formStateBinding`, then `scSelectSourceConfigUF` — **the key that decides insert or update** |
| `client`, `org`, `object_type`, `automated`, `table_name` | the binding, then `scSelectSourceConfigUF` (which also writes `automated` as `'0'`) |
| `domain_keys_json`, `code_values_mapping_json`, `schema_provider_json` | the binding and the select action; their own pages after |
| `input_columns_json`, `input_columns_positions_csv` | the binding and the select action; the headers and fixed-width pages |
| `input_format` (and its selection) | the binding, the select action's inference, then the file-type table |
| `is_part_files` | the binding |
| `input_format_data_json` | the binding and the select action; `scEditXlsxOptionsUF` |
| `scSingleOrMultiPartFileOption` (and its selection) | the select action from `is_part_files`; the single-or-multi-part table |
| `currentSheet` | the `readXlsxSheetOption` escape; the xlsx options page |
| `is_cumulative` | the binding, then `scSelectSourceConfigUF`; the single-or-multi-part page's dropdown |

**`pcGoToAddPipelineConfig`** clears the selection of nine tables and removes thirty-two keys. The
first seventeen are what the selection and `pcSelectPipelineConfigUF` write; the other fifteen are
written further in, and are listed because *starts empty* is the requirement, not *does not update*:

| Key | Set on the edit path by |
|---|---|
| `pcPipelineConfigTable` (and its selection) | the table's own widget key, and `pcSelectPipelineConfigUF` — **the key that decides insert or update** |
| `key`, `client`, `process_name`, `process_config_key`, `main_process_input_key`, `merged_process_input_keys`, `main_object_type`, `main_source_type`, `source_period_type`, `automated`, `description`, `max_rete_sessions_saved`, `injected_process_input_keys`, `rule_config_json`, `entity_rdf_type` | the table's `formStateBinding` (fifteen columns), most of them again by `pcSelectPipelineConfigUF` |
| `pcMainProcessInputKey` (and its selection) | `pcSelectPipelineConfigUF`, then the main-input table |
| `pcViewMergedProcessInputKeys`, `pcMergedProcessInputKeys`, `pcViewInjectedProcessInputKeys`, `pcInjectedProcessInputKeys` (and their selections) | the four merge and injection tables |
| `ufAllProcessInputKeys` | `pcPrepareSummaryUF` |
| `pcSummaryProcessInputs` (and its selection) | the summary's table |
| `pcProcessInputRegistry`, `pcProcessInputRegistry4MI` (and their selections) | `pcSetProcessInputRegistryKey` and the process-input dialogs' tables |
| `org`, `object_type`, `source_type`, `table_name`, `lookback_periods`, `user_email` | the process-input dialogs and `addProcessInputOk` |
| `serverError` | the interpreter, when the dialog's `insertRows` post fails |

**Both are tested as *empty*, not as *inserts*.** `proofFlows.test.ts` walks every page the edit path
writes to and asserts that `formState.snapshot(0)` is `{}` after *+ Add*;
`FlowRunner.addReturn.test.tsx` drives *edit → back → + Add → Save* through the screen and asserts
the second save is an insert. **Taking `key` out of the first list turns that save into
`update/source_config` and both tests red** — measured 2026-10-01.

**What the lists do not cover is an edit after an edit**, which the same change also made possible.
The selection rewrites every key the binding publishes, and `scSelectSourceConfigUF` now begins by
removing the two it does not always rewrite — `currentSheet` and `scSingleOrMultiPartFileOption` — so
a non-xlsx record does not open on the previous record's sheet.

## Field order is the Dart's, and that is on purpose

Inside a `fields` map the order is the order the Dart constructs the row in, so a `/dataTable`
payload captured from one app diffs cleanly against the other. The server unmarshals into a map and
does not care; the *diff* is what this buys, and it is how `lfLoadFilesUF` is checked against
`jetsclient_ide/src/datatable/fixtures/load_files_flutter_audit.log` — the same capture that checked
the read side.

There is no place to write that in the documents themselves: every object in the schema is closed,
so a `_comment` key is a validation error rather than a note. That is the right trade and it is why
this page exists.

## What is not here

~~**No action body that needs an `escape`.**~~ True of the two proof flows and false since the flows
that followed. `homeFiltersUF` names `updateHomeFilters` and `clearHomeFilters`, and
**`fileMappingUF`** names `downloadMapping` and `loadRawRows`; the bodies are in
`jetsclient_ide/src/actions/homeFilters.ts` and `jetsclient_ide/src/actions/fileMapping.ts` and the
registry is `jetsclient_ide/src/actions/registry.ts`. **An earlier version of this note said
`mapFileUF` named `downloadMapping` and it never did** — the two flows share the `file_mapping/`
directory and nothing else, and `mapFileUF`'s two bodies are a row seeder and a validator.

The count of escapes is still an upper bound rather than a target: two of the four first sized turned
into grammar rather than into escapes. And it is an upper bound on a narrower question than it first
seemed — `loadRawRows` is four lines the grammar can say exactly, and it is a body because the action
allowlist refuses its target rather than because the vocabulary is short.

**`sourceConfigUF` gives a third answer, and the count stayed at five, which is the wrong thing to
read off it.** `sourceConfigUF` names `readXlsxSheetOption` and `saveSourceConfigForFileType`; the
coverage document named `loadSourceConfigWithFileTypeInference` and `saveSourceConfigForFileType`.
The first arm is now twenty-one steps and a four-line body, because the `when` guard says everything
about it except `JSON.parse`. The second stays whole because `wholeState`'s `normalise` and `omit`
carry no guard, so the Dart's per-file-type projection of a *copy* of the state has no guarded form
— neither vocabulary nor permission, but a gap in the payload grammar. See
`jetsclient_ide/src/actions/sourceConfig.ts`.

~~**No `formStateInitializer`.**~~ `homeFiltersUF` sets one — `seedFromHomeFilters` — and it is the
only one in the corpus.

**No `.pc.json`, and this one is still true.** These documents describe user flows. A pipeline
configuration is another file type, with its own validator row.

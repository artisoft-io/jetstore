/**
 * Every action arm the eleven flows define, as step lists.
 *
 * **This exists because S.2a's exit condition said "expressible" and only five
 * arms had been expressed** (I-22). Writing the other fifty is what turns an
 * argument from a vocabulary into a demonstration — and it found two primitives
 * the grammar lacked and two more escapes than the sizing counted (I-23), which
 * is exactly what it was for.
 *
 * **`coverage/` is empty as of F.7 and the directory is gone.** For eight tasks
 * this file read two kinds of document: transcriptions in `coverage/`, wired to
 * nothing, and the runtime documents in `flows/` that track F promotes them into.
 * `sourceConfigUF` was the last transcription; every import below is now a
 * document a flow actually runs.
 *
 * **What the transcriptions did not prove, and it must not be forgotten now they
 * are gone.** They were written from reading the Dart, so the schema could tell
 * you a document was well formed and nothing could tell you it was *faithful*. A
 * delegate is a function body, not an object, so no corpus can be generated for it
 * — see `sizing_action_grammar.md` §2. Every one of the nine re-partitions found
 * something wrong (I-84, I-90, I-97, I-100, I-110, I-115, I-116, I-120, and F.7's
 * I-130), which is what re-deriving each arm from the Dart is for. Fidelity beyond
 * that is established by running a flow against a live server and diffing the
 * payload, as `lfLoadFilesUF` is.
 *
 * **The file keeps its name for one more task.** F.9 owns what becomes of it now
 * that the directory it is named after does not exist.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import clientRegistry from "./flows/clientRegistryUF.ua.json";
import fileMapping from "./flows/fileMappingUF.ua.json";
import homeFilters from "./flows/homeFiltersUF.ua.json";
import loadConfig from "./flows/loadConfigUF.ua.json";
import loadFiles from "./flows/loadFilesUF.ua.json";
import mapFile from "./flows/mapFileUF.ua.json";
import pipelineConfig from "./flows/pipelineConfigUF.ua.json";
import registerFileKey from "./flows/registerFileKeyUF.ua.json";
import sourceConfig from "./flows/sourceConfigUF.ua.json";
import startPipeline from "./flows/startPipelineUF.ua.json";
import workspacePull from "./flows/workspacePullUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "./schema";

const all: Record<string, unknown> = {
  loadFilesUF: loadFiles,
  registerFileKeyUF: registerFileKey,
  // F.1's re-partition: `mapperOk`, `mapperDraft` and `dialogCancel` moved here
  // out of `coverage/fileMappingUF.ua.json`, which is F4's rule — a coverage
  // document is partitioned by *delegate file* and a runtime one by *flow*, and
  // `file_mapping/` is one directory holding two flows (I-61). The three that
  // stay behind belong to `fileMappingUF`, which is F.8.
  mapFileUF: mapFile,
  // F.2's re-partition, and the one the plan's F4 was written about
  // (`actions/coverage/workspacePullUF.ua.json`, now gone): one *delegate file*
  // holding two switches for two flows. `wpLoadConfigConfirmUF` is a state
  // action of both, so it appears in both documents; `dialogCancel` appears in
  // neither, because no form or table of either flow offers it.
  loadConfigUF: loadConfig,
  workspacePullUF: workspacePull,
  // F.3's re-partition. One delegate file, one flow — so unlike F.1's and F.2's
  // nothing had to be split. What it dropped is `crShowVendorUF`, whose body
  // jumps the flow to `show_org` and which **no form and no table of the flow
  // offers** (I-91): the reachability shape of I-86, on a live arm rather than a
  // cancel. The two `delete*Action` arms are here because the *tables* offer
  // them, which is the third place an action name can appear and the first time
  // a migrated set has used it.
  clientRegistryUF: clientRegistry,
  // F.4's re-partition. One delegate file, one flow, and nothing to split — the
  // same shape as F.3's. What it changed is inside two arms rather than in the
  // partition: `spPipelineSelected` reads `unpackToList(unpack(x))` in the Dart
  // and the coverage document had `fromKeyList` alone, which is `unpackToList(x)`
  // and a different function on the value the table publishes (I-97).
  startPipelineUF: startPipeline,
  // F.5's re-partition. One delegate file, one flow, nothing to split — and the
  // first whose arms are not all state actions: `resubmitPipeline` and
  // `dialogCancel` are reached from the *table*, the second row of it, and from
  // the dialog that row opens. What changed inside an arm is a step the coverage
  // document did not have at all: `resubmitPipeline` begins
  // `state[session_id] = unpack(state[session_id])`, which reads as a no-op
  // assignment and is a conversion the server type-asserts on (I-100).
  homeFiltersUF: homeFilters,
  // F.6's re-partition. One delegate file, one flow, nothing to split — and the
  // largest of the nine at 14 `case` labels, a quarter of the 58. **The first
  // whose document holds an arm the nine delegate files do not declare**:
  // `addProcessInputOk` is the Save button of the two dialogs this flow's tables
  // open, and it lives in `modules/actions/config_delegates.dart`
  // (`processInputFormActions`) because those dialogs carry their own
  // `formActionsDelegate` rather than the flow's. So the count *rises* here
  // (I-114). `dialogCancel` is the same story and F.5 has already written it up
  // as I-101: dead in the delegate it was transcribed from, live in the document
  // it becomes.
  pipelineConfigUF: pipelineConfig,
  // F.7's re-partition. One delegate file holding two flows — the shape F.1 and
  // F.2 met — and **the arm that has to be duplicated rather than moved is the
  // one F.1 moved**: `dialogCancel` is a `case` label of *both* switches in
  // `file_mapping/form_action_delegates.dart`, and both flows have a form that
  // offers it. `mapFileUF`'s worksheet is a dialog and so is
  // `loadRawRowsDialog`, which `fmFileMappingTableUF` opens. F.1 took the name
  // with the other two and left three behind, so the coverage document this task
  // promoted was short an arm no step-level re-reading would have found (I-120).
  fileMappingUF: fileMapping,
  // F.7's re-partition, and the last. One delegate file, one flow, nothing to
  // split — and **the first whose document holds fewer arms than the delegate
  // declares for two different reasons at once**. `configureFilesFormActions`
  // declares eight `case` labels; six are here. `scStartUF` is a live `case` whose
  // body is `return null` and which no state, form or table names — the shape of
  // I-91's `crShowVendorUF` with nothing behind it (I-132). `dialogCancel` is the
  // shape of I-86 rather than of I-101: this flow has **no dialog form at all**,
  // its one confirmation being `showConfirmationDialog`, which is a built-in
  // Yes/No and not a form whose Cancel button dispatches an action name.
  sourceConfigUF: sourceConfig,
};

describe("the flows' action documents", () => {
  it.each(Object.keys(all))("%s validates against the grammar", (key) => {
    const result = ActionDocumentSchema.safeParse(all[key]);
    expect(result.success ? [] : result.error.issues).toEqual([]);
  });

  it("accounts for the 58 arms the delegates declare, less the ones nothing reaches", () => {
    // 58 `case ActionKeys.` labels across the nine `form_action_delegates.dart`
    // files. Three flows declare `dialogCancel` twice — once in the user-flow
    // delegate and once in the dialog delegate — which is one action reached
    // from two switches, not two actions. 58 − 3 = 55.
    //
    // **Still 55 after F.1's re-partition, and that is the check that it was a
    // move.** Three names left `coverage/fileMappingUF.ua.json` and three
    // arrived in `flows/mapFileUF.ua.json`; a copy rather than a move would read
    // as 58 here.
    //
    // **Still 55 after F.2's, and there the total conserves by cancellation
    // rather than by construction** — which is why the names are checked below
    // as well as the count. `coverage/workspacePullUF.ua.json` held six names
    // for a delegate file with seven arms (`dialogCancel` twice). The two
    // runtime documents hold six between them: `wpLoadConfigConfirmUF` is
    // **duplicated**, because it is a state action of both flows, and
    // `dialogCancel` is **dropped**, because neither flow has a form or a table
    // that offers it. +1 and −1 is not an invariant, so the assertion below is.
    //
    // **54 after F.3's, and the fall is the point rather than a slip.** That
    // re-partition dropped `crShowVendorUF` and duplicated nothing, so the
    // cancellation F.2 relied on does not apply and the total moves. The arm is
    // live in the Dart — it jumps the flow to `show_org` — and is offered by no
    // form and no table of `clientRegistryUF`, which is I-86's reachability
    // question asked of an arm that *does* something (I-91). **55 was never an
    // invariant**: it is a count of `case` labels minus the ones declared twice,
    // and a migrated corpus needs fewer of them than the delegates declare.
    //
    // **Still 54 after F.4's, and here the conservation is by construction
    // again.** `startPipelineUF` is one delegate file holding one flow, all
    // three of whose arms a state action names, so three names left
    // `coverage/` and the same three arrived in `flows/`. Nothing was dropped
    // and nothing duplicated — which is the F.1 shape rather than the F.2 or
    // F.3 one, and is why the count alone would not have told you.
    //
    // **Still 54 after F.5's, and its `dialogCancel` is the interesting one.**
    // Six names left `coverage/` and the same six arrived in `flows/`. Four are
    // state actions and two are not: `resubmitPipeline` is a table button and
    // `dialogCancel` is the only button of the dialog that table opens.
    //
    // **`dialogCancel` was the arm F.2 dropped from two flows and it is kept
    // here, which is not an inconsistency.** The test is *what can press it*,
    // and the answer changed because the port changed the question. In the Dart
    // this arm is unreachable through `homeFiltersFormActionsUF`: the dialog is
    // built by `showFormDialog` with `showFailureDetailsDialog`'s own
    // `formActionsDelegate`, which is `homeFormActions`
    // (`modules/form_config_impl.dart`), and only the *current state's* form
    // config has its delegate overridden with the flow's
    // (`screens/user_flow_screen.dart`). The port has **one action document per
    // flow rather than one per delegate**, so the dialog's Close button resolves
    // here — and `validateTableActions` requires exactly that. An arm dead in
    // the file it was transcribed from, live in the document it becomes (I-101).
    //
    // **55 after F.6's, and it is the first re-partition that adds.** Fourteen
    // names left `coverage/` and fifteen arrived in `flows/`. The extra is
    // `addProcessInputOk`, which is not one of the 58 at all: the denominator is
    // `case ActionKeys.` across the nine `user_flows/*/form_action_delegates.dart`
    // files, and this arm is in `modules/actions/config_delegates.dart`. **A
    // dialog a flow's table opens may be served by a delegate outside the flow's
    // directory**, so the corpus of arms a migrated flow needs is not a subset of
    // the corpus this count is drawn from (I-114). 54 was as much an invariant as
    // 55 was.
    //
    // **56 after F.8's, and it adds for a different reason than F.6's did.**
    // Three names left `coverage/fileMappingUF.ua.json` and four arrived in
    // `flows/`. The extra is `dialogCancel`, which is not a new arm at all: it is
    // **already in `flows/mapFileUF.ua.json`**, because `file_mapping/` is one
    // directory holding two flows and F.1 *moved* the name rather than
    // duplicating it. This is F.2's `wpLoadConfigConfirmUF` exactly — one arm two
    // flows need — and the difference is that F.2 saw it at the split and F.1
    // could not, because the rule that a table-opened dialog's buttons resolve in
    // the flow's own document was not written until I-101, four tasks later.
    //
    // **54 after F.7's, and the fall of two is the whole of the dead-arm
    // decision F.9 was going to have to take.** Eight names left
    // `coverage/sourceConfigUF.ua.json` and six arrived in `flows/`. Both
    // absentees are `case` labels of `configureFilesFormActions` that nothing can
    // press: `scStartUF`, whose body is `return null` and which appears in no
    // `UserFlowState`, no `FormActionConfig` and no `ActionConfig` (I-132), and
    // `dialogCancel`, which is I-86's shape rather than I-101's — the flow has no
    // dialog *form*, so there is no Cancel button to resolve.
    //
    // **56 was no more an invariant than 55 and 54 were.** This is a count of
    // reachable arms, and it moves whenever a re-partition finds an arm nothing
    // reaches or a dialog served from outside the flow's directory.
    const names = Object.values(all).flatMap((doc) =>
      Object.keys((doc as ActionDocument).actions),
    );
    expect(names).toHaveLength(54);
    expect(new Set(names).size).toBeLessThan(names.length); // dialogCancel repeats across flows
  });

  it("holds sourceConfigUF's four state actions plus the two its table reaches", () => {
    // F.7. `configure_files/form_action_delegates.dart` declares eight `case`
    // labels in one switch, all of them `sourceConfigUF`'s — the file defines one
    // flow and `configureFilesFormActions` is named by nothing else, so this is
    // the F.3/F.4/F.5 shape with nothing to split and no sibling to have taken an
    // arm away (I-120's question, asked and answered *no*).
    //
    // Four state actions (`user_flow_config.dart`), and `scSourceConfigKey`'s two
    // `doAction` buttons — `dropStagingTable` → `dropTable` and `deleteSourceConfig`
    // (`configure_files/data_table_config.dart`). The eleven other forms take
    // `standardActions` and `scSummaryUF` takes Previous / Cancel / Completed, so
    // no form contributes an arm.
    expect(Object.keys((sourceConfig as ActionDocument).actions).sort()).toEqual([
      "addSourceConfig.ok",
      "deleteSourceConfig",
      "dropTable",
      "scAddSourceConfigUF",
      "scEditXlsxOptionsUF",
      "scSelectSourceConfigUF",
    ]);
  });

  it("leaves the organization out of the staging table name when there is none", () => {
    // **I-130, and it is the largest thing F.7 found.** `scAddSourceConfigUF` is
    // `state[table_name] = makeTableNameFromState(state)`, which forwards to
    // `makeTableName` (`modules/actions/delegate_helpers.dart`, `makeTableName`) —
    // and *that* function branches: `'${client}_${org}_$objectType'` when the org
    // is non-empty and `'${client}_$objectType'` when it is not. The coverage
    // document had the first arm's template, unguarded.
    //
    // The empty org is not hypothetical: `scAddSourceConfigUF`'s org dropdown
    // offers *No Organization* with `value: ''` explicitly
    // (`configure_files/form_config.dart`), so every no-org data source would have
    // been staged into `client__object_type` — a table name no existing record has
    // and one the loader would have to create a second time.
    const steps = (sourceConfig as ActionDocument).actions["scAddSourceConfigUF"]!.steps;
    expect(steps.map((s) => (s.do === "set" ? (s.value as { template: string }).template : s.do))).toEqual([
      "{client}_{org}_{object_type}",
      "{client}_{object_type}",
    ]);
    expect(steps.every((s) => s.when !== undefined)).toBe(true);
  });

  it("clears all thirteen keys the delete arm removes, not the six transcribed", () => {
    // I-84's original mechanism, on the largest list in the corpus. The Dart calls
    // `state.remove` thirteen times after encoding the request
    // (`configure_files/form_action_delegates.dart`, `deleteSourceConfig`); the
    // coverage document listed six. The seven missing ones are the record's
    // *content* — its headers, positions, domain keys, code-value mapping and
    // schema provider — and `source_config`'s insert statement reads every one of
    // them (`jets/datatable/sql_stmts.go`, `sqlInsertStmts`), so a user who deleted
    // a configuration and then created another would have inherited the deleted
    // one's.
    const steps = (sourceConfig as ActionDocument).actions["deleteSourceConfig"]!.steps;
    const removal = steps.find((s) => s.do === "remove")!;
    expect(removal.do === "remove" && removal.keys).toHaveLength(13);
    // The post is before the removal, as `deleteClientAction`'s is: the Dart
    // encodes the body *first* and then empties the map, and `wholeState` reads
    // the state at the moment the step runs.
    expect(steps.map((s) => s.do)).toEqual(["confirm", "set", "post", "clearSelection", "remove"]);
  });

  it("holds pipelineConfigUF's thirteen reachable arms plus the two its dialogs need", () => {
    // F.6. `pipeline_config/form_action_delegates.dart` declares fourteen `case`
    // labels in one switch. Thirteen are reachable through it — seven state
    // actions, the two `pc*GotTo*` buttons `pcViewMergeProcessInputsUF` and
    // `pcViewInjectedProcessInputsUF` carry inline (`form_config.dart`), and four
    // the tables offer: `deletePipelineConfig` on `pcPipelineConfigTable`,
    // `pcRemoveMergedProcessInput` and `pcRemoveInjectedProcessInput` on the two
    // view tables, and `pcSetProcessInputRegistryKey` on all three
    // `doActionShowDialog` buttons (`data_table_config.dart`).
    //
    // **The fourteenth, `dialogCancel`, is unreachable *through this delegate*
    // and is here anyway** — the two dialogs set `formActionsDelegate:
    // processInputFormActions`, and only the current state's form has its
    // delegate overridden with the flow's (`screens/user_flow_screen.dart`). It is
    // I-101's shape exactly, one flow later: the port has one action document per
    // flow rather than one per delegate, so the dialog's Cancel resolves here.
    // `addProcessInputOk` is its neighbour in that same switch and is the arm the
    // 58 never counted.
    expect(Object.keys((pipelineConfig as ActionDocument).actions).sort()).toEqual([
      "addProcessInputOk",
      "deletePipelineConfig",
      "dialogCancel",
      "pcAddInjectedProcessInputUF",
      "pcAddMergeProcessInputUF",
      "pcAddPipelineConfigUF",
      "pcGotToAddInjectedProcessInputUF",
      "pcGotToAddMergeProcessInputUF",
      "pcPrepareSummaryUF",
      "pcRemoveInjectedProcessInput",
      "pcRemoveMergedProcessInput",
      "pcSavePipelineConfigUF",
      "pcSelectMainProcessInputUF",
      "pcSelectPipelineConfigUF",
      "pcSetProcessInputRegistryKey",
    ]);
  });

  it("branches the two save arms on whether a row was selected, which the coverage document did not", () => {
    // **I-115, and it is the largest thing F.6 found.** `pcSavePipelineConfigUF`
    // reads `updateKey = unpack(state[pcPipelineConfigTable])` and posts to
    // `update/pipeline_config` with `key` when it is set, `pipeline_config`
    // without it otherwise (`pipeline_config/form_action_delegates.dart`,
    // `pcSavePipelineConfigUF`). The coverage document had one unguarded `post`
    // to `pipeline_config` and no `key` at all — so *Edit an Existing Pipeline
    // Configuration*, one of the flow's two branches from its first state, would
    // have inserted a duplicate row instead of updating the one the user chose.
    // `addProcessInputOk` is the same shape over `process_input` /
    // `update2/process_input`.
    const saves = (pipelineConfig as ActionDocument).actions["pcSavePipelineConfigUF"]!.steps;
    const posts = saves.filter((step) => step.do === "post");
    expect(posts.map((step) => (step.do === "post" ? step.table : null))).toEqual([
      "pipeline_config",
      "update/pipeline_config",
    ]);
    expect(posts.every((step) => step.when !== undefined)).toBe(true);
  });

  it("initialises the two key lists to empty rather than to themselves", () => {
    // **The transcription mechanism F.6 found, and it is a fifth** (I-116).
    // `pcAddPipelineConfigUF` opens `state[merged_process_input_keys] =
    // <String?>[]` — a fresh empty list. The coverage document wrote
    // `{ "fromKeyList": "merged_process_input_keys" }`, which reads the key it is
    // assigning to: the *left*-hand side supplied the right. Two `set` steps say
    // what the Dart says — a `{}` literal, then the conversion that decodes it —
    // which is F.4's finding again, that a lost composition needs no primitive.
    const steps = (pipelineConfig as ActionDocument).actions["pcAddPipelineConfigUF"]!.steps;
    expect(steps.slice(1, 5).map((s) => (s.do === "set" ? [s.key, Object.keys(s.value)[0]] : s.do))).toEqual([
      ["merged_process_input_keys", "literal"],
      ["merged_process_input_keys", "fromKeyList"],
      ["injected_process_input_keys", "literal"],
      ["injected_process_input_keys", "fromKeyList"],
    ]);
  });

  it("names the columns the registered query actually returns", () => {
    // The coverage document's `into` mapped `process_config_key` and
    // `entity_rdf_type` on to columns of the same names; the statement selects
    // `key` and `input_rdf_types::text`
    // (`modules/actions/utils/get_process_info.dart`, `getProcessInputRdfTypes`).
    // Both lookups would have missed, so `entity_rdf_type` would be null and
    // three of this flow's data tables filter `process_input` by it — every data
    // source of every entity type, on every page that picks one.
    const step = (pipelineConfig as ActionDocument).actions["pcAddPipelineConfigUF"]!.steps.at(-1)!;
    expect(step.do === "query" && step.into).toEqual({
      process_config_key: "key",
      entity_rdf_type: "input_rdf_types",
    });
  });

  it("holds every clientRegistryUF arm a form or a table offers, and no other", () => {
    // F.3. `client_registry/form_action_delegates.dart` declares seven `case`
    // labels across its two switches; six are here. The absentee is
    // `crShowVendorUF`, and the check that finds it is the one I-86 used: not
    // *what does the delegate declare* but *what can press it*. The flow's four
    // states take `standardActions` or Previous/Done (`form_config.dart`), the
    // `ufVendor` dialog offers `crAddVendorOk` and `dialogCancel`, and
    // `client`/`org` offer `deleteClientAction` and `deleteOrgAction`
    // (`data_table_config.dart`). Nothing names `crShowVendorUF`.
    expect(Object.keys((clientRegistry as ActionDocument).actions).sort()).toEqual([
      "crAddClientUF",
      "crAddVendorOk",
      "crSelectClientUF",
      "deleteClientAction",
      "deleteOrgAction",
      "dialogCancel",
    ]);
  });

  it("holds homeFiltersUF's four state actions plus the two its table reaches", () => {
    // F.5. `home_filters/form_action_delegates.dart` declares six `case` labels
    // in one switch — four `hf*` state actions falling through to one body,
    // `resubmitPipeline` and `dialogCancel`. All six are reachable and all six
    // are here, which is the first migrated flow where a *table* contributes
    // both an action and a form: `pipelineExecStatusTable`'s second row carries
    // `resubmitPipeline` and opens `showFailureDetailsDialog`, whose only button
    // is `dialogCancel`.
    expect(Object.keys((homeFilters as ActionDocument).actions).sort()).toEqual([
      "dialogCancel",
      "hfSelectFileKeyFilterUF",
      "hfSelectProcessUF",
      "hfSelectStatusUF",
      "hfSelectTimeWindowUF",
      "resubmitPipeline",
    ]);
    // The plan's *Actions* column reads 4 and counts `stateAction` declarations,
    // which F.4 established is the wrong count for arms. Six `case` labels, four
    // of them one body.
    const bodies = new Set(
      ["hfSelectProcessUF", "hfSelectStatusUF", "hfSelectFileKeyFilterUF", "hfSelectTimeWindowUF"].map(
        (k) => JSON.stringify((homeFilters as ActionDocument).actions[k]!.steps),
      ),
    );
    expect(bodies.size).toBe(1);
  });

  it("normalises session_id before posting it, which the coverage document did not", () => {
    // **I-100, and it is a 400 rather than a subtlety.** `resubmitPipeline` opens
    // `state[FSK.sessionId] = unpack(state[FSK.sessionId])`
    // (`modules/actions/config_delegates.dart`, `resubmitPipeline`) — a
    // self-assignment, which is why a transcription reads it as a no-op and drops
    // it. `session_id` is column 10 of `pipelineExecStatusTable`'s
    // `formStateBinding`, so a selection publishes it as a one-element list
    // (`components/data_table_source.dart`, `resetSecondaryKeys`), and the server
    // does `dataTableAction.Data[0]["session_id"].(string)`
    // (`jets/apiserver/api_tables.go`, `resubmit_pipeline`). Without the step the
    // payload carries `["s1"]` and every Resubmit answers
    // `session_id must be string in resubmit_pipeline`.
    const steps = (homeFilters as ActionDocument).actions["resubmitPipeline"]!.steps;
    expect(steps[0]).toEqual({ do: "set", key: "session_id", value: { fromKey: "session_id" } });
    expect(steps[1]!.do).toBe("post");
  });

  it("holds fileMappingUF's one state action plus the three its table and dialog reach", () => {
    // F.8. `file_mapping/form_action_delegates.dart` declares **seven** `case`
    // labels across two switches, and neither switch belongs to one flow:
    // `fileMappingFormActionsUF` is `fileMappingUF`'s (`downloadMapping`,
    // `fmSelectSourceConfigUF`, `dialogCancel`) and `fileMappingFormActions`
    // serves *both* `mapFileUF`'s worksheet and this flow's intake dialog
    // (`loadRawRowsOk`, `mapperOk`, `mapperDraft`, `dialogCancel`).
    //
    // **The partition is by what can press the arm, not by which switch declares
    // it** (I-101). One state action; `downloadMapping` and the dialog are
    // `fmFileMappingTableUF`'s (`file_mapping/data_table_config.dart`), and the
    // dialog's two buttons are `loadRawRows.Ok` and `dialogCancel`
    // (`FormKeys.loadRawRows`, `file_mapping/form_config.dart`).
    //
    // **`configureMappingPage` names no action.** It is the table's third button
    // and it is a `showScreen`, which navigates to `mapFileUF`'s route and carries
    // a `configForm` that lives in *that* flow's form document — which is why
    // `validateTableActions` checks `configForm` for the two dialog kinds and not
    // for this one (I-122).
    expect(Object.keys((fileMapping as ActionDocument).actions).sort()).toEqual([
      "dialogCancel",
      "downloadMapping",
      "fmSelectSourceConfigUF",
      "loadRawRows.Ok",
    ]);
  });

  it("unpacks all four of the selected source config's columns", () => {
    // The state action is `state[k] = unpack(state[k])` four times over
    // (`file_mapping/form_action_delegates.dart`, `fileMappingFormActionsUF`),
    // and `{"fromKey": k}` *is* `unpack` — so this is the one arm of the seven
    // re-partitions where I-100's mechanism was already spelled correctly.
    // Asserted because it reads like the no-op I-100 describes and a later tidy-up
    // would delete it: `fmInputSourceMappingUF` publishes all four through its
    // `formStateBinding`, so each arrives as a one-element list and the flow's
    // second table filters `process_mapping` on `table_name`.
    const steps = (fileMapping as ActionDocument).actions["fmSelectSourceConfigUF"]!.steps;
    expect(steps.map((s) => (s.do === "set" ? [s.key, Object.keys(s.value)[0]] : s.do))).toEqual([
      ["client", "fromKey"],
      ["org", "fromKey"],
      ["table_name", "fromKey"],
      ["object_type", "fromKey"],
    ]);
  });

  it("holds every startPipelineUF arm a state names, and the flow has no others", () => {
    // F.4. `start_pipeline/form_action_delegates.dart` declares three `case`
    // labels in one switch and the four states name them —
    // `spPrepareStartPipeline` twice, which is why the plan's *Actions* column
    // says 4 and the document has 3. **No form and no table adds a fourth**: the
    // three selection forms take `standardActions`, `spSummaryUF` takes
    // Previous / Cancel / Completed (`start_pipeline/form_config.dart`), and the
    // six tables the forms name carry only `clearHomeFilters`
    // (`start_pipeline/data_table_config.dart`) — so this flow owes no dialog
    // form and no table arm.
    expect(Object.keys((startPipeline as ActionDocument).actions).sort()).toEqual([
      "spPipelineSelected",
      "spPrepareStartPipeline",
      "spStartPipelineUF",
    ]);
  });

  it("unpacks a selection before decoding the array literal inside it", () => {
    // I-97, and the reason it is asserted on the *document* as well as on the
    // behaviour (`proofFlows.test.ts`): the coverage document expressed
    // `unpackToList(unpack(x))` as `fromKeyList` alone, and the two `set` steps
    // per key are what make the composition explicit. A future edit collapsing
    // them back would read as a tidy-up.
    const steps = (startPipeline as ActionDocument).actions["spPipelineSelected"]!.steps;
    expect(steps.map((s) => (s.do === "set" ? [s.key, Object.keys(s.value)[0]] : s.do))).toEqual([
      ["merged_process_input_keys", "fromKey"],
      ["merged_process_input_keys", "fromKeyList"],
      ["injected_process_input_keys", "fromKey"],
      ["injected_process_input_keys", "fromKeyList"],
    ]);
  });

  it("re-partitions the workspace_pull delegate by flow, not by file", () => {
    // The whole of F4, checked on the one delegate file it was written about.
    // Every name here is a `case ActionKeys.` label of
    // `workspace_pull/form_action_delegates.dart`; the two switches in that file
    // are two flows, and this is where they part.
    expect(Object.keys((loadConfig as ActionDocument).actions).sort()).toEqual([
      "wpLoadAllClientConfigUF",
      "wpLoadConfigConfirmUF",
      "wpLoadConfigOkUF",
    ]);
    expect(Object.keys((workspacePull as ActionDocument).actions).sort()).toEqual([
      "wpLoadConfigConfirmUF",
      "wpPullWorkspaceConfirmUF",
      "wpPullWorkspaceOkUF",
    ]);
    // **The shared arm is the same body in both**, which is the thing a split
    // "from opposite ends" loses: two documents, one behaviour, no drift.
    expect((loadConfig as ActionDocument).actions["wpLoadConfigConfirmUF"]).toEqual(
      (workspacePull as ActionDocument).actions["wpLoadConfigConfirmUF"],
    );
  });

  it("names five escapes: I-23 found six and one of them became grammar", () => {
    const escapes = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((action) =>
        action.steps.filter((s) => s.do === "escape").map((s) => s.name),
      ),
    );
    // **`saveProcessMapping` is gone and that is F.1's finding.** I-23 counted
    // six escapes by transcribing the delegates, and this one was transcribed as
    // an escape because the grammar of the day could not post one row per
    // validation group and had no missing-key guard. It has both now — `rows:
    // "everyGroup"` and `require`, each justified beyond this flow — so
    // `mapperOk` and `mapperDraft` are steps. **A transcription says what the
    // grammar could express when it was written, not what the arm needs.**
    //
    // **F.8 asked the same question of its two and got two different answers,
    // which narrows the rule rather than repeating it (I-121).**
    // `downloadMapping` stays because no step can express a two-table joined read
    // whose rows become a file the browser saves. `loadRawRows` stays although the
    // grammar *can* express it — a `set` and a `post` with `transport:
    // "insertRows"` — because S.7's allowlist refuses the target: `insert_raw_rows`
    // runs `DELETE FROM jetsapi.process_mapping` in a pre-processing hook before
    // `InsertRows` reaches `VerifyUserPermission`
    // (`jets/datatable/data_table_action.go`, `InsertRawRows`). **So "can the
    // grammar say it" is necessary and not sufficient**, and the escape count is
    // an upper bound on the first question alone.
    //
    // **F.7 asked it of the last two and the count did not move, which is the
    // wrong thing to read off it.** `loadSourceConfigWithFileTypeInference` is
    // *gone*: F.2's `when` guard expresses all but one step of
    // `scSelectSourceConfigUF`, and what remains is `readXlsxSheetOption`, a
    // `JSON.parse` of a value held in form state. One name replaced another and
    // the arm went from a body to twenty-one steps plus four lines.
    // `saveSourceConfigForFileType` stays whole for a **third** reason — neither
    // vocabulary nor permission, but that `wholeState`'s `normalise` and `omit`
    // carry no `when`, so the Dart's per-file-type projection of a *copy* of the
    // state has no guarded form. See `sourceConfig.ts`.
    expect([...new Set(escapes)].sort()).toEqual([
      "downloadMapping",
      "loadRawRows",
      "readXlsxSheetOption",
      "saveSourceConfigForFileType",
      "updateHomeFilters",
    ]);
  });

  it("uses the primitives the grammar gained, and nothing undeclared", () => {
    const verbs = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) => a.steps.map((s) => s.do)),
    );
    // `query` and the `insertRows` transport are I-23's additions; if a future
    // edit removes their only user, this says so rather than leaving dead
    // grammar behind.
    expect(verbs).toContain("query");
    // F.2's `when`. **One site until F.6 and nine now**, which is the answer to
    // the question F.2 left open — it argued the guard was justified beyond the
    // one arm that needed it, and named `scSelectSourceConfigUF` as the arm that
    // would want it most. That is still F.7's to say. What F.6 adds is a
    // different shape from either: three arms whose *whole structure* is a
    // branch, two of them choosing between an insert target and its update twin
    // (I-115) and one clearing two keys when any of four is absent. A predicate
    // over form state was enough for all three, so the grammar gained nothing.
    //
    // **Nineteen after F.7, and ten of them are one flow's.** That is the answer
    // F.2 left open when it named `scSelectSourceConfigUF` as the arm that wanted
    // this construct most: eight guards in that arm — a two-way mapping of
    // `is_part_files`, a five-way file-type inference and the guard on the one
    // escape that survives — and two more on `scAddSourceConfigUF` (I-130).
    // `and`, `or`, `not` and `isNull` all have their first use here, and none of
    // them is new: `ConditionSchema` has carried all seven forms since S.1, kept
    // "although the corpus exercises four" on the argument that their absence
    // would be a silent behaviour change the first time a flow wanted one. This is
    // that flow.
    const guarded = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) =>
        a.steps.filter((s) => s.when !== undefined),
      ),
    );
    expect(guarded).toHaveLength(19);
    // And F.2's `csvFromKey`. **Two sites in the Dart and three here**, because
    // `loadConfigInternal` is a helper two arms call and the documents have no
    // helper — `wpLoadConfigOkUF` and `wpLoadAllClientConfigUF` each spell it.
    // Every one of the three writes `updateDbClients`.
    const csv = JSON.stringify(Object.values(all)).match(/"csvFromKey"/g) ?? [];
    expect(csv).toHaveLength(3);
    const transports = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) =>
        a.steps.filter((s) => s.do === "post").map((s) => s.transport ?? "simple"),
      ),
    );
    expect(transports).toContain("insertRows");
  });
});

describe("every form-state key exists in the Dart", () => {
  /**
   * Extracted from `constants.dart` at test time rather than from a fixture.
   *
   * **The constant-name-versus-value trap has now caught this project four
   * times** — A.3's `scFileTypeOption`, S.1's `FormConfig.key`, the sizing's
   * `ActionKeys.deleteClient`, and S.2a's `input_registry.session_id`. Every one
   * was a name written where the app uses a value. This check makes the fifth
   * impossible: a key that is not a declared constant's *value* fails here.
   */
  const constantValues = (() => {
    const dart = readFileSync(
      fileURLToPath(new URL("../../../jetsclient/lib/utils/constants.dart", import.meta.url)),
      "utf8",
    );
    const values = new Set<string>();
    for (const m of dart.matchAll(/static const \w+\s*=\s*"([^"]*)"/g)) values.add(m[1]!);
    return values;
  })();

  /**
   * Keys that are column names or request fields rather than form-state keys.
   * Listed rather than pattern-matched, so adding one is a decision.
   */
  const notFormStateKeys = new Set([
    "schemaName", // drop_table request field
    "tableName", // drop_table request field
    "workspaceName", // request extra
    "workspaceBranch",
    "featureBranch",
    "entity_rdf_type", // query result column
    "process_config_key", // query result column, and a form-state key
    "currentSheet", // read out of the xlsx options blob, and a form-state key
    "event", // put_schema_event_to_s3 request field
    /**
     * **A real form-state key the Dart does not name, and the first one.** F.7.
     *
     * `is_part_files` is column 11 of `scSourceConfigKey`'s `formStateBinding`
     * and is removed by `deleteSourceConfig`
     * (`configure_files/data_table_config.dart`, `form_action_delegates.dart`) —
     * so it is as much a form-state key as `input_format` is. It is here because
     * every one of its three Dart sites writes the bare string rather than an
     * `FSK` constant, and this check reads `constants.dart`.
     *
     * So the guarantee is narrower than this block's heading: it is *every key is
     * a declared constant's value*, which catches the name-for-value trap it was
     * built for and cannot see a key the Dart never declared. Recorded as I-135.
     */
    "is_part_files",
  ]);

  it("reads the constants file it depends on", () => {
    expect(constantValues.size).toBeGreaterThan(400);
    expect(constantValues.has("input_registry.session_id")).toBe(true);
  });

  it.each(Object.keys(all))("%s references only declared keys", (flowKey) => {
    const doc = all[flowKey] as ActionDocument;
    const keys = new Set<string>();
    const collect = (value: unknown): void => {
      if (value === null || typeof value !== "object") return;
      const node = value as Record<string, unknown>;
      for (const field of ["key", "listKey", "fromKey", "fromKeyList", "fromKeyAtIndex", "pgArrayFromKey", "csvFromKey", "valueFromKey", "over"]) {
        if (typeof node[field] === "string") keys.add(node[field]);
      }
      if (Array.isArray(node["keys"])) for (const k of node["keys"]) keys.add(k as string);
      for (const [name, child] of Object.entries(node)) {
        if (name === "fields" || name === "extras" || name === "into") {
          for (const k of Object.keys(child as object)) keys.add(k);
        }
        if (Array.isArray(child)) child.forEach(collect);
        else collect(child);
      }
    };
    Object.values(doc.actions).forEach((action) => action.steps.forEach(collect));

    const unknown = [...keys].filter((k) => !constantValues.has(k) && !notFormStateKeys.has(k));
    expect(unknown).toEqual([]);
  });
});

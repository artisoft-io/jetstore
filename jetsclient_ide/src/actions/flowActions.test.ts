/**
 * The ported flows' action documents, checked against the grammar and against the
 * Dart they were derived from: eleven on 2026-08-24, the ten imported below since
 * 2026-10-01.
 *
 * **This was `coverage.test.ts` until 2026-08-24, and the rename is the whole
 * of what changed then.** It exists because an early exit condition said
 * "expressible" and only five arms had been expressed; writing the other
 * fifty found two primitives the grammar lacked and two more escapes than the
 * sizing counted. For eight tasks it then read two kinds of document —
 * transcriptions under `coverage/`, wired to nothing, and the runtime documents
 * under `flows/` that the flow ports promoted them into. The last port promoted the last
 * transcription and deleted the directory, so **every import below is a document a
 * flow runs** and the old name described a fixture that no longer exists. The name
 * now says what the file reads; the history is here rather than in the filename.
 *
 * **What the transcriptions did not prove, and it must not be forgotten now they
 * are gone.** They were written from reading the Dart, so the schema could tell
 * you a document was well formed and nothing could tell you it was *faithful*. A
 * delegate is a function body, not an object, so no corpus can be generated for it.
 * Each of the nine re-partitions found
 * something wrong, which is what re-deriving each arm from the Dart is for. Fidelity beyond
 * that is established by running a flow against a live server and diffing the
 * payload, as `lfLoadFilesUF` is.
 *
 * **That list outlives the transcriptions and is written up in `../README.md`**,
 * reduced to the five distinct mechanisms it holds and stated for anyone who
 * authors or *generates* one of these documents rather than for this corpus alone.
 * 2026-08-24.
 */

import { describe, expect, it } from "vitest";

import clientRegistry from "../../../jets/workspace_assets/user_flows/clientRegistryUF.ua.json";
import fileMapping from "../../../jets/workspace_assets/user_flows/fileMappingUF.ua.json";
import homeFilters from "../../../jets/workspace_assets/user_flows/homeFiltersUF.ua.json";
import loadConfig from "../../../jets/workspace_assets/user_flows/loadConfigUF.ua.json";
import loadFiles from "../../../jets/workspace_assets/user_flows/loadFilesUF.ua.json";
import mapFile from "../../../jets/workspace_assets/user_flows/mapFileUF.ua.json";
import pipelineConfig from "../../../jets/workspace_assets/user_flows/pipelineConfigUF.ua.json";
import sourceConfig from "../../../jets/workspace_assets/user_flows/sourceConfigUF.ua.json";
import startPipeline from "../../../jets/workspace_assets/user_flows/startPipelineUF.ua.json";
import workspacePull from "../../../jets/workspace_assets/user_flows/workspacePullUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "./schema";
import dartConstantValues from "./fixtures/dart_constant_values.json";

const all: Record<string, unknown> = {
  loadFilesUF: loadFiles,
  // `registerFileKeyUF` was here until 2026-10-01, when it was
  // retired; see the 52 below.
  // The worksheet's re-partition: `mapperOk`, `mapperDraft` and `dialogCancel` moved here
  // out of `coverage/fileMappingUF.ua.json`, which is the partition rule — a coverage
  // document is partitioned by *delegate file* and a runtime one by *flow*, and
  // `file_mapping/` is one directory holding two flows. The three that
  // stay behind belong to `fileMappingUF`, ported later.
  mapFileUF: mapFile,
  // The workspace pull's re-partition, and the one the partition rule was written about
  // (`actions/coverage/workspacePullUF.ua.json`, now gone): one *delegate file*
  // holding two switches for two flows. `wpLoadConfigConfirmUF` is a state
  // action of both, so it appears in both documents; `dialogCancel` appears in
  // neither, because no form or table of either flow offers it.
  loadConfigUF: loadConfig,
  workspacePullUF: workspacePull,
  // The client registry's re-partition. One delegate file, one flow — so unlike the two above
  // nothing had to be split. What it dropped is `crShowVendorUF`, whose body
  // jumps the flow to `show_org` and which **no form and no table of the flow
  // offers**: the reachability shape seen before, on a live arm rather than a
  // cancel. The two `delete*Action` arms are here because the *tables* offer
  // them, which is the third place an action name can appear and the first time
  // a migrated set has used it.
  clientRegistryUF: clientRegistry,
  // The start pipeline's re-partition. One delegate file, one flow, and nothing to split — the
  // same shape as the client registry's. What it changed is inside two arms rather than in the
  // partition: `spPipelineSelected` reads `unpackToList(unpack(x))` in the Dart
  // and the coverage document had `fromKeyList` alone, which is `unpackToList(x)`
  // and a different function on the value the table publishes.
  startPipelineUF: startPipeline,
  // The home filters' re-partition. One delegate file, one flow, nothing to split — and the
  // first whose arms are not all state actions: `resubmitPipeline` and
  // `dialogCancel` are reached from the *table*, the second row of it, and from
  // the dialog that row opens. What changed inside an arm is a step the coverage
  // document did not have at all: `resubmitPipeline` begins
  // `state[session_id] = unpack(state[session_id])`, which reads as a no-op
  // assignment and is a conversion the server type-asserts on.
  homeFiltersUF: homeFilters,
  // The pipeline configuration's re-partition. One delegate file, one flow, nothing to split — and the
  // largest of the nine at 14 `case` labels, a quarter of the 58. **The first
  // whose document holds an arm the nine delegate files do not declare**:
  // `addProcessInputOk` is the Save button of the two dialogs this flow's tables
  // open, and it lives in `modules/actions/config_delegates.dart`
  // (`processInputFormActions`) because those dialogs carry their own
  // `formActionsDelegate` rather than the flow's. So the count *rises* here.
  // `dialogCancel` is the same story the home filters flow
  // already met: dead in the delegate it was transcribed from, live in the document
  // it becomes.
  pipelineConfigUF: pipelineConfig,
  // The file mapping table's re-partition. One delegate file holding two flows — the shape
  // the worksheet and the workspace pull met — and **the arm that has to be duplicated
  // rather than moved is the one the worksheet moved**: `dialogCancel` is a `case` label of *both* switches in
  // `file_mapping/form_action_delegates.dart`, and both flows have a form that
  // offers it. `mapFileUF`'s worksheet is a dialog and so is
  // `loadRawRowsDialog`, which `fmFileMappingTableUF` opens. The worksheet took the name
  // with the other two and left three behind, so the coverage document this port
  // promoted was short an arm no step-level re-reading would have found.
  fileMappingUF: fileMapping,
  // The source configuration's re-partition, and the last. One delegate file, one flow, nothing to
  // split — and **the first whose document holds fewer arms than the delegate
  // declares for two different reasons at once**. `configureFilesFormActions`
  // declares eight `case` labels; six are here. `scStartUF` is a live `case` whose
  // body is `return null` and which no state, form or table names — the shape of
  // `crShowVendorUF` with nothing behind it. `dialogCancel` is the
  // unoffered-cancel shape rather than the dead-delegate one: this flow has **no dialog form at all**,
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
    // **Still 55 after the worksheet's re-partition, and that is the check that it was a
    // move.** Three names left `coverage/fileMappingUF.ua.json` and three
    // arrived in `flows/mapFileUF.ua.json`; a copy rather than a move would read
    // as 58 here.
    //
    // **Still 55 after the workspace pull's, and there the total conserves by cancellation
    // rather than by construction** — which is why the names are checked below
    // as well as the count. `coverage/workspacePullUF.ua.json` held six names
    // for a delegate file with seven arms (`dialogCancel` twice). The two
    // runtime documents hold six between them: `wpLoadConfigConfirmUF` is
    // **duplicated**, because it is a state action of both flows, and
    // `dialogCancel` is **dropped**, because neither flow has a form or a table
    // that offers it. +1 and −1 is not an invariant, so the assertion below is.
    //
    // **54 after the client registry's, and the fall is the point rather than a slip.** That
    // re-partition dropped `crShowVendorUF` and duplicated nothing, so the
    // cancellation the workspace pull relied on does not apply and the total moves. The arm is
    // live in the Dart — it jumps the flow to `show_org` — and is offered by no
    // form and no table of `clientRegistryUF`, which is the reachability
    // question asked of an arm that *does* something. **55 was never an
    // invariant**: it is a count of `case` labels minus the ones declared twice,
    // and a migrated corpus needs fewer of them than the delegates declare.
    //
    // **Still 54 after the start pipeline's, and here the conservation is by construction
    // again.** `startPipelineUF` is one delegate file holding one flow, all
    // three of whose arms a state action names, so three names left
    // `coverage/` and the same three arrived in `flows/`. Nothing was dropped
    // and nothing duplicated — which is the worksheet's shape rather than the workspace
    // pull's or the client registry's, and is why the count alone would not have told you.
    //
    // **Still 54 after the home filters', and its `dialogCancel` is the interesting one.**
    // Six names left `coverage/` and the same six arrived in `flows/`. Four are
    // state actions and two are not: `resubmitPipeline` is a table button and
    // `dialogCancel` is the only button of the dialog that table opens.
    //
    // **`dialogCancel` was the arm the workspace pull dropped from two flows and it is kept
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
    // the file it was transcribed from, live in the document it becomes.
    //
    // **55 after the pipeline configuration's, and it is the first re-partition that adds.** Fourteen
    // names left `coverage/` and fifteen arrived in `flows/`. The extra is
    // `addProcessInputOk`, which is not one of the 58 at all: the denominator is
    // `case ActionKeys.` across the nine `user_flows/*/form_action_delegates.dart`
    // files, and this arm is in `modules/actions/config_delegates.dart`. **A
    // dialog a flow's table opens may be served by a delegate outside the flow's
    // directory**, so the corpus of arms a migrated flow needs is not a subset of
    // the corpus this count is drawn from. 54 was as much an invariant as
    // 55 was.
    //
    // **56 after the file mapping table's, and it adds for a different reason than the last did.**
    // Three names left `coverage/fileMappingUF.ua.json` and four arrived in
    // `flows/`. The extra is `dialogCancel`, which is not a new arm at all: it is
    // **already in `flows/mapFileUF.ua.json`**, because `file_mapping/` is one
    // directory holding two flows and the worksheet *moved* the name rather than
    // duplicating it. This is the workspace pull's `wpLoadConfigConfirmUF` exactly — one
    // arm two flows need — and the difference is that the workspace pull saw it at the
    // split and the worksheet could not, because the rule that a table-opened dialog's
    // buttons resolve in the flow's own document was not written until four ports later.
    //
    // **54 after the source configuration's, and the fall of two is the whole of the
    // dead-arm decision still to be taken.** Eight names left
    // `coverage/sourceConfigUF.ua.json` and six arrived in `flows/`. Both
    // absentees are `case` labels of `configureFilesFormActions` that nothing can
    // press: `scStartUF`, whose body is `return null` and which appears in no
    // `UserFlowState`, no `FormActionConfig` and no `ActionConfig`, and
    // `dialogCancel`, which is the unoffered-cancel shape rather than the dead-delegate one — the flow has no
    // dialog *form*, so there is no Cancel button to resolve.
    //
    // **56 was no more an invariant than 55 and 54 were.** This is a count of
    // reachable arms, and it moves whenever a re-partition finds an arm nothing
    // reaches or a dialog served from outside the flow's directory.
    //
    // **52 since 2026-10-01, and the first fall that is a retirement rather than
    // a re-partition.** `registerFileKeyUF` left with its two arms,
    // `rfkSubmitSchemaEventUF` and its own `dialogCancel`; its schema event is
    // posted from the Pipeline Status table's dialog
    // now, which is a Home-screen action document and not one of these.
    //
    // **53 the same day**: `homeFiltersUF` gained `putSchemaEvent`, an arm with no
    // Dart original, because its table's first row opens a dialog whose Save
    // button runs it. The familiar shape — an arm a flow needs that the
    // delegates never declared — arriving from the port's own work.
    //
    // **55 the same day again**: the same table's third row gained *Get Run Manifest* and *Get
    // Schema Event*, two more arms the Dart never declared, for the same reason.
    const names = Object.values(all).flatMap((doc) =>
      Object.keys((doc as ActionDocument).actions),
    );
    //
    // **59 the same day, and the last four are the wizards'**: each wizard
    // gained a *+ Add* arm that clears the edit
    // path's keys and jumps to the add page, and a *Cancel* arm that returns to
    // the table. `clientRegistryUF` swapped `crAddClientUF` for the dialog's
    // `crAddClientOk`, one for one.
    //
    // **58 since 2026-10-15 (Release 4.0.4)**: `startPipelineUF` no longer chooses
    // merged or historical sources, so `spPipelineSelected`, which unpacked the
    // pipeline's merged and injected key lists for the two tables that drew them,
    // went with them. The second retirement, after `registerFileKeyUF`'s.
    //
    // **59 since 2026-11-05**: `homeFiltersUF`
    // gained `checkPendingTasks`, the Pipeline Status table's *Check Pending*
    // button, carried because the flow draws that table.
    expect(names).toHaveLength(59);
    expect(new Set(names).size).toBeLessThan(names.length); // dialogCancel repeats across flows
  });

  it("holds sourceConfigUF's four state actions plus the two its table reaches", () => {
    // `configure_files/form_action_delegates.dart` declares eight `case`
    // labels in one switch, all of them `sourceConfigUF`'s — the file defines one
    // flow and `configureFilesFormActions` is named by nothing else, so this is
    // the one-file, one-flow shape with nothing to split and no sibling to have taken an
    // arm away.
    //
    // Four state actions (`user_flow_config.dart`), and `scSourceConfigKey`'s two
    // `doAction` buttons — `dropStagingTable` → `dropTable` and `deleteSourceConfig`
    // (`configure_files/data_table_config.dart`). The eleven other forms take
    // `standardActions` and `scSummaryUF` takes Previous / Cancel / Completed, so
    // no form contributes an arm.
    //
    // **Two more since 2026-10-01**:
    // `scGoToAddSourceConfig` is the table's *+ Add*, and `scCancelToList` is every
    // wizard page's *Cancel*, which returns to the table rather than leaving.
    expect(Object.keys((sourceConfig as ActionDocument).actions).sort()).toEqual([
      "addSourceConfig.ok",
      "deleteSourceConfig",
      "dropTable",
      "scAddSourceConfigUF",
      "scCancelToList",
      "scEditXlsxOptionsUF",
      "scGoToAddSourceConfig",
      "scSelectSourceConfigUF",
    ]);
  });

  it("leaves the organization out of the staging table name when there is none", () => {
    // **The largest thing the source configuration port found.** `scAddSourceConfigUF` is
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

  it("clears all fourteen keys the delete arm removes, not the six transcribed", () => {
    // A known transcription mechanism, on the largest list in the corpus. The Dart calls
    // `state.remove` thirteen times after encoding the request
    // (`configure_files/form_action_delegates.dart`, `deleteSourceConfig`); the
    // coverage document listed six. The seven missing ones are the record's
    // *content* — its headers, positions, domain keys, code-value mapping and
    // schema provider — and `source_config`'s insert statement reads every one of
    // them (`jets/datatable/sql_stmts.go`, `sqlInsertStmts`), so a user who deleted
    // a configuration and then created another would have inherited the deleted
    // one's. **Fourteen since Release 4.0.4 (2026-10-15)**: `is_cumulative` is
    // record content on the same terms, and the insert reads it too.
    const steps = (sourceConfig as ActionDocument).actions["deleteSourceConfig"]!.steps;
    const removal = steps.find((s) => s.do === "remove")!;
    expect(removal.do === "remove" && removal.keys).toHaveLength(14);
    // The post is before the removal, as `deleteClientAction`'s is: the Dart
    // encodes the body *first* and then empties the map, and `wholeState` reads
    // the state at the moment the step runs.
    //
    // **Fifteen `set` steps since 2026-10-16**, one per column the selection
    // publishes: `fromKey` unwraps a selected row's one-element arrays, as the
    // client and org deletes do. Posted as arrays they panicked the server's
    // domain-keys hook (a 502 at a client).
    expect(steps.map((s) => s.do)).toEqual([
      "confirm", ...Array<string>(15).fill("set"), "post", "clearSelection", "remove",
    ]);
  });

  it("holds pipelineConfigUF's thirteen reachable arms plus the two its dialogs need", () => {
    // `pipeline_config/form_action_delegates.dart` declares fourteen `case`
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
    // the home filters' shape exactly, one flow later: the port has one action document per
    // flow rather than one per delegate, so the dialog's Cancel resolves here.
    // `addProcessInputOk` is its neighbour in that same switch and is the arm the
    // 58 never counted.
    //
    // **Two more since 2026-10-01**:
    // `pcGoToAddPipelineConfig` is the table's *+ Add* and `pcCancelToList` every
    // wizard page's *Cancel*.
    expect(Object.keys((pipelineConfig as ActionDocument).actions).sort()).toEqual([
      "addProcessInputOk",
      "deletePipelineConfig",
      "dialogCancel",
      "pcAddInjectedProcessInputUF",
      "pcAddMergeProcessInputUF",
      "pcAddPipelineConfigUF",
      "pcCancelToList",
      "pcGoToAddPipelineConfig",
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
    // **The largest thing the pipeline configuration port found.** `pcSavePipelineConfigUF`
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
    // **The transcription mechanism this port found, and it is a fifth**.
    // `pcAddPipelineConfigUF` opens `state[merged_process_input_keys] =
    // <String?>[]` — a fresh empty list. The coverage document wrote
    // `{ "fromKeyList": "merged_process_input_keys" }`, which reads the key it is
    // assigning to: the *left*-hand side supplied the right. Two `set` steps say
    // what the Dart says — a `{}` literal, then the conversion that decodes it —
    // which is the start pipeline's finding again, that a lost composition needs no primitive.
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
    // `client_registry/form_action_delegates.dart` declares seven `case`
    // labels across its two switches; six are here. The absentee is
    // `crShowVendorUF`, and the check that finds it is the reachability one: not
    // *what does the delegate declare* but *what can press it*. The flow's four
    // states take `standardActions` or Previous/Done (`form_config.dart`), the
    // `ufVendor` dialog offers `crAddVendorOk` and `dialogCancel`, and
    // `client`/`org` offer `deleteClientAction` and `deleteOrgAction`
    // (`data_table_config.dart`). Nothing names `crShowVendorUF`.
    //
    // **`crAddClientUF` became `crAddClientOk` on 2026-10-01**:
    // the `create_client` state it was
    // the action of is gone, and the client is added from an *Add Client* dialog
    // on the `client` table, as a vendor is from the `org` table's.
    expect(Object.keys((clientRegistry as ActionDocument).actions).sort()).toEqual([
      "crAddClientOk",
      "crAddVendorOk",
      "crSelectClientUF",
      "deleteClientAction",
      "deleteOrgAction",
      "dialogCancel",
    ]);
  });

  it("holds homeFiltersUF's four state actions plus the three its table reaches", () => {
    // `home_filters/form_action_delegates.dart` declares six `case` labels
    // in one switch — four `hf*` state actions falling through to one body,
    // `resubmitPipeline` and `dialogCancel`. All six are reachable and all six
    // are here, which is the first migrated flow where a *table* contributes
    // both an action and a form: `pipelineExecStatusTable`'s second row carries
    // `resubmitPipeline` and opens `showFailureDetailsDialog`, whose only button
    // is `dialogCancel`.
    //
    // **Seven since 2026-10-01, and the seventh has no Dart `case` at all.**
    // `putSchemaEvent` is the Save button of `putSchemaEventDialog`, which the same
    // table's first row now opens;
    // the flow carries it because it draws the table, as it carries
    // `resubmitPipeline`.
    //
    // **Nine, the same day**: `getRunManifest` and
    // `getSchemaEvent` are the table's third-row buttons, here for the same reason.
    //
    // **Ten since 2026-11-05**: `checkPendingTasks`,
    // the first-row *Check Pending* button, beside *Put Schema Event*.
    expect(Object.keys((homeFilters as ActionDocument).actions).sort()).toEqual([
      "checkPendingTasks",
      "dialogCancel",
      "getRunManifest",
      "getSchemaEvent",
      "hfSelectFileKeyFilterUF",
      "hfSelectProcessUF",
      "hfSelectStatusUF",
      "hfSelectTimeWindowUF",
      "putSchemaEvent",
      "resubmitPipeline",
    ]);
    // A count of `stateAction` declarations reads 4, which the start pipeline
    // flow showed is the wrong count for arms. Six `case` labels, four
    // of them one body.
    const bodies = new Set(
      ["hfSelectProcessUF", "hfSelectStatusUF", "hfSelectFileKeyFilterUF", "hfSelectTimeWindowUF"].map(
        (k) => JSON.stringify((homeFilters as ActionDocument).actions[k]!.steps),
      ),
    );
    expect(bodies.size).toBe(1);
  });

  it("normalises session_id before posting it, which the coverage document did not", () => {
    // **A 400 rather than a subtlety.** `resubmitPipeline` opens
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
    // `file_mapping/form_action_delegates.dart` declares **seven** `case`
    // labels across two switches, and neither switch belongs to one flow:
    // `fileMappingFormActionsUF` is `fileMappingUF`'s (`downloadMapping`,
    // `fmSelectSourceConfigUF`, `dialogCancel`) and `fileMappingFormActions`
    // serves *both* `mapFileUF`'s worksheet and this flow's intake dialog
    // (`loadRawRowsOk`, `mapperOk`, `mapperDraft`, `dialogCancel`).
    //
    // **The partition is by what can press the arm, not by which switch declares
    // it**. One state action; `downloadMapping` and the dialog are
    // `fmFileMappingTableUF`'s (`file_mapping/data_table_config.dart`), and the
    // dialog's two buttons are `loadRawRows.Ok` and `dialogCancel`
    // (`FormKeys.loadRawRows`, `file_mapping/form_config.dart`).
    //
    // **`configureMappingPage` names no action.** It is the table's third button
    // and it is a `showScreen`, which navigates to `mapFileUF`'s route and carries
    // a `configForm` that lives in *that* flow's form document — which is why
    // `validateTableActions` checks `configForm` for the two dialog kinds and not
    // for this one.
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
    // re-partitions where the dropped self-assignment was already spelled correctly.
    // Asserted because it reads like the no-op a transcription drops and a later tidy-up
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
    // `start_pipeline/form_action_delegates.dart` declares three `case`
    // labels in one switch and the four states name them —
    // `spPrepareStartPipeline` twice, which is why a count of state actions
    // says 4 and the document has 3. **No form and no table adds a fourth**: the
    // three selection forms take `standardActions`, `spSummaryUF` takes
    // Previous / Cancel / Completed (`start_pipeline/form_config.dart`), and the
    // six tables the forms name carry only `clearHomeFilters`
    // (`start_pipeline/data_table_config.dart`) — so this flow owes no dialog
    // form and no table arm.
    //
    // **Two since 2026-10-15 (Release 4.0.4)**, when the flow stopped choosing
    // merged and historical sources — the pipeline configuration names them, with
    // their lookback periods. Three states name the two that are left;
    // `spPipelineSelected` unpacked the merged and injected key lists for the step
    // and the summary row that drew them, and went with both.
    expect(Object.keys((startPipeline as ActionDocument).actions).sort()).toEqual([
      "spPrepareStartPipeline",
      "spStartPipelineUF",
    ]);
  });

  // **"unpacks a selection before decoding the array literal inside it" was
  // retired on 2026-10-15 (Release 4.0.4)** with the `spPipelineSelected` arm it
  // asserted on. The two `set` steps per key it pinned had nothing left to
  // unpack for: no table of the flow reads the merged or injected key lists now.

  it("re-partitions the workspace_pull delegate by flow, not by file", () => {
    // The whole of the partition rule, checked on the one delegate file it was written about.
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

  it("names five escapes: the transcription found six and one of them became grammar", () => {
    const escapes = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((action) =>
        action.steps.filter((s) => s.do === "escape").map((s) => s.name),
      ),
    );
    // **`saveProcessMapping` is gone and that is the worksheet port's finding.** The coverage pass counted
    // six escapes by transcribing the delegates, and this one was transcribed as
    // an escape because the grammar of the day could not post one row per
    // validation group and had no missing-key guard. It has both now — `rows:
    // "everyGroup"` and `require`, each justified beyond this flow — so
    // `mapperOk` and `mapperDraft` are steps. **A transcription says what the
    // grammar could express when it was written, not what the arm needs.**
    //
    // **The file mapping table port asked the same question of its two and got two different answers,
    // which narrows the rule rather than repeating it.**
    // `downloadMapping` stays because no step can express a two-table joined read
    // whose rows become a file the browser saves. `loadRawRows` stays although the
    // grammar *can* express it — a `set` and a `post` with `transport:
    // "insertRows"` — because the schema's allowlist refuses the target: `insert_raw_rows`
    // runs `DELETE FROM jetsapi.process_mapping` in a pre-processing hook before
    // `InsertRows` reaches `VerifyUserPermission`
    // (`jets/datatable/data_table_action.go`, `InsertRawRows`). **So "can the
    // grammar say it" is necessary and not sufficient**, and the escape count is
    // an upper bound on the first question alone.
    //
    // **The source configuration port asked it of the last two and the count did not move, which is the
    // wrong thing to read off it.** `loadSourceConfigWithFileTypeInference` is
    // *gone*: the `when` guard expresses all but one step of
    // `scSelectSourceConfigUF`, and what remains is `readXlsxSheetOption`, a
    // `JSON.parse` of a value held in form state. One name replaced another and
    // the arm went from a body to twenty-one steps plus four lines.
    // `saveSourceConfigForFileType` stays whole for a **third** reason — neither
    // vocabulary nor permission, but that `wholeState`'s `normalise` and `omit`
    // carry no `when`, so the Dart's per-file-type projection of a *copy* of the
    // state has no guarded form. See `sourceConfig.ts`.
    // **Seven since 2026-10-01**: the Pipeline
    // Status table's clipboard buttons, which every flow drawing that table runs —
    // `fetchStageToClipboard` because a `post` drops the body it needs, and
    // `copySchemaEvent` because it reads one column of a query result. Neither is
    // a transcription of a Dart arm, so neither is a candidate for becoming grammar.
    expect([...new Set(escapes)].sort()).toEqual([
      "copySchemaEvent",
      "downloadMapping",
      "fetchStageToClipboard",
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
    // `query` and the `insertRows` transport are the coverage pass's additions; if a future
    // edit removes their only user, this says so rather than leaving dead
    // grammar behind.
    expect(verbs).toContain("query");
    // The `when` guard. **One site at first and nine later**, which is the answer to
    // the question its introduction left open — it argued the guard was justified beyond the
    // one arm that needed it, and named `scSelectSourceConfigUF` as the arm that
    // would want it most. That was the source configuration port's to say. What `pipelineConfigUF` adds is a
    // different shape from either: three arms whose *whole structure* is a
    // branch, two of them choosing between an insert target and its update twin
    // and one clearing two keys when any of four is absent. A predicate
    // over form state was enough for all three, so the grammar gained nothing.
    //
    // **Nineteen after the source configuration port, and ten of them are one flow's.** That is the
    // answer left open when the guard's introduction named `scSelectSourceConfigUF` as the arm that wanted
    // this construct most: eight guards in that arm — a two-way mapping of
    // `is_part_files`, a five-way file-type inference and the guard on the one
    // escape that survives — and two more on `scAddSourceConfigUF`.
    // `and`, `or`, `not` and `isNull` all have their first use here, and none of
    // them is new: `ConditionSchema` has carried all seven forms from the start, kept
    // "although the corpus exercises four" on the argument that their absence
    // would be a silent behaviour change the first time a flow wanted one. This is
    // that flow.
    const guarded = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) =>
        a.steps.filter((s) => s.when !== undefined),
      ),
    );
    expect(guarded).toHaveLength(19);
    // And `csvFromKey`. **Two sites in the Dart and three here**, because
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

describe("every form-state key is a declared constant's value", () => {
  /**
   * Extracted from `constants.dart` at test time rather than from a fixture.
   *
   * **This block was headed *every form-state key exists in the Dart* until
   * 2026-08-24.** It turned out that the guarantee is narrower — a key the
   * Dart never declares as a constant is invisible here, and there is one — and
   * recorded the gap in the `is_part_files` note below while leaving the heading
   * saying the wider thing. The heading is now the guarantee. Nothing about the
   * check changed, which is the point: **widening the extraction would start
   * matching column names**, and that is why the exemption list is enumerated.
   *
   * **The constant-name-versus-value trap has now caught this port four
   * times** — `scFileTypeOption`, `FormConfig.key`,
   * `ActionKeys.deleteClient`, and `input_registry.session_id`. Every one
   * was a name written where the app uses a value. This check makes the fifth
   * impossible: a key that is not a declared constant's *value* fails here.
   */
  /**
   * **A frozen fixture since the Flutter app was deleted, and the check survives its source.**
   *
   * This read `constants.dart` and extracted every `static const … = "…"`. That
   * file was deleted with the Flutter app on 2026-08-26, so the 455 values it
   * declared are committed in `fixtures/dart_constant_values.json`, extracted at
   * `b4cda702` — the last commit that had them.
   *
   * **What is lost is the freshness, not the check.** The keys this guards are in
   * the action documents and are still live; what can no longer happen is the
   * fixture updating when the Dart does, because there is no Dart. A key added to
   * a document and absent here used to mean *you wrote a constant's name where the
   * app wants its value*; it now means that **or** *this is a new key*, and only a
   * reader can say which. That is a real weakening and is written down rather than
   * papered over.
   */
  const constantValues = new Set<string>(dartConstantValues.values);

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
    // `put_schema_event_to_s3`'s request field. **No document in `all` names it
    // since 2026-10-01**, when `registerFileKeyUF` was retired; kept because an
    // unused exemption costs nothing and the field is still the endpoint's.
    "event",
    /**
     * **A real form-state key the Dart does not name, and the first one.**
     *
     * `is_part_files` is column 11 of `scSourceConfigKey`'s `formStateBinding`
     * and is removed by `deleteSourceConfig`
     * (`configure_files/data_table_config.dart`, `form_action_delegates.dart`) —
     * so it is as much a form-state key as `input_format` is. It is here because
     * every one of its three Dart sites writes the bare string rather than an
     * `FSK` constant, and this check reads `constants.dart`.
     *
     * So the guarantee is *every key is a declared constant's value*, which
     * catches the name-for-value trap it was built for and cannot see a key the
     * Dart never declared. The block heading was corrected
     * to say it on 2026-08-24.
     */
    "is_part_files",
    /**
     * **The clipboard escape's parameters** (added
     * 2026-10-01). Real form-state keys, written by `set` steps and
     * consumed by `fetchStageToClipboard` (`stageClipboard.ts`,
     * `STAGE_CLIPBOARD_KEYS`); new with the port, so no Dart constant declares
     * them. `main_input_registry_key` needs no entry — it is a declared value.
     */
    "stage_clipboard.file_path",
    "stage_clipboard.label",
    "stage_clipboard.missing_message",
    /**
     * **Two keys the wizards' *+ Add* brought, both deliberate.** Added
     * 2026-10-01 — and both are the *new key* reading of a failure
     * here, not the name-for-value one.
     *
     * `ufNewClient` is the *Add Client* dialog's name field. It cannot be
     * `client`, which is what the Dart's create step used: the dialog shares the
     * flow's form state, and `client` there is the client table's selection, so
     * the field would open pre-filled with the selected client.
     *
     * `serverError` is where the interpreter records an `insertRows` post's
     * failure (`interpret.ts`, the `post` step). `pcGoToAddPipelineConfig`
     * removes it because the process-input dialog's failure would otherwise
     * survive into the next *+ Add*.
     */
    "ufNewClient",
    "serverError",
    /**
     * **A new `source_config` column, Release 4.0.4 (2026-10-15)**, and a real
     * form-state key on `is_part_files`' terms: column 14 of `scSourceConfigKey`'s
     * `formStateBinding`, the single-or-multi-part page's dropdown, and removed by
     * both *+ Add* and the delete arm. No Dart constant can declare it — the column
     * did not exist while the Dart did.
     */
    "is_cumulative",
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

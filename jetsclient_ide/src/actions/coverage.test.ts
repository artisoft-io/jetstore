/**
 * The coverage fixture: every action arm the nine flows define, as step lists.
 *
 * **This exists because S.2a's exit condition said "expressible" and only five
 * arms had been expressed** (I-22). Writing the other fifty is what turns an
 * argument from a vocabulary into a demonstration — and it found two primitives
 * the grammar lacked and two more escapes than the sizing counted (I-23), which
 * is exactly what it was for.
 *
 * **These documents are not wired to anything.** No flow references them and no
 * screen runs them; S.5 and Phase 3 migrate flows per flow. What they prove is
 * that the grammar can say what each arm does, checked by the schema in two
 * languages.
 *
 * **What they do not prove, and this must not be misread.** They are transcribed
 * from reading the Dart, so the schema can tell you a document is well formed and
 * nothing here can tell you it is *faithful*. A delegate is a function body, not
 * an object, so no corpus can be generated for it — see
 * `sizing_action_grammar.md` §2. Fidelity is established one flow at a time, when
 * S.5 runs a flow against a live server and diffs the payload, as `lfLoadFilesUF`
 * already is.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import fileMapping from "./coverage/fileMappingUF.ua.json";
import homeFilters from "./coverage/homeFiltersUF.ua.json";
import pipelineConfig from "./coverage/pipelineConfigUF.ua.json";
import sourceConfig from "./coverage/sourceConfigUF.ua.json";
import startPipeline from "./coverage/startPipelineUF.ua.json";
import clientRegistry from "./flows/clientRegistryUF.ua.json";
import loadConfig from "./flows/loadConfigUF.ua.json";
import loadFiles from "./flows/loadFilesUF.ua.json";
import mapFile from "./flows/mapFileUF.ua.json";
import registerFileKey from "./flows/registerFileKeyUF.ua.json";
import workspacePull from "./flows/workspacePullUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "./schema";

const coverage: Record<string, unknown> = {
  fileMappingUF: fileMapping,
  homeFiltersUF: homeFilters,
  pipelineConfigUF: pipelineConfig,
  sourceConfigUF: sourceConfig,
  startPipelineUF: startPipeline,
};
const proof: Record<string, unknown> = {
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
};
const all = { ...coverage, ...proof };

describe("the coverage fixture", () => {
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
    const names = Object.values(all).flatMap((doc) =>
      Object.keys((doc as ActionDocument).actions),
    );
    expect(names).toHaveLength(54);
    expect(new Set(names).size).toBeLessThan(names.length); // dialogCancel repeats across flows
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
    expect([...new Set(escapes)].sort()).toEqual([
      "downloadMapping",
      "loadRawRows",
      "loadSourceConfigWithFileTypeInference",
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
    // F.2's `when`, on the same terms — one site today, and it is
    // `wpPullWorkspaceConfirmUF`'s.
    const guarded = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) =>
        a.steps.filter((s) => s.when !== undefined),
      ),
    );
    expect(guarded).toHaveLength(1);
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
    "currentSheet", // read out of the xlsx options blob, not a form-state key
    "event", // put_schema_event_to_s3 request field
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

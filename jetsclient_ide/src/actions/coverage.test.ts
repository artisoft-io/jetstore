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

import clientRegistry from "./coverage/clientRegistryUF.ua.json";
import fileMapping from "./coverage/fileMappingUF.ua.json";
import homeFilters from "./coverage/homeFiltersUF.ua.json";
import pipelineConfig from "./coverage/pipelineConfigUF.ua.json";
import sourceConfig from "./coverage/sourceConfigUF.ua.json";
import startPipeline from "./coverage/startPipelineUF.ua.json";
import workspacePull from "./coverage/workspacePullUF.ua.json";
import loadFiles from "./flows/loadFilesUF.ua.json";
import registerFileKey from "./flows/registerFileKeyUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "./schema";

const coverage: Record<string, unknown> = {
  clientRegistryUF: clientRegistry,
  fileMappingUF: fileMapping,
  homeFiltersUF: homeFilters,
  pipelineConfigUF: pipelineConfig,
  sourceConfigUF: sourceConfig,
  startPipelineUF: startPipeline,
  workspacePullUF: workspacePull,
};
const proof: Record<string, unknown> = {
  loadFilesUF: loadFiles,
  registerFileKeyUF: registerFileKey,
};
const all = { ...coverage, ...proof };

describe("the coverage fixture", () => {
  it.each(Object.keys(all))("%s validates against the grammar", (key) => {
    const result = ActionDocumentSchema.safeParse(all[key]);
    expect(result.success ? [] : result.error.issues).toEqual([]);
  });

  it("accounts for all 58 action arms the delegates declare", () => {
    // 58 `case ActionKeys.` labels across the nine `form_action_delegates.dart`
    // files. Three flows declare `dialogCancel` twice — once in the user-flow
    // delegate and once in the dialog delegate — which is one action reached
    // from two switches, not two actions. 58 − 3 = 55.
    const names = Object.values(all).flatMap((doc) =>
      Object.keys((doc as ActionDocument).actions),
    );
    expect(names).toHaveLength(55);
    expect(new Set(names).size).toBeLessThan(names.length); // dialogCancel repeats across flows
  });

  it("names six escapes, not the four the sizing found", () => {
    const escapes = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((action) =>
        action.steps.filter((s) => s.do === "escape").map((s) => s.name),
      ),
    );
    expect([...new Set(escapes)].sort()).toEqual([
      "downloadMapping",
      "loadRawRows",
      "loadSourceConfigWithFileTypeInference",
      "saveProcessMapping",
      "saveSourceConfigForFileType",
      "updateHomeFilters",
    ]);
  });

  it("uses the two primitives the grammar gained, and nothing undeclared", () => {
    const verbs = Object.values(all).flatMap((doc) =>
      Object.values((doc as ActionDocument).actions).flatMap((a) => a.steps.map((s) => s.do)),
    );
    // `query` and the `insertRows` transport are I-23's additions; if a future
    // edit removes their only user, this says so rather than leaving dead
    // grammar behind.
    expect(verbs).toContain("query");
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
      for (const field of ["key", "listKey", "fromKey", "fromKeyList", "fromKeyAtIndex", "pgArrayFromKey", "over"]) {
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

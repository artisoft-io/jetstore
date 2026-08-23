/**
 * Tests for the document-set layer.
 *
 * Three things:
 *
 * 1. **The two shipping sets pass.** The rule this project applies everywhere: a
 *    real configuration that fails means the check is wrong, not the
 *    configuration.
 * 2. **Each check fires**, on a mutation of a real set rather than on a fixture
 *    invented to make it fire.
 * 3. **The end-state rule is the narrow one.** Requiring `ufCompleted` would
 *    reject two of the eleven shipping end states, and the test says so by name
 *    so that a later tightening has to argue with the evidence.
 */

import { describe, expect, it } from "vitest";

import loadFilesActionsDoc from "../actions/flows/loadFilesUF.ua.json";
import registerFileKeyActionsDoc from "../actions/flows/registerFileKeyUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import corpus from "./fixtures/user_flows.json";
import loadFilesFlowDoc from "./flows/loadFilesUF.uf.json";
import registerFileKeyFlowDoc from "./flows/registerFileKeyUF.uf.json";
import { FormDocumentSchema, type FormDocument } from "./form";
import loadFilesFormsDoc from "./forms/loadFilesUF.form.json";
import registerFileKeyFormsDoc from "./forms/registerFileKeyUF.form.json";
import { UserFlowSchema, type UserFlow } from "./schema";
import { validateDocumentSet, type DocumentSet } from "./documentSet";

const parse = (flowDoc: unknown, actionsDoc: unknown, formsDoc: unknown): DocumentSet => ({
  flow: UserFlowSchema.parse(flowDoc) as UserFlow,
  actions: ActionDocumentSchema.parse(actionsDoc) as ActionDocument,
  forms: FormDocumentSchema.parse(formsDoc) as FormDocument,
});

const sets: [string, DocumentSet][] = [
  ["registerFileKeyUF", parse(registerFileKeyFlowDoc, registerFileKeyActionsDoc, registerFileKeyFormsDoc)],
  ["loadFilesUF", parse(loadFilesFlowDoc, loadFilesActionsDoc, loadFilesFormsDoc)],
];

/** A deep copy, so a mutation in one case cannot reach another. */
const clone = (set: DocumentSet): DocumentSet => structuredClone(set);

describe("the shipping document sets", () => {
  it.each(sets)("%s is consistent", (_name, set) => {
    expect(validateDocumentSet(set)).toEqual([]);
  });
});

describe("a state naming a form that is not there", () => {
  it("is reported against the flow", () => {
    const set = clone(sets[1]![1]);
    set.flow.states["select_source_config"]!.formConfig = "notAForm";
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => [f.code, f.document, f.path])).toContainEqual([
      "missingForm",
      "flow",
      "/states/select_source_config/formConfig",
    ]);
  });
});

describe("a name no action document defines", () => {
  it("is reported when a state names it", () => {
    const set = clone(sets[1]![1]);
    set.flow.states["select_file_keys"]!.stateAction = "notAnAction";
    expect(validateDocumentSet(set).map((f) => f.code)).toContain("missingAction");
  });

  it("is reported when a form button names it", () => {
    // The half no per-document check can reach from either side: the name is in
    // the form, the definitions are in the actions, and neither file mentions
    // the other.
    const set = clone(sets[1]![1]);
    set.forms.forms["lfSelectSourceConfigUF"]!.actions[0]!.action = "notAnAction";
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => [f.code, f.document])).toContainEqual(["missingAction", "forms"]);
  });

  it("is not reported for a standard action", () => {
    const set = clone(sets[1]![1]);
    set.forms.forms["lfSelectSourceConfigUF"]!.actions[0]!.action = "ufContinueLater";
    expect(validateDocumentSet(set)).toEqual([]);
  });
});

describe("an end state whose form tries to advance", () => {
  it.each(["ufNext", "ufStartFlow"])("is reported for %s", (action) => {
    // `select_file_keys` is loadFilesUF's end state. Its form offers
    // `ufCompleted` today; swapping that for an advancing button is exactly the
    // defect found on a generated flow, where the state action fires and the
    // flow then reports no next step (`engine.ts:212`-`:215`).
    const set = clone(sets[1]![1]);
    set.forms.forms["lfSelectFileKeysUF"]!.actions[2]!.action = action;
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => f.code)).toContain("advanceFromEndState");
    expect(findings[0]!.message).toContain("end state");
  });

  it("is not reported for a non-end state offering ufNext", () => {
    // The rule must not fire on the ordinary case: `select_source_config` is not
    // an end state and its form's `ufNext` is how the flow moves.
    const set = clone(sets[1]![1]);
    expect(validateDocumentSet(set).map((f) => f.code)).not.toContain("advanceFromEndState");
  });

  it("is not reported for an end state that finishes with a custom action", () => {
    // `registerFileKeyUF`'s only state is an end state and its form offers
    // `rfkSubmitSchemaEventUF` and `ufCancel` — no `ufCompleted` at all. This is
    // why the rule is "must not advance" rather than "must offer ufCompleted".
    expect(validateDocumentSet(sets[0]![1])).toEqual([]);
  });
});

describe("the corpus the narrow rule was chosen from", () => {
  it("has eleven end states, and two of them do not use ufCompleted", () => {
    // Measured rather than asserted from memory: the Dart's form configs give
    // `ufCompleted` to nine of the eleven, and `rfkSubmitSchemaEvent` and
    // `fmMappingFormUF` finish another way. A future tightening to "an end state
    // must offer ufCompleted" has to deal with those two, so the count is pinned
    // here where such a change would be made.
    const flows = (corpus as { flows: Record<string, { states: Record<string, { isEnd?: boolean }> }> }).flows;
    const endStates = Object.values(flows).flatMap((flow) =>
      Object.entries(flow.states).filter(([, state]) => state.isEnd === true),
    );
    expect(endStates.length).toBe(11);
  });
});

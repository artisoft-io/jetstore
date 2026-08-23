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

import loadConfigActionsDoc from "../actions/flows/loadConfigUF.ua.json";
import loadFilesActionsDoc from "../actions/flows/loadFilesUF.ua.json";
import registerFileKeyActionsDoc from "../actions/flows/registerFileKeyUF.ua.json";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import corpus from "./fixtures/user_flows.json";
import loadConfigFlowDoc from "./flows/loadConfigUF.uf.json";
import loadFilesFlowDoc from "./flows/loadFilesUF.uf.json";
import registerFileKeyFlowDoc from "./flows/registerFileKeyUF.uf.json";
import { FormDocumentSchema, type FormDocument } from "./form";
import loadConfigFormsDoc from "./forms/loadConfigUF.form.json";
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
  // F.2's, and the only set so far whose form carries a button outside the
  // action bar — which is what makes the two cases at the bottom of this file
  // testable against a real configuration rather than an invented one.
  ["loadConfigUF", parse(loadConfigFlowDoc, loadConfigActionsDoc, loadConfigFormsDoc)],
];

/** The `loadConfigUF` set, by name rather than by index. */
const loadConfigSet = (): DocumentSet => clone(sets[2]![1]);

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

describe("a field taking its items from a query the form does not declare", () => {
  it("is reported against the form", () => {
    // The relation a schema cannot state: `itemsFrom` is an `Identifier`, and
    // whether it keys the sibling `queries` object is a fact about two properties
    // of one form. A `.refine()` would say it in the browser and vanish from the
    // emitted JSON Schema, so Go would not enforce it.
    const set = clone(sets[1]![1]);
    const form = set.forms.forms["lfSelectSourceConfigUF"]!;
    form.queries = { sourceConfigs: { sql: "SELECT client FROM jetsapi.client_registry" } };
    form.rows[0]!.push({
      field: "dropdown",
      key: "client",
      label: "Client",
      items: [{ value: "", label: "Select a Client" }],
      itemsFrom: "clientsTypo",
    });
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => [f.code, f.document, f.path])).toContainEqual([
      "missingItemSource",
      "forms",
      "/forms/lfSelectSourceConfigUF/queries",
    ]);
  });

  it("is not reported when the query is declared on the same form", () => {
    const set = clone(sets[1]![1]);
    const form = set.forms.forms["lfSelectSourceConfigUF"]!;
    form.queries = { clients: { sql: "SELECT client FROM jetsapi.client_registry" } };
    form.rows[0]!.push({
      field: "dropdown",
      key: "client",
      label: "Client",
      items: [{ value: "", label: "Select a Client" }],
      itemsFrom: "clients",
    });
    expect(validateDocumentSet(set)).toEqual([]);
  });

  it("is not reported for a form with no queries and no item sources", () => {
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

/**
 * An inline `button` field is a button. Task F.2.
 *
 * **Both checks had to learn about it and neither would have failed loudly.** A
 * form's buttons were `form.actions` until F.2 added a field kind that is also a
 * button; a check reading only the action bar goes on passing and stops seeing
 * half the form (`form.ts`, `buttonsOf`).
 */
describe("a button inside the rows", () => {
  const inlineOf = (set: DocumentSet) => {
    const rows = set.forms.forms["wpLoadConfigUF"]!.rows;
    const field = rows.flat().find((f) => f.field === "button")!;
    return field as Extract<typeof field, { field: "button" }>;
  };

  it("is checked against the action document like any other", () => {
    const set = loadConfigSet();
    inlineOf(set).action = "notAnAction";
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => [f.code, f.document, f.path])).toContainEqual([
      "missingAction",
      "forms",
      "/forms/wpLoadConfigUF/rows",
    ]);
  });

  it("cannot advance from an end state either", () => {
    // I-57 on the surface it did not know about. `confirm` is an end state; a
    // `ufNext` in its rows would fire the state action and then report no next
    // step, exactly as one in its action bar would.
    const set = loadConfigSet();
    set.forms.forms["wpConfirmLoadConfigUF"]!.rows.push([
      { field: "button", action: "ufNext", label: "Next" },
    ]);
    const findings = validateDocumentSet(set);
    expect(findings.map((f) => f.code)).toContain("advanceFromEndState");
  });
});

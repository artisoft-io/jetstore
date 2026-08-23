/**
 * Checks that span a flow's *set* of documents rather than any one of them.
 *
 * ## Why this is a layer of its own, and why it can never be a save-time check
 *
 * A flow is four files — `<key>.uf.json`, `<key>.ua.json`, `<key>.form.json` and
 * the `table_configs/*.tc.json` its forms name. Every enforcement point this
 * project has built so far validates **one document**: `schema.ts` and
 * `validate.ts` for the flow, the emitted JSON Schemas for the other three, and
 * in Go a `validatorFor` that dispatches on one suffix and a `Finding` that
 * describes one file.
 *
 * **The save path cannot do better, and that is correct rather than a
 * limitation.** A `.uf.json` is legitimately saved before its `.ua.json` exists;
 * an author writing a flow does not produce four complete files atomically. So
 * "the set is consistent" is only answerable at *load*, which is where this runs.
 *
 * That gap is not theoretical. It was found from the outside, twice in one day
 * (I-50, I-51), by the agentic_ai project generating document sets and asking
 * what would consume them: four validation layers passed a set whose flow named
 * an action no document defined. **Every layer validates a document alone, and
 * nothing asked whether the set was complete.**
 *
 * ## The findings are not `validate.ts`'s
 *
 * `Finding` there has a closed `code` union that the Go port mirrors
 * (`jets/userflow/validate.go`). These codes must never reach Go, because Go can
 * never raise them — so they are a separate type with the same shape rather than
 * three more members of a union that would then be wrong on one side.
 */

import { isStandardAction } from "./engine";
import type { ActionDocument } from "../actions/schema";
import { itemSourcesOf, type FormDocument, type Form } from "./form";
import type { UserFlow } from "./schema";

export type SetFindingCode =
  /** A state names a form the form document does not define. */
  | "missingForm"
  /** A state or a form button names an action the action document does not define. */
  | "missingAction"
  /** An end state's form offers a button that tries to advance. */
  | "advanceFromEndState"
  /** A field's `itemsFrom` names a query its own form does not declare. */
  | "missingItemSource";

export interface SetFinding {
  severity: "error";
  code: SetFindingCode;
  message: string;
  /** A JSON Pointer into the document the finding is about. */
  path: string;
  /** Which of the set the pointer indexes, since a set finding spans files. */
  document: "flow" | "actions" | "forms";
}

/** The buttons that try to move the flow forward. */
const ADVANCING_ACTIONS = new Set(["ufNext", "ufStartFlow"]);

/**
 * An end state's form must not offer a button that advances.
 *
 * **This is a real defect and the engine makes it a costly one.** `step` runs the
 * state's action *before* asking where to go (`engine.ts:212`–`:215`): on an end
 * state `nextStateKey` returns null, so the action has already fired and the user
 * is shown `No next step from "<state>"`. The side effect is committed and the
 * screen reports an error — the worst ordering of the two.
 *
 * **The rule is "must not advance", not "must offer `ufCompleted`",** and the
 * corpus is why the narrower version is the right one. Nine of the eleven end
 * states declare `ufCompleted`; the other two finish through a *custom* action —
 * `rfkSubmitSchemaEvent` offers `rfkSubmitSchemaEventUF` and `dialogCancel`, and
 * `fmMappingFormUF` offers no `uf*` action at all. Requiring `ufCompleted` would
 * reject two shipping forms. **All eleven satisfy "no `ufNext`".**
 *
 * **The hazard is inherited, not introduced.** The Dart's `standardActions` —
 * the default button set a UserFlow form gets — is Previous / Cancel / **Next**
 * (`models/user_flow_config.dart:18`–`:37`), so an end state's form has always
 * had to override it, and
 * `validateConfiguration()` has never checked that it did — it checks the flow
 * side only, that a non-end state has somewhere to go. Authoring the form as data
 * moves the same unenforced convention into a second document; it does not create
 * it. What is new is that a schema *could* now be asked, and cannot: `isEnd` is in
 * the flow document and the button set is in the form document.
 *
 * Found by the agentic_ai project on a generated flow, 2026-08-23, by walking the
 * graph rather than by validating anything.
 */
function checkEndStateButtons(flow: UserFlow, forms: FormDocument): SetFinding[] {
  const findings: SetFinding[] = [];
  for (const [stateKey, state] of Object.entries(flow.states)) {
    if (!state.isEnd) continue;
    const form = forms.forms[state.formConfig];
    if (form === undefined) continue; // reported by checkForms
    for (const action of form.actions) {
      if (!ADVANCING_ACTIONS.has(action.action)) continue;
      findings.push({
        severity: "error",
        code: "advanceFromEndState",
        message:
          `state "${stateKey}" is an end state, but its form "${state.formConfig}" offers ` +
          `"${action.action}" — the state action would run and the flow would then report ` +
          `no next step`,
        path: `/forms/${state.formConfig}/actions`,
        document: "forms",
      });
    }
  }
  return findings;
}

/** Every state's `formConfig` resolves in the form document. */
function checkForms(flow: UserFlow, forms: FormDocument): SetFinding[] {
  const findings: SetFinding[] = [];
  for (const [stateKey, state] of Object.entries(flow.states)) {
    if (forms.forms[state.formConfig] !== undefined) continue;
    findings.push({
      severity: "error",
      code: "missingForm",
      message: `state "${stateKey}" names form "${state.formConfig}", which the form document does not define`,
      path: `/states/${stateKey}/formConfig`,
      document: "flow",
    });
  }
  return findings;
}

/**
 * Every action name that is not a standard one resolves in the action document.
 *
 * **Two sources, and the second is the one a per-document check cannot see at
 * all.** A state's `stateAction` is named in the flow; a form button's `action`
 * is named in the form. Neither document contains the definitions, and the
 * definitions' document contains no reference to either.
 */
function checkActions(flow: UserFlow, actions: ActionDocument, forms: FormDocument): SetFinding[] {
  const findings: SetFinding[] = [];
  const known = new Set(Object.keys(actions.actions));

  for (const [stateKey, state] of Object.entries(flow.states)) {
    if (state.stateAction === undefined || known.has(state.stateAction)) continue;
    findings.push({
      severity: "error",
      code: "missingAction",
      message: `state "${stateKey}" names action "${state.stateAction}", which the action document does not define`,
      path: `/states/${stateKey}/stateAction`,
      document: "flow",
    });
  }

  for (const [formKey, form] of Object.entries(forms.forms) as [string, Form][]) {
    form.actions.forEach((action, index) => {
      if (isStandardAction(action.action) || known.has(action.action)) return;
      findings.push({
        severity: "error",
        code: "missingAction",
        message: `form "${formKey}" has a button naming action "${action.action}", which the action document does not define`,
        path: `/forms/${formKey}/actions/${index}`,
        document: "forms",
      });
    });
  }
  return findings;
}

/**
 * Every `itemsFrom` names a query the same form declares. Task I.2b.
 *
 * **This one is inside a single document, and it is here anyway.** The file's
 * header says these checks span the set, so the widening is stated rather than
 * slipped in: what they actually share is being *relations a schema cannot
 * express*, and the set is where that has been true so far. `itemsFrom` is an
 * `Identifier` and whether it is a key of the sibling `queries` object is a
 * relation between two properties of one form — sayable in Zod with `.refine()`,
 * which `z.toJSONSchema` drops, so Go would not enforce it. That is the split
 * I.2a called the worst of the three options.
 *
 * **The Go save path could check it and does not.** Unlike the escape names and
 * the action names, nothing here needs the browser bundle or a second document —
 * `ValidateFormDocument` has the whole form in front of it. What it lacks is a Go
 * parser for the form document, which is a second implementation of this rule to
 * keep in step with this one, and I-16's copied-schema arrangement exists
 * precisely because one artifact with a drift test beats two readings of it. So
 * the check lives once, here, where the load path and a generator both reach it.
 */
function checkItemSources(forms: FormDocument): SetFinding[] {
  const findings: SetFinding[] = [];
  for (const [formKey, form] of Object.entries(forms.forms) as [string, Form][]) {
    const declared = new Set(Object.keys(form.queries ?? {}));
    for (const source of itemSourcesOf(form)) {
      if (declared.has(source.query)) continue;
      findings.push({
        severity: "error",
        code: "missingItemSource",
        message:
          `form "${formKey}" field "${source.key}" takes its items from query ` +
          `"${source.query}", which the form does not declare`,
        path: `/forms/${formKey}/queries`,
        document: "forms",
      });
    }
  }
  return findings;
}

export interface DocumentSet {
  flow: UserFlow;
  actions: ActionDocument;
  forms: FormDocument;
}

/**
 * Every check that needs more than one of a flow's documents.
 *
 * Returns all findings rather than the first, because an author fixing a set
 * wants the list — the same reason `resolveEscapes` reports every unresolved
 * name at once.
 *
 * **Escape resolution is deliberately not here.** It needs the compiled registry
 * rather than the documents, so it stays in `store.ts` where the registry is;
 * this function is callable with three parsed documents and nothing else, which
 * is what lets a generator run it without a browser.
 */
export function validateDocumentSet(set: DocumentSet): SetFinding[] {
  return [
    ...checkForms(set.flow, set.forms),
    ...checkActions(set.flow, set.actions, set.forms),
    ...checkEndStateButtons(set.flow, set.forms),
    ...checkItemSources(set.forms),
  ];
}

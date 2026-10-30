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
import { actionRowsOf, type TableAction, type TableConfigDocument } from "../datatable/table";
import { buttonsOf, fieldsOf, itemSourcesOf, type FormDocument, type Form } from "./form";
import type { UserFlow } from "./schema";

export type SetFindingCode =
  /** A state, or a table action's `configForm`, names a form the form document does not define. */
  | "missingForm"
  /** A state, a form button or a table action names an action the action document does not define. */
  | "missingAction"
  /** An end state's form offers a button that tries to advance. */
  | "advanceFromEndState"
  /** A field's `itemsFrom` names a query its own form does not declare. */
  | "missingItemSource"
  /** A table's *+ Add* clears state and leaves a key the add path would start with. */
  | "addKeepsState";

export interface SetFinding {
  severity: "error";
  code: SetFindingCode;
  message: string;
  /** A JSON Pointer into the document the finding is about. */
  path: string;
  /** Which of the set the pointer indexes, since a set finding spans files. */
  document: "flow" | "actions" | "forms" | "tables";
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
 * reject two shipping forms. **All eleven satisfy "no `ufNext`".** (Ten end
 * states and one custom finisher, `fmMappingFormUF`, since `registerFileKeyUF`
 * was retired on 2026-10-01 by `jetstore_maintenance_02` `AD.4`; the argument
 * stands on the one.)
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
    // **Every button, not only the action bar's** — F.2 added a `button` field
    // and an inline one advances exactly as an action-bar one does.
    for (const action of buttonsOf(form)) {
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
    // The pointer names the action bar for a bar button and the form for an
    // inline one: `buttonsOf` flattens two shapes into one list and the index is
    // no longer a position in `actions` once it passes that boundary.
    const barCount = form.actions.length;
    buttonsOf(form).forEach((action, index) => {
      if (isStandardAction(action.action) || known.has(action.action)) return;
      findings.push({
        severity: "error",
        code: "missingAction",
        message: `form "${formKey}" has a button naming action "${action.action}", which the action document does not define`,
        path: index < barCount ? `/forms/${formKey}/actions/${index}` : `/forms/${formKey}/rows`,
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
    // `repeat.from` is the same relation on the form itself rather than on a
    // field: the query whose rows decide how many groups there are.
    const sources = [...itemSourcesOf(form)];
    if (form.repeat !== undefined) sources.push({ key: "repeat", query: form.repeat.from });
    for (const source of sources) {
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

/**
 * A table action's `actionName` and `configForm` resolve. Task F.3.
 *
 * **The third source of action names, and the first flow to have one is F.3's.**
 * `checkActions` above reads a state's `stateAction` and a form's buttons,
 * because when it was written those were the only two places a name could
 * appear. A table's `doAction` and `doActionShowDialog` name an entry in the
 * *flow's* action document too (`datatable/actionDispatch.ts`, `requestFor`
 * returns `runAction` for both), and nothing checked it. `clientRegistryUF` is
 * the first migrated flow whose tables carry actions — `client.tc.json` names
 * `deleteClientAction`, `org.tc.json` names `deleteOrgAction` and opens
 * `ufVendor` — so the gap had no way to show before now. `loadFilesUF`, the one
 * earlier set with table actions, happens to define both of its (I-88).
 *
 * **And `configForm` is the same gap on the form side.** `showDialog` and
 * `doActionShowDialog` name a form, and the four forms the corpus calls
 * *unreferenced* (`user_flows.json`, `unreferencedFormKeys`) are exactly the
 * four a table names this way — `ufVendor`, `loadRawRowsDialog`,
 * `pcNewProcessInputDialog` and `pcNewProcessInputDialog4MI`. Unreferenced there
 * means *named by no state*, which is what the corpus measures; it does not mean
 * unreachable, and F23 read it as though it did (I-89).
 *
 * **Both action rows, since F.5, and that widening found the only two references
 * `pipelineExecStatusTable` has.** A second row is presentation — the Dart draws
 * `secondRowActions` under `actions` and the two lists hold the same class
 * (`components/data_table.dart`) — so a check that read the first row alone would
 * have reported that table as naming nothing, while its `resubmitPipeline`
 * `doAction` and its `showFailureDetailsDialog` `showDialog` are both below the
 * fold. The 37 flow tables have no second row at all, which is why F.3 could not
 * have seen this.
 *
 * **This runs where the tables are, not inside `validateDocumentSet`.** That
 * function takes three parsed documents and nothing else, which is what lets a
 * generator run it without a browser and without fetching a `table_configs/`
 * directory; the tables arrive later in `FlowStore.load`, after the set has been
 * checked. Two functions rather than a widened one, on the same reasoning
 * `validateAllGroups` was added beside `validateForm` rather than replacing it.
 */
export function validateTableActions(
  actions: ActionDocument,
  forms: FormDocument,
  tables: Record<string, TableConfigDocument>,
): SetFinding[] {
  const findings: SetFinding[] = [];
  const knownActions = new Set(Object.keys(actions.actions));
  const knownForms = new Set(Object.keys(forms.forms));

  for (const [tableKey, table] of Object.entries(tables)) {
    // A static table cannot carry actions — `actions` is on the `query` arm of
    // the discriminated union alone (`datatable/table.ts`), which is the same
    // guard `actionNamesOf` opens with and is exact rather than a narrowing
    // convenience.
    if (table.source !== "query") continue;
    // The pointer keeps the rows apart — `/actions/3` and
    // `/secondRowActions/3` are different buttons — so the finding names the
    // property an author would edit rather than an index into a flattened list.
    // **Every row through `actionRowsOf` as of `AE.1`** (jetstore_maintenance_02,
    // 2026-10-01): this listed two rows by name, so a third would have been drawn
    // and never checked.
    for (const [row, list] of actionRowsOf(table)) {
      list.forEach((action: TableAction, index: number) => {
        const name = action.actionName;
        if (
          name !== undefined &&
          (action.action === "doAction" || action.action === "doActionShowDialog") &&
          !isStandardAction(name) &&
          !knownActions.has(name)
        ) {
          findings.push({
            severity: "error",
            code: "missingAction",
            message:
              `table "${tableKey}" action "${action.key}" runs "${name}", which the action ` +
              `document does not define`,
            path: `/${tableKey}/${row}/${index}/actionName`,
            document: "tables",
          });
        }
        const form = action.configForm;
        if (
          form !== undefined &&
          (action.action === "showDialog" || action.action === "doActionShowDialog") &&
          !knownForms.has(form)
        ) {
          findings.push({
            severity: "error",
            code: "missingForm",
            message:
              `table "${tableKey}" action "${action.key}" opens form "${form}", which the form ` +
              `document does not define`,
            path: `/${tableKey}/${row}/${index}/configForm`,
            document: "tables",
          });
        }
      });
    }
  }
  return findings;
}

/**
 * A table's *+ Add* that clears state must clear every key the add would start
 * with. `jetstore_maintenance_02` `AM.4` (`R-15`, generalised).
 *
 * **Why.** A wizard that opens on a table and returns to it after *Save* holds
 * one form state for the whole run, and decides insert or update from what is in
 * it. So *edit, back to the table, + Add, Save* updates the record just edited
 * unless the *+ Add* removes what the edit left (`D06`; the two lists and the
 * reason for each key are in `jets/workspace_assets/user_flows/actions.md`).
 * Phase 1 pinned the two committed lists by test. This is the same check for
 * any flow, including one a model writes.
 *
 * **What it applies to.** A table on a state's form whose `style: "add"` button
 * runs a `doAction` that removes keys (an unguarded `remove` step) and then goes
 * to another state (an unguarded `goToState`), which is the add step. A *+ Add*
 * that removes nothing does not reset state and is not checked.
 *
 * **What the clear list must hold**, from the documents alone:
 *
 * 1. every key the add step's form binds, so the add step opens empty; and
 * 2. when the list removes any key the table's own selection publishes (the
 *    field's `key` and the table's `formStateBinding`), all of them. The key
 *    that decides insert or update is one of these: `key` in both committed
 *    wizards. Whether a key decides is in code (`saveSourceConfigForFileType`
 *    reads `key`) and not in any document, so the rule cannot name that key. It
 *    names the set the key comes from instead.
 *
 * The condition on (2) is there for a *+ Add* that does not reset an edit at
 * all. `rulesUnitTestUF`'s runs table publishes the run selected for *View
 * Result*, its *+ Add* clears the previous test selection and none of the run's
 * keys, and its start posts named fields rather than the whole state. Requiring
 * the run's keys there would refuse a correct document (`Q-26`).
 */
export function validateAddClearsState(
  set: DocumentSet,
  tables: Record<string, TableConfigDocument>,
): SetFinding[] {
  const findings: SetFinding[] = [];
  const reported = new Set<string>();
  for (const state of Object.values(set.flow.states)) {
    const form = set.forms.forms[state.formConfig];
    if (form === undefined) continue; // `checkForms` reports it
    for (const field of fieldsOf(form)) {
      if (field.field !== "dataTable") continue;
      const table = tables[field.table];
      if (table === undefined || table.source !== "query") continue;
      const selection = [field.key, ...(table.formStateBinding?.otherColumns ?? []).map((c) => c.stateKey)];
      for (const [, list] of actionRowsOf(table)) {
        for (const button of list) {
          if (button.style !== "add" || button.action !== "doAction" || button.actionName === undefined) continue;
          const action = set.actions.actions[button.actionName];
          if (action === undefined) continue; // `validateTableActions` reports it
          const removeAt = action.steps.findIndex((step) => step.do === "remove" && step.when === undefined);
          const goTo = action.steps.find((step) => step.do === "goToState" && step.when === undefined);
          if (removeAt < 0 || goTo === undefined || goTo.do !== "goToState") continue;
          const cleared = new Set(
            action.steps.flatMap((step) => (step.do === "remove" && step.when === undefined ? step.keys : [])),
          );
          const addForm = set.forms.forms[set.flow.states[goTo.state]?.formConfig ?? ""];
          const required = new Set<string>();
          for (const f of addForm === undefined ? [] : fieldsOf(addForm)) {
            if ("key" in f && typeof f.key === "string") required.add(f.key);
          }
          if (selection.some((key) => cleared.has(key))) for (const key of selection) required.add(key);
          const missing = [...required].filter((key) => !cleared.has(key));
          if (missing.length === 0) continue;
          const message =
            `"${button.actionName}", the + Add on table "${field.table}", clears state before step ` +
            `"${goTo.state}" and leaves ${missing.map((k) => `"${k}"`).join(", ")}, so an add after an ` +
            `edit would start from the edited record`;
          if (reported.has(message)) continue;
          reported.add(message);
          findings.push({
            severity: "error",
            code: "addKeepsState",
            message,
            path: `/actions/${button.actionName}/steps/${removeAt}`,
            document: "actions",
          });
        }
      }
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

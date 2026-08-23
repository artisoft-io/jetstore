/**
 * Criterion 32, driven end to end: a projected template configured one step at a time.
 *
 * **Task M.5, and it is a demonstration rather than a unit test.** Everything before it
 * checked one piece — the generator's output against four validation layers, the
 * assembly against a fixture. This walks the `qc_metrics` projection the way an author
 * would: read the four documents off disk exactly as the generator committed them, load
 * them the way `FlowStore` does, press Next until the flow ends, and watch a `.pc.json`
 * come out of the escape at the other end.
 *
 * ## Why it is a test and not a screenshot
 *
 * **There is no flow runner in this application yet** — verified 2026-08-23 with the
 * `ui_refresh` session, whose gap it is: `App.tsx` registers no `/flow/:key` route,
 * `FlowStore` has no non-test consumer, nothing outside tests reads a `.form.json`, and
 * the only `EscapeRegistry` value in non-test code is `emptyRegistry`. The two
 * "migrated" flows are migrated in the sense that their documents exist and
 * `proofFlows.test.ts` drives them, not in the sense that a browser runs one.
 *
 * So criterion 32's *in the IDE* cannot be shown in a browser today, and this file does
 * not pretend otherwise. What it does show is that **every part of the mechanism the
 * criterion names is built and works together**: the shipped `engine`, the shipped
 * action interpreter, the shipped `validateForm`, the generator's documents, and this
 * project's escape. What is missing is the screen that hosts them, and it is missing for
 * every flow rather than for projected ones. That is the honest report, and it is the
 * one `proofFlows.test.ts` makes for the two hand-written flows too.
 *
 * ## What it would catch
 *
 * The chooser bug M.4 found, and I-84, and the `ufNext`-on-an-end-state defect this task
 * found — all three passed every layer that reads one document, and all three are
 * visible the moment something presses the buttons in order.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { emptyRegistry, type EscapeRegistry } from "../actions/escapes";
import { runAction, type ActionHost } from "../actions/interpret";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { FormState } from "../datatable/formState";
import { isStandardAction, startAt, step } from "../userflow/engine";
import { FormDocumentSchema, type Form, type FormDocument } from "../userflow/form";
import { UserFlowSchema, type UserFlow } from "../userflow/schema";
import { escapeReferences } from "../userflow/store";
import { resolveEscapes } from "../actions/escapes";
import { strictPolicy, validateFlow } from "../userflow/validate";
import { isFormValid } from "../userflow/validateForm";
import { createCpipesTemplateApply, configPath, applyPlanPath, type ApplyPlan } from "./templateApply";

/** The generator's committed output, read rather than imported so tsc keeps its size out. */
const PROJECTIONS = fileURLToPath(new URL("../../../tools/cpipes_contract/projections/", import.meta.url));
const read = (name: string): unknown => JSON.parse(readFileSync(`${PROJECTIONS}${name}`, "utf8"));

const TEMPLATE = "qc_metrics";
const WORKSPACE = "test_ws";

/** A workspace that serves the apply plan and remembers what was written to it. */
function workspace(plan: unknown) {
  const saved: { fileName: string; content: string }[] = [];
  return {
    saved,
    api: {
      readFile: async (_ws: string, node: { label: string }) => {
        if (node.label !== applyPlanPath(TEMPLATE)) throw new Error(`no such file: ${node.label}`);
        return { fileName: node.label, label: node.label, content: JSON.stringify(plan) };
      },
      saveFile: async (_ws: string, fileName: string, content: string) => {
        saved.push({ fileName, content });
      },
    },
  };
}

const host: ActionHost = {
  validate: () => true,
  confirm: async () => true,
  post: async () => ({ statusCode: 200 }),
  query: async () => null,
  notify: () => {},
  setBusy: () => {},
  goToState: () => {},
  close: () => {},
  userEmail: () => "michel@artisoft.io",
  now: () => 1_700_000_000_000,
};

/**
 * Fills one form the way a careful author would: every required field, no optional one.
 *
 * **Leaving the optional fields empty is the point rather than laziness.** I-80 settled
 * that every declarable property gets a field, so `partition_output` alone offers
 * properties the corpus never sets; a config that carried all of them as `""` would be a
 * different config from the one the author meant. What comes out the other end is the
 * check that "empty is absent" holds.
 */
function fill(form: Form, formState: FormState, choose: (key: string) => string | undefined): void {
  for (const row of form.rows) {
    for (const field of row) {
      if (field.field === "label" || field.field === "spacer") continue;
      const key = field.key;
      const required = (field.rules ?? []).some((r) => r.rule === "required");
      const json = (field.rules ?? []).some((r) => r.rule === "json");
      if (field.field === "dropdown") {
        const picked = choose(key) ?? field.items.map((i) => i.value).find((v) => v !== "");
        if (picked !== undefined) formState.setValue(0, key, picked);
        continue;
      }
      if (!required) continue;
      // **The value is the key.** A demonstration artefact that is read by people wants
      // every value to say which step of the wizard produced it; `expr_value` nine times
      // over would hide exactly the thing the committed config is evidence for.
      formState.setValue(0, key, json ? `["${key}"]` : key);
    }
  }
}

describe("criterion 32 — a template configured step at a time", () => {
  it("loads as a triple, walks to its end, and writes a config", async () => {
    const flow = UserFlowSchema.parse(read(`${TEMPLATE}.uf.json`)) as UserFlow;
    const actions = ActionDocumentSchema.parse(read(`${TEMPLATE}.ua.json`)) as ActionDocument;
    const forms = FormDocumentSchema.parse(read(`${TEMPLATE}.form.json`)) as FormDocument;
    const plan = read(`${TEMPLATE}.apply.json`) as ApplyPlan;

    // 1. The load path, in the order `FlowStore.load` runs it. I-84 is what makes the
    //    action document part of this rather than an afterthought: without it the read
    //    throws before anything below is reached.
    // **Under the strict policy, which is the one that refuses a save.** Nothing ships
    // the deployment's policy to the browser, so an author on a strict deployment sees a
    // warning and *then* has the save refused; the generator is where that has to fail.
    expect(validateFlow(flow, strictPolicy).filter((f) => f.severity === "error")).toEqual([]);
    const ws = workspace(plan);
    const registry: EscapeRegistry = {
      ...emptyRegistry,
      actions: { cpipesTemplateApply: createCpipesTemplateApply({ workspaceName: WORKSPACE, api: ws.api as never }) },
    };
    expect(resolveEscapes(escapeReferences(flow, actions), registry)).toEqual([]);

    // 2. The walk. `select` for every column mapping — the simplest of `ColumnMapping`'s
    //    eight variants, and the one the corpus reaches for most.
    const formState = new FormState();
    const choose = (key: string): string | undefined =>
      /^metric_columns\.\d+\.type$/.test(key) ? "select" : undefined;

    let position = startAt(flow);
    const visited: string[] = [];
    let outcome: string | null = null;
    for (let guard = 0; guard < 200; guard += 1) {
      const state = flow.states[position.stateKey]!;
      const form = forms.forms[state.formConfig]!;
      visited.push(position.stateKey);
      fill(form, formState, choose);
      expect({ [position.stateKey]: isFormValid(form, formState, 0) }).toEqual({ [position.stateKey]: true });

      const button = form.actions.map((a) => a.action).find((a) => a === "ufCompleted" || a === "ufNext")!;
      expect(isStandardAction(button)).toBe(true);
      const result = await step(button, {
        flow,
        position,
        formState,
        group: 0,
        runStateAction: (name) =>
          runAction({ action: actions.actions[name]!, host, formState, field: { group: 0, key: "t" }, registry, flowKey: TEMPLATE }),
        validate: () => isFormValid(form, formState, 0),
        exit: () => {},
      });
      position = result.position;
      outcome = result.outcome;
      if (result.finished) break;
    }

    // 3. **The escape returned nothing to say, which is how it reports success.** A
    //    string here would be the message an author sees instead of a saved config.
    expect(outcome).toBeNull();

    // **The walk is what "step at a time" means**; the document's 119 states are what
    // the author could have chosen. Twenty-four: the bindings, then nine column
    // mappings as a chooser and a form each, then the partition writer — itself four,
    // because its output channel is a second chooser and its writer config and `when`
    // are nested states of their own.
    expect(visited.length).toBe(24);
    expect(visited.length).toBeLessThan(Object.keys(flow.states).length / 4);
    expect(visited[0]).toBe("bindings");
    expect(new Set(visited).size).toBe(visited.length);

    // 4. What came out.
    expect(ws.saved.map((s) => s.fileName)).toEqual([configPath(TEMPLATE)]);
    const config = JSON.parse(ws.saved[0]!.content) as Record<string, unknown>;
    await expect(JSON.stringify(config, null, 2) + "\n").toMatchFileSnapshot(
      `${PROJECTIONS}${TEMPLATE}.demonstrated.pc.json`,
    );
  });

  it("says what is missing rather than writing half a config", async () => {
    const flow = UserFlowSchema.parse(read(`${TEMPLATE}.uf.json`)) as UserFlow;
    const actions = ActionDocumentSchema.parse(read(`${TEMPLATE}.ua.json`)) as ActionDocument;
    const plan = read(`${TEMPLATE}.apply.json`) as ApplyPlan;

    // Nothing collected at all: every chooser is unanswered and every binding absent.
    const ws = workspace(plan);
    const escape = createCpipesTemplateApply({ workspaceName: WORKSPACE, api: ws.api as never });
    const outcome = await escape({ formState: new FormState(), group: 0, flowKey: TEMPLATE });

    expect(outcome).toContain("not complete");
    expect(outcome).toContain("no type was chosen");
    // **The failure is a message, not a file.** An author who reaches the end with a gap
    // gets told where it is and keeps everything they entered.
    expect(ws.saved).toEqual([]);
    expect(Object.keys(actions.actions)).toEqual(["cpipesTemplateApply"]);
    expect(flow.states[flow.startAtKey]).toBeDefined();
  });
});

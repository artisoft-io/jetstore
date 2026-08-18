/**
 * The two proof flows, driven end to end. Task S.5.
 *
 * **This is the test the whole phase was built to make possible.** Everything
 * before it checked one piece against the Dart: the query builder's payload, the
 * table's paging, the schema's shape, the grammar's steps. This drives a flow
 * the way a user does — press Next, watch the graph move, press Save, watch the
 * request go — with nothing compiled in but the widgets.
 *
 * The flow, its actions and its forms are all read from the documents.
 */

import { describe, expect, it } from "vitest";

import { emptyRegistry } from "../actions/escapes";
import { runAction, type ActionHost } from "../actions/interpret";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import loadFilesActionsDoc from "../actions/flows/loadFilesUF.ua.json";
import registerFileKeyActionsDoc from "../actions/flows/registerFileKeyUF.ua.json";
import { FormState } from "../datatable/formState";
import { FormDocumentSchema, type FormDocument } from "./form";
import loadFilesFormsDoc from "./forms/loadFilesUF.form.json";
import registerFileKeyFormsDoc from "./forms/registerFileKeyUF.form.json";
import { advance, back, evaluateCondition, isStandardAction, startAt, step, FlowError } from "./engine";
import loadFilesFlowDoc from "./flows/loadFilesUF.uf.json";
import registerFileKeyFlowDoc from "./flows/registerFileKeyUF.uf.json";
import { UserFlowSchema, type UserFlow } from "./schema";
import { isFormValid, validateForm } from "./validateForm";

const field = { group: 0, key: "t" };

function harness(
  flowDoc: unknown,
  actionsDoc: unknown,
  formsDoc: unknown,
) {
  const flow = UserFlowSchema.parse(flowDoc) as UserFlow;
  const actions = ActionDocumentSchema.parse(actionsDoc) as ActionDocument;
  const forms = FormDocumentSchema.parse(formsDoc) as FormDocument;
  const formState = new FormState();
  const posts: { endpoint: string; body: Record<string, unknown> }[] = [];
  const events: string[] = [];

  const host: ActionHost = {
    validate: () => true,
    confirm: async () => true,
    post: async (r) => {
      posts.push(r);
      return { statusCode: 200 };
    },
    query: async () => null,
    notify: (level, message) => events.push(`notify:${level}:${message}`),
    setBusy: (b) => events.push(b ? "busy" : "idle"),
    goToState: (s) => events.push(`goToState:${s}`),
    close: () => events.push("close"),
    userEmail: () => "michel@artisoft.io",
    now: () => 1_700_000_000_000,
  };

  let position = startAt(flow);
  const formFor = (key: string) => forms.forms[flow.states[key]!.formConfig]!;

  const press = async (action: string) => {
    if (!isStandardAction(action)) {
      // A form button naming an entry in the action document.
      return runAction({
        action: actions.actions[action]!,
        host,
        formState,
        field,
        registry: emptyRegistry,
        flowKey: "test",
      });
    }
    const result = await step(action, {
      flow,
      position,
      formState,
      group: 0,
      runStateAction: (name) =>
        runAction({
          action: actions.actions[name]!,
          host,
          formState,
          field,
          registry: emptyRegistry,
          flowKey: "test",
        }),
      validate: () => isFormValid(formFor(position.stateKey), formState, 0),
      exit: () => events.push("exit"),
    });
    position = result.position;
    return result.outcome;
  };

  return {
    flow,
    forms,
    formState,
    posts,
    events,
    press,
    formFor,
    at: () => position.stateKey,
    visited: () => position.visited,
  };
}

describe("the documents are complete and consistent", () => {
  it.each([
    ["registerFileKeyUF", registerFileKeyFlowDoc, registerFileKeyActionsDoc, registerFileKeyFormsDoc],
    ["loadFilesUF", loadFilesFlowDoc, loadFilesActionsDoc, loadFilesFormsDoc],
  ])("%s validates and every state's form exists", (_name, flowDoc, actionsDoc, formsDoc) => {
    const flow = UserFlowSchema.parse(flowDoc) as UserFlow;
    const forms = FormDocumentSchema.parse(formsDoc) as FormDocument;
    ActionDocumentSchema.parse(actionsDoc);
    for (const [key, state] of Object.entries(flow.states)) {
      expect({ [key]: forms.forms[state.formConfig] !== undefined }).toEqual({ [key]: true });
    }
  });

  it("names only actions the flow's own documents define", () => {
    // A form button or a stateAction pointing at nothing is the failure S.4
    // cannot catch — it validates each document alone, not the set.
    for (const [flowDoc, actionsDoc, formsDoc] of [
      [registerFileKeyFlowDoc, registerFileKeyActionsDoc, registerFileKeyFormsDoc],
      [loadFilesFlowDoc, loadFilesActionsDoc, loadFilesFormsDoc],
    ] as const) {
      const flow = UserFlowSchema.parse(flowDoc) as UserFlow;
      const actions = ActionDocumentSchema.parse(actionsDoc) as ActionDocument;
      const forms = FormDocumentSchema.parse(formsDoc) as FormDocument;
      const known = new Set(Object.keys(actions.actions));

      for (const state of Object.values(flow.states)) {
        if (state.stateAction !== undefined) expect(known).toContain(state.stateAction);
      }
      for (const form of Object.values(forms.forms)) {
        for (const action of form.actions) {
          if (!isStandardAction(action.action)) expect(known).toContain(action.action);
        }
      }
    }
  });
});

describe("register_file_key, end to end", () => {
  const setup = () => harness(registerFileKeyFlowDoc, registerFileKeyActionsDoc, registerFileKeyFormsDoc);

  it("starts on its only state, which is also its end", () => {
    const h = setup();
    expect(h.at()).toBe("submit_schema_event");
    expect(h.flow.states["submit_schema_event"]!.isEnd).toBe(true);
  });

  it("refuses to submit an empty form, and says what is missing", () => {
    const h = setup();
    const errors = validateForm(h.formFor("submit_schema_event"), h.formState, 0);
    expect(errors.map((e) => e.message)).toEqual([
      "Please provide a file key",
      "Please provide a Schema Event json",
    ]);
  });

  it("refuses a schema event that is not json", () => {
    const h = setup();
    h.formState.setValue(0, "file_key", "s3://bucket/key.csv");
    h.formState.setValue(0, "schemaEventJson", "{ not json");
    const errors = validateForm(h.formFor("submit_schema_event"), h.formState, 0);
    expect(errors).toHaveLength(1);
    expect(errors[0]!.message).toContain("Schema Event is not a valid json");
  });

  it("submits the schema event and closes", async () => {
    const h = setup();
    h.formState.setValue(0, "file_key", "s3://bucket/key.csv");
    h.formState.setValue(0, "schemaEventJson", '{"schema":"v1"}');
    expect(await h.press("rfkSubmitSchemaEventUF")).toBeNull();

    expect(h.posts).toHaveLength(1);
    expect(h.posts[0]).toEqual({
      endpoint: "/registerFileKey",
      body: {
        action: "put_schema_event_to_s3",
        data: [{ file_key: "s3://bucket/key.csv", event: '{"schema":"v1"}' }],
      },
    });
    // Spinner up, request, spinner down, dialog closed — the Dart's order.
    expect(h.events).toEqual(["busy", "idle", "close"]);
  });

  it("completing the flow exits without posting twice", async () => {
    const h = setup();
    h.formState.setValue(0, "file_key", "k");
    h.formState.setValue(0, "schemaEventJson", "{}");
    await h.press("ufCompleted");
    // `submit_schema_event` carries `stateAction: rfkSubmitSchemaEventUF`, so
    // Completed runs it once — the same action the Save button runs.
    expect(h.posts).toHaveLength(1);
    expect(h.events).toContain("exit");
  });
});

describe("load_files, end to end", () => {
  const setup = () => harness(loadFilesFlowDoc, loadFilesActionsDoc, loadFilesFormsDoc);

  const selectSourceConfig = (h: ReturnType<typeof setup>) => {
    h.formState.setValue(0, "lfSourceConfigTable", ["sc-1"]);
    h.formState.setValue(0, "client", ["CI"]);
    h.formState.setValue(0, "org", ["D1"]);
    h.formState.setValue(0, "object_type", ["hc:Eligibility"]);
    h.formState.setValue(0, "table_name", ["CI_D1_hc:Eligibility"]);
  };

  it("will not advance until a source config is selected", async () => {
    const h = setup();
    expect(h.at()).toBe("select_source_config");
    await h.press("ufNext");
    expect(h.at()).toBe("select_source_config");

    selectSourceConfig(h);
    await h.press("ufNext");
    expect(h.at()).toBe("select_file_keys");
    expect(h.visited()).toEqual(["select_source_config", "select_file_keys"]);
  });

  it("goes back to the first step, and refuses to go further", async () => {
    const h = setup();
    selectSourceConfig(h);
    await h.press("ufNext");
    await h.press("ufPrevious");
    expect(h.at()).toBe("select_source_config");
    await expect(h.press("ufPrevious")).rejects.toThrow(FlowError);
  });

  it("loads the selected files and exits", async () => {
    const h = setup();
    selectSourceConfig(h);
    await h.press("ufNext");
    h.formState.setValue(0, "lfFileKeyStagingTable", ["fk-1", "fk-2"]);
    h.formState.setValue(0, "file_key", ["a.csv", "b.csv"]);
    h.formState.setValue(0, "input_registry.session_id", ["11", "22"]);
    h.formState.setValue(0, "source_period_key", ["101", "202"]);

    await h.press("ufCompleted");

    expect(h.posts).toHaveLength(1);
    const body = h.posts[0]!.body as { action: string; data: Record<string, string>[] };
    expect(body.action).toBe("insert_rows");
    // One row per selected file key — the fan-out, driven by the document.
    expect(body.data.map((r) => r["file_key"])).toEqual(["a.csv", "b.csv"]);
    expect(body.data.map((r) => r["session_id"])).toEqual(["1700000000000", "1700000000001"]);
    expect(body.data[0]!["client"]).toBe("CI");
    expect(h.events).toContain("exit");
  });

  it("cancelling runs no action at all", async () => {
    // `ufCancel` is the one exit that skips the state's action, which is what
    // makes it a cancel rather than a save.
    const h = setup();
    selectSourceConfig(h);
    await h.press("ufNext");
    await h.press("ufCancel");
    expect(h.posts).toHaveLength(0);
    expect(h.events).toContain("exit");
  });
});

describe("the page stack", () => {
  it("unwinds to an already-visited page rather than pushing a duplicate", () => {
    // `sourceConfigUF` reconverges on one state from four different branches;
    // a plain stack would send Previous back through the branch.
    const position = { stateKey: "c", visited: ["a", "b", "c"] };
    expect(advance(position, "b").visited).toEqual(["a", "b"]);
    expect(advance(position, "d").visited).toEqual(["a", "b", "c", "d"]);
  });

  it("refuses to go back from the first page", () => {
    expect(() => back({ stateKey: "a", visited: ["a"] })).toThrow(FlowError);
  });
});

describe("conditions", () => {
  const formState = new FormState();
  formState.setValue(0, "scalar", "csv");
  formState.setValue(0, "one", ["csv"]);
  formState.setValue(0, "many", ["a", "b"]);

  it("unwraps a one-element selection on either side of equals", () => {
    // `user_flow_config.dart:168` — the reason a table's selection compares
    // equal to a literal without the author unwrapping it.
    expect(evaluateCondition({ op: "equals", key: "one", value: "csv" }, formState, 0)).toBe(true);
    expect(evaluateCondition({ op: "equals", key: "scalar", valueFromKey: "one" }, formState, 0)).toBe(true);
  });

  it("is false rather than throwing when either side is missing", () => {
    expect(evaluateCondition({ op: "equals", key: "absent", value: "csv" }, formState, 0)).toBe(false);
    expect(evaluateCondition({ op: "contains", key: "absent", value: "a" }, formState, 0)).toBe(false);
  });

  it("requires a list on the left of contains, and a scalar on the right", () => {
    expect(evaluateCondition({ op: "contains", key: "many", value: "b" }, formState, 0)).toBe(true);
    expect(evaluateCondition({ op: "contains", key: "scalar", value: "csv" }, formState, 0)).toBe(false);
  });

  it("treats an empty string and an empty list alike", () => {
    formState.setValue(0, "blank", "");
    expect(evaluateCondition({ op: "isNullOrEmpty", key: "blank" }, formState, 0)).toBe(true);
    expect(evaluateCondition({ op: "isNullOrEmpty", key: "absent" }, formState, 0)).toBe(true);
    expect(evaluateCondition({ op: "isNullOrEmpty", key: "many" }, formState, 0)).toBe(false);
  });

  it("combines with not, and, or", () => {
    const yes = { op: "equals", key: "scalar", value: "csv" } as const;
    const no = { op: "equals", key: "scalar", value: "xlsx" } as const;
    expect(evaluateCondition({ op: "not", condition: no }, formState, 0)).toBe(true);
    expect(evaluateCondition({ op: "and", conditions: [yes, no] }, formState, 0)).toBe(false);
    expect(evaluateCondition({ op: "or", conditions: [yes, no] }, formState, 0)).toBe(true);
  });
});

describe("the emitted form schema", () => {
  it("matches the committed artifact", async () => {
    const { readFileSync, writeFileSync } = await import("node:fs");
    const { fileURLToPath } = await import("node:url");
    const { emitJsonSchema } = await import("./form");
    const path = fileURLToPath(new URL("./form.schema.json", import.meta.url));
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") writeFileSync(path, emitted);
    expect(readFileSync(path, "utf8")).toBe(emitted);
  });
});

/**
 * The flows that run from documents, driven end to end. Tasks S.5 and F.2.
 *
 * **This is the test the whole phase was built to make possible.** Everything
 * before it checked one piece against the Dart: the query builder's payload, the
 * table's paging, the schema's shape, the grammar's steps. This drives a flow
 * the way a user does — press Next, watch the graph move, press Save, watch the
 * request go — with nothing compiled in but the widgets.
 *
 * The flow, its actions and its forms are all read from the documents.
 *
 * **F.2 widened it from "the two proof flows" to "the flows that run".** The
 * harness below is the flow runner minus a DOM, so a migrated flow belongs here
 * whether or not it is one of Phase 2's two — and `loadConfigUF` and
 * `workspacePullUF` are the pair that share a delegate file, which is the thing
 * the re-partition has to get right and which no single-flow test can see.
 * `mapFileUF` is the exception and stays in `FlowRunner.test.tsx`: its form
 * repeats, so it needs the screen that sizes the groups.
 */

import { describe, expect, it } from "vitest";

import { emptyRegistry, type EscapeRegistry } from "../actions/escapes";
import {
  currentDataRegistryFilters,
  currentHomeFilters,
  homeFiltersFormValidator,
  resetHomeFilters,
  savedHomeFilterState,
  seedFromHomeFilters,
  setIdFilter,
  updateHomeFilters,
} from "../actions/homeFilters";
import { downloadMapping, loadRawRows } from "../actions/fileMapping";
import { runAction, type ActionHost } from "../actions/interpret";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import clientRegistryActionsDoc from "../actions/flows/clientRegistryUF.ua.json";
import fileMappingActionsDoc from "../actions/flows/fileMappingUF.ua.json";
import homeFiltersActionsDoc from "../actions/flows/homeFiltersUF.ua.json";
import pipelineConfigActionsDoc from "../actions/flows/pipelineConfigUF.ua.json";
import loadConfigActionsDoc from "../actions/flows/loadConfigUF.ua.json";
import loadFilesActionsDoc from "../actions/flows/loadFilesUF.ua.json";
import registerFileKeyActionsDoc from "../actions/flows/registerFileKeyUF.ua.json";
import startPipelineActionsDoc from "../actions/flows/startPipelineUF.ua.json";
import workspacePullActionsDoc from "../actions/flows/workspacePullUF.ua.json";
import { FormState } from "../datatable/formState";
import { FormDocumentSchema, type FormDocument } from "./form";
import clientRegistryFormsDoc from "./forms/clientRegistryUF.form.json";
import fileMappingFormsDoc from "./forms/fileMappingUF.form.json";
import homeFiltersFormsDoc from "./forms/homeFiltersUF.form.json";
import pipelineConfigFormsDoc from "./forms/pipelineConfigUF.form.json";
import loadConfigFormsDoc from "./forms/loadConfigUF.form.json";
import loadFilesFormsDoc from "./forms/loadFilesUF.form.json";
import registerFileKeyFormsDoc from "./forms/registerFileKeyUF.form.json";
import startPipelineFormsDoc from "./forms/startPipelineUF.form.json";
import workspacePullFormsDoc from "./forms/workspacePullUF.form.json";
import { advance, back, evaluateCondition, isStandardAction, startAt, step, FlowError } from "./engine";
import clientRegistryFlowDoc from "./flows/clientRegistryUF.uf.json";
import fileMappingFlowDoc from "./flows/fileMappingUF.uf.json";
import homeFiltersFlowDoc from "./flows/homeFiltersUF.uf.json";
import pipelineConfigFlowDoc from "./flows/pipelineConfigUF.uf.json";
import loadConfigFlowDoc from "./flows/loadConfigUF.uf.json";
import loadFilesFlowDoc from "./flows/loadFilesUF.uf.json";
import registerFileKeyFlowDoc from "./flows/registerFileKeyUF.uf.json";
import startPipelineFlowDoc from "./flows/startPipelineUF.uf.json";
import workspacePullFlowDoc from "./flows/workspacePullUF.uf.json";
import { validateDocumentSet } from "./documentSet";
import { UserFlowSchema, type UserFlow } from "./schema";
import { isFormValid, validateForm } from "./validateForm";

const field = { group: 0, key: "t" };

function harness(
  flowDoc: unknown,
  actionsDoc: unknown,
  formsDoc: unknown,
  /**
   * The escapes this run resolves. Defaulted to `emptyRegistry`, which is what
   * every flow before F.5 needed — `homeFiltersUF` is the first whose behaviour
   * is partly in an escape body, so it is the first that has to supply one.
   */
  registry: EscapeRegistry = emptyRegistry,
  /**
   * What a `query` step reads. Defaulted to *no rows*, which is what every flow
   * before F.6 needed — `pipelineConfigUF` is the only one with a `query` step at
   * all (I-23), so it is the only caller that has to supply one.
   */
  query: ActionHost["query"] = async () => null,
  /**
   * What a `read` answers. Defaulted to *no rows*, which is what every flow
   * before F.8 needed — `fileMappingUF`'s `downloadMapping` is the first escape
   * of the corpus that reads rows rather than writing them.
   */
  read: ActionHost["read"] = async () => [],
) {
  const flow = UserFlowSchema.parse(flowDoc) as UserFlow;
  const actions = ActionDocumentSchema.parse(actionsDoc) as ActionDocument;
  const forms = FormDocumentSchema.parse(formsDoc) as FormDocument;
  const formState = new FormState();
  const posts: { endpoint: string; body: Record<string, unknown> }[] = [];
  const events: string[] = [];
  const downloads: { fileName: string; content: string }[] = [];

  const host: ActionHost = {
    validate: () => true,
    confirm: async () => true,
    post: async (r) => {
      posts.push(r);
      return { statusCode: 200 };
    },
    query,
    read,
    download: (fileName, content) => downloads.push({ fileName, content }),
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
        registry,
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
          registry,
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
    downloads,
    press,
    formFor,
    at: () => position.stateKey,
    visited: () => position.visited,
  };
}

describe("the documents are complete and consistent", () => {
  // **Moved to `documentSet.ts` rather than kept here.** These two checks were
  // written in this file because it was the only place holding a whole set, and
  // that is exactly why nothing else got them: a generator producing a set had
  // no way to run them, and `store.ts` did not either. They are one exported
  // function now, and this test is the corpus half of it — `documentSet.test.ts`
  // carries the cases that make each check fire.
  it.each([
    ["registerFileKeyUF", registerFileKeyFlowDoc, registerFileKeyActionsDoc, registerFileKeyFormsDoc],
    ["loadFilesUF", loadFilesFlowDoc, loadFilesActionsDoc, loadFilesFormsDoc],
    ["loadConfigUF", loadConfigFlowDoc, loadConfigActionsDoc, loadConfigFormsDoc],
    ["workspacePullUF", workspacePullFlowDoc, workspacePullActionsDoc, workspacePullFormsDoc],
    ["clientRegistryUF", clientRegistryFlowDoc, clientRegistryActionsDoc, clientRegistryFormsDoc],
    ["startPipelineUF", startPipelineFlowDoc, startPipelineActionsDoc, startPipelineFormsDoc],
    ["homeFiltersUF", homeFiltersFlowDoc, homeFiltersActionsDoc, homeFiltersFormsDoc],
    ["pipelineConfigUF", pipelineConfigFlowDoc, pipelineConfigActionsDoc, pipelineConfigFormsDoc],
    ["fileMappingUF", fileMappingFlowDoc, fileMappingActionsDoc, fileMappingFormsDoc],
  ])("%s is a consistent set", (_name, flowDoc, actionsDoc, formsDoc) => {
    expect(
      validateDocumentSet({
        flow: UserFlowSchema.parse(flowDoc) as UserFlow,
        actions: ActionDocumentSchema.parse(actionsDoc) as ActionDocument,
        forms: FormDocumentSchema.parse(formsDoc) as FormDocument,
      }),
    ).toEqual([]);
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

/**
 * F.2's two flows, which are one delegate file and two state machines.
 *
 * **The re-partition is what these test, and the payload is what it costs to get
 * wrong.** `updateDbClients` is the key the server branches on — a comma-joined
 * client list means *load these*, its absence means *load them all*
 * (`jets/datatable/workspace_helper_functions.go`, `loadWorkspaceConfigAction`) —
 * and the coverage document promoted verbatim would have posted neither.
 */
describe("load_config, end to end", () => {
  const setup = () => {
    const h = harness(loadConfigFlowDoc, loadConfigActionsDoc, loadConfigFormsDoc);
    // The route's parameter, which `FlowRunner` seeds from the query string.
    h.formState.setValue(0, "workspace_name", "jets_ws");
    h.formState.setValue(0, "workspace_uri", "git@github.com:artisoft-io/jets_ws.git");
    return h;
  };

  it("starts on the form that offers the client list", () => {
    const h = setup();
    expect(h.at()).toBe("load_config");
    expect(h.formFor("load_config").rows.flat().map((f) => f.field)).toEqual([
      "text",
      "text",
      "dataTable",
      "spacer",
      "spacer",
      "button",
    ]);
  });

  it("refuses to advance with no client selected", async () => {
    const h = setup();
    const errors = validateForm(h.formFor("load_config"), h.formState, 0);
    expect(errors.map((e) => e.message)).toEqual(["Select Client to load their configuration"]);
    // `stay()` with no message: the engine leaves the errors to the caller's
    // list rather than putting a banner over them (`engine.ts`, `ufNext`).
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("load_config");
    expect(h.posts).toEqual([]);
  });

  it("carries the selection to the confirmation page as its read-only twin", async () => {
    const h = setup();
    h.formState.setValue(0, "wpClientList", ["ACME", "USI"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("confirm");
    expect(h.formState.getValue(0, "wpClientListRO")).toEqual(["ACME", "USI"]);
  });

  it("posts the selection as a comma-joined updateDbClients", async () => {
    const h = setup();
    h.formState.setValue(0, "wpClientList", ["ACME", "USI"]);
    await h.press("ufNext");
    expect(await h.press("ufCompleted")).toBeNull();
    expect(h.posts).toHaveLength(1);
    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["action"]).toBe("workspace_insert_rows");
    expect(body["fromClauses"]).toEqual([{ table: "load_workspace_config" }]);
    expect(body["workspaceName"]).toBe("jets_ws");
    const row = (body["data"] as Record<string, unknown>[])[0]!;
    expect(row["updateDbClients"]).toBe("ACME,USI");
    expect(row["user_email"]).toBe("michel@artisoft.io");
    expect(row["workspace_name"]).toBe("jets_ws");
  });

  it("sends updateDbClients as null when the inline button loads them all", async () => {
    const h = setup();
    h.formState.setValue(0, "wpClientList", ["ACME"]);
    // The inline `button` field, dispatched exactly as an action-bar button is.
    const button = h.formFor("load_config").rows.flat().find((f) => f.field === "button")!;
    expect("action" in button && button.action).toBe("wpLoadAllClientConfigUF");
    expect(await h.press("wpLoadAllClientConfigUF")).toBeNull();
    const row = (h.posts[0]!.body["data"] as Record<string, unknown>[])[0]!;
    // **Null, not `""` and not `{}`.** The server reads a null here as
    // "-initWorkspaceDb", which is the whole point of the button.
    expect(row["updateDbClients"]).toBeNull();
    expect(row["wpClientList"]).toBeUndefined();
    expect(h.events).toContain("close");
  });

  it("does not offer an advancing button on its end state", () => {
    const h = setup();
    expect(h.flow.states["confirm"]!.isEnd).toBe(true);
    expect(h.formFor("confirm").actions.map((a) => a.action)).toEqual([
      "ufPrevious",
      "ufCancel",
      "ufCompleted",
    ]);
  });
});

describe("pull_workspace, end to end", () => {
  const setup = () => {
    const h = harness(workspacePullFlowDoc, workspacePullActionsDoc, workspacePullFormsDoc);
    for (const [key, value] of [
      ["key", "12"],
      ["workspace_name", "jets_ws"],
      ["workspace_branch", "main"],
      ["feature_branch", "jets_ai"],
      ["workspace_uri", "git@github.com:artisoft-io/jets_ws.git"],
    ] as const) {
      h.formState.setValue(0, key, value);
    }
    return h;
  };

  it("branches to the client list only when the selected-clients option is ticked", async () => {
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadSelectedClientConfgOption"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("select_clients");
  });

  it("goes straight to the confirmation otherwise", async () => {
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpCompileWorkspaceOption"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("confirm");
  });

  it("drops a client selection the chosen actions no longer use", async () => {
    // The `when` guard, and the reason it had to exist. The user ticks
    // "SELECTED clients", picks two, goes Previous, unticks it, and goes Next:
    // without the guarded `remove` the confirmation page still lists two
    // clients and the post still names them.
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadSelectedClientConfgOption"]);
    await h.press("ufNext");
    h.formState.setValue(0, "wpClientList", ["ACME", "USI"]);
    await h.press("ufNext");
    expect(h.formState.getValue(0, "wpClientListRO")).toEqual(["ACME", "USI"]);

    await h.press("ufPrevious");
    await h.press("ufPrevious");
    expect(h.at()).toBe("pull_workspace");
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadClientConfgOption"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("confirm");
    expect(h.formState.getValue(0, "wpClientList")).toBeUndefined();
    expect(h.formState.getValue(0, "wpClientListRO")).toBeUndefined();
  });

  it("keeps the selection when the option is still ticked", async () => {
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadSelectedClientConfgOption"]);
    h.formState.setValue(0, "wpClientList", ["ACME"]);
    await h.press("ufNext");
    expect(h.formState.getValue(0, "wpClientList")).toEqual(["ACME"]);
  });

  it("survives an untouched option table, which the Dart does not", async () => {
    // `final l = state[otherWorkspaceActionOptions] as List` is a hard cast, not
    // an assert, so in Flutter this throws — see the register. Here the guard's
    // `contains` is false on a missing key and the step is skipped.
    const h = setup();
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("confirm");
  });

  it("posts the whole state with the workspace extras beside it", async () => {
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadSelectedClientConfgOption"]);
    await h.press("ufNext");
    h.formState.setValue(0, "wpClientList", ["ACME", "USI"]);
    await h.press("ufNext");
    expect(await h.press("ufCompleted")).toBeNull();
    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["fromClauses"]).toEqual([{ table: "pull_workspace" }]);
    expect(body["workspaceName"]).toBe("jets_ws");
    expect(body["workspaceBranch"]).toBe("main");
    expect(body["featureBranch"]).toBe("jets_ai");
    const row = (body["data"] as Record<string, unknown>[])[0]!;
    expect(row["key"]).toBe("12");
    expect(row["workspace_uri"]).toBe("git@github.com:artisoft-io/jets_ws.git");
    expect(row["updateDbClients"]).toBe("ACME,USI");
    expect(row["otherWorkspaceActionOptions"]).toEqual(["wpLoadSelectedClientConfgOption"]);
    expect(row["wpPullWorkspaceConfirmOptions"]).toEqual(["wpLoadSelectedClientConfgOption"]);
  });

  it("shares wpLoadConfigConfirmUF with load_config and means the same thing", async () => {
    const h = setup();
    h.formState.setValue(0, "otherWorkspaceActionOptions", ["wpLoadSelectedClientConfgOption"]);
    await h.press("ufNext");
    h.formState.setValue(0, "wpClientList", ["ACME"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.formState.getValue(0, "wpClientListRO")).toEqual(["ACME"]);
  });
});

/**
 * F.3's flow, and the first whose *tables* run actions.
 *
 * **The step order is what these are for.** The Dart builds its request body
 * with `jsonEncode` and *then* mutates form state, so `clearSelectedRow` and
 * `state.remove(client)` sit above the `await` and change nothing that is sent
 * (`client_registry/form_action_delegates.dart`, `clientRegistryFormActions`).
 * A step grammar has no "encode now, send later": `clearSelection` writes null,
 * `setValue(…, null)` **deletes the key** (`datatable/formState.ts`, `setValue`),
 * and `wholeState` is a snapshot taken when the `post` step runs. The coverage
 * document transcribed the Dart's line order, which puts `clearSelection` before
 * the post — and `delete/client` takes `client` as its only bound parameter
 * (`jets/datatable/sql_stmts.go`, `"delete/client"`), so the delete would have
 * run with a NULL, matched nothing, and returned 200 (I-90).
 */
describe("client_registry, end to end", () => {
  const setup = () => harness(clientRegistryFlowDoc, clientRegistryActionsDoc, clientRegistryFormsDoc);

  it("refuses to advance until an option is chosen", async () => {
    const h = setup();
    expect(h.at()).toBe("select_client_vendor");
    expect(validateForm(h.formFor("select_client_vendor"), h.formState, 0).map((e) => e.message))
      .toEqual(["An option must be selected."]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("select_client_vendor");
  });

  it("branches to create_client or select_client on the chosen option", async () => {
    const create = setup();
    create.formState.setValue(0, "ufClientOrVendorOption", ["ufClientOption"]);
    expect(await create.press("ufNext")).toBeNull();
    expect(create.at()).toBe("create_client");

    const select = setup();
    select.formState.setValue(0, "ufClientOrVendorOption", ["ufVendorOption"]);
    expect(await select.press("ufNext")).toBeNull();
    expect(select.at()).toBe("select_client");
  });

  it("creates a client, sending ufClientDetails as details and dropping both after", async () => {
    const h = setup();
    h.formState.setValue(0, "ufClientOrVendorOption", ["ufClientOption"]);
    await h.press("ufNext");
    h.formState.setValue(0, "client", "ACME");
    h.formState.setValue(0, "ufClientDetails", "a note");
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("show_org");

    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["action"]).toBe("insert_rows");
    expect(body["fromClauses"]).toEqual([{ table: "client_registry" }]);
    const row = (body["data"] as Record<string, unknown>[])[0]!;
    expect(row["client"]).toBe("ACME");
    expect(row["details"]).toBe("a note");
    // **The Dart sends `ufClientDetails` too**, because the removes are below
    // the encode. The coverage document had `omit: ["ufClientDetails"]`, which is
    // inert — the server projects a row through `ColumnKeys` — and says
    // something about the payload that is not true.
    expect(row["ufClientDetails"]).toBe("a note");
    expect(h.formState.getValue(0, "details")).toBeUndefined();
    expect(h.formState.getValue(0, "ufClientDetails")).toBeUndefined();
  });

  it("unpacks the selected client so the org table can filter on it", async () => {
    const h = setup();
    h.formState.setValue(0, "ufClientOrVendorOption", ["ufVendorOption"]);
    await h.press("ufNext");
    h.formState.setValue(0, "client", ["ACME"]);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("show_org");
    // `org.tc.json`'s where clause reads `client` as a scalar.
    expect(h.formState.getValue(0, "client")).toBe("ACME");
  });

  it("ends on show_org, which offers no advancing button", () => {
    const h = setup();
    expect(h.flow.states["show_org"]!.isEnd).toBe(true);
    expect(h.formFor("show_org").actions.map((a) => a.action)).toEqual(["ufPrevious", "ufCompleted"]);
  });

  it("deletes the client it posted, not the one it has just cleared", async () => {
    const h = setup();
    h.formState.setValue(0, "client", ["ACME"]);
    expect(await h.press("deleteClientAction")).toBeNull();
    const row = (h.posts[0]!.body["data"] as Record<string, unknown>[])[0]!;
    expect(row["client"]).toBe("ACME");
    // Cleared afterwards, which is the whole ordering question.
    expect(h.formState.getValue(0, "client")).toBeUndefined();
  });

  it("deletes an organization with both of its bound columns present", async () => {
    const h = setup();
    h.formState.setValue(0, "client", "ACME");
    h.formState.setValue(0, "org", ["ACME_EAST"]);
    expect(await h.press("deleteOrgAction")).toBeNull();
    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["fromClauses"]).toEqual([{ table: "delete/org" }]);
    const row = (body["data"] as Record<string, unknown>[])[0]!;
    expect(row["client"]).toBe("ACME");
    expect(row["org"]).toBe("ACME_EAST");
    expect(h.formState.getValue(0, "org")).toBeUndefined();
    // The client survives: the user stays on this client's organizations.
    expect(h.formState.getValue(0, "client")).toBe("ACME");
  });

  it("posts nothing and clears nothing when the confirmation is declined", async () => {
    // The harness answers every `confirm` with yes, so this one is driven
    // directly. A declined confirmation returns null and stops the arm — a user
    // changing their mind is not an error (`actions/interpret.ts`).
    const formState = new FormState();
    formState.setValue(0, "client", ["ACME"]);
    const posts: unknown[] = [];
    const actions = ActionDocumentSchema.parse(clientRegistryActionsDoc) as ActionDocument;
    const outcome = await runAction({
      action: actions.actions["deleteClientAction"]!,
      host: {
        validate: () => true,
        confirm: async () => false,
        post: async (r) => {
          posts.push(r);
          return { statusCode: 200 };
        },
        query: async () => null,
        read: async () => [],
        download: () => undefined,
        notify: () => undefined,
        setBusy: () => undefined,
        goToState: () => undefined,
        close: () => undefined,
        userEmail: () => "michel@artisoft.io",
        now: () => 0,
      },
      formState,
      field,
      registry: emptyRegistry,
      flowKey: "test",
    });
    expect(outcome).toBeNull();
    expect(posts).toEqual([]);
    expect(formState.getValue(0, "client")).toEqual(["ACME"]);
  });

  it("adds a vendor through the dialog form, and closes either way", async () => {
    const h = setup();
    h.formState.setValue(0, "client", "ACME");
    h.formState.setValue(0, "org", "ACME_EAST");
    h.formState.setValue(0, "ufVendorDetails", "east region");
    expect(await h.press("crAddVendorOk")).toBeNull();
    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["fromClauses"]).toEqual([{ table: "client_org_registry" }]);
    const row = (body["data"] as Record<string, unknown>[])[0]!;
    expect(row["details"]).toBe("east region");
    expect(row["org"]).toBe("ACME_EAST");
    // `postInsertRows` pops the dialog on success; `transport: "insertRows"`.
    expect(h.events).toContain("close");
    // The Dart posts a *copy* with `details` added, so the key never enters form
    // state. The grammar has no copy, so the arm puts it back.
    expect(h.formState.getValue(0, "details")).toBeUndefined();
  });

  it("its dialog form is named by a table, not by a state", () => {
    // The four forms `user_flows.json` calls unreferenced are the four a table
    // opens; `ufVendor` is this flow's (I-89). It is in the form document and no
    // state's `formConfig`.
    const stateForms = new Set(Object.values(setup().flow.states).map((s) => s.formConfig));
    expect(stateForms.has("ufVendor")).toBe(false);
    expect(Object.keys(setup().forms.forms)).toContain("ufVendor");
  });
});

/**
 * F.4's flow, and the one whose arms convert a value twice.
 *
 * **`unpackToList(unpack(x))` is a composition and the coverage document had it
 * as one call.** `spPipelineSelected` reads two `pipeline_config` columns that
 * hold Postgres `text[]` literals (`start_pipeline/form_action_delegates.dart`,
 * `startPipelineFormActionsUF`). A data table publishes a secondary column as a
 * *one-element list* — `resetSecondaryKeys` appends one string per selected row
 * (`components/data_table_source.dart`, `resetSecondaryKeys`) — so the state
 * holds `["{5,6}"]`, and `unpack` has to strip the selection before
 * `unpackToList` can decode the literal. `fromKeyList` alone is `unpackToList`
 * alone, which sees an array and returns it unchanged (I-97).
 */
describe("start_pipeline, end to end", () => {
  const setup = () => harness(startPipelineFlowDoc, startPipelineActionsDoc, startPipelineFormsDoc);

  /** What selecting a row of `pipeline_config_key` writes into form state. */
  const selectPipelineConfig = (
    h: ReturnType<typeof setup>,
    mergedProcessInputKeys: string,
  ) => {
    h.formState.setValue(0, "pipeline_config_key", ["12"]);
    h.formState.setValue(0, "client", ["ACME"]);
    h.formState.setValue(0, "process_name", ["claimsProcess"]);
    h.formState.setValue(0, "main_process_input_key", ["3"]);
    h.formState.setValue(0, "merged_process_input_keys", [mergedProcessInputKeys]);
    h.formState.setValue(0, "injected_process_input_keys", ["{9}"]);
    h.formState.setValue(0, "main_object_type", ["hc:Claim"]);
    h.formState.setValue(0, "main_source_type", ["file"]);
    h.formState.setValue(0, "description", ["Nightly claims run"]);
    h.formState.setValue(0, "main_process_input.table_name", ["ACME_D1_hcClaim"]);
  };

  const selectMainDataSource = (h: ReturnType<typeof setup>) => {
    h.formState.setValue(0, "main_input_registry_key", ["12"]);
    h.formState.setValue(0, "main_input_file_key", ["s3://in/claims.csv"]);
    h.formState.setValue(0, "source_period_key", ["101"]);
  };

  it("will not advance until a pipeline configuration is selected", async () => {
    const h = setup();
    expect(h.at()).toBe("select_pipeline_config");
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("select_pipeline_config");
  });

  it("decodes the array literal inside the selection, not the selection", async () => {
    const h = setup();
    selectPipelineConfig(h, "{5,6}");
    await h.press("ufNext");
    // Two keys the flow's own tables filter on: `mergeProcessInputTable` and
    // `spInjectedProcessInput` each read one as `key IN (…)`.
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual(["5", "6"]);
    expect(h.formState.getValue(0, "injected_process_input_keys")).toEqual(["9"]);
  });

  it("takes the merged branch only when the pipeline has merged process inputs", async () => {
    const withMerged = setup();
    selectPipelineConfig(withMerged, "{5,6}");
    await withMerged.press("ufNext");
    selectMainDataSource(withMerged);
    await withMerged.press("ufNext");
    expect(withMerged.at()).toBe("select_merged_data_sources");

    const without = setup();
    selectPipelineConfig(without, "{}");
    await without.press("ufNext");
    expect(without.formState.getValue(0, "merged_process_input_keys")).toEqual([]);
    selectMainDataSource(without);
    await without.press("ufNext");
    expect(without.at()).toBe("summaryUF");
  });

  it("would have taken the merged branch on every pipeline without the unpack", async () => {
    // **Measured rather than argued**, the way I-90 was. This is the coverage
    // document's `spPipelineSelected` — `fromKeyList` alone — run on the empty
    // case: the literal survives as a one-element list, the flow reads it as a
    // non-empty selection, and every pipeline is sent through a step for merged
    // sources it does not have.
    const flow = UserFlowSchema.parse(startPipelineFlowDoc) as UserFlow;
    const formState = new FormState();
    formState.setValue(0, "merged_process_input_keys", ["{}"]);
    const collapsed = ActionDocumentSchema.parse({
      schemaVersion: 1,
      actions: {
        spPipelineSelected: {
          description: "as the coverage document had it",
          steps: [
            { do: "set", key: "merged_process_input_keys", value: { fromKeyList: "merged_process_input_keys" } },
          ],
        },
      },
    }) as ActionDocument;
    await runAction({
      action: collapsed.actions["spPipelineSelected"]!,
      host: {
        validate: () => true,
        confirm: async () => true,
        post: async () => ({ statusCode: 200 }),
        query: async () => null,
        read: async () => [],
        download: () => undefined,
        notify: () => undefined,
        setBusy: () => undefined,
        goToState: () => undefined,
        close: () => undefined,
        userEmail: () => "michel@artisoft.io",
        now: () => 0,
      },
      formState,
      field,
      registry: emptyRegistry,
      flowKey: "test",
    });
    expect(formState.getValue(0, "merged_process_input_keys")).toEqual(["{}"]);
    const state = flow.states["select_main_data_source"]!;
    const choices = "choices" in state ? state.choices : undefined;
    expect(choices).toHaveLength(1);
    expect(evaluateCondition(choices![0]!.when, formState, 0)).toBe(true);
  });

  it("collects the merged sources before the main one", async () => {
    const h = setup();
    selectPipelineConfig(h, "{5,6}");
    await h.press("ufNext");
    selectMainDataSource(h);
    await h.press("ufNext");
    h.formState.setValue(0, "merged_input_registry_keys", ["31", "32"]);
    await h.press("ufNext");
    expect(h.at()).toBe("summaryUF");
    // `[merged, main].expand(...)` — the order `spSummaryDataSources` displays.
    expect(h.formState.getValue(0, "spAllDataSourceKeys")).toEqual(["31", "32", "12"]);
    // Unpacked so the read-only summary field shows a string.
    expect(h.formState.getValue(0, "description")).toBe("Nightly claims run");
  });

  it("submits the pipeline execution as the whole state, normalised", async () => {
    const h = setup();
    selectPipelineConfig(h, "{5,6}");
    await h.press("ufNext");
    selectMainDataSource(h);
    await h.press("ufNext");
    h.formState.setValue(0, "merged_input_registry_keys", ["31", "32"]);
    await h.press("ufNext");
    await h.press("ufCompleted");

    expect(h.posts).toHaveLength(1);
    const body = h.posts[0]!.body as Record<string, unknown>;
    expect(body["action"]).toBe("insert_rows");
    expect(body["fromClauses"]).toEqual([{ table: "pipeline_execution_status" }]);
    // **`state[FSK.wsName] ?? ''` becomes null here and the server cannot tell.**
    // `DataTableAction.WorkspaceName` is a Go `string`
    // (`jets/datatable/data_table_action.go`, `DataTableAction`), and
    // `encoding/json` unmarshals a JSON null into a string as the zero value —
    // so the two spellings arrive identically. Stated because the divergence is
    // real and the reason it does not matter is not obvious (I-98).
    expect(body["workspaceName"]).toBeNull();

    const row = (body["data"] as Record<string, unknown>[])[0]!;
    // `makePgArray`, outward: the one key the Dart brace-wraps by hand.
    expect(row["merged_input_registry_keys"]).toBe("{31,32}");
    expect(row["pipeline_config_key"]).toBe("12");
    expect(row["main_input_registry_key"]).toBe("12");
    expect(row["client"]).toBe("ACME");
    expect(row["process_name"]).toBe("claimsProcess");
    expect(row["source_period_key"]).toBe("101");
    expect(row["status"]).toBe("submitted");
    expect(row["user_email"]).toBe("michel@artisoft.io");
    expect(row["session_id"]).toBe("1700000000000");
    // The four keys the Dart copies rather than reads: `object_type` and
    // `file_key` are the main input's, written beside the originals.
    expect(row["main_object_type"]).toBe("hc:Claim");
    expect(row["object_type"]).toBe("hc:Claim");
    expect(row["main_input_file_key"]).toBe("s3://in/claims.csv");
    expect(row["file_key"]).toBe("s3://in/claims.csv");
    // `wholeState` carries the working keys too, exactly as `data: [state]` does.
    expect(row["spAllDataSourceKeys"]).toEqual(["31", "32", "12"]);
    expect(h.events).toContain("busy");
    expect(h.events).toContain("idle");
    expect(h.events).toContain("exit");
  });

  it("posts nothing when the summary form does not validate", async () => {
    // The harness answers `host.validate` with yes, so the `validate` step is
    // driven directly — the Dart returns null from a failed
    // `formKey.currentState!.validate()`, which is an abort and not an error.
    const posts: unknown[] = [];
    const actions = ActionDocumentSchema.parse(startPipelineActionsDoc) as ActionDocument;
    const outcome = await runAction({
      action: actions.actions["spStartPipelineUF"]!,
      host: {
        validate: () => false,
        confirm: async () => true,
        post: async (r) => {
          posts.push(r);
          return { statusCode: 200 };
        },
        query: async () => null,
        read: async () => [],
        download: () => undefined,
        notify: () => undefined,
        setBusy: () => undefined,
        goToState: () => undefined,
        close: () => undefined,
        userEmail: () => "michel@artisoft.io",
        now: () => 0,
      },
      formState: new FormState(),
      field,
      registry: emptyRegistry,
      flowKey: "test",
    });
    expect(outcome).toBeNull();
    expect(posts).toEqual([]);
  });

  it("requires the summary's two read-only identity fields", () => {
    // `startPipelineFormValidator` answers "Select an option" for `client` and
    // `process_name` and nothing for `description`
    // (`start_pipeline/form_action_delegates.dart`, `startPipelineFormValidator`).
    // Both are read-only and both are filled by the pipeline selection, so the
    // rule fires only when that selection never happened.
    const h = setup();
    expect(validateForm(h.formFor("summaryUF"), h.formState, 0).map((e) => e.key)).toEqual([
      "client",
      "process_name",
    ]);
  });

  it("ends on summaryUF, which offers no advancing button", () => {
    const h = setup();
    expect(h.flow.states["summaryUF"]!.isEnd).toBe(true);
    expect(h.formFor("summaryUF").actions.map((a) => a.action)).toEqual([
      "ufPrevious",
      "ufCancel",
      "ufCompleted",
    ]);
  });
});

/**
 * `homeFiltersUF`, end to end. Task F.5.
 *
 * **The first flow whose behaviour is partly an escape**, so the harness runs with
 * a registry rather than with `emptyRegistry` — which is the point of driving it
 * here rather than only asserting the document. What the escape produces is two
 * lists of `WhereClause` objects that a *different* screen's query builder reads,
 * and nothing about the flow says whether they are right.
 */
describe("home_filters, end to end", () => {
  const registry: EscapeRegistry = {
    actions: { updateHomeFilters },
    initializers: { seedFromHomeFilters },
    rowInitializers: {},
    validators: { homeFiltersFormValidator },
    cellFilters: {},
    predicates: {},
    queries: {},
  };
  const setup = () => {
    resetHomeFilters();
    return harness(homeFiltersFlowDoc, homeFiltersActionsDoc, homeFiltersFormsDoc, registry);
  };

  it("walks its five states and ends where the table is", async () => {
    const h = setup();
    expect(h.at()).toBe("select_process");
    for (const next of ["select_status", "select_file_key_filter", "select_time_window", "view_status_table"]) {
      // The file-key state requires a filter type, so it is chosen on the way
      // through — the same value a user would pick to reach the next screen.
      if (h.at() === "select_file_key_filter") {
        h.formState.setValue(0, "hfFileKeyFilterTypeTableUF", ["None"]);
      }
      expect(await h.press("ufNext")).toBeNull();
      expect(h.at()).toBe(next);
    }
    expect(h.flow.states["view_status_table"]!.isEnd).toBe(true);
    expect(h.formFor("view_status_table").actions.map((a) => a.action)).toEqual([
      "ufPrevious",
      "ufCancel",
      "ufCompleted",
    ]);
  });

  it("compiles the answers into the two filter lists, in the Dart's order", async () => {
    const h = setup();
    h.formState.setValue(0, "process_name", ["loadFile", "runRules"]);
    h.formState.setValue(0, "status", ["failed"]);
    h.formState.setValue(0, "hfFileKeyMatchType", ["starts_with"]);
    h.formState.setValue(0, "hfFileKeySubstring", ["client1/"]);
    h.formState.setValue(0, "hfStartOffset", ["3 days"]);
    h.formState.setValue(0, "hfEndTime", ["2026-08-24T12:00:00Z"]);
    expect(await h.press("hfSelectTimeWindowUF")).toBeNull();

    // **Order is asserted, not just membership.** The list is sent as an array and
    // becomes `WHERE` clauses in that order, so a reordering is a different query
    // text against a live table (I-90's shape, on a different document).
    expect(currentHomeFilters()).toEqual([
      { table: "pipeline_execution_status", column: "process_name", defaultValue: ["loadFile", "runRules"], lookupColumnInFormState: false },
      { table: "pipeline_execution_status", column: "status", defaultValue: ["failed"], lookupColumnInFormState: false },
      { table: "pipeline_execution_status", column: "main_input_file_key", like: "client1/%", defaultValue: [], lookupColumnInFormState: false },
      { table: "pipeline_execution_status", column: "start_time", ge: "now()-interval '3 days'", defaultValue: [], lookupColumnInFormState: false },
      { table: "pipeline_execution_status", column: "start_time", le: "timestamp '2026-08-24T12:00:00Z'", defaultValue: [], lookupColumnInFormState: false },
    ]);
    expect(currentDataRegistryFilters()!.map((w) => [w.column, w.like ?? w.ge ?? w.le])).toEqual([
      ["file_key", "client1/%"],
      ["last_update", "now()-interval '3 days'"],
      ["last_update", "timestamp '2026-08-24T12:00:00Z'"],
    ]);
  });

  it("treats the None filter type as no file-key clause at all", async () => {
    const h = setup();
    // `None`'s value column is the empty string, so `fkMatchType` is `""` and the
    // Dart's switch falls to `default:` — no clause on either list.
    h.formState.setValue(0, "hfFileKeyMatchType", [""]);
    h.formState.setValue(0, "hfFileKeySubstring", ["ignored"]);
    await h.press("hfSelectFileKeyFilterUF");
    expect(currentHomeFilters()).toEqual([]);
    expect(currentDataRegistryFilters()).toEqual([]);
  });

  it("requires a file-key fragment only once a filter type other than None is chosen", () => {
    const h = setup();
    const form = h.formFor("select_file_key_filter");
    const context = { formState: h.formState, group: 0, flowKey: "homeFiltersUF" };

    // Nothing chosen: the table's own `required` rule fires and the substring's
    // cross-field rule does not.
    expect(validateForm(form, h.formState, 0).map((e) => e.key)).toEqual(["hfFileKeyFilterTypeTableUF"]);
    expect(homeFiltersFormValidator(context, "hfFileKeySubstring", null)).toBeNull();

    h.formState.setValue(0, "hfFileKeyFilterTypeTableUF", ["None"]);
    expect(homeFiltersFormValidator(context, "hfFileKeySubstring", null)).toBeNull();

    h.formState.setValue(0, "hfFileKeyFilterTypeTableUF", ["Contains"]);
    expect(homeFiltersFormValidator(context, "hfFileKeySubstring", null)).toBe(
      "Enter a file key fragment",
    );
    expect(homeFiltersFormValidator(context, "hfFileKeySubstring", ["abc"])).toBeNull();
  });

  it("saves the answers and seeds the next run from them", async () => {
    const first = setup();
    first.formState.setValue(0, "hfProcessTableUF", ["loadFile"]);
    first.formState.setValue(0, "process_name", ["loadFile"]);
    first.formState.setValue(0, "hfStartOffset", ["12 hours"]);
    await first.press("hfSelectProcessUF");
    expect(savedHomeFilterState()).toEqual({
      hfProcessTableUF: ["loadFile"],
      process_name: ["loadFile"],
      hfStartOffset: ["12 hours"],
    });

    // A second run of the flow, with a fresh form state — the router outlives the
    // screen and the form state does not, which is the whole reason
    // `formStateInitializer` exists and why `homeFiltersUF` is its only user.
    const second = harness(homeFiltersFlowDoc, homeFiltersActionsDoc, homeFiltersFormsDoc, registry);
    seedFromHomeFilters({ formState: second.formState, group: 0, flowKey: "homeFiltersUF" });
    expect(second.formState.getValue(0, "hfProcessTableUF")).toEqual(["loadFile"]);
    expect(second.formState.getValue(0, "hfStartOffset")).toEqual(["12 hours"]);
  });

  it("unpacks session_id before posting the resubmit, or the server answers 400", async () => {
    const h = setup();
    // What the table publishes: `session_id` is column 10 of the
    // `formStateBinding`, so a selection is a one-element list.
    h.formState.setValue(0, "session_id", ["sess-1"]);
    expect(await h.press("resubmitPipeline")).toBeNull();
    expect(h.posts).toHaveLength(1);
    expect(h.posts[0]!.body).toEqual({
      action: "resubmit_pipeline",
      data: [{ session_id: "sess-1" }],
    });
  });

  it("replaces the filters wholesale when a session id list is entered", () => {
    resetHomeFilters();
    setIdFilter("session_id", "s1, s2 ,s3");
    expect(currentHomeFilters()).toEqual([
      { table: "pipeline_execution_status", column: "session_id", defaultValue: ["s1", "s2", "s3"], lookupColumnInFormState: false },
    ]);
    // The data-registry half joins rather than filtering, and the join column
    // differs between the two prompts — `input_session_id` here.
    expect(currentDataRegistryFilters()!.map((w) => w.joinWith ?? w.defaultValue)).toEqual([
      "pipeline_execution_status.input_session_id",
      ["s1", "s2", "s3"],
    ]);

    setIdFilter("request_id", "r1");
    expect(currentDataRegistryFilters()![0]!.joinWith).toBe(
      "pipeline_execution_status.input_request_id",
    );
  });
});

/**
 * `pipelineConfigUF`, end to end. Task F.6.
 *
 * **The largest flow in the corpus and the one the plan kept until now** — ten
 * states, twelve forms, fifteen arms, ten tables. What is asserted here is what
 * the coverage document got wrong rather than what it got right: a branch it did
 * not have (I-115), an initialisation it read off its own left-hand side (I-116),
 * a query whose columns it renamed, and a concatenation that would have kept one
 * element of each list.
 *
 * **The `query` step runs for the first time.** Every earlier flow's harness
 * answered `null`, which the interpreter reads as *no rows*; this one supplies
 * the row `getProcessInputRdfTypes` returns.
 */
describe("pipeline_config, end to end", () => {
  const processConfigRow = { key: "pc-42", input_rdf_types: "rdf:Claim" };
  const setup = () =>
    harness(
      pipelineConfigFlowDoc,
      pipelineConfigActionsDoc,
      pipelineConfigFormsDoc,
      emptyRegistry,
      async () => processConfigRow,
    );

  /** Everything `pcAddPipelineConfigUF`'s form needs to pass its own rules. */
  const fillAddForm = (h: ReturnType<typeof setup>) => {
    h.formState.setValue(0, "client", ["acme"]);
    h.formState.setValue(0, "process_name", ["loadFile"]);
  };

  it("takes the add branch and walks the seven states of it", async () => {
    const h = setup();
    expect(h.at()).toBe("select_add_or_edit");
    h.formState.setValue(0, "pcAddOrEditPipelineConfigOption", ["ufAddOption"]);
    await h.press("ufNext");
    expect(h.at()).toBe("add_pipeline_config");

    fillAddForm(h);
    await h.press("ufNext");
    expect(h.at()).toBe("select_main_process_input");

    h.formState.setValue(0, "pcMainProcessInputKey", ["pi-1"]);
    await h.press("ufNext");
    expect(h.at()).toBe("view_merge_process_inputs");
    await h.press("ufNext");
    expect(h.at()).toBe("view_injected_process_inputs");
    await h.press("ufNext");
    expect(h.at()).toBe("set_pipeline_automation");

    h.formState.setValue(0, "source_period_type", ["month_period"]);
    h.formState.setValue(0, "automated", ["1"]);
    h.formState.setValue(0, "rule_config_json", ["[]"]);
    await h.press("ufNext");
    expect(h.at()).toBe("summaryUF");
  });

  it("takes the edit branch on the other option", async () => {
    const h = setup();
    h.formState.setValue(0, "pcAddOrEditPipelineConfigOption", ["ufEditOption"]);
    await h.press("ufNext");
    expect(h.at()).toBe("select_pipeline_config");
  });

  it("initialises the two key lists to empty and reads the query by column name", async () => {
    // I-116 and the `into` correction together, because they are the two halves
    // of one arm. Both lists must be *present and empty*, because the arms that
    // grow them are the flow's whole middle.
    const h = setup();
    h.formState.setValue(0, "pcAddOrEditPipelineConfigOption", ["ufAddOption"]);
    await h.press("ufNext");
    fillAddForm(h);
    await h.press("ufNext");

    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual([]);
    expect(h.formState.getValue(0, "injected_process_input_keys")).toEqual([]);
    expect(h.formState.getValue(0, "max_rete_sessions_saved")).toBe("0");
    // `key` and `input_rdf_types::text`, not `process_config_key` and
    // `entity_rdf_type` — the columns the statement selects.
    expect(h.formState.getValue(0, "process_config_key")).toBe("pc-42");
    expect(h.formState.getValue(0, "entity_rdf_type")).toBe("rdf:Claim");
  });

  it("stops the add arm when the process name is missing, after the initialisation", async () => {
    // The Dart's null check is *below* the three assignments, so a flow that
    // fails here still has its lists. Step order, which is I-90's subject.
    const h = setup();
    expect(await h.press("pcAddPipelineConfigUF")).toBe(
      "Error: null process_name in formState",
    );
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual([]);
  });

  it("unpacks the selection before decoding the array literal inside it", async () => {
    // I-97, inherited from F.4 with nothing to build: `merged_process_input_keys`
    // is `int[] NOT NULL DEFAULT '{}'`, every column of a data-table read is
    // scanned into a `sql.NullString`, and the table publishes a secondary column
    // as a one-element list. So form state holds `["{5,6}"]`.
    const h = setup();
    h.formState.setValue(0, "merged_process_input_keys", ["{5,6}"]);
    h.formState.setValue(0, "injected_process_input_keys", ["{}"]);
    h.formState.setValue(0, "main_process_input_key", ["pi-1"]);
    await h.press("pcSelectPipelineConfigUF");
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual(["5", "6"]);
    expect(h.formState.getValue(0, "injected_process_input_keys")).toEqual([]);
    // The last step re-publishes the main key so the table shows it selected.
    expect(h.formState.getValue(0, "pcMainProcessInputKey")).toBe("pi-1");
  });

  it("jumps to the add page and comes back, which is the edge that caused I-18", async () => {
    const h = setup();
    h.formState.setValue(0, "pcAddOrEditPipelineConfigOption", ["ufAddOption"]);
    await h.press("ufNext");
    fillAddForm(h);
    await h.press("ufNext");
    h.formState.setValue(0, "pcMainProcessInputKey", ["pi-1"]);
    await h.press("ufNext");
    expect(h.at()).toBe("view_merge_process_inputs");

    // The button is *inside the rows* rather than in the action bar (F.2's
    // `button` field), and pressing it runs an arm whose first step is
    // `goToState`. The harness runs the arm; the screen applies the jump, which
    // is why the event is what is asserted here.
    h.formState.setValue(0, "pcMergedProcessInputKeys", ["pi-7"]);
    expect(await h.press("pcGotToAddMergeProcessInputUF")).toBeNull();
    expect(h.events).toContain("goToState:add_merge_process_inputs");
    // And it clears the add page's own selection on the way in, so a second visit
    // does not open with the previous choice ticked.
    expect(h.formState.getValue(0, "pcMergedProcessInputKeys")).toBeUndefined();

    h.formState.setValue(0, "pcMergedProcessInputKeys", ["pi-7"]);
    await h.press("pcAddMergeProcessInputUF");
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual(["pi-7"]);
    // Twice does not duplicate, which is what `appendUnique` is for.
    h.formState.setValue(0, "pcMergedProcessInputKeys", ["pi-7"]);
    await h.press("pcAddMergeProcessInputUF");
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual(["pi-7"]);

    h.formState.setValue(0, "pcViewMergedProcessInputKeys", ["pi-7"]);
    await h.press("pcRemoveMergedProcessInput");
    expect(h.formState.getValue(0, "merged_process_input_keys")).toEqual([]);
  });

  it("collects every process input key rather than the first of each list", async () => {
    // **The `appendUnique` widening, measured.** The Dart concatenates
    // `[injected, merged, main].expand((x) => x)`; before F.6 an `appendUnique`
    // over a list took element zero, so this would have been
    // `["inj-1", "mrg-1", "pi-1"]` and the summary table would have shown three
    // rows where the configuration has five.
    const h = setup();
    h.formState.setValue(0, "injected_process_input_keys", ["inj-1", "inj-2"]);
    h.formState.setValue(0, "merged_process_input_keys", ["mrg-1", "mrg-2"]);
    h.formState.setValue(0, "main_process_input_key", "pi-1");
    expect(await h.press("pcPrepareSummaryUF")).toBeNull();
    expect(h.formState.getValue(0, "ufAllProcessInputKeys")).toEqual([
      "inj-1",
      "inj-2",
      "mrg-1",
      "mrg-2",
      "pi-1",
    ]);
  });

  it("inserts when nothing is selected and updates when a row is", async () => {
    // **I-115.** The coverage document had one unguarded insert, so the flow's
    // *Edit an Existing Pipeline Configuration* branch would have added a second
    // row rather than changing the one the user picked.
    const fill = (h: ReturnType<typeof setup>) => {
      h.formState.setValue(0, "process_name", "loadFile");
      h.formState.setValue(0, "process_config_key", "pc-42");
      h.formState.setValue(0, "client", "acme");
      h.formState.setValue(0, "max_rete_sessions_saved", "0");
      h.formState.setValue(0, "rule_config_json", "[]");
      h.formState.setValue(0, "source_period_type", "month_period");
      h.formState.setValue(0, "main_process_input_key", "pi-1");
      h.formState.setValue(0, "main_object_type", "Claim");
      h.formState.setValue(0, "main_source_type", "file");
      h.formState.setValue(0, "automated", "1");
      h.formState.setValue(0, "description", "nightly");
      h.formState.setValue(0, "merged_process_input_keys", ["5", "6"]);
      h.formState.setValue(0, "injected_process_input_keys", []);
    };

    const add = setup();
    fill(add);
    expect(await add.press("pcSavePipelineConfigUF")).toBeNull();
    expect(add.posts).toHaveLength(1);
    expect(add.posts[0]!.body["fromClauses"]).toEqual([{ table: "pipeline_config" }]);
    const inserted = (add.posts[0]!.body["data"] as Record<string, unknown>[])[0]!;
    expect(inserted["key"]).toBeUndefined();
    // `makePgArray`, outwards: the column is `int[]` and the wire form is the
    // Postgres literal, not a JSON array.
    expect(inserted["merged_process_input_keys"]).toBe("{5,6}");
    expect(inserted["injected_process_input_keys"]).toBe("{}");
    expect(inserted["user_email"]).toBe("michel@artisoft.io");

    const edit = setup();
    fill(edit);
    edit.formState.setValue(0, "pcPipelineConfigTable", ["cfg-9"]);
    expect(await edit.press("pcSavePipelineConfigUF")).toBeNull();
    expect(edit.posts).toHaveLength(1);
    expect(edit.posts[0]!.body["fromClauses"]).toEqual([{ table: "update/pipeline_config" }]);
    expect((edit.posts[0]!.body["data"] as Record<string, unknown>[])[0]!["key"]).toBe("cfg-9");
  });

  it("deletes the selected configuration and clears the table behind it", async () => {
    const h = setup();
    h.formState.setValue(0, "key", ["cfg-9"]);
    h.formState.setValue(0, "pcPipelineConfigTable", ["cfg-9"]);
    expect(await h.press("deletePipelineConfig")).toBeNull();
    expect(h.posts).toHaveLength(1);
    expect(h.posts[0]!.body["fromClauses"]).toEqual([{ table: "delete/pipeline_config" }]);
    const row = (h.posts[0]!.body["data"] as Record<string, unknown>[])[0]!;
    expect(row["key"]).toBe("cfg-9");
    // **The Dart encodes the payload before clearing the selection and this
    // clears first, so `pcPipelineConfigTable` is absent here and present
    // there.** `delete/pipeline_config` declares `ColumnKeys: []string{"key"}`
    // (`jets/datatable/sql_stmts.go`), so the server reads one column and the
    // difference cannot be observed. The other order would be observable: the
    // Dart clears the selection whether or not the delete succeeds.
    expect(row["pcPipelineConfigTable"]).toBeUndefined();
  });

  it("saves a process input from the dialog, adding or updating on the same test the Dart uses", async () => {
    // `addProcessInputOk` is not one of the 58 (I-114): it lives in
    // `modules/actions/config_delegates.dart` because the dialogs carry their own
    // delegate. Its add/update test is on `key` *unpacked or not* — the Dart reads
    // `formState.getValue(0, FSK.key) != null` rather than `unpack(...)`, which is
    // why the guard here is `isNull` where `pcSavePipelineConfigUF`'s is
    // `isNullOrEmpty`.
    const add = setup();
    add.formState.setValue(0, "client", ["acme"]);
    add.formState.setValue(0, "org", ["north"]);
    add.formState.setValue(0, "source_type", ["file"]);
    add.formState.setValue(0, "object_type", ["Claim"]);
    add.formState.setValue(0, "table_name", ["claims"]);
    add.formState.setValue(0, "lookback_periods", "0");
    expect(await add.press("addProcessInputOk")).toBeNull();
    expect(add.posts[0]!.body["fromClauses"]).toEqual([{ table: "process_input" }]);
    expect((add.posts[0]!.body["data"] as Record<string, unknown>[])[0]!["org"]).toBe("north");

    const update = setup();
    update.formState.setValue(0, "key", "pi-3");
    update.formState.setValue(0, "source_type", ["domain_table"]);
    expect(await update.press("addProcessInputOk")).toBeNull();
    expect(update.posts[0]!.body["fromClauses"]).toEqual([{ table: "update2/process_input" }]);
    // An org belongs to a file or a database table and to nothing else, so the
    // Dart blanks it for every other source type.
    expect((update.posts[0]!.body["data"] as Record<string, unknown>[])[0]!["org"]).toBe("");
  });

  it("refuses to save a process input with no source type", async () => {
    const h = setup();
    h.formState.setValue(0, "client", ["acme"]);
    expect(await h.press("addProcessInputOk")).toBe(
      "Cannot save this data source: its source type is not set.",
    );
    expect(h.posts).toHaveLength(0);
  });

  it("derives the registry key from four values, and clears it when one is missing", async () => {
    const h = setup();
    h.formState.setValue(0, "process_name", "loadFile");
    h.formState.setValue(0, "object_type", "Claim");
    h.formState.setValue(0, "table_name", "claims");
    h.formState.setValue(0, "source_type", "file");
    await h.press("pcSetProcessInputRegistryKey");
    expect(h.formState.getValue(0, "pcProcessInputRegistry")).toBe("loadFileClaimclaimsfile");
    expect(h.formState.getValue(0, "pcProcessInputRegistry4MI")).toBe("loadFileClaimclaimsfile");

    // The Dart removes both keys and returns rather than concatenating nulls into
    // a key that would match no row. A `template` with a missing key substitutes
    // the empty string, so the guard is what stops a plausible-looking wrong key.
    const partial = setup();
    partial.formState.setValue(0, "process_name", "loadFile");
    await partial.press("pcSetProcessInputRegistryKey");
    expect(partial.formState.getValue(0, "pcProcessInputRegistry")).toBeUndefined();
    expect(partial.formState.getValue(0, "pcProcessInputRegistry4MI")).toBeUndefined();
  });

  it("ends on a form that cannot advance, and carries the two options I-62 left", () => {
    const summary = pipelineConfigFormsDoc.forms.pcSummaryUF;
    expect(summary.actions.map((a) => a.action)).toEqual([
      "ufPrevious",
      "ufCancel",
      "ufCompleted",
    ]);
    // I-62's second half, built here because this flow is its only consumer: four
    // `defaultValue` sites in the 50-form corpus and two `digitsOnly`, all six on
    // these twelve forms.
    const ruleConfig = summary.rows.flat().find((f) => "key" in f && f.key === "rule_config_json");
    expect(ruleConfig).toMatchObject({ defaultValue: "[]", isReadOnly: true });
    const lookback = pipelineConfigFormsDoc.forms.pcNewProcessInputDialog.rows
      .flat()
      .find((f) => "key" in f && f.key === "lookback_periods");
    expect(lookback).toMatchObject({ defaultValue: "0", textRestriction: "digitsOnly" });
  });
});

/**
 * `fileMappingUF`, the flow above the worksheet. Task F.8.
 *
 * **Two states and four arms, and three of the four are reached from the table
 * rather than from a state** — which is the shape I-89 and I-101 predicted and
 * this is the seventh flow to meet. `fmFileMappingTableUF` carries all three
 * kinds the dispatcher reads: a `showScreen` into `mapFileUF`, a `showDialog`
 * that opens `loadRawRowsDialog`, and a `doAction` naming `downloadMapping`.
 *
 * **The two escapes are driven here rather than only resolved**, for the reason
 * F.5's block gives: what they produce — a CSV the browser saves, and a payload
 * the server parses — is not visible from the document, and the document is the
 * only thing the set checks look at.
 */
describe("file_mapping, end to end", () => {
  const registry: EscapeRegistry = {
    ...emptyRegistry,
    actions: { downloadMapping, loadRawRows },
  };
  const setup = (read?: ActionHost["read"]) =>
    harness(
      fileMappingFlowDoc,
      fileMappingActionsDoc,
      fileMappingFormsDoc,
      registry,
      undefined,
      read,
    );

  /** What a selected row of `fmInputSourceMappingUF` publishes. */
  const selectSource = (h: ReturnType<typeof setup>) => {
    h.formState.setValue(0, "fmInputSourceMappingUF", ["17"]);
    h.formState.setValue(0, "client", ["ACME"]);
    h.formState.setValue(0, "org", ["EAST"]);
    h.formState.setValue(0, "object_type", ["claim"]);
    h.formState.setValue(0, "table_name", ["acme_east_claim"]);
  };

  it("will not advance until a file configuration is selected", async () => {
    const h = setup();
    expect(h.at()).toBe("select_source_config");
    expect(validateForm(h.formFor("select_source_config"), h.formState, 0).map((e) => e.message)).toEqual([
      "A file configuration must be selected.",
    ]);
    // `stay()` with no message, as `load_config` above: the engine leaves the
    // errors to the caller's list rather than putting a banner over them.
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("select_source_config");
  });

  it("unpacks all four of the selection's columns on the way through", async () => {
    const h = setup();
    selectSource(h);
    expect(await h.press("ufNext")).toBeNull();
    expect(h.at()).toBe("file_mapping");
    // Each arrived as a one-element list from the table's `formStateBinding` and
    // is a scalar now. `table_name` is the one that matters most: the second
    // state's table filters `process_mapping` on it, and a `["x"]` there would
    // match nothing (`fmFileMappingTableUF.tc.json`, its `where`).
    for (const [key, value] of [
      ["client", "ACME"],
      ["org", "EAST"],
      ["object_type", "claim"],
      ["table_name", "acme_east_claim"],
    ]) {
      expect(h.formState.getValue(0, key!)).toBe(value);
    }
  });

  it("ends on file_mapping, which offers no advancing button", () => {
    const h = setup();
    expect(h.flow.states["file_mapping"]!.isEnd).toBe(true);
    expect(h.formFor("file_mapping").actions.map((a) => a.action)).toEqual([
      "ufPrevious",
      "ufCompleted",
    ]);
  });

  it("downloads the mapping as a csv, quoting every cell and skipping nulls", async () => {
    const rows = [
      ["ACME", "EAST", "claim", "hc:dob", "DOB", "to_date", null, null, null],
      ["ACME", "EAST", "claim", "hc:id", "MEMBER_ID", null, null, "0", "id required"],
    ];
    const reads: unknown[] = [];
    const h = setup(async (request) => {
      reads.push(request);
      return rows;
    });
    selectSource(h);

    expect(await h.press("downloadMapping")).toBeNull();

    // **The three `unpack`s are what this asserts.** The where clauses carry
    // scalars; a selection published as `["ACME"]` would send `values: [["ACME"]]`
    // and match no row, with a 200 and an empty file.
    const body = (reads[0] as { body: Record<string, unknown> }).body;
    expect(body["whereClauses"]).toEqual([
      { table: "source_config", column: "client", values: ["ACME"] },
      { table: "source_config", column: "org", values: ["EAST"] },
      { table: "source_config", column: "object_type", values: ["claim"] },
      { table: "source_config", column: "table_name", joinWith: "process_mapping.table_name" },
    ]);

    expect(h.downloads).toHaveLength(1);
    expect(h.downloads[0]!.fileName).toBe("mapping.csv");
    expect(h.downloads[0]!.content.split("\n")).toEqual([
      '"client","org","object_type","data_property","input_column","function_name","argument","default_value","error_message"',
      '"ACME","EAST","claim","hc:dob","DOB","to_date",,,',
      '"ACME","EAST","claim","hc:id","MEMBER_ID",,,"0","id required"',
      "",
    ]);
  });

  it("says the read failed rather than saving an empty file", async () => {
    const h = setup(async () => null);
    selectSource(h);
    // Null rather than the message, which is the Dart's: `downloadMapping`
    // returns null from every branch and shows a snackbar
    // (`file_mapping/form_action_helpers.dart`, `downloadMapping`).
    expect(await h.press("downloadMapping")).toBeNull();
    expect(h.events).toContain("notify:error:Unknown Error reading data from table");
    expect(h.downloads).toEqual([]);
  });

  it("posts the pasted text as one row and closes the dialog", async () => {
    const h = setup();
    h.formState.setValue(0, "raw_rows", "client,org\nACME,EAST\n");
    h.formState.setValue(0, "table_name", "acme_east_claim");

    expect(await h.press("loadRawRows.Ok")).toBeNull();
    expect(h.posts).toHaveLength(1);
    expect(h.posts[0]!.body["action"]).toBe("insert_raw_rows");
    expect(h.posts[0]!.body["fromClauses"]).toEqual([{ table: "raw_rows/process_mapping" }]);
    // The whole of form state as one row, `user_email` written in first — which
    // is what `InsertRawRows` reads before it parses anything
    // (`jets/datatable/data_table_action.go`, `InsertRawRows`).
    const data = h.posts[0]!.body["data"] as Record<string, unknown>[];
    expect(data).toHaveLength(1);
    expect(data[0]!["user_email"]).toBe("michel@artisoft.io");
    expect(data[0]!["raw_rows"]).toBe("client,org\nACME,EAST\n");
    expect(h.events).toContain("close");
  });

  it("records the server's message under serverError and closes anyway", async () => {
    const h = setup();
    // `postInsertRows` pops on every branch, which is why the dialog goes away
    // and the message is left in form state for the screen behind it
    // (`modules/actions/delegate_helpers.dart`, `postInsertRows`).
    const actions = ActionDocumentSchema.parse(fileMappingActionsDoc) as ActionDocument;
    const outcome = await runAction({
      action: actions.actions["loadRawRows.Ok"]!,
      host: {
        validate: () => true,
        confirm: async () => true,
        post: async () => ({ statusCode: 409 }),
        read: async () => [],
        download: () => undefined,
        query: async () => null,
        notify: () => undefined,
        setBusy: () => undefined,
        goToState: () => undefined,
        close: () => undefined,
        userEmail: () => "michel@artisoft.io",
        now: () => 0,
      },
      formState: h.formState,
      field,
      registry,
      flowKey: "fileMappingUF",
    });
    expect(outcome).toBe("Duplicate record. Please verify.");
    expect(h.formState.getValue(0, "serverError")).toBe("Duplicate record. Please verify.");
  });

  it("its dialog form is named by a table, not by a state", () => {
    const h = setup();
    // I-89's rule, and the last of the four forms the flow corpus calls
    // *unreferenced*. Two states, three forms; `loadRawRowsDialog` is
    // `fmFileMappingTableUF`'s `loadRawRows` `configForm`.
    const named = Object.values(h.flow.states).map((state) => state.formConfig);
    expect(named.sort()).toEqual(["fmFileMappingUF", "fmSelectSourceConfigUF"]);
    expect(Object.keys(h.forms.forms)).toHaveLength(3);
    expect(h.forms.forms["loadRawRowsDialog"]!.actions.map((a) => a.action)).toEqual([
      "loadRawRows.Ok",
      "dialogCancel",
    ]);
  });

  it("closes on cancel and posts nothing", async () => {
    const h = setup();
    expect(await h.press("dialogCancel")).toBeNull();
    expect(h.posts).toEqual([]);
    expect(h.events).toEqual(["close"]);
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

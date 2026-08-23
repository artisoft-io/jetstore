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

import { emptyRegistry } from "../actions/escapes";
import { runAction, type ActionHost } from "../actions/interpret";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import clientRegistryActionsDoc from "../actions/flows/clientRegistryUF.ua.json";
import loadConfigActionsDoc from "../actions/flows/loadConfigUF.ua.json";
import loadFilesActionsDoc from "../actions/flows/loadFilesUF.ua.json";
import registerFileKeyActionsDoc from "../actions/flows/registerFileKeyUF.ua.json";
import workspacePullActionsDoc from "../actions/flows/workspacePullUF.ua.json";
import { FormState } from "../datatable/formState";
import { FormDocumentSchema, type FormDocument } from "./form";
import clientRegistryFormsDoc from "./forms/clientRegistryUF.form.json";
import loadConfigFormsDoc from "./forms/loadConfigUF.form.json";
import loadFilesFormsDoc from "./forms/loadFilesUF.form.json";
import registerFileKeyFormsDoc from "./forms/registerFileKeyUF.form.json";
import workspacePullFormsDoc from "./forms/workspacePullUF.form.json";
import { advance, back, evaluateCondition, isStandardAction, startAt, step, FlowError } from "./engine";
import clientRegistryFlowDoc from "./flows/clientRegistryUF.uf.json";
import loadConfigFlowDoc from "./flows/loadConfigUF.uf.json";
import loadFilesFlowDoc from "./flows/loadFilesUF.uf.json";
import registerFileKeyFlowDoc from "./flows/registerFileKeyUF.uf.json";
import workspacePullFlowDoc from "./flows/workspacePullUF.uf.json";
import { validateDocumentSet } from "./documentSet";
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

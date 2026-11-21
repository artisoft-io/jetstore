/**
 * Tests for the flow store.
 *
 * Driven against a stub `WorkspaceApi` rather than a live server: what is new
 * is the *store*, and the transport underneath it is the one the editor shipped
 * with, verified against `localhost:8080`. What is worth asserting here is the
 * part that is new — that a flow which does not validate does not load, and that
 * the reason it did not load is usable.
 */

import { describe, expect, it, vi } from "vitest";

import { emptyRegistry, type EscapeRegistry } from "../actions/escapes";
import { productionRegistry } from "../actions/registry";
import loadFilesActions from "../../../jets/workspace_assets/user_flows/loadFilesUF.ua.json";
import homeFiltersActions from "../../../jets/workspace_assets/user_flows/homeFiltersUF.ua.json";
import pipelineConfigActions from "../../../jets/workspace_assets/user_flows/pipelineConfigUF.ua.json";
import fileMappingActions from "../../../jets/workspace_assets/user_flows/fileMappingUF.ua.json";
import sourceConfigActions from "../../../jets/workspace_assets/user_flows/sourceConfigUF.ua.json";
import type { SaveFinding, SaveResult, WorkspaceApi } from "../api/workspace";
import loadFilesFlow from "../../../jets/workspace_assets/user_flows/loadFilesUF.uf.json";
import homeFiltersFlow from "../../../jets/workspace_assets/user_flows/homeFiltersUF.uf.json";
import pipelineConfigFlow from "../../../jets/workspace_assets/user_flows/pipelineConfigUF.uf.json";
import fileMappingFlow from "../../../jets/workspace_assets/user_flows/fileMappingUF.uf.json";
import sourceConfigFlow from "../../../jets/workspace_assets/user_flows/sourceConfigUF.uf.json";
import lfFileKeyStagingTable from "../../../jets/workspace_assets/table_configs/lfFileKeyStagingTable.tc.json";
import lfSourceConfigTable from "../../../jets/workspace_assets/table_configs/lfSourceConfigTable.tc.json";
import inputRegistryTable from "../datatable/tables/inputRegistryTable.tc.json";
import { tablePath } from "../datatable/table";
import homeFiltersForms from "../../../jets/workspace_assets/user_flows/homeFiltersUF.form.json";
import pipelineConfigForms from "../../../jets/workspace_assets/user_flows/pipelineConfigUF.form.json";
import fileMappingForms from "../../../jets/workspace_assets/user_flows/fileMappingUF.form.json";
import sourceConfigForms from "../../../jets/workspace_assets/user_flows/sourceConfigUF.form.json";
import pcPipelineConfigTable from "../../../jets/workspace_assets/table_configs/pcPipelineConfigTable.tc.json";
import pcMainProcessInputKey from "../../../jets/workspace_assets/table_configs/pcMainProcessInputKey.tc.json";
import pcViewMerged from "../../../jets/workspace_assets/table_configs/pcViewMergedProcessInputKeys.tc.json";
import pcViewInjected from "../../../jets/workspace_assets/table_configs/pcViewInjectedProcessInputKeys.tc.json";
import pcSummaryProcessInputs from "../../../jets/workspace_assets/table_configs/pcSummaryProcessInputs.tc.json";
import pcMerged from "../../../jets/workspace_assets/table_configs/pcMergedProcessInputKeys.tc.json";
import pcInjected from "../../../jets/workspace_assets/table_configs/pcInjectedProcessInputKeys.tc.json";
import pcProcessInputRegistry from "../../../jets/workspace_assets/table_configs/pcProcessInputRegistry.tc.json";
import pcProcessInputRegistry4MI from "../../../jets/workspace_assets/table_configs/pcProcessInputRegistry4MI.tc.json";
import fmInputSourceMapping from "../../../jets/workspace_assets/table_configs/fmInputSourceMappingUF.tc.json";
import fmFileMappingTable from "../../../jets/workspace_assets/table_configs/fmFileMappingTableUF.tc.json";
import scSourceConfigKey from "../../../jets/workspace_assets/table_configs/scSourceConfigKey.tc.json";
import scSingleOrMultiPart from "../../../jets/workspace_assets/table_configs/scSingleOrMultiPartFileOption.tc.json";
import inputFormatTable from "../../../jets/workspace_assets/table_configs/input_format.tc.json";
import hfProcessTable from "../../../jets/workspace_assets/table_configs/hfProcessTableUF.tc.json";
import hfStatusTable from "../../../jets/workspace_assets/table_configs/hfStatusTableUF.tc.json";
import hfFileKeyFilterTypeTable from "../../../jets/workspace_assets/table_configs/hfFileKeyFilterTypeTableUF.tc.json";
import execStatusTable from "../../../jets/workspace_assets/table_configs/pipelineExecStatusTable.tc.json";
import loadFilesForms from "../../../jets/workspace_assets/user_flows/loadFilesUF.form.json";
import {
  FLOW_DIR,
  FlowLoadError,
  FlowStore,
  RELOAD_HINT,
  actionPath,
  describeFlowSaveWarnings,
  escapeReferences,
  flowPath,
  formPath,
  isUnknownEnumeratedValue,
  serialise,
  tableConfigOf,
  tableKeysOf,
} from "./store";
import { strictPolicy } from "./validate";

/**
 * A workspace as a map from path to text.
 *
 * **The stub answers `readWorkspaceDocument`, and that change is the
 * point — with the method renamed on 2026-08-26 when the runtime read moved to
 * its own, lower, capability.** It used to answer `readFile(ws, node)` and key off `node.key`, which
 * the real `WorkspaceApi` never does: `readFile` takes a tree node and asks
 * `fileNameOf` for the path, and that returns null unless `node.type === "file"`.
 * The store was synthesising `{ label, key } as never`, so the stub accepted a
 * shape the implementation would have refused. Stubbing the method the store
 * actually calls is what keeps the two honest.
 */
function stubApi(files: Record<string, string>, warnings: Record<string, SaveFinding[]> = {}) {
  const saved: Record<string, string> = {};
  const api = {
    fileTree: vi.fn(async () =>
      Object.keys(files).map((path) => ({ label: path.split("/").pop(), key: path })),
    ),
    readWorkspaceDocument: vi.fn(async (_ws: string, path: string) => {
      const content = files[path];
      if (content === undefined) throw new Error(`no such file: ${path}`);
      return { fileName: path, label: path, content };
    }),
    // Answers what `WorkspaceApi.saveFile` answers: the save check's warnings
    // for that file, none unless the case names some.
    saveFile: vi.fn(async (_ws: string, name: string, content: string): Promise<SaveResult> => {
      saved[name] = content;
      return { warnings: warnings[name] ?? [] };
    }),
  } as unknown as WorkspaceApi;
  return { api, saved };
}

/**
 * A form document good enough to satisfy the set checks, generated from a flow.
 *
 * Most cases here are about the *flow* — a transition to nowhere, an escape that
 * does not resolve — and a hand-written form per case would put three documents
 * in front of the reader to assert one thing about one of them. So the default is
 * one spacer-only form per `formConfig`, with the button an end state may have:
 * `ufNext` on a normal state, `ufCompleted` on an end state, which is exactly
 * what `validateDocumentSet` asks. Cases that are about a real form pass
 * the real document.
 */
function formsFor(flow: unknown): unknown {
  const states = (flow as { states: Record<string, { formConfig: string; isEnd?: boolean }> }).states;
  const forms: Record<string, unknown> = {};
  for (const state of Object.values(states)) {
    if (typeof state?.formConfig !== "string") continue;
    forms[state.formConfig] = {
      rows: [[{ field: "spacer" }]],
      actions: [{ action: state.isEnd ? "ufCompleted" : "ufNext", label: "Go" }],
    };
  }
  return { schemaVersion: 1, forms };
}

const workspace = (key: string, flow: unknown, actions: unknown, forms?: unknown) => ({
  [flowPath(key)]: serialise(flow),
  [actionPath(key)]: serialise(actions),
  [formPath(key)]: serialise(forms ?? formsFor(flow)),
});

const registryWith = (names: string[]): EscapeRegistry => ({
  ...emptyRegistry,
  actions: Object.fromEntries(names.map((n) => [n, async () => null])),
  initializers: { seedFromHomeFilters: () => {} },
});

const storeFor = (
  files: Record<string, string>,
  registry = emptyRegistry,
  warnings: Record<string, SaveFinding[]> = {},
) => {
  const { api, saved } = stubApi(files, warnings);
  return { store: new FlowStore(api, { workspaceName: "ws", registry }), api, saved };
};

describe("paths", () => {
  it("puts both documents in one directory, keyed only by the file name", () => {
    // One directory per file type, as `pipes_config/*.pc.json` already does.
    expect(flowPath("loadFilesUF")).toBe(`${FLOW_DIR}/loadFilesUF.uf.json`);
    expect(actionPath("loadFilesUF")).toBe(`${FLOW_DIR}/loadFilesUF.ua.json`);
  });
});

describe("listing", () => {
  it("finds flows by their file extension, not from a manifest", async () => {
    const { store } = storeFor({
      ...workspace("loadFilesUF", loadFilesFlow, loadFilesActions),
      "user_flows/notes.md": "ignore me",
      "pipes_config/x.pc.json": "{}",
    });
    expect(await store.list()).toEqual(["loadFilesUF"]);
  });
});

describe("loading", () => {
  it("loads a flow and its actions together", async () => {
    const { store } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    const loaded = await store.load("loadFilesUF");
    expect(loaded.key).toBe("loadFilesUF");
    expect(loaded.flow.startAtKey).toBe("select_source_config");
    expect(Object.keys(loaded.actions.actions)).toHaveLength(3);
  });

  it("refuses a document that is not valid JSON, naming the file", async () => {
    const { store } = storeFor({
      [flowPath("x")]: "{ not json",
      [actionPath("x")]: serialise(loadFilesActions),
      [formPath("x")]: serialise(formsFor(loadFilesFlow)),
    });
    await expect(store.load("x")).rejects.toThrow(FlowLoadError);
  });

  it("refuses a document the schema rejects, and points at the field", async () => {
    const broken = structuredClone(loadFilesFlow) as { states: Record<string, object> };
    // An end state that also transitions — the rule the schema makes structural.
    (broken.states["select_file_keys"] as { defaultNextState?: string }).defaultNextState = "x";
    const { store } = storeFor(workspace("x", broken, loadFilesActions));

    const error = await store.load("x").catch((e: FlowLoadError) => e);
    expect(error).toBeInstanceOf(FlowLoadError);
    expect((error as FlowLoadError).findings.some((f) => f.path.startsWith("/states"))).toBe(true);
  });

  it("escapes a pointer segment that contains a slash or a tilde", async () => {
    // **Reachable and unexercised by anything realistic**, which is an
    // earlier lesson: the `Identifier` pattern forbids `/` and `~` in a state key, so a
    // *valid* document never produces such a segment — but an invalid one is
    // exactly what this code path is for, and a key of `a/b` would otherwise
    // emit a pointer that reads as two segments. RFC 6901 §3: `~` then `/`.
    const broken = structuredClone(loadFilesFlow) as { states: Record<string, unknown> };
    broken.states["a/b~c"] = { description: "", formConfig: "f", isEnd: true };
    const { store } = storeFor(workspace("x", broken, loadFilesActions));

    const error = (await store.load("x").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    // The union rejects the state itself, so the pointer stops at the key —
    // which is the segment that needed escaping.
    expect(error.findings.map((f) => f.path)).toContain("/states/a~1b~0c");
  });

  it("refuses a flow whose transition goes nowhere, with the pointer to it", async () => {
    const broken = structuredClone(loadFilesFlow) as {
      states: Record<string, { defaultNextState?: string }>;
    };
    broken.states["select_source_config"]!.defaultNextState = "typo";
    const { store } = storeFor(workspace("x", broken, loadFilesActions));

    const error = (await store.load("x").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error.findings.map((f) => [f.code, f.path])).toEqual([
      ["unknownTarget", "/states/select_source_config/defaultNextState"],
    ]);
  });

  it("refuses a flow whose escape names are not in this build", async () => {
    // **The check only this side can make.** The server validates shape and
    // internal references; whether `seedFromHomeFilters` exists is knowable
    // only where the registry is compiled in.
    const { store } = storeFor(workspace("homeFiltersUF", homeFiltersFlow, homeFiltersActions));
    const error = (await store
      .load("homeFiltersUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("updateHomeFilters");

    // The two clipboard escapes since 2026-10-01: the flow draws Pipeline Status,
    // whose third row runs them.
    const { store: ok } = storeFor(
      workspace("homeFiltersUF", homeFiltersFlow, homeFiltersActions),
      registryWith(["updateHomeFilters", "fetchStageToClipboard", "copySchemaEvent"]),
    );
    await expect(ok.load("homeFiltersUF")).resolves.toBeTruthy();
  });

  it("reports a reachability warning without refusing the flow", async () => {
    // Warnings do not block a load; only errors do. Under the strict policy the
    // same finding would — which is the policy switch, and it is the server's call.
    const flow = structuredClone(loadFilesFlow) as { states: Record<string, unknown> };
    flow.states["orphan"] = { description: "d", formConfig: "f", isEnd: true };
    const { api } = stubApi(workspace("x", flow, loadFilesActions));
    const lenient = new FlowStore(api, { workspaceName: "ws", registry: emptyRegistry });
    await expect(lenient.load("x")).resolves.toBeTruthy();

    const strict = new FlowStore(api, {
      workspaceName: "ws",
      registry: emptyRegistry,
      policy: strictPolicy,
    });
    await expect(strict.load("x")).rejects.toThrow(FlowLoadError);
  });
});

/** `loadFilesUF` with its real form document and the two tables that form names. */
const loadFilesWorkspace = () => ({
  ...workspace("loadFilesUF", loadFilesFlow, loadFilesActions, loadFilesForms),
  [tablePath("lfSourceConfigTable")]: serialise(lfSourceConfigTable),
  [tablePath("lfFileKeyStagingTable")]: serialise(lfFileKeyStagingTable),
});

/**
 * `homeFiltersUF`'s whole set, against the registry the app ships.
 *
 * **This is the assertion the task exists to make**, and it is stronger than any
 * of the per-document ones: four documents, four table configurations and six
 * escape names across three namespaces, checked by the same six passes
 * `FlowStore.load` runs in the browser. Every earlier flow could load with
 * `emptyRegistry`; this one cannot, which is why `productionRegistry` is not
 * incidental here.
 */
const homeFiltersWorkspace = () => ({
  ...workspace("homeFiltersUF", homeFiltersFlow, homeFiltersActions, homeFiltersForms),
  [tablePath("hfProcessTableUF")]: serialise(hfProcessTable),
  [tablePath("hfStatusTableUF")]: serialise(hfStatusTable),
  [tablePath("hfFileKeyFilterTypeTableUF")]: serialise(hfFileKeyFilterTypeTable),
  [tablePath("pipelineExecStatusTable")]: serialise(execStatusTable),
});

describe("homeFiltersUF against the shipping registry", () => {
  it("loads, with every escape name in this build", async () => {
    const { store } = storeFor(homeFiltersWorkspace(), productionRegistry);
    const loaded = await store.load("homeFiltersUF");
    expect(Object.keys(loaded.forms.forms).sort()).toEqual([
      "hfSelectFileKeyFilterUF",
      "hfSelectProcessUF",
      "hfSelectStatusUF",
      "hfSelectTimeWindowUF",
      "hfViewStatusTableUF",
      // `pipelineExecStatusTable`'s *Put Schema Event* opens it, so every set
      // that draws the table carries it (since
      // 2026-10-01). Like `showFailureDetailsDialog`, no state names it.
      "putSchemaEventDialog",
      "showFailureDetailsDialog",
    ]);
    // Four tables, and the fourth is the one registered on the non-flow side.
    // `showFailureDetailsDialog` names no table, so the count is the five
    // states' four distinct ones.
    expect(Object.keys(loaded.tables).sort()).toEqual([
      "hfFileKeyFilterTypeTableUF",
      "hfProcessTableUF",
      "hfStatusTableUF",
      "pipelineExecStatusTable",
    ]);
  });

  it("refuses the set when the dialog form is missing, through the second row", async () => {
    // `showFailureDetailsDialog` is named by no *state* — it is
    // `pipelineExecStatusTable`'s `viewFailureDetails` `configForm`, on the second
    // action row. Dialog forms in the form document and the widened `validateTableActions` together.
    const files = homeFiltersWorkspace();
    const forms = structuredClone(homeFiltersForms) as { forms: Record<string, unknown> };
    delete forms.forms["showFailureDetailsDialog"];
    files[formPath("homeFiltersUF")] = serialise(forms);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("homeFiltersUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings[0]!.message).toContain("showFailureDetailsDialog");
  });
});

/**
 * `pipelineConfigUF`'s nine tables, which is every table its eleven forms name.
 *
 * **Ten and twelve until 2026-10-01**, when the add-or-edit step was removed with
 * its form and its option table: the flow opens
 * on `pcPipelineConfigTable` and adds from a button on it.
 *
 * **Two of the ten are named by no state's form at all** —
 * `pcProcessInputRegistry` and `pcProcessInputRegistry4MI` are inside the two
 * dialogs, which are reached from a table's `doActionShowDialog`. So the table
 * set of this flow is not derivable from the flow document *or* from the states'
 * forms; it needs the dialog forms, which need the rule that they are part of
 * the form document.
 */
const pipelineConfigWorkspace = () => ({
  ...workspace("pipelineConfigUF", pipelineConfigFlow, pipelineConfigActions, pipelineConfigForms),
  [tablePath("pcPipelineConfigTable")]: serialise(pcPipelineConfigTable),
  [tablePath("pcMainProcessInputKey")]: serialise(pcMainProcessInputKey),
  [tablePath("pcViewMergedProcessInputKeys")]: serialise(pcViewMerged),
  [tablePath("pcViewInjectedProcessInputKeys")]: serialise(pcViewInjected),
  [tablePath("pcSummaryProcessInputs")]: serialise(pcSummaryProcessInputs),
  [tablePath("pcMergedProcessInputKeys")]: serialise(pcMerged),
  [tablePath("pcInjectedProcessInputKeys")]: serialise(pcInjected),
  [tablePath("pcProcessInputRegistry")]: serialise(pcProcessInputRegistry),
  [tablePath("pcProcessInputRegistry4MI")]: serialise(pcProcessInputRegistry4MI),
});

describe("pipelineConfigUF against the shipping registry", () => {
  it("loads eleven forms and nine tables, and resolves its registered query", async () => {
    const { store } = storeFor(pipelineConfigWorkspace(), productionRegistry);
    const loaded = await store.load("pipelineConfigUF");
    // Ten states and twelve forms: the two dialogs are the difference, and
    // "one form per state" would have been short by exactly them.
    expect(Object.keys(loaded.flow.states)).toHaveLength(9);
    expect(Object.keys(loaded.forms.forms)).toHaveLength(11);
    expect(Object.keys(loaded.tables)).toHaveLength(9);
    // The seventh escape namespace, resolved through the same pass as the other
    // six: `pcAddPipelineConfigUF` names `processInputRdfTypes` and this build
    // registers it (`actions/queries.ts`).
    expect(
      escapeReferences(loaded.flow, loaded.actions).filter((r) => r.kind === "queries"),
    ).toEqual([
      { kind: "queries", name: "processInputRdfTypes", at: "/actions/pcAddPipelineConfigUF/steps/6" },
    ]);
  });

  it("refuses a query name this build does not register", async () => {
    const files = pipelineConfigWorkspace();
    const actions = structuredClone(pipelineConfigActions) as {
      actions: Record<string, { steps: Record<string, unknown>[] }>;
    };
    const steps = actions.actions["pcAddPipelineConfigUF"]!.steps;
    steps[steps.length - 1]!["name"] = "processInputRdfTypesV2";
    files[actionPath("pipelineConfigUF")] = serialise(actions);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("pipelineConfigUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    // Not "escape", because this namespace holds a statement rather than a body
    // and the message is where an author is told what to go and register.
    expect(error.findings[0]!.message).toContain(
      'no registered query named "processInputRdfTypesV2"',
    );
  });

  it("catches both halves of the table-action check at once, which no earlier flow could", async () => {
    // **The first set where a table names an action *and* a form on the same
    // button.** All three `doActionShowDialog` actions run
    // `pcSetProcessInputRegistryKey` and open one of the two dialogs
    // (`pipeline_config/data_table_config.dart`), so removing either definition
    // must fail the load — and before `validateTableActions` existed neither
    // would have.
    const drop = async (mutate: (files: Record<string, string>) => void) => {
      const files = pipelineConfigWorkspace();
      mutate(files);
      const { store } = storeFor(files, productionRegistry);
      return (await store.load("pipelineConfigUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    };

    const withoutAction = await drop((files) => {
      const actions = structuredClone(pipelineConfigActions) as { actions: Record<string, unknown> };
      delete actions.actions["pcSetProcessInputRegistryKey"];
      files[actionPath("pipelineConfigUF")] = serialise(actions);
    });
    expect(withoutAction).toBeInstanceOf(FlowLoadError);
    expect(withoutAction.findings.map((f) => f.message).join("\n")).toContain(
      'runs "pcSetProcessInputRegistryKey"',
    );

    const withoutForm = await drop((files) => {
      const forms = structuredClone(pipelineConfigForms) as { forms: Record<string, unknown> };
      delete forms.forms["pcNewProcessInputDialog4MI"];
      files[formPath("pipelineConfigUF")] = serialise(forms);
    });
    expect(withoutForm).toBeInstanceOf(FlowLoadError);
    expect(withoutForm.findings.map((f) => f.message).join("\n")).toContain(
      'opens form "pcNewProcessInputDialog4MI"',
    );
  });
});

describe("the form and table documents", () => {
  it("loads all four kinds, which nothing did before", async () => {
    const { store } = storeFor(loadFilesWorkspace(), productionRegistry);
    const loaded = await store.load("loadFilesUF");

    expect(Object.keys(loaded.forms.forms).sort()).toEqual([
      "lfSelectFileKeysUF",
      "lfSelectSourceConfigUF",
    ]);
    expect(Object.keys(loaded.tables).sort()).toEqual([
      "lfFileKeyStagingTable",
      "lfSourceConfigTable",
    ]);
  });

  it("takes the table set from the forms, not from the flow", async () => {
    // A table is shared between flows and named by a field, so the flow document
    // says nothing about which ones a run needs. Reading them off the forms is
    // why `table_configs/` is keyed by table rather than by flow.
    const { store } = storeFor(loadFilesWorkspace(), productionRegistry);
    const loaded = await store.load("loadFilesUF");
    expect(tableKeysOf(loaded.forms)).toEqual(["lfSourceConfigTable", "lfFileKeyStagingTable"]);
    expect(JSON.stringify(loadFilesFlow)).not.toContain("lfSourceConfigTable");
  });

  it("hands the widget a runtime configuration, through the table document's inverse", async () => {
    const { store } = storeFor(loadFilesWorkspace(), productionRegistry);
    const loaded = await store.load("loadFilesUF");
    const config = tableConfigOf(loaded, "lfSourceConfigTable");
    expect(config.key).toBe("lfSourceConfigTable");
    expect(config.fromClauses[0]!.tableName).toBe("source_config");
    // Restored rather than authored — see the `apiAction` note in `table.ts`.
    expect(config.apiAction).toBe("read");
  });

  it("refuses a flow whose table document is not in the workspace", async () => {
    const files = loadFilesWorkspace();
    delete files[tablePath("lfFileKeyStagingTable")];
    const { store } = storeFor(files, productionRegistry);

    const error = (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("table_configs/lfFileKeyStagingTable.tc.json");
  });

  it("refuses a flow whose form document is not in the workspace", async () => {
    const files = loadFilesWorkspace();
    delete files[formPath("loadFilesUF")];
    const { store } = storeFor(files, productionRegistry);

    const error = (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("user_flows/loadFilesUF.form.json");
  });

  it("refuses a set whose end state offers a button that advances", async () => {
    // The check no schema can make: `isEnd` is in one document and the button set
    // in the other, and both documents are valid. Nothing ran it at load until
    // the flow runner wired it here.
    const forms = structuredClone(loadFilesForms) as {
      forms: Record<string, { actions: { action: string; label: string }[] }>;
    };
    forms.forms["lfSelectFileKeysUF"]!.actions.push({ action: "ufNext", label: "Next" });
    const files = {
      ...loadFilesWorkspace(),
      [formPath("loadFilesUF")]: serialise(forms),
    };
    const { store } = storeFor(files, productionRegistry);

    const error = (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("is an end state");
    expect(error.findings[0]!.path).toBe("forms/forms/lfSelectFileKeysUF/actions");
  });

  it("refuses a table escape this build does not register, naming the file", async () => {
    // The client-only half of validation, now reaching table documents too: the
    // registry is compiled into the bundle and the server cannot enumerate it.
    const table = structuredClone(lfSourceConfigTable) as {
      columns: { name: string; cellFilter?: string }[];
    };
    table.columns[1]!.cellFilter = "noSuchFilter";
    const files = {
      ...loadFilesWorkspace(),
      [tablePath("lfSourceConfigTable")]: serialise(table),
    };
    const { store } = storeFor(files, productionRegistry);

    const error = (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("no cellFilters escape named \"noSuchFilter\"");
    expect(error.message).toContain("table_configs/lfSourceConfigTable.tc.json/columns/1/cellFilter");
  });
});

describe("a table document that names both escapes", () => {
  // `inputRegistryTable` is one of the three configurations carrying both
  // escape names — `cellFilter: fileKeyLabel` on a column and
  // `isEnabled: hasDataRegistryFilters` on its `clearFilters` action. No flow
  // the port has migrated uses it, so this is a synthetic set: a one-state
  // flow whose form names that table. **The point is the resolution, not the
  // flow** — until `actions/registry.ts` existed both names failed, and the two
  // sites that could have said so are here and in `registry.test.ts`.
  const syntheticFlow = {
    schemaVersion: 1,
    startAtKey: "pick",
    states: {
      pick: { description: "pick a file", formConfig: "pickForm", isEnd: true },
    },
  };
  const syntheticForms = {
    schemaVersion: 1,
    forms: {
      pickForm: {
        rows: [[{ field: "dataTable", key: "inputRegistryTable", table: "inputRegistryTable" }]],
        actions: [{ action: "ufCompleted", label: "Done" }],
      },
    },
  };
  const files = () => ({
    ...workspace("x", syntheticFlow, { schemaVersion: 1, actions: {} }, syntheticForms),
    [tablePath("inputRegistryTable")]: serialise(inputRegistryTable),
  });

  it("loads under the production registry", async () => {
    const { store } = storeFor(files(), productionRegistry);
    const loaded = await store.load("x");
    expect(Object.keys(loaded.tables)).toEqual(["inputRegistryTable"]);
  });

  it("is refused by a build that registers neither", async () => {
    const { store } = storeFor(files(), emptyRegistry);
    const error = (await store.load("x").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.message).toContain("fileKeyLabel");
    expect(error.message).toContain("hasDataRegistryFilters");
  });
});

describe("saving", () => {
  it("writes the actions first, so a partial save leaves a runnable flow", async () => {
    // Not atomic and it cannot be: the endpoint takes one file. Actions first
    // means a stale flow with new actions, which still runs; the reverse does
    // not.
    const { store, api, saved } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    const loaded = await store.load("loadFilesUF");
    await store.save(loaded);

    const order = (api.saveFile as unknown as { mock: { calls: [string, string][] } }).mock.calls.map(
      (c) => c[1],
    );
    expect(order).toEqual([
      actionPath("loadFilesUF"),
      formPath("loadFilesUF"),
      flowPath("loadFilesUF"),
    ]);
    expect(saved[flowPath("loadFilesUF")]).toBe(serialise(loadFilesFlow));
  });

  /**
   * **A save the server accepts with warnings is a save, and the warnings come
   * back.** `WorkspaceApi.saveFile` has answered them since the save check began
   * returning its warnings; `save` dropped them, so a flow's documents saved with
   * nothing said about what the check found.
   */
  it("answers the save check's warnings, per document, and still writes all three", async () => {
    const formWarning: SaveFinding = {
      severity: "warning",
      code: "x_unknown",
      message: "ignored: unknown field",
      path: "/forms/a/extra",
    };
    const flowWarning: SaveFinding = { severity: "warning", code: "x_line", message: "a line finding", line: 3 };
    const { store, saved } = storeFor(
      workspace("loadFilesUF", loadFilesFlow, loadFilesActions),
      emptyRegistry,
      { [formPath("loadFilesUF")]: [formWarning], [flowPath("loadFilesUF")]: [flowWarning] },
    );
    const result = await store.save(await store.load("loadFilesUF"));

    expect(result.files).toEqual([
      { path: actionPath("loadFilesUF"), warnings: [] },
      { path: formPath("loadFilesUF"), warnings: [formWarning] },
      { path: flowPath("loadFilesUF"), warnings: [flowWarning] },
    ]);
    expect(Object.keys(saved)).toHaveLength(3);

    // Worded as the Workspace IDE words a save's warnings, under each file that has any.
    expect(describeFlowSaveWarnings(result)).toBe(
      [
        `Saved ${formPath("loadFilesUF")}, with 1 warning:`,
        "  /forms/a/extra: ignored: unknown field",
        `Saved ${flowPath("loadFilesUF")}, with 1 warning:`,
        "  line 3: a line finding",
      ].join("\n"),
    );
  });

  it("answers no warnings, and no text, when the check found none", async () => {
    const { store } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    const result = await store.save(await store.load("loadFilesUF"));
    expect(result.files.every((f) => f.warnings.length === 0)).toBe(true);
    expect(describeFlowSaveWarnings(result)).toBeNull();
  });

  it("still throws on a refused document, and writes none after it", async () => {
    const { store, api, saved } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    const loaded = await store.load("loadFilesUF");
    (api.saveFile as unknown as { mockImplementationOnce(f: () => Promise<never>): void }).mockImplementationOnce(
      async () => {
        throw new Error("user_flows/loadFilesUF.ua.json cannot be saved");
      },
    );
    await expect(store.save(loaded)).rejects.toThrow(/cannot be saved/);
    expect(saved).toEqual({});
  });

  it("round-trips byte-for-byte, so a save with no edit is a no-op diff", async () => {
    const { store, saved } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    await store.save(await store.load("loadFilesUF"));
    expect(saved[flowPath("loadFilesUF")]).toBe(serialise(loadFilesFlow));
    expect(saved[actionPath("loadFilesUF")]).toBe(serialise(loadFilesActions));
  });
});

/**
 * `fileMappingUF` against the registry this build ships.
 *
 * **The first set whose escapes are the reason the flow needs a server at all.**
 * `homeFiltersUF`'s escapes compile filters and `pipelineConfigUF`'s registered
 * query reads one row; these two are a joined read and a raw-rows post, and both
 * resolve here or the flow does not load.
 */
const fileMappingWorkspace = () => ({
  ...workspace("fileMappingUF", fileMappingFlow, fileMappingActions, fileMappingForms),
  [tablePath("fmInputSourceMappingUF")]: serialise(fmInputSourceMapping),
  [tablePath("fmFileMappingTableUF")]: serialise(fmFileMappingTable),
});

describe("fileMappingUF against the shipping registry", () => {
  it("loads three forms for two states, and both of its tables", async () => {
    const { store } = storeFor(fileMappingWorkspace(), productionRegistry);
    const loaded = await store.load("fileMappingUF");
    expect(Object.keys(loaded.flow.states)).toHaveLength(2);
    // Three: the two states' forms and `loadRawRowsDialog`, which
    // `fmFileMappingTableUF` opens. The last of the four the flow corpus
    // calls *unreferenced*.
    expect(Object.keys(loaded.forms.forms).sort()).toEqual([
      "fmFileMappingUF",
      "fmSelectSourceConfigUF",
      "loadRawRowsDialog",
    ]);
    expect(Object.keys(loaded.tables).sort()).toEqual([
      "fmFileMappingTableUF",
      "fmInputSourceMappingUF",
    ]);
    // Both action escapes resolve against `productionRegistry`, which is what
    // makes the two the flow depends on real rather than declared.
    expect(
      escapeReferences(loaded.flow, loaded.actions)
        .filter((r) => r.kind === "actions")
        .map((r) => r.name)
        .sort(),
    ).toEqual(["downloadMapping", "loadRawRows"]);
  });

  it("refuses the set when the table's doAction names an action nothing defines", async () => {
    // The table-action check's first half, on the flow chosen for it. `downloadMappingRows`
    // is a `doAction` naming `downloadMapping`, and no state and no form button
    // names it — so before `validateTableActions` this was the one reference a
    // per-document check could not see.
    const files = fileMappingWorkspace();
    const actions = structuredClone(fileMappingActions) as { actions: Record<string, unknown> };
    delete actions.actions["downloadMapping"];
    files[actionPath("fileMappingUF")] = serialise(actions);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("fileMappingUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => f.message).join("\n")).toContain('runs "downloadMapping"');
  });

  it("refuses the set when the dialog the table opens is not in the form document", async () => {
    const files = fileMappingWorkspace();
    const forms = structuredClone(fileMappingForms) as { forms: Record<string, unknown> };
    delete forms.forms["loadRawRowsDialog"];
    files[formPath("fileMappingUF")] = serialise(forms);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("fileMappingUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => f.message).join("\n")).toContain(
      'opens form "loadRawRowsDialog"',
    );
  });

  it("does not ask for the form a showScreen names, because it is another flow's", async () => {
    // **`configureMappingPage` carries `configForm: "fmMappingFormUF"` and that
    // form is in `mapFileUF.form.json`.** A `showScreen` navigates to another
    // route and the form is rendered by whatever serves it, so checking it
    // against *this* flow's document would demand a copy of a form that already
    // has a home. `validateTableActions` checks `configForm` for `showDialog` and
    // `doActionShowDialog` only, and this loads with it absent.
    const { store } = storeFor(fileMappingWorkspace(), productionRegistry);
    const loaded = await store.load("fileMappingUF");
    expect(Object.keys(loaded.forms.forms)).not.toContain("fmMappingFormUF");
  });
});

/**
 * `sourceConfigUF` against the registry this build ships. The last flow migrated.
 *
 * **Twelve states and four tables, which is the widest load the store has been
 * asked for** — eleven and three since 2026-10-01, when the add-or-edit step
 * and its option table were removed — and the first whose form document declares a query with `params`,
 * the port's spelling of `stateKeyPredicates`.
 */
const sourceConfigWorkspace = () => ({
  ...workspace("sourceConfigUF", sourceConfigFlow, sourceConfigActions, sourceConfigForms),
  [tablePath("scSourceConfigKey")]: serialise(scSourceConfigKey),
  [tablePath("scSingleOrMultiPartFileOption")]: serialise(scSingleOrMultiPart),
  [tablePath("input_format")]: serialise(inputFormatTable),
});

describe("sourceConfigUF against the shipping registry", () => {
  it("loads twelve forms for twelve states, and all three of its tables", async () => {
    const { store } = storeFor(sourceConfigWorkspace(), productionRegistry);
    const loaded = await store.load("sourceConfigUF");
    // Twelve since 2026-11-21: `edit_feed_profile` and its form, the feed profile's page.
    expect(Object.keys(loaded.flow.states)).toHaveLength(12);
    // Twelve for twelve: every form is named by a state and the flow owes no
    // dialog form, which is why `dialogCancel` is not one of its arms.
    expect(Object.keys(loaded.forms.forms)).toHaveLength(12);
    expect(Object.keys(loaded.tables).sort()).toEqual([
      "input_format",
      "scSingleOrMultiPartFileOption",
      "scSourceConfigKey",
    ]);
    expect(
      escapeReferences(loaded.flow, loaded.actions)
        .filter((r) => r.kind === "actions")
        .map((r) => r.name)
        .sort(),
    ).toEqual(["readXlsxSheetOption", "saveSourceConfigForFileType"]);
  });

  it("refuses the set when the table's Delete names an action nothing defines", async () => {
    // The table-action check on this flow's one query table: `scSourceConfigKey` carries both of
    // the arms no state and no form button names — `dropTable` and
    // `deleteSourceConfig` (`configure_files/data_table_config.dart`).
    const files = sourceConfigWorkspace();
    const actions = structuredClone(sourceConfigActions) as { actions: Record<string, unknown> };
    delete actions.actions["deleteSourceConfig"];
    files[actionPath("sourceConfigUF")] = serialise(actions);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("sourceConfigUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => f.message).join("\n")).toContain('runs "deleteSourceConfig"');
  });

  it("refuses the set when a dropdown names a query its form does not declare", async () => {
    // The `stateKeyPredicates` half of dropdown queries, checked where it lives: `orgs` is
    // declared on `scAddSourceConfigUF` with `params: ["client"]`, and
    // `checkItemSources` is the only thing that can say `itemsFrom` resolves.
    const files = sourceConfigWorkspace();
    const forms = structuredClone(sourceConfigForms) as {
      forms: Record<string, { queries?: Record<string, unknown> }>;
    };
    delete forms.forms["scAddSourceConfigUF"]!.queries!["orgs"];
    files[formPath("sourceConfigUF")] = serialise(forms);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("sourceConfigUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => f.message).join("\n")).toContain('query "orgs"');
  });

  it("refuses the set when + Add no longer clears the key that decides insert or update", async () => {
    // `checkFlowSet`'s `validateAddClearsState`, at load. `key` is published by
    // the table's selection, and `saveSourceConfigForFileType` updates whenever
    // it is set, so an add after an edit would update the edited record.
    const files = sourceConfigWorkspace();
    const actions = structuredClone(sourceConfigActions) as {
      actions: Record<string, { steps: { do: string; keys?: string[] }[] }>;
    };
    const remove = actions.actions["scGoToAddSourceConfig"]!.steps.find((step) => step.do === "remove")!;
    remove.keys = remove.keys!.filter((key) => key !== "key");
    files[actionPath("sourceConfigUF")] = serialise(actions);
    const { store } = storeFor(files, productionRegistry);
    const error = (await store
      .load("sourceConfigUF")
      .catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings).toHaveLength(1);
    expect(error.findings[0]!.path).toBe("actions/actions/scGoToAddSourceConfig/steps/3");
    expect(error.findings[0]!.message).toContain('leaves "key"');
  });
});

/**
 * Which documents `load` reads, in which order. A flow its own documents refuse
 * is refused before its tables are read, so a refusal costs three reads rather
 * than three plus one per table.
 */
describe("the reads a refused flow costs", () => {
  const tableReads = (api: WorkspaceApi): string[] =>
    (api.readWorkspaceDocument as unknown as { mock: { calls: [string, string][] } }).mock.calls
      .map(([, path]) => path)
      .filter((path) => path.startsWith("table_configs/"));

  it("reads no table when the set check refuses the flow", async () => {
    // An end state offering a button that advances: a set finding, which needs
    // the flow and its forms and none of the tables.
    const forms = structuredClone(loadFilesForms) as {
      forms: Record<string, { actions: { action: string; label: string }[] }>;
    };
    forms.forms["lfSelectFileKeysUF"]!.actions.push({ action: "ufNext", label: "Next" });
    const { store, api } = storeFor(
      { ...loadFilesWorkspace(), [formPath("loadFilesUF")]: serialise(forms) },
      productionRegistry,
    );

    const error = (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
    expect(error.message).toContain("is an end state");
    expect(tableReads(api)).toEqual([]);
  });

  it("reads no table when a transition goes nowhere", async () => {
    const broken = structuredClone(loadFilesFlow) as {
      states: Record<string, { defaultNextState?: string }>;
    };
    broken.states["select_source_config"]!.defaultNextState = "typo";
    const { store, api } = storeFor({
      ...loadFilesWorkspace(),
      [flowPath("loadFilesUF")]: serialise(broken),
    }, productionRegistry);

    await expect(store.load("loadFilesUF")).rejects.toThrow(FlowLoadError);
    expect(tableReads(api)).toEqual([]);
  });

  it("reads every table once the flow's own documents pass", async () => {
    const { store, api } = storeFor(loadFilesWorkspace(), productionRegistry);
    await store.load("loadFilesUF");
    expect(tableReads(api).sort()).toEqual([
      tablePath("lfFileKeyStagingTable"),
      tablePath("lfSourceConfigTable"),
    ]);
  });
});

/**
 * A tab opened before a deployment keeps the schemas its bundle was built with,
 * so a document using a value the deployment added to an enumerated field is
 * refused until the page is reloaded. The refusal says so.
 */
describe("a value this page's schemas do not list", () => {
  const loadWith = async (path: string, document: unknown) => {
    const { store } = storeFor({ ...loadFilesWorkspace(), [path]: serialise(document) }, productionRegistry);
    return (await store.load("loadFilesUF").catch((e: FlowLoadError) => e)) as FlowLoadError;
  };

  it("asks for a reload when a table action's style is not one this page knows", async () => {
    const table = structuredClone(lfSourceConfigTable) as { actions: { style: string }[] };
    table.actions[0]!.style = "outline";
    const error = await loadWith(tablePath("lfSourceConfigTable"), table);
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => [f.path, f.message.endsWith(RELOAD_HINT)])).toEqual([
      ["/actions/0/style", true],
    ]);
  });

  it("asks for a reload when a step's kind is not one this page knows", async () => {
    const actions = structuredClone(loadFilesActions) as {
      actions: Record<string, { steps: { do: string }[] }>;
    };
    actions.actions["lfSyncFileKey"]!.steps[0]!.do = "retry";
    const error = await loadWith(actionPath("loadFilesUF"), actions);
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings.map((f) => [f.path, f.message.endsWith(RELOAD_HINT)])).toEqual([
      ["/actions/lfSyncFileKey/steps/0", true],
    ]);
  });

  it("asks for a reload when a table's source is not one this page knows", async () => {
    const table = { ...structuredClone(lfSourceConfigTable), source: "view" };
    const error = await loadWith(tablePath("lfSourceConfigTable"), table);
    expect(error.findings.map((f) => [f.path, f.message.endsWith(RELOAD_HINT)])).toEqual([
      ["/source", true],
    ]);
  });

  it("does not ask for one when the document is wrong in another way", async () => {
    const table = structuredClone(lfSourceConfigTable) as { actions: Record<string, unknown>[] };
    table.actions[0]!["colour"] = "red";
    const error = await loadWith(tablePath("lfSourceConfigTable"), table);
    expect(error).toBeInstanceOf(FlowLoadError);
    expect(error.findings).toHaveLength(1);
    expect(error.findings[0]!.message).not.toContain(RELOAD_HINT);
  });

  it("does not take a union refused on different fields for an unknown value", () => {
    // Each member refuses a different field, so no one field holds a value
    // that matches no member: the document is wrong, not newer.
    expect(
      isUnknownEnumeratedValue({
        code: "invalid_union",
        path: [],
        message: "Invalid input",
        errors: [
          [{ code: "invalid_value", path: ["do"] }],
          [{ code: "invalid_type", path: ["keys"] }],
        ],
      }),
    ).toBe(false);
  });
});

describe("escape references", () => {
  it("finds the initializer and every escape step, with a pointer to each", () => {
    const references = escapeReferences(
      homeFiltersFlow as never,
      homeFiltersActions as never,
    );
    expect(references.filter((r) => r.kind === "initializers")).toEqual([
      { kind: "initializers", name: "seedFromHomeFilters", at: "/formStateInitializer" },
    ]);
    const actions = references.filter((r) => r.kind === "actions");
    // Four `updateHomeFilters` steps, and since 2026-10-01 the two clipboard
    // escapes of the table's third row.
    expect(actions).toHaveLength(6);
    expect(actions[0]!.at).toMatch(/^\/actions\/.+\/steps\/\d+$/);
  });
});

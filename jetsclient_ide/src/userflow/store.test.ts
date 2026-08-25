/**
 * Tests for the flow store (task S.3).
 *
 * Driven against a stub `WorkspaceApi` rather than a live server: what S.3 adds
 * is the *store*, and the transport underneath it is the one Phase 1 shipped and
 * A.4a verified against `localhost:8080`. What is worth asserting here is the
 * part that is new — that a flow which does not validate does not load, and that
 * the reason it did not load is usable.
 */

import { describe, expect, it, vi } from "vitest";

import { emptyRegistry, type EscapeRegistry } from "../actions/escapes";
import { productionRegistry } from "../actions/registry";
import loadFilesActions from "../actions/flows/loadFilesUF.ua.json";
import homeFiltersActions from "../actions/flows/homeFiltersUF.ua.json";
import pipelineConfigActions from "../actions/flows/pipelineConfigUF.ua.json";
import fileMappingActions from "../actions/flows/fileMappingUF.ua.json";
import sourceConfigActions from "../actions/flows/sourceConfigUF.ua.json";
import type { WorkspaceApi } from "../api/workspace";
import loadFilesFlow from "./flows/loadFilesUF.uf.json";
import homeFiltersFlow from "./flows/homeFiltersUF.uf.json";
import pipelineConfigFlow from "./flows/pipelineConfigUF.uf.json";
import fileMappingFlow from "./flows/fileMappingUF.uf.json";
import sourceConfigFlow from "./flows/sourceConfigUF.uf.json";
import lfFileKeyStagingTable from "../datatable/tables/lfFileKeyStagingTable.tc.json";
import lfSourceConfigTable from "../datatable/tables/lfSourceConfigTable.tc.json";
import inputRegistryTable from "../datatable/tables/inputRegistryTable.tc.json";
import { tablePath } from "../datatable/table";
import homeFiltersForms from "./forms/homeFiltersUF.form.json";
import pipelineConfigForms from "./forms/pipelineConfigUF.form.json";
import fileMappingForms from "./forms/fileMappingUF.form.json";
import sourceConfigForms from "./forms/sourceConfigUF.form.json";
import pcAddOrEditOption from "../datatable/tables/pcAddOrEditPipelineConfigOption.tc.json";
import pcPipelineConfigTable from "../datatable/tables/pcPipelineConfigTable.tc.json";
import pcMainProcessInputKey from "../datatable/tables/pcMainProcessInputKey.tc.json";
import pcViewMerged from "../datatable/tables/pcViewMergedProcessInputKeys.tc.json";
import pcViewInjected from "../datatable/tables/pcViewInjectedProcessInputKeys.tc.json";
import pcSummaryProcessInputs from "../datatable/tables/pcSummaryProcessInputs.tc.json";
import pcMerged from "../datatable/tables/pcMergedProcessInputKeys.tc.json";
import pcInjected from "../datatable/tables/pcInjectedProcessInputKeys.tc.json";
import pcProcessInputRegistry from "../datatable/tables/pcProcessInputRegistry.tc.json";
import pcProcessInputRegistry4MI from "../datatable/tables/pcProcessInputRegistry4MI.tc.json";
import fmInputSourceMapping from "../datatable/tables/fmInputSourceMappingUF.tc.json";
import fmFileMappingTable from "../datatable/tables/fmFileMappingTableUF.tc.json";
import scAddOrEditOption from "../datatable/tables/scAddOrEditSourceConfigOption.tc.json";
import scSourceConfigKey from "../datatable/tables/scSourceConfigKey.tc.json";
import scSingleOrMultiPart from "../datatable/tables/scSingleOrMultiPartFileOption.tc.json";
import inputFormatTable from "../datatable/tables/input_format.tc.json";
import hfProcessTable from "../datatable/tables/hfProcessTableUF.tc.json";
import hfStatusTable from "../datatable/tables/hfStatusTableUF.tc.json";
import hfFileKeyFilterTypeTable from "../datatable/tables/hfFileKeyFilterTypeTableUF.tc.json";
import execStatusTable from "../datatable/tables/pipelineExecStatusTable.tc.json";
import loadFilesForms from "./forms/loadFilesUF.form.json";
import {
  FLOW_DIR,
  FlowLoadError,
  FlowStore,
  actionPath,
  escapeReferences,
  flowPath,
  formPath,
  serialise,
  tableConfigOf,
  tableKeysOf,
} from "./store";
import { strictPolicy } from "./validate";

/**
 * A workspace as a map from path to text.
 *
 * **The stub now answers `readWorkspaceFile`, and the change is the point of
 * I-65.** It used to answer `readFile(ws, node)` and key off `node.key`, which
 * the real `WorkspaceApi` never does: `readFile` takes a tree node and asks
 * `fileNameOf` for the path, and that returns null unless `node.type === "file"`.
 * The store was synthesising `{ label, key } as never`, so the stub accepted a
 * shape the implementation would have refused. Stubbing the method the store
 * actually calls is what keeps the two honest.
 */
function stubApi(files: Record<string, string>) {
  const saved: Record<string, string> = {};
  const api = {
    fileTree: vi.fn(async () =>
      Object.keys(files).map((path) => ({ label: path.split("/").pop(), key: path })),
    ),
    readWorkspaceFile: vi.fn(async (_ws: string, path: string) => {
      const content = files[path];
      if (content === undefined) throw new Error(`no such file: ${path}`);
      return { fileName: path, label: path, content };
    }),
    saveFile: vi.fn(async (_ws: string, name: string, content: string) => {
      saved[name] = content;
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
 * what `validateDocumentSet` asks (I-57). Cases that are about a real form pass
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

const storeFor = (files: Record<string, string>, registry = emptyRegistry) => {
  const { api, saved } = stubApi(files);
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
    // An end state that also transitions — the rule S.1 made structural.
    (broken.states["select_file_keys"] as { defaultNextState?: string }).defaultNextState = "x";
    const { store } = storeFor(workspace("x", broken, loadFilesActions));

    const error = await store.load("x").catch((e: FlowLoadError) => e);
    expect(error).toBeInstanceOf(FlowLoadError);
    expect((error as FlowLoadError).findings.some((f) => f.path.startsWith("/states"))).toBe(true);
  });

  it("escapes a pointer segment that contains a slash or a tilde", async () => {
    // **Reachable and unexercised by anything realistic**, which is I-24's
    // lesson: the `Identifier` pattern forbids `/` and `~` in a state key, so a
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

    const { store: ok } = storeFor(
      workspace("homeFiltersUF", homeFiltersFlow, homeFiltersActions),
      registryWith(["updateHomeFilters"]),
    );
    await expect(ok.load("homeFiltersUF")).resolves.toBeTruthy();
  });

  it("reports a reachability warning without refusing the flow", async () => {
    // Warnings do not block a load; only errors do. Under the strict policy the
    // same finding would — which is I-17's switch, and it is the server's call.
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
 * `homeFiltersUF`'s whole set, against the registry the app ships. Task F.5.
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
      "showFailureDetailsDialog",
    ]);
    // Four tables, and the fourth is the one registered on the non-flow side
    // (F18). `showFailureDetailsDialog` names no table, so the count is the five
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
    // action row. I-89 and F.5's widening of `validateTableActions` together.
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
 * `pipelineConfigUF`'s ten tables, which is every table its twelve forms name.
 *
 * **Two of the ten are named by no state's form at all** —
 * `pcProcessInputRegistry` and `pcProcessInputRegistry4MI` are inside the two
 * dialogs, which are reached from a table's `doActionShowDialog`. So the table
 * set of this flow is not derivable from the flow document *or* from the states'
 * forms; it needs the dialog forms, which need I-89's rule that they are part of
 * the form document.
 */
const pipelineConfigWorkspace = () => ({
  ...workspace("pipelineConfigUF", pipelineConfigFlow, pipelineConfigActions, pipelineConfigForms),
  [tablePath("pcAddOrEditPipelineConfigOption")]: serialise(pcAddOrEditOption),
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
  it("loads twelve forms and ten tables, and resolves its registered query", async () => {
    const { store } = storeFor(pipelineConfigWorkspace(), productionRegistry);
    const loaded = await store.load("pipelineConfigUF");
    // Ten states and twelve forms: I-89's two dialogs are the difference, and
    // "one form per state" would have been short by exactly them.
    expect(Object.keys(loaded.flow.states)).toHaveLength(10);
    expect(Object.keys(loaded.forms.forms)).toHaveLength(12);
    expect(Object.keys(loaded.tables)).toHaveLength(10);
    // F.6's seventh escape namespace, resolved through the same pass as the other
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

  it("catches both halves of I-88's check at once, which no earlier flow could", async () => {
    // **The first set where a table names an action *and* a form on the same
    // button.** All three `doActionShowDialog` actions run
    // `pcSetProcessInputRegistryKey` and open one of the two dialogs
    // (`pipeline_config/data_table_config.dart`), so removing either definition
    // must fail the load — and before F.3 built `validateTableActions` neither
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
  it("loads all four kinds, which is what I-51 said nothing did", async () => {
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

  it("hands the widget a runtime configuration, through I.3a's inverse", async () => {
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

  it("refuses a set whose end state offers a button that advances (I-57)", async () => {
    // The check no schema can make: `isEnd` is in one document and the button set
    // in the other, and both documents are valid. Nothing ran it at load until
    // F.0a wired it here.
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
  // `inputRegistryTable` is one of the three configurations carrying both of
  // I-54's names — `cellFilter: fileKeyLabel` on a column and
  // `isEnabled: hasDataRegistryFilters` on its `clearFilters` action. No flow
  // this project has migrated uses it, so this is a synthetic set: a one-state
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

  it("round-trips byte-for-byte, so a save with no edit is a no-op diff", async () => {
    const { store, saved } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    await store.save(await store.load("loadFilesUF"));
    expect(saved[flowPath("loadFilesUF")]).toBe(serialise(loadFilesFlow));
    expect(saved[actionPath("loadFilesUF")]).toBe(serialise(loadFilesActions));
  });
});

/**
 * `fileMappingUF` against the registry this build ships. Task F.8.
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
    // `fmFileMappingTableUF` opens (I-89). The last of the four the flow corpus
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
    // I-88's first half, on the flow F.3 endorsed for it. `downloadMappingRows`
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
    // `doActionShowDialog` only (I-122), and this loads with it absent.
    const { store } = storeFor(fileMappingWorkspace(), productionRegistry);
    const loaded = await store.load("fileMappingUF");
    expect(Object.keys(loaded.forms.forms)).not.toContain("fmMappingFormUF");
  });
});

/**
 * `sourceConfigUF` against the registry this build ships. Task F.7, and the last.
 *
 * **Twelve states and four tables, which is the widest load the store has been
 * asked for** — and the first whose form document declares a query with `params`,
 * the port's spelling of `stateKeyPredicates`.
 */
const sourceConfigWorkspace = () => ({
  ...workspace("sourceConfigUF", sourceConfigFlow, sourceConfigActions, sourceConfigForms),
  [tablePath("scAddOrEditSourceConfigOption")]: serialise(scAddOrEditOption),
  [tablePath("scSourceConfigKey")]: serialise(scSourceConfigKey),
  [tablePath("scSingleOrMultiPartFileOption")]: serialise(scSingleOrMultiPart),
  [tablePath("input_format")]: serialise(inputFormatTable),
});

describe("sourceConfigUF against the shipping registry", () => {
  it("loads twelve forms for twelve states, and all four of its tables", async () => {
    const { store } = storeFor(sourceConfigWorkspace(), productionRegistry);
    const loaded = await store.load("sourceConfigUF");
    expect(Object.keys(loaded.flow.states)).toHaveLength(12);
    // Twelve for twelve: every form is named by a state and the flow owes no
    // dialog form, which is why `dialogCancel` is not one of its arms.
    expect(Object.keys(loaded.forms.forms)).toHaveLength(12);
    expect(Object.keys(loaded.tables).sort()).toEqual([
      "input_format",
      "scAddOrEditSourceConfigOption",
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
    // I-88 on this flow's one query table: `scSourceConfigKey` carries both of
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
    // `stateKeyPredicates`' half of I-11, checked where it lives: `orgs` is
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
    expect(actions).toHaveLength(4);
    expect(actions[0]!.at).toMatch(/^\/actions\/.+\/steps\/\d+$/);
  });
});

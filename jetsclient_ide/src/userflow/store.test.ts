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
import loadFilesActions from "../actions/flows/loadFilesUF.ua.json";
import homeFiltersActions from "../actions/coverage/homeFiltersUF.ua.json";
import type { WorkspaceApi } from "../api/workspace";
import loadFilesFlow from "./flows/loadFilesUF.uf.json";
import homeFiltersFlow from "./flows/homeFiltersUF.uf.json";
import {
  FLOW_DIR,
  FlowLoadError,
  FlowStore,
  actionPath,
  escapeReferences,
  flowPath,
  serialise,
} from "./store";
import { strictPolicy } from "./validate";

/** A workspace as a map from path to text. */
function stubApi(files: Record<string, string>) {
  const saved: Record<string, string> = {};
  const api = {
    fileTree: vi.fn(async () =>
      Object.keys(files).map((path) => ({ label: path.split("/").pop(), key: path })),
    ),
    readFile: vi.fn(async (_ws: string, node: { key: string }) => {
      const content = files[node.key];
      if (content === undefined) throw new Error(`no such file: ${node.key}`);
      return { fileName: node.key, label: node.key, content };
    }),
    saveFile: vi.fn(async (_ws: string, name: string, content: string) => {
      saved[name] = content;
    }),
  } as unknown as WorkspaceApi;
  return { api, saved };
}

const workspace = (key: string, flow: unknown, actions: unknown) => ({
  [flowPath(key)]: serialise(flow),
  [actionPath(key)]: serialise(actions),
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
    expect(order).toEqual([actionPath("loadFilesUF"), flowPath("loadFilesUF")]);
    expect(saved[flowPath("loadFilesUF")]).toBe(serialise(loadFilesFlow));
  });

  it("round-trips byte-for-byte, so a save with no edit is a no-op diff", async () => {
    const { store, saved } = storeFor(workspace("loadFilesUF", loadFilesFlow, loadFilesActions));
    await store.save(await store.load("loadFilesUF"));
    expect(saved[flowPath("loadFilesUF")]).toBe(serialise(loadFilesFlow));
    expect(saved[actionPath("loadFilesUF")]).toBe(serialise(loadFilesActions));
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

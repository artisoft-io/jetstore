import { describe, expect, it } from "vitest";
import { ApiClient } from "./client";
import {
  WorkspaceApi,
  decodeLabel,
  describeSaveWarnings,
  fileNameOf,
  walk,
  type WorkspaceNode,
} from "./workspace";

function node(partial: Partial<WorkspaceNode>): WorkspaceNode {
  return {
    key: "k",
    pageMatchKey: "",
    type: "file",
    size: 0,
    label: "f.jr",
    route_path: "",
    route_params: null,
    children: null,
    ...partial,
  };
}

function stub(queue: Array<{ status: number; body: unknown }>) {
  const calls: Array<{ url: string; body: any }> = [];
  const impl = (async (url: string, init?: RequestInit) => {
    calls.push({ url: String(url), body: init?.body ? JSON.parse(String(init.body)) : undefined });
    const next = queue.shift() ?? { status: 200, body: {} };
    return {
      ok: next.status < 300,
      status: next.status,
      text: async () => JSON.stringify(next.body),
    } as Response;
  }) as unknown as typeof fetch;
  return { impl, calls };
}

async function api(queue: Array<{ status: number; body: unknown }>) {
  const { impl, calls } = stub([{ status: 200, body: { token: "t", user_email: "a@b.c" } }, ...queue]);
  const client = new ApiClient("", impl);
  await client.login("a@b.c", "pw");
  return { ws: new WorkspaceApi(client), calls };
}

describe("fileNameOf", () => {
  it("prefers route_params.file_name", () => {
    const n = node({ route_params: { file_name: "jet_rules%2Fa.jr" }, pageMatchKey: "other" });
    expect(fileNameOf(n)).toBe("jet_rules%2Fa.jr");
  });

  it("falls back to pageMatchKey", () => {
    expect(fileNameOf(node({ pageMatchKey: "lookups%2Fb.jr" }))).toBe("lookups%2Fb.jr");
  });

  it("returns null for directories and sections", () => {
    expect(fileNameOf(node({ type: "dir", pageMatchKey: "jet_rules" }))).toBeNull();
    expect(fileNameOf(node({ type: "section", pageMatchKey: "s" }))).toBeNull();
  });
});

describe("decodeLabel", () => {
  it("decodes the escaped path the server issues", () => {
    expect(decodeLabel("jet_rules%2Fmain%2Frules.jr")).toBe("jet_rules/main/rules.jr");
  });

  it("returns the input unchanged when it will not decode", () => {
    expect(decodeLabel("100%")).toBe("100%");
  });
});

describe("walk", () => {
  it("visits every node depth-first", () => {
    const tree = [
      node({
        type: "dir",
        label: "jet_rules",
        children: [node({ label: "a.jr" }), node({ label: "b.jr" })],
      }),
      node({ label: "c.jr" }),
    ];
    expect([...walk(tree)].map((n) => n.label)).toEqual(["jet_rules", "a.jr", "b.jr", "c.jr"]);
  });
});

describe("WorkspaceApi", () => {
  it("requests the file tree with the shape the Go handler expects", async () => {
    const { ws, calls } = await api([
      { status: 200, body: { result_type: "workspace_file_structure", result_data: [] } },
    ]);
    await ws.fileTree("org1_ws");
    expect(calls[1]?.body).toMatchObject({
      action: "workspace_query_structure",
      fromClauses: [{ table: "workspace_file_structure" }],
      workspaceName: "org1_ws",
    });
  });

  it("returns an empty tree when the server answers with another result type", async () => {
    const { ws } = await api([{ status: 200, body: { result_type: "something_else" } }]);
    expect(await ws.fileTree("ws")).toEqual([]);
  });

  it("reads a file, passing file_name through without re-encoding", async () => {
    const { ws, calls } = await api([{ status: 200, body: { file_content: "rule text" } }]);
    const n = node({
      label: "rules.jr",
      route_params: { workspace_name: "ws", file_name: "jet_rules%2Frules.jr" },
    });
    const file = await ws.readFile("ws", n);

    expect(file.content).toBe("rule text");
    expect(file.fileName).toBe("jet_rules%2Frules.jr");
    expect(file.label).toBe("jet_rules/rules.jr");
    expect(calls[1]?.body.data[0].file_name).toBe("jet_rules%2Frules.jr");
    expect(calls[1]?.body.action).toBe("get_workspace_file_content");
  });

  /**
   * The runtime read, which is a different action and therefore a different
   * capability — `jetstore_read` rather than `workspace_ide`. Asserted at the
   * action name because that is the entire difference on the wire, and getting it
   * wrong would work in every test that stubs by path and fail for every user who
   * is not a knowledge engineer.
   */
  it("reads a document through the runtime action, not the editor's", async () => {
    const { ws, calls } = await api([{ status: 200, body: { file_content: "{}" } }]);
    const file = await ws.readWorkspaceDocument("ws", "user_flows/loadFilesUF.uf.json");

    expect(calls[1]?.body.action).toBe("get_workspace_document");
    expect(calls[1]?.body.data[0].file_name).toBe("user_flows%2FloadFilesUF.uf.json");
    expect(file.content).toBe("{}");
    expect(file.label).toBe("user_flows/loadFilesUF.uf.json");
  });

  it("treats a missing file_content as an empty file rather than failing", async () => {
    const { ws } = await api([{ status: 200, body: {} }]);
    const file = await ws.readFile("ws", node({ pageMatchKey: "a.jr" }));
    expect(file.content).toBe("");
  });

  it("refuses to read a directory", async () => {
    const { ws } = await api([]);
    await expect(ws.readFile("ws", node({ type: "dir", label: "d" }))).rejects.toThrow(/not a file/);
  });

  it("has no size limit — a multi-megabyte file loads like any other", async () => {
    // The Flutter client refused anything at or above 250,000 bytes. The largest
    // rule file in the corpus is ~1.18 MB, and must simply open.
    const big = "x".repeat(1_200_000);
    const { ws } = await api([{ status: 200, body: { file_content: big } }]);
    const file = await ws.readFile("ws", node({ pageMatchKey: "big.jr", size: 1_200_000 }));
    expect(file.content.length).toBe(1_200_000);
  });

  it("saves with the action and payload the server switches on", async () => {
    const { ws, calls } = await api([{ status: 200, body: {} }]);
    await ws.saveFile("ws", "jet_rules%2Fa.jr", "new text");
    expect(calls[1]?.body).toMatchObject({
      action: "save_workspace_file_content",
      workspaceName: "ws",
      data: [{ file_name: "jet_rules%2Fa.jr", file_content: "new text" }],
    });
  });

  it("returns the warnings a save answers with, and none from a server that sends none", async () => {
    const warning = {
      severity: "warning",
      code: "cpipes_config_unknown_field",
      message: 'ignored when the pipeline runs: unknown field "output_table"',
      path: "/output_table",
    };
    const { ws } = await api([
      { status: 200, body: { file_name: "a.pc.json", warnings: [warning] } },
      { status: 200, body: { file_name: "a.pc.json", warnings: [] } },
      { status: 200, body: { file_name: "a.pc.json" } },
    ]);
    expect(await ws.saveFile("ws", "a.pc.json", "{}")).toEqual({ warnings: [warning] });
    expect(await ws.saveFile("ws", "a.pc.json", "{}")).toEqual({ warnings: [] });
    expect(await ws.saveFile("ws", "a.pc.json", "{}")).toEqual({ warnings: [] });
  });

  it("maps the workspace registry query onto summaries", async () => {
    const { ws } = await api([
      { status: 200, body: { rows: [["org1_ws", "git@x", "org1_ai"], ["jets_ws", "git@y", "jets_ai"]] } },
    ]);
    const list = await ws.listWorkspaces();
    expect(list).toEqual([
      { name: "org1_ws", uri: "git@x", branch: "org1_ai" },
      { name: "jets_ws", uri: "git@y", branch: "jets_ai" },
    ]);
  });
});

describe("describeSaveWarnings", () => {
  it("names the file, then each warning at its coordinate, as the server writes a refusal", () => {
    const text = describeSaveWarnings("a.pc.json", [
      { severity: "warning", code: "c", message: "at a pointer", path: "/a/b" },
      { severity: "warning", code: "c", message: "at a line", line: 12 },
      { severity: "warning", code: "c", message: "at both", path: "/a", line: 3 },
      { severity: "warning", code: "c", message: "about the file" },
    ]);
    expect(text.split("\n")).toEqual([
      "Saved a.pc.json, with 4 warnings:",
      "  /a/b: at a pointer",
      "  line 12: at a line",
      "  /a:3: at both",
      "  about the file",
    ]);
    expect(describeSaveWarnings("x.sql", [{ severity: "warning", code: "c", message: "m" }])).toBe(
      "Saved x.sql, with 1 warning:\n  m",
    );
  });
});

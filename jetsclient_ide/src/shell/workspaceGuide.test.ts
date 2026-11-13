/**
 * The *Workspace guide* button's logic: the manifest's mapping from the file open
 * in the IDE to its page, the landing page for a file with none, and no guide
 * when the manifest is missing or describes another workspace.
 */

import { describe, expect, it, vi } from "vitest";

import { queryEscape } from "../api/workspace";
import type { HelpTab } from "./helpTopics";
import {
  WORKSPACE_MANIFEST_PATH,
  describesWorkspace,
  guidePageFor,
  openWorkspaceGuide,
  parseWorkspaceManifest,
  readWorkspaceManifest,
  workspacePathOf,
  type WorkspaceManifest,
} from "./workspaceGuide";

/** A manifest of the generator's shape, for `jets_ws`. */
const MANIFEST: WorkspaceManifest = {
  workspace: "jets_ws",
  index: "/docs/workspace/",
  pages: {
    "data_model/jets_model.jr": "/docs/workspace/reference/domain/jets-model/",
    "jet_rules/process/process_main.jr": "/docs/workspace/reference/rules/process-main/",
    "jet_rules/process/process_support.jr": "/docs/workspace/reference/rules/",
    "lookups/code_lookups.jr": "/docs/workspace/reference/lookups/",
    "docs/customisation/observe.md": "/docs/workspace/customisation/observe/",
  },
};

/** The IDE's key for a file: the server's `file_name`, Go's `url.QueryEscape` of the path. */
const key = (path: string) => queryEscape(path);

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function fakeTab(href = "about:blank") {
  const close = vi.fn<() => void>();
  const focus = vi.fn<() => void>();
  const tab: HelpTab & { close: typeof close; focus: typeof focus } = {
    opener: "the app",
    location: { href },
    close,
    focus,
  };
  return tab;
}

describe("workspacePathOf", () => {
  it("decodes the IDE's escaped file name to the workspace-relative path", () => {
    expect(workspacePathOf("jet_rules%2Fprocess%2Fprocess_main.jr")).toBe(
      "jet_rules/process/process_main.jr",
    );
    expect(workspacePathOf("data_model%2Fmy+model.jr")).toBe("data_model/my model.jr");
  });

  it("normalises backslashes and a leading ./ or /", () => {
    expect(workspacePathOf("./lookups/code_lookups.jr")).toBe("lookups/code_lookups.jr");
    expect(workspacePathOf("/lookups\\code_lookups.jr")).toBe("lookups/code_lookups.jr");
  });
});

describe("guidePageFor", () => {
  it("maps a data-model file to its domain page", () => {
    expect(guidePageFor(MANIFEST, key("data_model/jets_model.jr"))).toBe(
      "/docs/workspace/reference/domain/jets-model/",
    );
  });

  it("maps a main rule file to its rules page", () => {
    expect(guidePageFor(MANIFEST, key("jet_rules/process/process_main.jr"))).toBe(
      "/docs/workspace/reference/rules/process-main/",
    );
  });

  it("maps a lookup's source file to the lookups page", () => {
    expect(guidePageFor(MANIFEST, key("lookups/code_lookups.jr"))).toBe(
      "/docs/workspace/reference/lookups/",
    );
  });

  it("maps a written page under docs/ to its own page", () => {
    expect(guidePageFor(MANIFEST, key("docs/customisation/observe.md"))).toBe(
      "/docs/workspace/customisation/observe/",
    );
  });

  it("opens the landing page for a file the manifest does not name", () => {
    expect(guidePageFor(MANIFEST, key("pipes_config/load.pc.json"))).toBe("/docs/workspace/");
  });

  it("opens the landing page when no file is open", () => {
    expect(guidePageFor(MANIFEST, null)).toBe("/docs/workspace/");
  });

  it("never opens a page outside /docs/", () => {
    const hostile = { ...MANIFEST, pages: { "a.jr": "//evil.example/", "b.jr": "https://x/" } };
    expect(guidePageFor(hostile, key("a.jr"))).toBe("/docs/workspace/");
    expect(guidePageFor(hostile, key("b.jr"))).toBe("/docs/workspace/");
  });
});

describe("the manifest", () => {
  it("parses the generator's shape, and tolerates an added field", () => {
    expect(parseWorkspaceManifest({ ...MANIFEST, generated: "2026-11-13" })).not.toBeNull();
  });

  it("refuses a body that is not a manifest", () => {
    expect(parseWorkspaceManifest({ workspace: "jets_ws" })).toBeNull();
    expect(parseWorkspaceManifest({ ...MANIFEST, index: "https://elsewhere/" })).toBeNull();
    expect(parseWorkspaceManifest("<html>")).toBeNull();
  });

  it("is read from the site, same origin", async () => {
    const fetchImpl = vi.fn(async () => jsonResponse(MANIFEST));
    expect(await readWorkspaceManifest(fetchImpl as unknown as typeof fetch)).toEqual(MANIFEST);
    expect(fetchImpl).toHaveBeenCalledWith(
      WORKSPACE_MANIFEST_PATH,
      expect.objectContaining({ credentials: "same-origin" }),
    );
  });

  it("is missing on a 404, a refusal, a body that is not JSON, or no answer", async () => {
    const read = (f: () => Promise<Response>) => readWorkspaceManifest(f as unknown as typeof fetch);
    expect(await read(async () => new Response("<!doctype html>", { status: 404 }))).toBeNull();
    expect(await read(async () => new Response("", { status: 401 }))).toBeNull();
    expect(await read(async () => new Response("<!doctype html>", { status: 200 }))).toBeNull();
    expect(
      await read(async () => {
        throw new TypeError("failed to fetch");
      }),
    ).toBeNull();
  });

  it("describes only the workspace it names", () => {
    expect(describesWorkspace(MANIFEST, "jets_ws")).toBe(true);
    expect(describesWorkspace(MANIFEST, "another_ws")).toBe(false);
    expect(describesWorkspace(MANIFEST, "")).toBe(false);
    expect(describesWorkspace(null, "jets_ws")).toBe(false);
  });
});

describe("openWorkspaceGuide", () => {
  it("opens the tab first, then the session, then points the tab at the file's page", async () => {
    const order: string[] = [];
    const tab = fakeTab();
    const message = await openWorkspaceGuide({
      workspace: "jets_ws",
      fileName: key("data_model/jets_model.jr"),
      openTab: () => {
        order.push("tab");
        return tab;
      },
      openDocsSession: async () => {
        order.push("session");
      },
      readManifest: async () => {
        order.push("manifest");
        return MANIFEST;
      },
    });
    expect(message).toBeNull();
    expect(order).toEqual(["tab", "session", "manifest"]);
    expect(tab.location.href).toBe("/docs/workspace/reference/domain/jets-model/");
    expect(tab.opener).toBeNull();
    expect(tab.focus).toHaveBeenCalled();
  });

  it("opens the landing page for an unmapped file", async () => {
    const tab = fakeTab();
    await openWorkspaceGuide({
      workspace: "jets_ws",
      fileName: key("reports/summary.sql"),
      openTab: () => tab,
      openDocsSession: async () => {},
      readManifest: async () => MANIFEST,
    });
    expect(tab.location.href).toBe("/docs/workspace/");
  });

  it("reports a blocked tab", async () => {
    const message = await openWorkspaceGuide({
      workspace: "jets_ws",
      fileName: null,
      openTab: () => null,
      openDocsSession: async () => {},
      readManifest: async () => MANIFEST,
    });
    expect(message).toMatch(/blocked the documentation tab/);
  });

  it("closes a tab it opened when the session is refused, and reports why", async () => {
    const tab = fakeTab();
    const message = await openWorkspaceGuide({
      workspace: "jets_ws",
      fileName: null,
      openTab: () => tab,
      openDocsSession: async () => {
        throw new Error("no documentation");
      },
      readManifest: async () => MANIFEST,
    });
    expect(message).toBe("no documentation");
    expect(tab.close).toHaveBeenCalled();
  });

  it("reports a missing manifest, and leaves an existing tab alone", async () => {
    const tab = fakeTab("http://localhost/docs/user/");
    const message = await openWorkspaceGuide({
      workspace: "jets_ws",
      fileName: null,
      openTab: () => tab,
      openDocsSession: async () => {},
      readManifest: async () => null,
    });
    expect(message).toBe("The documentation has no guide for jets_ws.");
    expect(tab.close).not.toHaveBeenCalled();
    expect(tab.location.href).toBe("http://localhost/docs/user/");
  });

  it("reports a manifest naming another workspace", async () => {
    const tab = fakeTab();
    const message = await openWorkspaceGuide({
      workspace: "another_ws",
      fileName: null,
      openTab: () => tab,
      openDocsSession: async () => {},
      readManifest: async () => MANIFEST,
    });
    expect(message).toBe("The documentation has no guide for another_ws.");
    expect(tab.close).toHaveBeenCalled();
  });
});

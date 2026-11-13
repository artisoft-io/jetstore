/**
 * @vitest-environment jsdom
 *
 * **The Workspace IDE's *Workspace guide* button.** Shown only when the
 * installation's documentation has a workspace guide and the guide's manifest
 * names the workspace the IDE has open; pressed, it opens the guide's page for
 * the file in the editor, in the documentation tab *Help* uses.
 *
 * The manifest is read with the global `fetch` (same origin, authorised by the
 * documentation cookie), so it is stubbed separately from the `ApiClient`'s.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { AppShell } from "../shell/AppShell";
import { DOCS_TAB_NAME } from "../shell/helpTopics";
import { WORKSPACE_MANIFEST_PATH } from "../shell/workspaceGuide";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

const MANIFEST = {
  workspace: "ws",
  index: "/docs/workspace/",
  pages: { "jet_rules/a.jr": "/docs/workspace/reference/rules/a/" },
};

/** The app's own server: sign-in, the documentation session, the workspace actions. */
function stubServer(docsSessions: string[]) {
  return vi.fn(async (url: string | URL, init?: RequestInit) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Ada",
          user_email: "ada@example.com",
          is_admin: false,
          capabilities: [WORKSPACE_IDE],
        }),
        { status: 200 },
      );
    }
    if (String(url) === "/docsSession") {
      docsSessions.push(String(url));
      return new Response(null, { status: 204 });
    }
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    switch (body["action"]) {
      case "raw_query":
        return new Response(JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"]] }), {
          status: 200,
        });
      case "workspace_query_structure":
        return new Response(
          JSON.stringify({
            result_type: "workspace_file_structure",
            result_data: [
              {
                key: "jet_rules",
                pageMatchKey: "jet_rules",
                type: "dir",
                size: 0,
                label: "jet_rules",
                children: [
                  {
                    key: "jet_rules/a.jr",
                    pageMatchKey: "jet_rules%2Fa.jr",
                    type: "file",
                    size: 12,
                    label: "jet_rules/a.jr",
                    route_params: { workspace_name: "ws", file_name: "jet_rules%2Fa.jr" },
                    children: null,
                  },
                ],
              },
            ],
          }),
          { status: 200 },
        );
      case "get_workspace_file_content":
        return new Response(JSON.stringify({ file_content: "# a rule file\n" }), { status: 200 });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

/** The documentation site's answer to the manifest, through the global `fetch`. */
function stubSite(answer: () => Response) {
  const site = vi.fn(async (url: string | URL) => {
    if (String(url) === WORKSPACE_MANIFEST_PATH) return answer();
    return new Response("", { status: 404 });
  });
  vi.stubGlobal("fetch", site);
  return site;
}

async function mount() {
  const docsSessions: string[] = [];
  const api = new ApiClient("", stubServer(docsSessions));
  await api.login("ada@example.com", "pw");
  render(
    <MemoryRouter initialEntries={["/workspace"]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByTitle("jet_rules/a.jr");
  return { docsSessions };
}

const guideButton = () => screen.queryByRole("button", { name: "Workspace guide" });

describe("the Workspace guide button", () => {
  it("is absent when the documentation has no workspace guide", async () => {
    const site = stubSite(() => new Response("<!doctype html>", { status: 404 }));
    const { docsSessions } = await mount();
    await waitFor(() => expect(site).toHaveBeenCalledWith(WORKSPACE_MANIFEST_PATH, expect.anything()));
    expect(docsSessions.length).toBeGreaterThan(0);
    expect(guideButton()).toBeNull();
  });

  it("is absent when the guide describes another workspace", async () => {
    const site = stubSite(() => new Response(JSON.stringify({ ...MANIFEST, workspace: "other_ws" })));
    await mount();
    await waitFor(() => expect(site).toHaveBeenCalled());
    // Let the manifest's answer settle before asserting the absence.
    await new Promise((r) => setTimeout(r, 0));
    expect(guideButton()).toBeNull();
  });

  it("is present when the guide describes the open workspace, and opens the open file's page", async () => {
    stubSite(() => new Response(JSON.stringify(MANIFEST)));
    await mount();
    const button = await screen.findByRole("button", { name: "Workspace guide" });

    fireEvent.click(screen.getByTitle("jet_rules/a.jr"));
    await screen.findByRole("button", { name: "Close jet_rules/a.jr" });

    const tab = { opener: "the app" as unknown, location: { href: "about:blank" }, close: vi.fn(), focus: vi.fn() };
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    fireEvent.click(button);

    expect(open).toHaveBeenCalledWith("", DOCS_TAB_NAME);
    await waitFor(() => expect(tab.location.href).toBe("/docs/workspace/reference/rules/a/"));
    expect(tab.opener).toBeNull();
  });

  it("opens the guide's landing page with no file open", async () => {
    stubSite(() => new Response(JSON.stringify(MANIFEST)));
    await mount();
    const button = await screen.findByRole("button", { name: "Workspace guide" });

    const tab = { opener: "the app" as unknown, location: { href: "about:blank" }, close: vi.fn(), focus: vi.fn() };
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    fireEvent.click(button);

    await waitFor(() => expect(tab.location.href).toBe("/docs/workspace/"));
  });
});

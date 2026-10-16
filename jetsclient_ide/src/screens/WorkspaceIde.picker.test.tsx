/**
 * @vitest-environment jsdom
 *
 * **Which workspace the IDE opens on, and the toolbar that acts on it.**
 * 2026-10-11.
 *
 * On the bare `/workspace` route the picker opened on the first workspace in the
 * list every time, so a user who had picked another one and left the screen came
 * back to the wrong workspace. It now opens on the one picked earlier in the
 * session, else on the deployment's `WORKSPACE`, and remembers a change in
 * session storage. The route still wins when it names one.
 *
 * And the registry's workspace actions were drawn under the picker, enabled from
 * the open workspace's registry row rather than from a selection. They went back
 * to the registry on 2026-10-15 at Michel's request, because a compile started
 * from the IDE showed no progress there (the compile's status is on the
 * registry's table); the toolbar keeps Refresh alone.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { AppShell } from "../shell/AppShell";
import { PICKED_WORKSPACE_KEY, WorkspaceIde, WORKSPACE_IDE, defaultWorkspace } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());
afterEach(cleanup);

/** Three workspaces, in the order `listWorkspaces` sorts them. */
const WORKSPACES = [
  ["alpha_ws", "git@x:alpha", "main"],
  ["jets_ws", "git@x:jets", "jets_ai"],
  ["zeta_ws", "git@x:zeta", "main"],
];

function registryRow(name: string, status: string): (string | null)[] {
  return ["1", name, "main", "feat", `git@x:${name}`, "", status, "log", "u@x", "now"];
}

function stubServer(active: string | null) {
  const posts: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: [WORKSPACE_IDE],
        }),
        { status: 200 },
      );
    }
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push(body);
    switch (body["action"]) {
      case "raw_query":
        return new Response(JSON.stringify({ rows: WORKSPACES }), { status: 200 });
      case "get_workspace_uri":
        if (active === null) return new Response(JSON.stringify({ error: "no" }), { status: 500 });
        return new Response(
          JSON.stringify({ workspace_name: active, workspace_branch: "jets_ai", workspace_uri: "" }),
          { status: 200 },
        );
      case "workspace_query_structure":
        return new Response(JSON.stringify({ result_type: "workspace_file_structure", result_data: [] }), {
          status: 200,
        });
      case "workspace_read": {
        const where = (body["whereClauses"] as { column?: string; values?: string[] }[] | undefined)?.find(
          (w) => w.column === "workspace_name",
        );
        const name = where?.values?.[0] ?? "";
        // `zeta_ws` is mid-compile, which gated the toolbar's workspace actions
        // while it had them.
        const rows = name === "" ? [] : [registryRow(name, name === "zeta_ws" ? "in progress" : "modified")];
        return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
      }
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

async function mount(active: string | null, at = "/workspace") {
  const { fetchImpl, posts } = stubServer(active);
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <MemoryRouter initialEntries={[at]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
          <Route path="workspaces/:workspace_name/home" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  return { posts };
}

const picker = () => screen.getByLabelText("Workspace") as HTMLSelectElement;
const treeReadFor = (posts: Record<string, unknown>[], name: string) =>
  posts.some((p) => p["action"] === "workspace_query_structure" && p["workspaceName"] === name);

describe("the workspace the IDE opens on", () => {
  it("is the deployment's WORKSPACE, not the first in the list", async () => {
    const { posts } = await mount("jets_ws");
    await waitFor(() => expect(picker().value).toBe("jets_ws"));
    await waitFor(() => expect(treeReadFor(posts, "jets_ws")).toBe(true));
    expect(treeReadFor(posts, "alpha_ws")).toBe(false);
  });

  it("is the one picked earlier this session, and a change is remembered", async () => {
    await mount("jets_ws");
    await waitFor(() => expect(picker().value).toBe("jets_ws"));
    fireEvent.change(picker(), { target: { value: "zeta_ws" } });
    await waitFor(() => expect(window.sessionStorage.getItem(PICKED_WORKSPACE_KEY)).toBe("zeta_ws"));

    // Leaving the screen and coming back.
    cleanup();
    const { posts } = await mount("jets_ws");
    await waitFor(() => expect(picker().value).toBe("zeta_ws"));
    expect(treeReadFor(posts, "jets_ws")).toBe(false);
  });

  it("follows the route when the route names one, and remembers it", async () => {
    await mount("jets_ws", "/workspaces/alpha_ws/home");
    await waitFor(() => expect(picker().value).toBe("alpha_ws"));
    await waitFor(() => expect(window.sessionStorage.getItem(PICKED_WORKSPACE_KEY)).toBe("alpha_ws"));
  });

  it("falls back to the first when the deployment's workspace cannot be read", async () => {
    await mount(null);
    await waitFor(() => expect(picker().value).toBe("alpha_ws"));
  });

  it("never selects a workspace that is not in the list", () => {
    const list = WORKSPACES.map(([name, uri, branch]) => ({ name: name!, uri: uri!, branch: branch! }));
    expect(defaultWorkspace(list, "gone_ws", "jets_ws")).toBe("jets_ws");
    expect(defaultWorkspace(list, null, "gone_ws")).toBe("alpha_ws");
    expect(defaultWorkspace([], "x", "y")).toBe("");
  });
});

/**
 * Two cases here observed the toolbar's workspace actions and their gates, and
 * their premise went with the buttons on 2026-10-15: the gates are asserted on
 * the registry now (`WorkspaceRegistry.test.tsx`). What is left is Refresh, and
 * the registry row the toolbar still reads for the workspace the picker names.
 */
describe("the workspace toolbar in the IDE", () => {
  it("draws Refresh alone, and none of the workspace actions", async () => {
    await mount("jets_ws");
    await screen.findByRole("button", { name: "Refresh" });
    for (const label of [
      "Export Client Config", "Load Client Config", "Compile Workspace", "Rules Unit Tests",
      "Commit & Push Workspace", "Push Only", "Pull Workspace", "Git Status", "Git Command",
      "View Last Log",
    ]) {
      expect(screen.queryByRole("button", { name: label }), label).toBeNull();
    }
  });

  it("re-reads the row when the picker changes", async () => {
    const { posts } = await mount("jets_ws");
    const rowReadFor = (name: string) =>
      posts.some(
        (p) =>
          p["action"] === "workspace_read" &&
          (p["whereClauses"] as { column?: string; values?: string[] }[] | undefined)?.some(
            (w) => w.column === "workspace_name" && w.values?.[0] === name,
          ),
      );
    await waitFor(() => expect(rowReadFor("jets_ws")).toBe(true));
    expect(rowReadFor("zeta_ws")).toBe(false);
    fireEvent.change(picker(), { target: { value: "zeta_ws" } });
    await waitFor(() => expect(rowReadFor("zeta_ws")).toBe(true));
  });
});

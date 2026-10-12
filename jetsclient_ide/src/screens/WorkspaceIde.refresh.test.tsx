/**
 * @vitest-environment jsdom
 *
 * **The Workspace IDE toolbar's Refresh re-reads what the IDE shows**: the file
 * tree, the toolbar's registry row and the changes table. 2026-10-12, Michel:
 * files that arrive outside the IDE -- a pull, a new unit test directory -- did
 * not appear until the page was reloaded. Open tabs stay open, as they do when a
 * compiled view adds a file.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());
afterEach(cleanup);

vi.mock("../editor/Editor", () => ({
  Editor: ({ fileName, content }: EditorProps) => <textarea aria-label={`Edit ${fileName}`} defaultValue={content} />,
}));

const FILE_A = "jet_rules/a.jr";
const FILE_B = "jet_rules/b.jr";

function fileNode(name: string) {
  return {
    key: name,
    pageMatchKey: name,
    type: "file",
    size: 12,
    label: name,
    route_params: { workspace_name: "ws", file_name: encodeURIComponent(name) },
    children: null,
  };
}

/** The tree holds a.jr, and b.jr from the second read on: a file pulled in. */
function stubServer() {
  const posts: Record<string, unknown>[] = [];
  let treeReads = 0;
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
        return new Response(JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"]] }), { status: 200 });
      case "workspace_query_structure": {
        treeReads++;
        const files = treeReads === 1 ? [fileNode(FILE_A)] : [fileNode(FILE_A), fileNode(FILE_B)];
        return new Response(
          JSON.stringify({
            result_type: "workspace_file_structure",
            result_data: [
              { key: "jet_rules", pageMatchKey: "jet_rules", type: "dir", size: 0, label: "jet_rules", children: files },
            ],
          }),
          { status: 200 },
        );
      }
      case "get_workspace_file_content":
        return new Response(JSON.stringify({ file_content: "# a rule file\n" }), { status: 200 });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

const registryReads = (posts: Record<string, unknown>[]) =>
  posts.filter(
    (p) =>
      p["action"] === "workspace_read" &&
      (p["whereClauses"] as { table?: string }[] | undefined)?.some((w) => w.table === "workspace_registry"),
  ).length;

const changesReads = (posts: Record<string, unknown>[]) =>
  posts.filter((p) =>
    (p["fromClauses"] as { table?: string }[] | undefined)?.some((f) => f.table === "workspace_changes"),
  ).length;

async function mount() {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <MemoryRouter initialEntries={["/workspace"]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByTitle(FILE_A);
  await waitFor(() => expect(registryReads(posts)).toBeGreaterThan(0));
  return { posts };
}

/** The toolbar's Refresh, not the changes table's own. */
const toolbarRefresh = () =>
  within(screen.getByLabelText("Workspace actions")).getByRole("button", { name: "Refresh" });

describe("Refresh on the Workspace IDE toolbar", () => {
  it("re-reads the tree and the toolbar's row, and keeps the open tab", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByTitle(FILE_A));
    await screen.findByRole("button", { name: `Close ${FILE_A}` });
    expect(screen.queryByTitle(FILE_B)).toBeNull();
    const rowReadsBefore = registryReads(posts);

    expect(toolbarRefresh().hasAttribute("disabled")).toBe(false);
    fireEvent.click(toolbarRefresh());

    expect(await screen.findByTitle(FILE_B)).toBeTruthy();
    await waitFor(() => expect(registryReads(posts)).toBe(rowReadsBefore + 1));
    expect(screen.getByRole("button", { name: `Close ${FILE_A}` })).toBeTruthy();
  });

  it("re-reads the changes table shown when no tab is open", async () => {
    const { posts } = await mount();
    await waitFor(() => expect(changesReads(posts)).toBeGreaterThan(0));
    const before = changesReads(posts);

    fireEvent.click(toolbarRefresh());

    await waitFor(() => expect(changesReads(posts)).toBeGreaterThan(before));
  });
});

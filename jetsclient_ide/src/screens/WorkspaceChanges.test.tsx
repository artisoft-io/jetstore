/**
 * @vitest-environment jsdom
 *
 * The workspace home's base content. What is asserted here is the **request the
 * table sent and the request the buttons sent**, not the rows drawn — the same
 * rule `TableScreen.test.tsx` states, and for the same reason: without the
 * workspace filter the table shows every workspace's changes and looks entirely
 * correct.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { WorkspaceChanges } from "./WorkspaceChanges";

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

const rows = [["7", "ws1", "0", "jet_rules/main.jr", "jetrules", "", "michel@artisoft.io", "2026-09-05"]];

function stubServer() {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ path, body });
    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: ["workspace_ide"],
        }),
        { status: 200 },
      );
    }
    if (body["action"] === "read") {
      return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
    }
    return new Response(JSON.stringify({}), { status: 200 });
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

async function mount(workspace = "ws1") {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <WorkspaceChanges api={api} workspace={workspace} />
      </NotificationsProvider>
    </ApiProvider>,
  );
  await waitFor(() => expect(posts.filter((p) => p.body["action"] === "read").length).toBe(1));
  return { posts };
}

const reads = (posts: Posted[]) => posts.filter((p) => p.body["action"] === "read");
const actionsOf = (posts: Posted[], action: string) =>
  posts.filter((p) => p.body["action"] === action);

afterEach(cleanup);
beforeEach(() => {
  vi.spyOn(window, "confirm").mockReturnValue(true);
});

describe("the workspace home's changes table", () => {
  it("filters by the workspace it was given, which is the whole mechanism", async () => {
    const { posts } = await mount("ws1");
    const where = reads(posts)[0]!.body["whereClauses"] as { column: string; value: unknown }[];
    // Without this the table shows every workspace's changes and looks correct.
    expect(where.some((w) => w.column === "workspace_name")).toBe(true);
  });

  it("queries jetsapi.workspace_changes", async () => {
    const { posts } = await mount();
    const from = reads(posts)[0]!.body["fromClauses"] as { schema: string; table: string }[];
    expect(from[0]).toMatchObject({ schema: "jetsapi", table: "workspace_changes" });
  });

  it("is headed Workspace Changes, not the Dart's copy-pasted label", async () => {
    await mount();
    // `CORPUS_CORRECTIONS` — the Dart said "Workspace Registry" on the changes
    // table, and the corpus faithfully records that it did.
    await screen.findByRole("heading", { name: "Workspace Changes" });
  });

  it("reverts the selected change, sending only what the handler reads", async () => {
    const { posts } = await mount();
    fireEvent.click(await screen.findByLabelText("Select row 1"));
    fireEvent.click(screen.getByRole("button", { name: "Delete/Revert Changes" }));
    await waitFor(() => expect(actionsOf(posts, "delete_workspace_changes").length).toBe(1));
    const body = actionsOf(posts, "delete_workspace_changes")[0]!.body;
    expect(body["workspaceName"]).toBe("ws1");
    // `DeleteWorkspaceChanges` reads `file_name` off each row and the workspace
    // off the envelope, and reads nothing else — the Dart also sent `key`, `oid`
    // and `user_email`, and the handler ignores all three.
    expect(body["data"]).toEqual([{ file_name: "jet_rules/main.jr" }]);
  });

  it("asks before reverting, and sends nothing when refused", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const { posts } = await mount();
    fireEvent.click(await screen.findByLabelText("Select row 1"));
    fireEvent.click(screen.getByRole("button", { name: "Delete/Revert Changes" }));
    await waitFor(() => expect(window.confirm).toHaveBeenCalled());
    expect(actionsOf(posts, "delete_workspace_changes")).toEqual([]);
  });

  it("reverts every change with the workspace alone", async () => {
    const { posts } = await mount();
    fireEvent.click(screen.getByRole("button", { name: "Delete/Revert ALL Changes" }));
    await waitFor(() => expect(actionsOf(posts, "delete_all_workspace_changes").length).toBe(1));
    // The handler reads only the workspace off the envelope; there is no
    // selection to send and the button is not gated on one.
    expect(actionsOf(posts, "delete_all_workspace_changes")[0]!.body["workspaceName"]).toBe("ws1");
  });
});

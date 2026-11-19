/**
 * @vitest-environment jsdom
 *
 * **Ticking a row of the workspace registry asks the server nothing.**
 *
 * Until November 2026 it did. The table publishes its selected row's
 * `workspace_name` into form state, and `makeQuery` read `workspace_name` back
 * out of form state into the request's `workspaceName`, so a tick changed the
 * table's own request. `useDataTable` fetches whenever its request changes, so
 * the tick sent `workspace_read` again; the reply replaced the rows and cleared
 * the selection, and the restore from form state ticked the box again a moment
 * later. A user saw the box flicker off and back on, and a browser test that read
 * the box between the two failed.
 *
 * Both halves are asserted here: no request follows a tick, and the box is never
 * set unchecked after it. The second is watched on the input element itself, by
 * recording every value React writes to its `checked` property, because the
 * flicker ends checked and a final read cannot see it.
 *
 * And a real refresh still asks: the Refresh button and a heading's sort.
 */

import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { PromptsProvider } from "../shell/prompts";
import { setActiveWorkspace } from "../actions/registry";
import { WorkspaceRegistry } from "./WorkspaceRegistry";

afterEach(() => {
  cleanup();
  setActiveWorkspace({ name: "", branch: "", uri: "" });
});

/** Rows of `jetsapi.workspace_registry`, positional as the server returns them. */
const rows: (string | null)[][] = [
  ["1", "acme_ws", "main", "feat", "git@example/acme", "Acme", "modified", "", "u@x", "now"],
  ["2", "globex_ws", "main", "feat", "git@example/globex", "Globex", "modified", "", "u@x", "now"],
];

async function mount() {
  const reads: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({ token: "t0", name: "A", user_email: "a@x", is_admin: false, capabilities: ["workspace_ide"] }),
        { status: 200 },
      );
    }
    if (body["action"] === "get_workspace_uri") {
      return new Response(
        JSON.stringify({ workspace_name: "jets_ws", workspace_branch: "jets_ai", workspace_uri: "git@example/jets", workspace_file_key_label_re: "" }),
        { status: 200 },
      );
    }
    if (body["action"] === "workspace_read") {
      reads.push(body);
      return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
    }
    return new Response(JSON.stringify({ error: "unexpected" }), { status: 422 });
  }) as unknown as typeof fetch;

  const api = new ApiClient("", fetchImpl);
  await api.login("a@x", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <PromptsProvider>
          <MemoryRouter initialEntries={["/workspaces"]}>
            <Routes>
              <Route path="/workspaces" element={<WorkspaceRegistry api={api} />} />
            </Routes>
          </MemoryRouter>
        </PromptsProvider>
      </NotificationsProvider>
    </ApiProvider>,
  );
  await screen.findByText("acme_ws");
  await settle();
  return { reads };
}

/** Lets every pending fetch, reply and effect run. */
async function settle() {
  for (let i = 0; i < 5; i++) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  }
}

/** The row's checkbox, with every value React writes to `checked` recorded. */
function watchedBox(name: string) {
  const box = within(screen.getByText(name).closest("tr")!).getByRole("checkbox") as HTMLInputElement;
  const written: boolean[] = [];
  const own = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "checked")!;
  Object.defineProperty(box, "checked", {
    configurable: true,
    get() {
      return own.get!.call(this) as boolean;
    },
    set(value: boolean) {
      written.push(value);
      own.set!.call(this, value);
    },
  });
  return { box, written };
}

describe("the workspace registry's tick", () => {
  it("sends no request, and the box stays checked", async () => {
    const { reads } = await mount();
    const before = reads.length;
    const { box, written } = watchedBox("acme_ws");

    fireEvent.click(box);
    expect(box.checked).toBe(true);
    await settle();

    expect(reads.length - before).toBe(0);
    expect(written).not.toContain(false);
    expect(box.checked).toBe(true);
  });

  it("moves the tick to another row without a request", async () => {
    const { reads } = await mount();
    const before = reads.length;
    const acme = watchedBox("acme_ws");
    const globex = watchedBox("globex_ws");

    fireEvent.click(acme.box);
    await settle();
    // React writes the unchecked state while the other row is ticked; from the
    // second tick on, nothing may write it.
    globex.written.length = 0;
    fireEvent.click(globex.box);
    await settle();

    expect(reads.length - before).toBe(0);
    expect(globex.written).not.toContain(false);
    expect(globex.box.checked).toBe(true);
    // Single select: the first row's tick went when the second was ticked.
    expect(acme.box.checked).toBe(false);
  });

  it("still asks on Refresh, and keeps the tick through it", async () => {
    const { reads } = await mount();
    const { box } = watchedBox("acme_ws");
    fireEvent.click(box);
    await settle();
    const before = reads.length;

    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    await settle();

    expect(reads.length - before).toBe(1);
  });

  it("still asks when a heading sorts the table", async () => {
    const { reads } = await mount();
    const before = reads.length;

    fireEvent.click(screen.getByRole("button", { name: "Workspace Name" }));
    await settle();

    expect(reads.length - before).toBe(1);
    expect(reads.at(-1)!["sortColumn"]).toBe("workspace_name");
  });
});

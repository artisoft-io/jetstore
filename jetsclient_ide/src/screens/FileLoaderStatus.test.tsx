/**
 * @vitest-environment jsdom
 *
 * The file loader status screen. Task D.10, from **I-260**.
 *
 * **Three of these five cases are `Home.test.tsx`'s, moved rather than written.**
 * The table came off that screen and its properties came with it: the two display
 * filters that the translation once conflated, and the client filter that reaches
 * a table with a `client` column. **A table that leaves a screen with a filter is
 * a table that can arrive on the next one without it**, and nothing about the move
 * would have said so — `useTableBinding` supplies the client store by default
 * (D.3), so the assertion is the only thing that distinguishes *supplied* from
 * *never asked for*.
 *
 * On `Home.test.tsx`'s shape, which is on `WorkspaceRegistry.test.tsx`'s:
 * everything below the api client is real and the only stub is `fetch`.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { resetSelectedClient, setSelectedClient } from "../shell/selectedClient";
import { FileLoaderStatus } from "./FileLoaderStatus";

afterEach(() => {
  cleanup();
  resetSelectedClient();
});

/**
 * `input_loader_status` rows, positional as the server returns them.
 *
 * Column 7 is `table_name` and column 11 is `session_id` — the two indices
 * `viewDomainTable` names in `navigationParams`, so getting them wrong here is
 * what the navigation assertion would catch.
 */
const loaderRows: (string | null)[][] = [
  [
    "1", "acme", "acme_org", "claim", "2026", "8", "25", "claim_staging", "9000",
    "completed", "s3://in/claims.csv", "sess-1", "0",
    "File contains 0 bad rows,recovered error: a column was short", "u@x", "now",
  ],
];

interface Posted {
  body: Record<string, unknown>;
}

function stubServer() {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ body });
    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: ["run_pipelines"],
        }),
        { status: 200 },
      );
    }
    if (body["action"] === "read") {
      return new Response(
        JSON.stringify({ rows: loaderRows, totalRowCount: loaderRows.length }),
        { status: 200 },
      );
    }
    return new Response(
      JSON.stringify({ error: `unexpected action ${String(body["action"])}` }),
      { status: 422 },
    );
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

async function mount() {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <MemoryRouter initialEntries={["/fileLoaderStatus"]}>
          <Routes>
            <Route path="/fileLoaderStatus" element={<FileLoaderStatus api={api} />} />
            <Route
              path="/domainTableViewer/:table_name/:session_id"
              element={<div>domain table viewer</div>}
            />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  // A visible row value rather than the caption: a caption is drawn before the
  // rows arrive, so awaiting it and then querying a cell is racy (I-104's flake).
  await screen.findByText("acme_org");
  return { posts, api };
}

const lastRead = (posts: Posted[]) => {
  const reads = posts.map((p) => p.body).filter((b) => b["action"] === "read");
  return reads[reads.length - 1]!;
};

describe("the screen the loader tab became", () => {
  it("draws the document's own label as the page's only heading", async () => {
    await mount();
    // `TableScreen`'s rule: the document says *File Loader Status* and
    // `DataTable` draws it, so an `<h1>` here would say it twice.
    expect(screen.getAllByRole("heading").map((h) => h.textContent)).toEqual([
      "File Loader Status",
    ]);
  });

  it("names the two display filters apart", async () => {
    await mount();
    // **Moved from `Home.test.tsx` with the table.** The translation used to send
    // both columns to `fileKeyLabel`: the file key is shortened to its last
    // segment and the error message keeps its text with the recovered-error
    // prefix removed. One name for both would have rendered the message as
    // `.../` plus whatever followed its last slash — here, the whole string,
    // since it has none.
    expect(screen.getByText(".../claims.csv")).toBeTruthy();
    expect(screen.getByText("a column was short")).toBeTruthy();
  });

  it("splices the shell's selected client into the query", async () => {
    // **The claim that had to travel with the table.** Nothing on this screen
    // passes `selectedClient`; `useTableBinding` reads the store, which is D.3's
    // decision precisely because a forgotten context yields a filter that is
    // quietly off and returns *more* rows rather than an error.
    setSelectedClient("acme");
    const { posts } = await mount();
    const where = lastRead(posts)["whereClauses"] as Record<string, unknown>[];
    expect(where).toContainEqual({
      table: "input_loader_status",
      column: "client",
      values: ["acme"],
    });
  });

  it("sends no client clause when nothing is selected", async () => {
    const { posts } = await mount();
    const where = (lastRead(posts)["whereClauses"] ?? []) as Record<string, unknown>[];
    expect(where.some((w) => w["column"] === "client")).toBe(false);
  });

  it("navigates in-app to the domain table viewer, with the row's two columns", async () => {
    // **The reason this screen is not a `TableScreen`.** *View Loaded Data* reads
    // `table_name` and `session_id` off the selected row by column index, which
    // needs a selection, which needs the binding — so the action bar and the form
    // state are the point rather than decoration.
    await mount();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select row 1" }));
    fireEvent.click(screen.getByRole("button", { name: "View Loaded Data" }));
    await screen.findByText("domain table viewer");
  });

  it("refreshes rather than throwing, which is the bar's half of D.10", async () => {
    // `refreshTable` is one of the two actions A.5 returned to the widget, and
    // `requestFor` throws `UnsupportedActionType` for it by design. Until D.10
    // nothing caught that: pressing *Refresh* raised out of an event handler and
    // the button looked inert. See `ActionBar`'s `WidgetActions`.
    const { posts } = await mount();
    const before = posts.filter((p) => p.body["action"] === "read").length;
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    await waitFor(() =>
      expect(posts.filter((p) => p.body["action"] === "read").length).toBeGreaterThan(before),
    );
  });
});

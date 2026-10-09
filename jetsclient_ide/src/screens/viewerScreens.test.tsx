/**
 * @vitest-environment jsdom
 *
 * C.11 and C.12 — the two viewer screens, which are C.7's component again.
 *
 * **A separate file from `TableScreen.test.tsx` because the property is
 * different, not because the component is.** That file asserts what a screen
 * *sends*; these two tables declare no columns at all, so what is under test here
 * is what a screen *receives* — the `columnDef` the server supplies in place of a
 * configured column list.
 *
 * F81 records that four of the non-flow 25 declare none and that
 * `columnsFromResponse` has consumed the reply since A.4b, so the schema
 * relaxation C.4 made was the whole gap. **That is a claim about a code path no
 * screen had ever taken**, which is exactly I-104's shape — a half of a contract
 * tested from the end that cannot fail — so it is taken here rather than trusted.
 *
 * C.12 also names `preview_file`, the one `apiAction` in either corpus that no
 * other configuration uses, and the stub answers on the arm name.
 */

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import inputFileViewerTable from "../datatable/tables/inputFileViewerTable.tc.json";
import inputTable from "../datatable/tables/inputTable.tc.json";
import type { TableConfigDocument } from "../datatable/table";
import { TableScreen } from "./TableScreen";

afterEach(cleanup);

interface Posted {
  body: Record<string, unknown>;
}

/** What the server sends when the request names no columns. `DoReadAction`. */
const columnDef = [
  { index: 0, name: "claim_number", label: "Claim Number", tooltips: "", isnumeric: false },
  { index: 1, name: "paid_amount", label: "Paid Amount", tooltips: "", isnumeric: true },
];

function stubServer() {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ body });
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: [],
        }),
        { status: 200 },
      );
    }
    // **The arm name is an assertion.** `read` for C.11 and `preview_file` for
    // C.12; a document that lost its `apiAction` would arrive here as `read` and
    // the preview case would see the wrong rows rather than an error, so the two
    // answer different data.
    if (body["action"] === "read") {
      return new Response(
        JSON.stringify({ columnDef, rows: [["c-1", "120.50"]], totalRowCount: 1 }),
        { status: 200 },
      );
    }
    if (body["action"] === "preview_file") {
      return new Response(
        JSON.stringify({
          columnDef: [{ index: 0, name: "line", label: "Line", tooltips: "", isnumeric: false }],
          rows: [["a,b,c"]],
          totalRowCount: 1,
        }),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify({ error: `unexpected ${String(body["action"])}` }), {
      status: 422,
    });
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

async function mount(which: "domain" | "preview") {
  const { fetchImpl, posts } = stubServer();
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  const config =
    which === "domain"
      ? {
          entry: "/domainTableViewer/claim_staging/sess-1",
          path: "/domainTableViewer/:table_name/:session_id",
          tableKey: "inputTable",
          document: inputTable as TableConfigDocument,
          title: "Staging Table or Domain Table View",
        }
      : {
          entry: "/filePreviewPath/s3%3A%2F%2Fin%2Fclaims.csv",
          path: "/filePreviewPath/:file_key",
          tableKey: "inputFileViewerTable",
          document: inputFileViewerTable as TableConfigDocument,
          title: "Input File Preview",
        };

  render(
    <MemoryRouter initialEntries={[config.entry]}>
      <Routes>
        <Route
          path={config.path}
          element={
            <TableScreen
              api={api}
              tableKey={config.tableKey}
              document={config.document}
              title={config.title}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
  return { posts };
}

const reads = (posts: Posted[]) => posts.map((p) => p.body).filter((b) => b["action"] !== undefined);

describe("C.11 — the domain and staging table viewer", () => {
  it("takes both the table name and the session from the url", async () => {
    // **The `from` clause is empty in the document** — `{schema: "public"}` with
    // no table — and `makeQuery` fills it from `table_name`. So this screen puts
    // a route parameter into the FROM as well as into the WHERE, which is one
    // more than C.7's does and is the reason it is the viewer.
    const { posts } = await mount("domain");
    await screen.findByText("c-1");
    const read = reads(posts).find((b) => b["action"] === "read")!;
    expect(read["fromClauses"]).toEqual([{ schema: "public", table: "claim_staging" }]);
    expect(read["whereClauses"]).toEqual([
      { table: "", column: "session_id", values: ["sess-1"] },
    ]);
  });

  it("draws the columns the server described, having declared none", async () => {
    const { posts } = await mount("domain");
    await screen.findByText("c-1");
    // The request asked for nothing: an empty `columns`, an empty `sortColumn`.
    const read = reads(posts).find((b) => b["action"] === "read")!;
    expect(read["columns"]).toEqual([]);
    expect(read["sortColumn"]).toBe("");
    // And the headers on screen are the reply's.
    expect(screen.getByRole("columnheader", { name: /Claim Number/ })).toBeTruthy();
    expect(screen.getByRole("columnheader", { name: /Paid Amount/ })).toBeTruthy();
  });

  it("shows its title, which C.7's and C.8's screens do not have", async () => {
    // `ScreenKeys.fileRegistryTable` declares `title:` where
    // `ScreenKeys.execStatusDetailsTable` has it commented out — so this is
    // `TableScreen`'s optional prop getting its first consumer.
    await mount("domain");
    expect(screen.getByRole("heading", { name: "Staging Table or Domain Table View" })).toBeTruthy();
  });
});

describe("C.12 — the input file preview", () => {
  it("asks the preview_file arm, with the file key from the url", async () => {
    const { posts } = await mount("preview");
    await screen.findByText("a,b,c");
    const read = reads(posts).find((b) => b["action"] === "preview_file")!;
    expect(read).toBeTruthy();
    // `useParams` decodes the segment, so the key reaches the server as it was
    // written rather than percent-encoded — which matters because it is an s3
    // path and the server compares it to one.
    expect(read["whereClauses"]).toEqual([
      { table: "", column: "file_key", values: ["s3://in/claims.csv"] },
    ]);
  });

  it("keeps the Dart's page size of 50, which is not the default 10", async () => {
    // The one number that distinguishes this configuration from every other
    // column-less one, and it survives the round trip.
    const { posts } = await mount("preview");
    await screen.findByText("a,b,c");
    expect(reads(posts).find((b) => b["action"] === "preview_file")!["limit"]).toBe(50);
  });

  it("shows its title", async () => {
    await mount("preview");
    expect(screen.getByRole("heading", { name: "Input File Preview" })).toBeTruthy();
  });
});

describe("what both screens do not do", () => {
  it("draws no action bar, because neither table configures one", async () => {
    await mount("domain");
    await screen.findByText("c-1");
    // No `actions`, no `secondRowActions` — so no dispatcher, no escape registry
    // and no dialog host. `TableScreen` passes no `actions` slot at all, so the
    // group `ActionBar` renders is absent rather than empty.
    expect(screen.queryByRole("group", { name: "Table actions" })).toBeNull();
    for (const document of [inputTable, inputFileViewerTable] as TableConfigDocument[]) {
      if (document.source !== "query") throw new Error("both are query tables");
      expect(document.actions).toBeUndefined();
      expect(document.secondRowActions).toBeUndefined();
    }
  });

  it("sends no implicit client filter, because neither table has a client column", async () => {
    // The column list is empty in the document, so `hasClientColumn` is false
    // whatever the shell's picker holds — asserted rather than assumed, because
    // C.6 made the picker exist and these two screens sit under the same shell.
    const { posts } = await mount("domain");
    await screen.findByText("c-1");
    const where = reads(posts).find((b) => b["action"] === "read")!["whereClauses"] as Record<
      string,
      unknown
    >[];
    expect(where.some((w) => w["column"] === "client")).toBe(false);
  });

  it("reads once, so learning its columns from the reply does not re-query", async () => {
    // **The case the `columnDef` path could plausibly fail.** `useDataTable`
    // replaces the table's columns and resets the sort from the response, and a
    // column list that fed back into the payload would make every reply a second
    // request — a loop that would look like a slow screen rather than a defect.
    const { posts } = await mount("domain");
    await screen.findByText("c-1");
    await waitFor(() =>
      expect(reads(posts).filter((b) => b["action"] === "read")).toHaveLength(1),
    );
  });
});

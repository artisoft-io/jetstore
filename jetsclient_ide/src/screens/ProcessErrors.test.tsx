/**
 * @vitest-environment jsdom
 *
 * **C.9 driven end to end**, on `WorkspaceRegistry.test.tsx`'s shape and for its
 * reasons. Everything below `ApiClient` is real; the only stub is `fetch`,
 * answering as the apiserver's `/dataTable` switch does.
 *
 * Four things here are not in any earlier screen test, and each is a mechanism
 * that would render a plausible screen if it were wrong:
 *
 *  - **The request carries `session_id` from the route.** Without it the screen
 *    draws correctly and shows every session's errors. C.7 measured the same
 *    hazard by deleting `routeParams` from its context: 3 of 8 cases failed and
 *    **5 passed**. The equivalent mutation is measured below.
 *  - **The repeating dialog draws one table per input source**, each filtering on
 *    *its own group's* values — including a column name resolved through
 *    `lookupColumnInFormState`, which is the one field in either corpus that puts
 *    a form-state key where a column name goes.
 *  - **The explorer's three tables issue nothing.** They are the first tables in
 *    this app whose rows come from form state, and the assertion that matters is
 *    a request count that does not move.
 *  - **The seam between them is a selection.** Choosing a class fills the entity
 *    list and choosing an entity fills the details — two `map` models indexed by
 *    what the table beside them published.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { ProcessErrors, documentFindings } from "./ProcessErrors";

afterEach(cleanup);

/** `jetsapi.process_errors` joined to `pipeline_execution_status`, positional. */
const errorRows: (string | null)[][] = [
  ["11", "900", "loadClaims", "Claim", "CLM-1", "jk-1", "dob", "a long stack trace", "sess-1", "1", "now"],
  ["12", "900", "loadClaims", "Claim", "CLM-2", "jk-2", "", "another failure", "sess-1", "0", "now"],
];

/** One row per input source, as `viewInputRecordsDialog`'s query returns them. */
const inputFieldRows: (string | null)[][] = [
  ["1", "Main Input", "claims", "0", "sess-main", "900", "Claim", "CLM-1"],
  ["2", "Merged Input", "members", "3", "sess-merged", "900", "Claim", "CLM-1"],
];

/**
 * A saved rete session, in the three-deep shape the column actually holds.
 *
 * **Encoded exactly as the Dart decodes it**, which is the point of writing it
 * out rather than using plain objects: the cell is a JSON string, `rdf_types` is a
 * JSON string *inside* it, and the members of the two maps are JSON strings again
 * (`jetsclient/lib/modules/rete_session/model_handlers.dart`, which ends each
 * handler in `json.decode`). A fixture that skipped a level would pass against a
 * port that skipped the same one.
 */
const reteSession = JSON.stringify({
  rdf_types: JSON.stringify([["hc:Claim"], ["hc:Member"]]),
  entity_key_by_type: {
    "hc:Claim": JSON.stringify([["claim:1"], ["claim:2"]]),
    "hc:Member": JSON.stringify([["member:7"]]),
  },
  entity_details_by_key: {
    "claim:1": JSON.stringify([
      ["rdf:type", "hc:Claim", "named_resource"],
      ["hc:member", "member:7", "named_resource"],
      ["hc:amount", "125.00", "double"],
    ]),
    "member:7": JSON.stringify([["hc:dob", "not-a-date", "text"]]),
  },
});

interface Posted {
  body: Record<string, unknown>;
}

function stubServer(options: { reteCell?: string | null } = {}) {
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
          capabilities: ["jetstore_read"],
        }),
        { status: 200 },
      );
    }

    switch (body["action"]) {
      case "read": {
        const from = body["fromClauses"] as { table?: string }[] | undefined;
        const table = from?.[0]?.table ?? "";
        if (table === "process_errors") {
          return new Response(
            JSON.stringify({ rows: errorRows, totalRowCount: errorRows.length }),
            { status: 200 },
          );
        }
        // The input-records table names no table at all — `public` with an empty
        // name, resolved at query time — and declares no columns, so the server
        // is the one that describes the result. That is F81's mechanism, and the
        // dialog's tables reach it here.
        return new Response(
          JSON.stringify({
            rows: [["r1", "v1"]],
            totalRowCount: 1,
            columnDef: [
              { index: 0, name: "jets:key", label: "Key" },
              { index: 1, name: "value", label: "Value" },
            ],
          }),
          { status: 200 },
        );
      }
      case "raw_query": {
        const query = String(body["query"] ?? "");
        if (query.includes("rete_session_triples")) {
          const cell = "reteCell" in options ? options.reteCell : reteSession;
          return new Response(JSON.stringify({ rows: [[cell]] }), { status: 200 });
        }
        return new Response(JSON.stringify({ rows: [] }), { status: 200 });
      }
      case "raw_query_map": {
        const map = body["query_map"] as Record<string, string>;
        const result_map: Record<string, unknown> = {};
        for (const name of Object.keys(map)) result_map[name] = inputFieldRows;
        return new Response(JSON.stringify({ result_map }), { status: 200 });
      }
      default:
        return new Response(
          JSON.stringify({ error: `unexpected action ${String(body["action"])}` }),
          { status: 422 },
        );
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts };
}

function Banners() {
  const { error, status } = useNotifications();
  return (
    <>
      {error != null && <div role="alert">{error}</div>}
      {status != null && <div role="status">{status}</div>}
    </>
  );
}

async function mount(options: { session?: string; reteCell?: string | null } = {}) {
  const { fetchImpl, posts } = stubServer(
    "reteCell" in options ? { reteCell: options.reteCell } : {},
  );
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={[`/processErrors/${options.session ?? "sess-1"}`]}>
          <Routes>
            <Route path="/processErrors/:session_id" element={<ProcessErrors api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  // A row value rather than the heading: a static heading renders a tick before
  // the rows arrive, so waiting on it would let every assertion below run against
  // an empty table (I-104).
  await screen.findByText("CLM-1");
  return { posts, api };
}

async function selectErrorRow(jetsKey: string) {
  const row = screen.getByText(jetsKey).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  fireEvent.click(box);
  await waitFor(() => expect(box.checked).toBe(true));
}

const button = (label: string) => screen.getByRole("button", { name: label });

/** Every `read` request this screen made, by the table it was for. */
const readsOf = (posts: Posted[]) =>
  posts.filter((p) => p.body["action"] === "read").map((p) => p.body);

describe("the bundled documents", () => {
  it("parse and resolve every escape name they hold", () => {
    // Five table documents, one form document and one action document — the most
    // any screen in the track carries. A failure here names the document rather
    // than showing an empty screen.
    expect(documentFindings()).toEqual([]);
  });
});

describe("the screen", () => {
  it("filters the errors on the session id in the url, and on nothing else", async () => {
    const { posts } = await mount({ session: "sess-42" });
    const read = readsOf(posts)[0]!;
    expect(read["fromClauses"]).toEqual([
      { schema: "jetsapi", table: "process_errors" },
      { schema: "jetsapi", table: "pipeline_execution_status" },
    ]);
    // **The assertion the screen exists for.** `session_id` is a *form-state* key
    // in the configuration and there is no form state, so it resolves out of the
    // route (F67). Delete `routeParams` from `tableContext` and this is the case
    // that fails; the rest of this file still passes.
    expect(read["whereClauses"]).toEqual([
      { table: "process_errors", column: "session_id", values: ["sess-42"] },
      // `table: ""` is what `makeWhereClause` emits for an unqualified clause and
      // is asserted rather than trimmed: it is on the wire today, the Dart sends
      // the same, and a test that quietly normalised it would be describing a
      // payload nobody sends.
      { table: "", column: "pipeline_execution_status_key", joinWith: "pipeline_execution_status.key" },
    ]);
  });

  it("clamps the error message column, which is the only column that asks", async () => {
    await mount();
    // C.7 brought `maxLines` and `columnWidth` into the schema and `DataTable`
    // draws them; `error_message` on this table is the third of the four columns
    // in the corpus that set them, and the first inside a screen with dialogs.
    const cell = screen.getByText("a long stack trace");
    expect(cell.className).toContain("clamp");
  });

  it("offers both row buttons only once a row is selected", async () => {
    await mount();
    expect(button("View Input Records").hasAttribute("disabled")).toBe(true);
    expect(button("View Rule Session").hasAttribute("disabled")).toBe(true);
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Input Records").hasAttribute("disabled")).toBe(false));
    expect(button("View Rule Session").hasAttribute("disabled")).toBe(false);
  });
});

describe("the input-records dialog", () => {
  it("draws one table per input source and filters each on its own row", async () => {
    const { posts } = await mount();
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Input Records").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("View Input Records"));

    const dialog = await screen.findByRole("dialog", { name: "Input Records for a Domain Key" });
    // Two rows returned by `inputFields`, so two validation groups and two
    // tables. This is F.1's `repeat`, on a screen's dialog rather than on a flow —
    // and the whole of what C.9 needed for the shape the plan's §4 risk was aimed
    // at.
    await waitFor(() => expect(within(dialog).getAllByRole("table")).toHaveLength(2));

    const dialogReads = readsOf(posts).filter(
      (r) => (r["withClauses"] as unknown[] | undefined)?.length === 1,
    );
    expect(dialogReads).toHaveLength(2);

    // **Each table's `WITH` carries its own row's four values**, substituted from
    // that group's form state by the seed. The lookback differs between the two
    // rows, which is what makes this an assertion about the group rather than
    // about the query.
    const withSql = dialogReads.map((r) => (r["withClauses"] as { stmt: string }[])[0]!.stmt);
    expect(withSql[0]).toContain('"claims"');
    expect(withSql[0]).toContain("SELECT 'sess-main'");
    expect(withSql[1]).toContain('"members"');
    expect(withSql[1]).toContain("SELECT 'sess-merged'");
    expect(withSql[0]).not.toContain("{table_name}");

    // **`lookupColumnInFormState` resolved.** The document says the column is
    // whatever `domainKeyColumn` holds; the seed wrote `Claim:domain_key`, and
    // what reaches the wire is a column name rather than a key. Getting this
    // wrong sends `domainKeyColumn` as a column and the server answers 500 — but
    // only for a workspace that has such a table, which is why it is asserted on
    // the request.
    for (const read of dialogReads) {
      expect(read["whereClauses"]).toEqual([
        { table: "", column: "session_id", joinWith: "sessions.sess_id" },
        { table: "", column: "Claim:domain_key", values: ["CLM-1"] },
      ]);
    }
  });

  it("closes on Close and leaves the screen where it was", async () => {
    const { posts } = await mount();
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Input Records").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("View Input Records"));
    const dialog = await screen.findByRole("dialog", { name: "Input Records for a Domain Key" });
    await waitFor(() => expect(within(dialog).getAllByRole("table")).toHaveLength(2));
    const before = posts.length;
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    // **A viewer's Close is `dialogCancel` and posts nothing** — the distinction
    // `isDialogCancel` exists for, on a form whose only button is styled
    // `dialogOk`. Asserted as *no new action of any kind* rather than as a total
    // count, because the count is also what a stray refetch would move and the two
    // failures should not share an assertion.
    expect(posts.slice(before)).toEqual([]);
    expect(screen.getByText("CLM-1")).toBeTruthy();
  });
});

describe("the rule session explorer", () => {
  it("loads the saved session once and then queries nothing", async () => {
    const { posts } = await mount();
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Rule Session").hasAttribute("disabled")).toBe(false));
    const before = posts.length;
    fireEvent.click(button("View Rule Session"));

    const dialog = await screen.findByRole("dialog", { name: "Rete Session Explorer" });
    await within(dialog).findByText("hc:Claim");

    // **One request for the whole dialog**, and it is the escape's. The three
    // tables are `source: "formState"` and issue nothing; a table that had been
    // left on the query arm would show up here as a `read` this stub answers with
    // the wrong rows and the assertion would still pass — so the count is the
    // check and the arm is what it measures.
    const added = posts.slice(before);
    expect(added).toHaveLength(1);
    expect(String(added[0]!.body["query"])).toContain("rete_session_triples");
    expect(added[0]!.body["action"]).toBe("raw_query");
  });

  it("fills the entity list from the class and the details from the entity", async () => {
    await mount();
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Rule Session").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("View Rule Session"));
    const dialog = await screen.findByRole("dialog", { name: "Rete Session Explorer" });

    // Nothing is selected, so the two dependent tables are gated — their where
    // clauses name a key that form state does not hold, and that is
    // `hasBlockingFilter` rather than an empty result (F82).
    await within(dialog).findByText("hc:Claim");
    expect(within(dialog).queryByText("claim:1")).toBeNull();

    const classRow = within(dialog).getByText("hc:Claim").closest("tr")!;
    fireEvent.click(within(classRow).getByRole("checkbox"));
    await within(dialog).findByText("claim:1");
    // The other class's entities are not shown: the model is indexed, not merged.
    expect(within(dialog).queryByText("member:7")).toBeNull();

    const entityRow = within(dialog).getByText("claim:1").closest("tr")!;
    fireEvent.click(within(entityRow).getByRole("checkbox"));
    await within(dialog).findByText("hc:amount");
    expect(within(dialog).getByText("125.00")).toBeTruthy();
  });

  it("follows a named resource and refuses a literal", async () => {
    await mount();
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Rule Session").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("View Rule Session"));
    const dialog = await screen.findByRole("dialog", { name: "Rete Session Explorer" });
    await within(dialog).findByText("hc:Claim");
    fireEvent.click(within(within(dialog).getByText("hc:Claim").closest("tr")!).getByRole("checkbox"));
    await within(dialog).findByText("claim:1");
    fireEvent.click(within(within(dialog).getByText("claim:1").closest("tr")!).getByRole("checkbox"));
    await within(dialog).findByText("hc:member");

    // A literal first: `hc:amount` is a `double`, and the guard refuses it. The
    // Dart prints "Navigating to resources only" and returns; the document says
    // the same thing as `when: { op: "equals", … "named_resource" }`, which is why
    // this screen registers one escape and not two.
    fireEvent.click(within(within(dialog).getByText("hc:amount").closest("tr")!).getByRole("checkbox"));
    fireEvent.click(within(dialog).getByRole("button", { name: "Visit Object Entity" }));
    await waitFor(() => expect(within(dialog).getByText("hc:amount")).toBeTruthy());
    expect(within(dialog).queryByText("hc:dob")).toBeNull();

    // Then the named resource, which moves the selection to the other entity.
    fireEvent.click(within(within(dialog).getByText("hc:member").closest("tr")!).getByRole("checkbox"));
    fireEvent.click(within(dialog).getByRole("button", { name: "Visit Object Entity" }));
    await within(dialog).findByText("hc:dob");
  });

  it("shows an empty explorer when the row saved no session, and does not report", async () => {
    // `rete_session_triples` is null unless the pipeline was configured to save
    // sessions, and the errors table has a `rete_session_saved` column for exactly
    // that reason. An empty explorer is the answer; a banner would report a
    // configuration choice as a failure.
    await mount({ reteCell: null });
    await selectErrorRow("CLM-1");
    await waitFor(() => expect(button("View Rule Session").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("View Rule Session"));
    const dialog = await screen.findByRole("dialog", { name: "Rete Session Explorer" });
    await waitFor(() => expect(within(dialog).getAllByRole("table")).toHaveLength(3));
    expect(within(dialog).queryByText("hc:Claim")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

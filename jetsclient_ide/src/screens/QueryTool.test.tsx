/**
 * @vitest-environment jsdom
 *
 * The Query Tool. Task C.4.
 *
 * **Driven through a stubbed `fetch` with everything below `ApiClient` real**, so
 * these assert the request that leaves the browser rather than the payload a
 * builder returns. That is I-104's rule, and this screen is the case it was
 * written for: `columnDef` — the mechanism the whole screen rests on — appears in
 * `model.test.ts` and in `live.test.ts` and in **no test that renders**, so the
 * table's ability to draw columns it learned from a response has never been
 * observed on a screen.
 *
 * The harness renders a notification banner, because a screen raises banners and
 * `AppShell` renders them: a test that omits it asserts on field errors and calls
 * them server errors.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { QueryTool, validateQuery } from "./QueryTool";

afterEach(cleanup);

/** A `columnDef` of the shape `execQuery` builds out of the pgx descriptions. */
const COLUMN_DEF = [
  { index: 0, name: "client", label: "client", tooltips: "DataType oid 25, size -1 (text)" },
  { index: 1, name: "n", label: "n", isnumeric: true, tooltips: "DataType oid 20, size 8 (int8)" },
];

interface Options {
  capabilities?: string[];
  reply?: { status?: number; body: unknown };
}

async function clientWith(options: Options = {}) {
  const sent: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Ada",
          user_email: "ada@example.com",
          is_admin: false,
          capabilities: options.capabilities ?? ["workspace_ide"],
        }),
        { status: 200 },
      );
    }
    sent.push(JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>);
    const reply = options.reply ?? {
      body: { rows: [["acme", "3"]], columnDef: COLUMN_DEF },
    };
    return new Response(JSON.stringify({ token: "t1", ...(reply.body as object) }), {
      status: reply.status ?? 200,
    });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("ada@example.com", "pw");
  return { api, sent };
}

function Banner() {
  const { error } = useNotifications();
  return error != null ? <p role="alert">{error}</p> : null;
}

function renderScreen(api: ApiClient) {
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banner />
        <MemoryRouter initialEntries={["/query-tool"]}>
          <QueryTool api={api} />
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

function typeQuery(text: string): void {
  fireEvent.change(screen.getByLabelText("Query"), { target: { value: text } });
}

const SQL = "select client, count(*) from jetsapi.source_config group by 1";

describe("the query tool", () => {
  it("asks the server nothing until a statement is submitted", async () => {
    const { sent } = await clientWith().then((c) => {
      renderScreen(c.api);
      return c;
    });
    typeQuery(SQL);
    // The blocking clause: `query.ready` is unset, so the table has nothing to
    // ask. Typing is not submitting, and the Dart's `WhereClause(column: '',
    // formStateKey: FSK.queryReady)` is what says so.
    // D.11: the Query Tool blocks on an unsubmitted statement, not a selection.
    await waitFor(() => expect(screen.getByText("Submit a query above to see rows.")).toBeTruthy());
    expect(sent).toEqual([]);
  });

  it("sends the statement verbatim, with requestColumnDef and nothing else", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));

    await waitFor(() => expect(sent).toHaveLength(1));
    // Three fields. No `fromClauses`, no `whereClauses`, no `columns`, and no
    // `offset`/`limit` — the raw-statement body carries a statement and none of
    // the structure, which is `fetchData`'s branch reproduced.
    expect(sent[0]).toEqual({
      action: "raw_query_tool",
      query: SQL,
      requestColumnDef: true,
    });
  });

  it("renders columns it did not know it had", async () => {
    const { api } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));

    // **The point of the screen, and the thing no test rendered before.** The
    // document declares `columns: []`; these two headers exist only because the
    // response described them.
    await screen.findByText("acme");
    const table = screen.getByRole("table");
    // The sort indicator is part of the header's text, so it is trimmed off
    // rather than asserted around — the claim here is which columns exist.
    const headers = within(table)
      .getAllByRole("columnheader")
      .map((th) => (th.textContent ?? "").replace(/[▲▼]/g, "").trim());
    expect(headers).toEqual(["client", "n"]);
    // Sorted on the first column of what came back, which is what
    // `state.setSortingColumn(columnIndex: 0)` does in the Dart once a columnDef
    // arrives — the document names no `sortColumn` and could not.
    expect(within(table).getAllByRole("columnheader")[0]!.getAttribute("aria-sort")).toBe(
      "descending",
    );
    expect(within(table).getByText("3")).toBeTruthy();
  });

  it("switches the action to exec_ddl for the DDL button, and clears the other key", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await waitFor(() => expect(sent).toHaveLength(1));

    typeQuery("drop table public.scratch");
    fireEvent.click(screen.getByRole("button", { name: "Submit DDL" }));
    await waitFor(() => expect(sent).toHaveLength(2));
    // `makeRawQuery` reads the plain key first, so a DDL press that left it set
    // would re-send the previous SELECT under the previous action. The screen
    // clears it; this is the assertion that says so.
    expect(sent[1]).toEqual({
      action: "exec_ddl",
      query: "drop table public.scratch",
      requestColumnDef: true,
    });
  });

  it("re-runs an unchanged statement, which no payload change would trigger", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await waitFor(() => expect(sent).toHaveLength(1));

    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    // The request body is byte-identical, so the fetch is keyed on a value that
    // did not move. `requestRefresh` is the channel that makes the second press
    // mean something.
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]).toEqual(sent[0]);
  });

  it("does not re-run the statement while the user types the next one", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await waitFor(() => expect(sent).toHaveLength(1));

    // **The regression guard for I-165.** `useFormField` notifies on every
    // keystroke, and `useTableBinding` used to ask `FormState.isKeyUpdated`,
    // which latches: nothing clears the set once a watched key has changed. So
    // every character typed here re-executed the previous statement against the
    // database. Six changes, and the count must not move.
    for (const text of ["d", "dr", "dro", "drop", "drop ", "drop t"]) typeQuery(text);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(sent).toHaveLength(1);
  });

  it("refuses an empty statement and says which of the two reasons it is", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await screen.findByText("Query must be provided.");

    typeQuery("x");
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await screen.findByText("Query too short.");
    expect(sent).toEqual([]);
  });

  it("reports a failed statement without clearing the box", async () => {
    const { api } = await clientWith({
      reply: { status: 422, body: { error: "syntax error at or near \"selct\"" } },
    });
    renderScreen(api);
    typeQuery("selct 1");
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));

    await screen.findByRole("alert");
    // A statement that failed is the one the user most wants to edit.
    expect((screen.getByLabelText("Query") as HTMLTextAreaElement).value).toBe("selct 1");
  });

  it("disables both buttons without the workspace_ide capability", async () => {
    const { api } = await clientWith({ capabilities: ["jetstore_read"] });
    renderScreen(api);
    for (const name of ["Submit Query", "Submit DDL"]) {
      const button = screen.getByRole("button", { name }) as HTMLButtonElement;
      // **Both, which is the divergence from the Flutter form.** There
      // `queryToolOk` declares the capability and `queryToolDdlOk` declares none,
      // so the SELECT button is disabled for a user the DDL button is offered to.
      // The server refuses either way; this does not offer the wrong one. I-166.
      expect(button.disabled, name).toBe(true);
      expect(button.getAttribute("title"), name).toContain("workspace_ide");
    }
  });

  it("has no page controls, because the result has no pages", async () => {
    const { api } = await clientWith();
    renderScreen(api);
    typeQuery(SQL);
    fireEvent.click(screen.getByRole("button", { name: "Submit Query" }));
    await screen.findByText("acme");
    // The whole result set arrives in one response; a Next button would re-post an
    // identical body and relabel the same rows.
    expect(screen.queryByRole("button", { name: "Next page" })).toBeNull();
    expect(screen.queryByLabelText("Rows per page")).toBeNull();
  });
});

describe("validateQuery", () => {
  it("separates absent from too short, as the Dart delegate does", () => {
    expect(validateQuery("")).toBe("Query must be provided.");
    expect(validateQuery("x")).toBe("Query too short.");
    expect(validateQuery("xy")).toBeNull();
  });
});

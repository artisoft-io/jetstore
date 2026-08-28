/**
 * @vitest-environment jsdom
 *
 * The client picker, task C.6.
 *
 * **In its own file rather than in `AppShell.test.tsx`**, because the thing under
 * test is a store that outlives the component and a request that fires on mount,
 * and because three other track C sessions are editing the shell this week.
 *
 * The cases are about the two states a reader would assume away: a failed
 * registry query, which is deliberately silent, and a selection that reaches the
 * store rather than only the `<select>`.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import { ApiClient } from "../api/client";
import { AppShell } from "./AppShell";
import { CLIENT_LIST_QUERY, resetSelectedClient, selectedClient } from "./selectedClient";

afterEach(() => {
  cleanup();
  resetSelectedClient();
});

function stub(clients: (payload: Record<string, unknown>) => Response) {
  const posts: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push(body);
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
    return clients(body);
  }) as unknown as typeof fetch;
  return { fetchImpl, posts };
}

async function mount(clients: (payload: Record<string, unknown>) => Response) {
  const { fetchImpl, posts } = stub(clients);
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <MemoryRouter>
      {/* The client filter is a nav *entry* since D.3, not a fixed position
          after the row (I-259 puts it second, between Home and Workspaces), so a
          nav without the slot renders no picker. Passing the slot alone is what
          this file is about; the ordering is `App.tsx`'s. */}
      <AppShell api={api} nav={[{ kind: "client-filter" }]} />
    </MemoryRouter>,
  );
  return { posts };
}

const ok = () => new Response(JSON.stringify({ rows: [["acme"], ["globex"]] }), { status: 200 });

describe("the client picker", () => {
  it("asks for the registry with the Dart's statement, once", async () => {
    // Verbatim from the Flutter login delegate. The `LIMIT 200` is the Dart's and
    // is reproduced rather than raised: a deployment with more clients than that
    // has a picker problem the port should not paper over.
    const { posts } = await mount(ok);
    await waitFor(() => expect(posts.filter((b) => b["action"] === "raw_query")).toHaveLength(1));
    expect(posts.find((b) => b["action"] === "raw_query")!["query"]).toBe(CLIENT_LIST_QUERY);
  });

  it("offers the prompt entry first and clears the filter when it is chosen", async () => {
    await mount(ok);
    const picker = (await screen.findByLabelText("Filter Client")) as HTMLSelectElement;
    await waitFor(() => expect(picker.options).toHaveLength(3));
    expect([...picker.options].map((o) => o.text)).toEqual(["Filter Client", "acme", "globex"]);

    fireEvent.change(picker, { target: { value: "acme" } });
    expect(selectedClient()).toBe("acme");
    // The Dart's first entry has a null value, and choosing it means *no filter*
    // rather than a client named "". `setSelectedClient` folds the empty string
    // to null so the query builder's `== null` test is the one that decides.
    fireEvent.change(picker, { target: { value: "" } });
    expect(selectedClient()).toBeNull();
  });

  it("renders with no clients and raises no banner when the query fails", async () => {
    // **Deliberately silent, and this is the case that says so.** The selection
    // narrows what the user may already see, so losing it shows more rows rather
    // than fewer and no query depends on the list. A banner here would greet every
    // user of a deployment with an empty `client_registry`.
    await mount(() => new Response(JSON.stringify({ error: "no" }), { status: 500 }));
    const picker = (await screen.findByLabelText("Filter Client")) as HTMLSelectElement;
    await waitFor(() => expect(picker.options).toHaveLength(1));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(selectedClient()).toBeNull();
  });
});

/**
 * D.3's two shell changes: where the filter sits, and what the brand became.
 *
 * **In this file rather than in `AppShell.test.tsx`** for the same reason the
 * header gives for the picker: what is under test is the navigation surface the
 * left panel used to be, and both halves of it arrived together.
 */
describe("the navigation surface that replaced the left panel", () => {
  it("renders the client filter where the nav array puts it, not after the row", async () => {
    // I-259 orders the row Home | Filter Client | Workspaces. The slot is an
    // entry, so the array is the order and this asserts exactly that.
    const { fetchImpl } = stub(ok);
    const api = new ApiClient("", fetchImpl);
    await api.login("michel@artisoft.io", "pw");
    render(
      <MemoryRouter>
        <AppShell
          api={api}
          nav={[
            { to: "/home", label: "Home" },
            { kind: "client-filter" },
            { to: "/workspaces", label: "Workspaces" },
          ]}
        />
      </MemoryRouter>,
    );
    const row = await screen.findByLabelText("Screens");
    const labels = [...row.children].map((el) =>
      el.tagName === "SELECT" ? "Filter Client" : el.textContent,
    );
    expect(labels).toEqual(["Home", "Filter Client", "Workspaces"]);
  });

  it("turns the brand into the flow menu when one is given, and leaves it alone when not", async () => {
    // I-261: the flows lost their entry points when X.1 retired the left panel's
    // menu tree. The brand is where they came back.
    const { fetchImpl } = stub(ok);
    const api = new ApiClient("", fetchImpl);
    await api.login("michel@artisoft.io", "pw");
    const { unmount } = render(
      <MemoryRouter>
        <AppShell
          api={api}
          nav={[]}
          flowMenu={[
            { to: "/flow/clientRegistryUF", label: "Clients & Vendors" },
            { to: "/ruleConfig", label: "Rules Config" },
          ]}
        />
      </MemoryRouter>,
    );
    const menu = await screen.findByLabelText("Open a configuration flow");
    expect(menu.tagName).toBe("SUMMARY");
    // `/ruleConfig` is in the list and is not a flow — I-268. A launcher keyed on
    // flow ids would have had to special-case it, which is why `to` is a route.
    expect(screen.getByText("Rules Config").getAttribute("href")).toContain("/ruleConfig");
    expect(screen.getByText("Clients & Vendors").getAttribute("href")).toContain(
      "/flow/clientRegistryUF",
    );
    unmount();

    render(
      <MemoryRouter>
        <AppShell api={api} nav={[]} />
      </MemoryRouter>,
    );
    expect(await screen.findByText(/JetStore/)).toBeTruthy();
    expect(screen.queryByLabelText("Open a configuration flow")).toBeNull();
  });
});

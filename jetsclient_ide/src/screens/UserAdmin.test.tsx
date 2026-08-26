/**
 * @vitest-environment jsdom
 *
 * **C.13 driven end to end.** Everything below `ApiClient` is real; `fetch` is the
 * only stub, answering as the apiserver's `/dataTable` switch does.
 *
 * This is the first screen in the track whose dialog **writes**, so the cases here
 * are about the request rather than about the rendering: what `update/users`
 * receives, what `delete/users` receives, and what happens when the user says no.
 *
 * **The capability claims are asserted as they are, not as they should be.** Both
 * buttons declare `user_profile` and the route admits only the admin account, for
 * whom `ApiClient.can` returns true whatever it is asked — so the claim is inert
 * here and the test says so rather than pretending it gates anything.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { UserAdmin, documentFindings } from "./UserAdmin";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/** `jetsapi.users`, positional: name, email, is_active, roles, last_update. */
const users: (string | null)[][] = [
  ["Ada", "ada@example.com", "1", "{knowledge_engineer}", "now"],
  ["Grace", "grace@example.com", "0", "{ops_user}", "now"],
];

/** `jetsapi.roles`: role, details, last_update. */
const roles: (string | null)[][] = [
  ["client_advocate", "Administer client configuration", "now"],
  ["knowledge_engineer", "Super user role", "now"],
  ["ops_user", "Load files and execute pipelines", "now"],
];

interface Posted {
  body: Record<string, unknown>;
}

function stubServer(options: { isAdmin?: boolean; capabilities?: string[] } = {}) {
  const posts: Posted[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ body });

    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Admin",
          user_email: "admin@artisoft.io",
          is_admin: options.isAdmin ?? true,
          capabilities: options.capabilities ?? [],
        }),
        { status: 200 },
      );
    }

    switch (body["action"]) {
      case "read": {
        const from = body["fromClauses"] as { table?: string }[] | undefined;
        const rows = from?.[0]?.table === "roles" ? roles : users;
        return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
      }
      case "insert_rows":
        return new Response("{}", { status: 200 });
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

async function mount(options: { isAdmin?: boolean; capabilities?: string[] } = {}) {
  const { fetchImpl, posts } = stubServer(options);
  const api = new ApiClient("", fetchImpl);
  await api.login("admin@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={["/userAdmin"]}>
          <Routes>
            <Route path="/userAdmin" element={<UserAdmin api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  // A row value rather than the heading — the heading renders a tick early (I-104).
  await screen.findByText("ada@example.com");
  return { posts, api };
}

async function selectUser(email: string) {
  const row = screen.getByText(email).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  fireEvent.click(box);
  await waitFor(() => expect(box.checked).toBe(true));
}

const button = (label: string) => screen.getByRole("button", { name: label });
const inserts = (posts: Posted[], table: string) =>
  posts
    .map((p) => p.body)
    .filter(
      (b) =>
        b["action"] === "insert_rows" &&
        (b["fromClauses"] as { table?: string }[] | undefined)?.[0]?.table === table,
    );

describe("the bundled documents", () => {
  it("parse and resolve every escape name they hold", () => {
    expect(documentFindings()).toEqual([]);
  });
});

describe("the screen", () => {
  it("reads the users with a plain read and no filter", async () => {
    const { posts } = await mount();
    const read = posts.filter((p) => p.body["action"] === "read")[0]!.body;
    // No route parameter, no client filter, no where clause — which is what makes
    // this the one screen in the track whose table asks for everything.
    expect(read["fromClauses"]).toEqual([{ schema: "jetsapi", table: "users" }]);
    // Absent rather than empty: `makeQuery` omits the key when there is no clause,
    // and `undefined` is what a table with nothing to filter on actually sends.
    expect(read["whereClauses"]).toBeUndefined();
  });

  it("gates both row buttons on a selected row", async () => {
    await mount();
    expect(button("Update User Profile").hasAttribute("disabled")).toBe(true);
    expect(button("Delete User").hasAttribute("disabled")).toBe(true);
    await selectUser("ada@example.com");
    await waitFor(() => expect(button("Update User Profile").hasAttribute("disabled")).toBe(false));
    expect(button("Delete User").hasAttribute("disabled")).toBe(false);
  });

  it("leaves Delete enabled for the admin account although it names a capability", async () => {
    // **The claim is reproduced and it is inert here**, which is the assertion.
    // `deleteUser` declares `capability: "user_profile"` and this account holds
    // none — `capabilities: []` — yet the button is live, because `can()`
    // short-circuits on `isAdmin` exactly as `HasCapability` does server-side.
    // Only the admin account can reach this route, so the claim can never
    // withhold a control from anyone who is here.
    await mount({ isAdmin: true, capabilities: [] });
    await selectUser("ada@example.com");
    await waitFor(() => expect(button("Delete User").hasAttribute("disabled")).toBe(false));
    expect(button("Delete User").title).toBe("");
  });
});

describe("the edit dialog", () => {
  async function openDialog(email = "ada@example.com") {
    const mounted = await mount();
    await selectUser(email);
    await waitFor(() => expect(button("Update User Profile").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Update User Profile"));
    const dialog = await screen.findByRole("dialog", { name: "Update User Profile" });
    return { ...mounted, dialog };
  }

  it("seeds the name and email from the row and leaves both read-only", async () => {
    const { dialog } = await openDialog();
    const name = within(dialog).getByLabelText("Name") as HTMLInputElement;
    const email = within(dialog).getByLabelText("Email") as HTMLInputElement;
    expect([name.value, email.value]).toEqual(["Ada", "ada@example.com"]);
    // `navigationParams` reads them by *column index* off the selected row, which
    // is the positional contract every `columnIdx` binding rests on — so a column
    // reordered in the document seeds the wrong field, and this is where it shows.
    expect([name.readOnly, email.readOnly]).toEqual([true, true]);
  });

  it("draws the roles table inside the dialog", async () => {
    const { dialog } = await openDialog();
    await within(dialog).findByText("knowledge_engineer");
    expect(within(dialog).getAllByRole("table")).toHaveLength(1);
  });

  it("refuses to submit with no status chosen, and stays open", async () => {
    const { dialog, posts } = await openDialog();
    // The seeded row has `is_active: "1"`, so clear it to reach the rule. This is
    // the one validator branch the Dart actually runs — see the note below on the
    // one it does not.
    const status = within(dialog).getByLabelText("Active User?") as HTMLSelectElement;
    fireEvent.change(status, { target: { value: "" } });
    const before = inserts(posts, "update/users").length;
    fireEvent.click(within(dialog).getByRole("button", { name: "Submit" }));
    await within(dialog).findByText("User state Active / Inactive must be selected.");
    expect(inserts(posts, "update/users").length).toBe(before);
    // **The dialog is still open.** I-186: `runAction` returns null both for "ran
    // to the end" and for "a validate step failed", and closing on the second puts
    // the error on screen for one frame inside a dialog being unmounted.
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("posts the email, the status and the roles to update/users", async () => {
    const { dialog, posts } = await openDialog();
    const status = within(dialog).getByLabelText("Active User?") as HTMLSelectElement;
    fireEvent.change(status, { target: { value: "0" } });
    await within(dialog).findByText("knowledge_engineer");
    const roleRow = within(dialog).getByText("ops_user").closest("tr")!;
    fireEvent.click(within(roleRow).getByRole("checkbox"));
    fireEvent.click(within(dialog).getByRole("button", { name: "Submit" }));

    await waitFor(() => expect(inserts(posts, "update/users")).toHaveLength(1));
    const row = (inserts(posts, "update/users")[0]!["data"] as Record<string, unknown>[])[0]!;
    // **`roles` is a list and the server renames it.** `update/users`'s
    // `ColumnKeys` end in `encrypted_roles`, and the handler builds that column
    // from `Data[irow]["roles"]` (`jets/datatable/data_table_action.go`, the
    // `update/users` case of the insert-row loop) — where the "encryption" is
    // `encryptedRole := role`, the real call commented out beside it. So the
    // client sends `roles`, always has, and this asserts the wire rather than the
    // column.
    expect(row["user_email"]).toBe("ada@example.com");
    expect(row["is_active"]).toBe("0");
    // **Both roles, and the first of them is the one the row already held.** The
    // action's `navigationParams` copies column 3 into `userRolesTable` as well as
    // into `roles`, which is how the roles table arrives with the user's existing
    // roles ticked — a dialog that opened with none would silently strip them on
    // the first save, since `update/users` writes the column unconditionally.
    expect(row["roles"]).toEqual(["knowledge_engineer", "ops_user"]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("posts nothing on Cancel", async () => {
    const { dialog, posts } = await openDialog();
    const before = posts.length;
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts.slice(before)).toEqual([]);
  });
});

describe("deleting a user", () => {
  it("asks first, and sends one row per selected account", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const { posts } = await mount();
    await selectUser("ada@example.com");
    await waitFor(() => expect(button("Delete User").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Delete User"));

    await waitFor(() => expect(inserts(posts, "delete/users")).toHaveLength(1));
    expect(window.confirm).toHaveBeenCalledWith(
      "Are you sure you want to delete the selected user(s)?",
    );
    const sent = inserts(posts, "delete/users")[0]!;
    // `fanOut` over the table's published selection: the Dart walks
    // `formState.getValue(0, DTKeys.usersTable)` and builds one `{user_email}` per
    // entry (`modules/actions/user_delegates.dart`, `userAdminFormActions`). One
    // row here because one account is selected; the table is multi-select.
    expect(sent["data"]).toEqual([{ user_email: "ada@example.com" }]);
    await screen.findByRole("status");
  });

  /**
   * **The case the one above could not distinguish, and it was wrong until C.3b.**
   *
   * The document said `{"fromKey": "userTable"}` inside its `fanOut`, and
   * `evaluate`'s `fromKey` arm calls `unpack`, which returns element **0** of a
   * list (`interpret.ts`, `unpack`). So two selected accounts produced two rows
   * both naming the first — Ada deleted twice and Grace left in place, with a
   * success notification and a refreshed table saying it had worked.
   *
   * **One selected row cannot see it**, which is why the case above passed for as
   * long as it existed: with a single entry, element 0 *is* the row. The fix is
   * `fromKeyAtIndex`, the value that exists for exactly this and throws outside a
   * fan-out. Found at C.3b while writing the same construct for
   * `deleteWorkspaceFiles`, which is the honest provenance — nothing about this
   * screen prompted a re-read.
   */
  it("sends a distinct row for each of two selected accounts", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const { posts } = await mount();
    await selectUser("ada@example.com");
    await selectUser("grace@example.com");
    await waitFor(() => expect(button("Delete User").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Delete User"));

    await waitFor(() => expect(inserts(posts, "delete/users")).toHaveLength(1));
    const sent = inserts(posts, "delete/users")[0]!;
    expect(sent["data"]).toEqual([
      { user_email: "ada@example.com" },
      { user_email: "grace@example.com" },
    ]);
  });

  it("sends nothing when the confirmation is refused", async () => {
    vi.stubGlobal("confirm", vi.fn(() => false));
    const { posts } = await mount();
    await selectUser("ada@example.com");
    await waitFor(() => expect(button("Delete User").hasAttribute("disabled")).toBe(false));
    const before = posts.length;
    fireEvent.click(button("Delete User"));
    await waitFor(() => expect(window.confirm).toHaveBeenCalled());
    expect(inserts(posts, "delete/users")).toEqual([]);
    // **And it does not refresh either.** Re-reading after a refusal is harmless
    // and says the opposite of what happened — a table that flickers is a table
    // that did something.
    expect(posts.length).toBe(before);
    expect(screen.queryByRole("status")).toBeNull();
  });
});

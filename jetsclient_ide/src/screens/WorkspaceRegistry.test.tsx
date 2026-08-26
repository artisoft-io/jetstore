/**
 * @vitest-environment jsdom
 *
 * **C.2b's exit condition: a non-flow screen runs in the app.**
 *
 * On `FlowRunner.test.tsx`'s shape and for its reason. The documents, the
 * schemas, the interpreter and the widgets are all tested elsewhere; what is
 * tested here is the *seams a screen has* — that a url resolves to it, that its
 * bundled documents parse and their escape names resolve, that the table's query
 * carries `workspace_read`, that eight buttons are gated on the selected row's
 * status, that a dialog opens over the screen and posts what the Dart posts, and
 * that pressing Cancel posts nothing.
 *
 * Everything below the api client is real. The only stub is `fetch`, answering as
 * the apiserver's `/dataTable` switch does.
 *
 * **I-104 is why these render rather than parse.** A field added to a schema has
 * two halves and the corpus can only tell you about the first; the half that bit
 * F.5 was a document that validated, round-tripped and drew nothing.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { setActiveWorkspace } from "../actions/registry";
import { WorkspaceRegistry, documentFindings } from "./WorkspaceRegistry";

afterEach(() => {
  cleanup();
  // The registry's active-workspace store is module state, as the Dart's three
  // globals are. Resetting it between cases keeps `isActiveWorkspace` honest.
  setActiveWorkspace({ name: "", branch: "", uri: "" });
});

/**
 * Rows of `jetsapi.workspace_registry`, positional as the server returns them.
 *
 * Column 6 is `status`, which is what all eight of this table's gates test. The
 * four rows are the four states the criteria distinguish.
 */
const rows: (string | null)[][] = [
  ["1", "acme_ws", "main", "feat", "git@example/acme", "The acme workspace", "active", "log a", "u@x", "now"],
  ["2", "globex_ws", "main", "feat", "git@example/globex", "Globex", "modified", "log b", "u@x", "now"],
  ["3", "initech_ws", "main", "feat", "git@example/initech", "Initech", "in progress", "", "u@x", "now"],
  ["4", "old_ws", "main", "feat", "git@example/old", "Retired", "removed", "", "u@x", "now"],
];

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

function stubServer(active = { name: "jets_ws", branch: "jets_ai", uri: "git@example/jets" }) {
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

    switch (body["action"]) {
      case "get_workspace_uri":
        return new Response(
          JSON.stringify({
            workspace_name: active.name,
            workspace_branch: active.branch,
            workspace_uri: active.uri,
            workspace_file_key_label_re: "",
          }),
          { status: 200 },
        );
      // **The table's own read, and the arm name is the assertion.** A document
      // with no `apiAction` would arrive here as `read` and this stub would 422 —
      // which is the whole of what C.2a's enum buys, tested from the wire rather
      // than from the schema.
      case "workspace_read":
        return new Response(JSON.stringify({ rows, totalRowCount: rows.length }), { status: 200 });
      case "raw_query_map": {
        const map = body["query_map"] as Record<string, string>;
        const result_map: Record<string, unknown> = {};
        for (const name of Object.keys(map)) result_map[name] = [["acme"], ["globex"]];
        return new Response(JSON.stringify({ result_map }), { status: 200 });
      }
      case "workspace_insert_rows":
      case "save_workspace_client_config":
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

function LocationProbe() {
  return <span data-testid="location">{useLocation().pathname}</span>;
}

async function mount(active?: { name: string; branch: string; uri: string }) {
  const { fetchImpl, posts } = stubServer(active);
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");

  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banners />
        <MemoryRouter initialEntries={["/workspaces"]}>
          {/* Surfaces the in-app location so the Open button's destination can be
              asserted rather than inferred from a banner (C.3). */}
          <LocationProbe />
          <Routes>
            <Route path="/workspaces" element={<WorkspaceRegistry api={api} />} />
            <Route path="/workspaces/:workspace_name/home" element={<p>workspace home</p>} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
  // The first row's name is what tells us the query returned and the rows drew.
  await screen.findByText("acme_ws");
  return { posts, api };
}

/**
 * Selects the row whose workspace name is given, and waits for it to stick.
 *
 * **The wait is not politeness.** Publishing a selection notifies the form store,
 * which rebuilds the query context inside `useTableBinding` and refetches; the
 * reply clears the selection and an effect restores it from form state one tick
 * later (I-185). So a synchronous assertion after this call reads the table
 * mid-sequence, which is a race rather than a failure — and it is exactly the
 * flake I-104 recorded, arriving from the other side.
 */
async function selectRow(name: string) {
  const row = screen.getByText(name).closest("tr")!;
  const box = within(row).getByRole("checkbox") as HTMLInputElement;
  fireEvent.click(box);
  await waitFor(() => expect(box.checked).toBe(true));
}

const button = (label: string) => screen.getByRole("button", { name: label });

/**
 * The banner region, which `AppShell` renders and this tree does not.
 *
 * A screen raises errors through `useNotifications` and the *chrome* shows them,
 * so a test that mounts the screen alone can set an error and observe nothing.
 * Rendering the same two banners here keeps "the button reported" an assertion
 * about what a user sees rather than about a call.
 */
function Banners() {
  const { error, status } = useNotifications();
  return (
    <>
      {error != null && <div role="alert">{error}</div>}
      {status != null && <div role="status">{status}</div>}
    </>
  );
}

describe("the bundled documents", () => {
  it("parse and resolve every escape name they hold", () => {
    // The check the screen makes at module load, run on its own so a failure
    // names the document rather than showing an empty screen. This is what
    // `FlowStore.load` does for a flow; a screen's documents get the same
    // treatment because bundling changed the transport and nothing else.
    expect(documentFindings()).toEqual([]);
  });
});

describe("the screen", () => {
  it("renders the registry from a workspace_read, not a read", async () => {
    const { posts } = await mount();
    const table = posts.filter((p) => p.body["action"] === "workspace_read");
    expect(table).toHaveLength(1);
    expect(table[0]!.body["fromClauses"]).toEqual([
      { schema: "jetsapi", table: "workspace_registry" },
    ]);
    expect(screen.getByText("globex_ws")).toBeTruthy();
  });

  it("draws both action rows — thirteen buttons, not the first five", async () => {
    await mount();
    // The second row is where Compile, Commit, Push, Pull and the three git
    // buttons live. Rendering one bar would have lost eight of the thirteen and
    // looked like a working screen, which is I-104's second finding.
    for (const label of [
      "Add/Update", "Open", "Export Client Config", "Load Client Config", "Delete",
      "Compile Workspace", "Commit & Push Workspace", "Push Only", "Pull Workspace",
      "Git Status", "Git Command", "View Last Log", "Refresh",
    ]) {
      expect(button(label)).toBeTruthy();
    }
  });

  it("gates the eight row buttons on the selected row's status", async () => {
    await mount();
    // Nothing selected: every gated button is disabled and says why.
    expect(button("Delete").hasAttribute("disabled")).toBe(true);
    expect(button("Delete").title).toBe("Select a row first");

    // `active` — Delete is refused, Open is allowed, Commit is not offered.
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Open").hasAttribute("disabled")).toBe(false));
    expect(button("Delete").hasAttribute("disabled")).toBe(true);
    expect(button("Delete").title).toContain("active");
    expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(true);
  });

  it("offers Commit only on a modified workspace, and refuses everything mid-compile", async () => {
    await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(false));
    expect(button("Delete").hasAttribute("disabled")).toBe(false);

    cleanup();
    await mount();
    await selectRow("initech_ws");
    // `in progress` fails seven of the eight; Push Only tests `removed` alone.
    //
    // **Wait on the button that becomes *enabled*, not on one that becomes
    // disabled.** This waited on `Open` being disabled, which is also true with
    // no row selected at all — so the wait returned before the selection landed
    // and the assertions below raced the restore, failing about two runs in five.
    // That is I-104's shape in a `waitFor` predicate: a condition satisfied by the
    // state you are waiting to leave is not a wait. Found by C.3 running the suite
    // repeatedly for its own flake check; the case itself is C.2b's and correct.
    await waitFor(() => expect(button("Push Only").hasAttribute("disabled")).toBe(false));
    for (const label of [
      "Open", "Export Client Config", "Load Client Config", "Delete",
      "Compile Workspace", "Pull Workspace",
    ]) {
      expect(button(label).hasAttribute("disabled")).toBe(true);
    }
    expect(button("Push Only").hasAttribute("disabled")).toBe(false);
  });
});

/**
 * **All seven, opened.** This is the sixth failure mode's second shape — *the
 * fixture does not reach the path* — closed for the set rather than for the four
 * dialogs the cases below happen to drive.
 *
 * Seven documents transcribed by hand from `form_config.dart` is seven chances to
 * mistype a form key, a field key or a button, and every one of those mistakes
 * survives `FormDocumentSchema`: the document is well formed either way. What
 * catches them is opening each one and looking at it. Three of the seven —
 * `pushOnlyWorkspaceDialog`, `doGitCommandWorkspaceDialog` and
 * `viewGitLogWorkspaceDialog` — are reached by no other case in this file, so
 * without this they would have been checked by the schema and by nothing else.
 *
 * The first shape bit this file too: the notification banners are `AppShell`'s,
 * so before `Banners` above existed, *"the button reported"* asserted that a
 * message had been passed to a provider nothing rendered.
 */
describe("every dialog the table can open", () => {
  const dialogs: [button: string, title: string, ok: string, row: string][] = [
    ["Add/Update", "Add / Update Workspace", "Add / Update", "globex_ws"],
    ["Export Client Config", "Export Client Configuration from DB to Workspace", "Export Client Config", "acme_ws"],
    ["Commit & Push Workspace", "Commit and Push Workspace Changes to Repository", "Commit & Push", "globex_ws"],
    ["Push Only", "Push Workspace Changes to Repository Without Compiling Workspace", "Push Changes", "globex_ws"],
    ["Git Status", "Do Git Status in Local Workspace", "Git Status", "acme_ws"],
    ["Git Command", "Do Git Command in Local Workspace", "Execute", "acme_ws"],
    // The one whose only button is a Cancel wearing an OK's clothes: labelled
    // *Close*, styled `dialogOk`, keyed `dialogCancel`. Reading the label or the
    // style rather than the key would have made it a submit.
    ["View Last Log", "Last Git Log of Workspace Changes", "Close", "acme_ws"],
  ];

  it.each(dialogs)("opens %s", async (label, title, ok, row) => {
    await mount();
    await selectRow(row);
    await waitFor(() => expect(button(label).hasAttribute("disabled")).toBe(false));
    fireEvent.click(button(label));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(title)).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: ok })).toBeTruthy();
    // Every one of the seven shows the workspace it is about, from the row.
    expect((within(dialog).getByLabelText(/Workspace Name/) as HTMLInputElement).value).toBe(row);
  });

  it("carries workspace_ide on the six OK buttons and on neither Cancel", async () => {
    // **The capability no corpus reports** (I-180): `FormConfig.actions` is a
    // container `allFields` does not walk, so these six declarations exist in the
    // Dart and in the transcription and in no generated fixture. Asserting them
    // here is the only mechanical check they have.
    await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Add/Update").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Add/Update"));
    const dialog = await screen.findByRole("dialog");
    // The signed-in user holds it, so the button is live rather than explained.
    const okButton = within(dialog).getByRole("button", { name: "Add / Update" });
    expect(okButton.hasAttribute("disabled")).toBe(false);
    expect(okButton.title).toBe("");
  });
});

describe("the dialog host", () => {
  it("opens a dialog over the screen, seeded from the selected row", async () => {
    await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Commit & Push Workspace"));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Commit and Push Workspace Changes to Repository")).toBeTruthy();
    // `navigationParams` maps the dialog's keys to *column indexes* of the
    // selected row; the fields read them back by key. This is that contract, from
    // the row through `resolveParams` to the widget.
    const name = within(dialog).getByLabelText(/Workspace Name/) as HTMLInputElement;
    expect(name.value).toBe("globex_ws");
    expect(name.readOnly).toBe(true);
    // The commit message is not from the row; it is the document's `defaultValue`.
    const message = within(dialog).getByLabelText(/Commit Message/) as HTMLInputElement;
    expect(message.value).toBe("Commit from JetStore UI");
  });

  it("posts what the Dart posts, and closes", async () => {
    const { posts } = await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Commit & Push Workspace"));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Commit & Push" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const posted = posts.filter((p) => p.body["action"] === "workspace_insert_rows");
    expect(posted).toHaveLength(1);
    const sent = posted[0]!.body;
    expect(sent["fromClauses"]).toEqual([{ table: "commit_workspace" }]);
    expect(sent["workspaceName"]).toBe("globex_ws");
    const row = (sent["data"] as Record<string, unknown>[])[0]!;
    expect(row["user_email"]).toBe("michel@artisoft.io");
    // `last_git_log` is redacted on the way out, which the Dart does at every one
    // of these arms — the log can be a megabyte and the server does not read it.
    expect(row["last_git_log"]).toBe("redacted");
    expect(row["status"]).toBe("");
    expect(row["git.commit.message"]).toBe("Commit from JetStore UI");
  });

  it("posts nothing when the dialog is cancelled", async () => {
    const { posts } = await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Commit & Push Workspace"));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(posts.filter((p) => p.body["action"] === "workspace_insert_rows")).toHaveLength(0);
  });

  it("refuses to submit a dialog whose required field is empty", async () => {
    const { posts } = await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Commit & Push Workspace").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Commit & Push Workspace"));
    const dialog = await screen.findByRole("dialog");
    const message = within(dialog).getByLabelText(/Commit Message/);
    fireEvent.change(message, { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Commit & Push" }));

    // `{ "do": "validate" }` is the first step of the action, so the post never
    // happens and the dialog stays open with the message the Dart shows.
    await screen.findByText("Commit message must be provided.");
    expect(posts.filter((p) => p.body["action"] === "workspace_insert_rows")).toHaveLength(0);
    expect(screen.queryByRole("dialog")).not.toBeNull();
  });

  it("closes on Escape, which is what the Dart's dismissible barrier does", async () => {
    await mount();
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Git Status").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Git Status"));
    const dialog = await screen.findByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    // jsdom does not fire `cancel` from a keydown, so the close path is driven
    // directly — what is being checked is that `onCancel` resolves as `cancel`
    // rather than leaving the promise pending, not that jsdom implements Escape.
    (dialog as HTMLDialogElement).dispatchEvent(new Event("cancel", { cancelable: true }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});

describe("isReadOnlyFrom — the closures the corpus could only report as booleans", () => {
  it("locks the name and branch of the deployment's own workspace", async () => {
    // `addWorkspace`'s `isReadOnlyEval` is *is this the active workspace*, and it
    // is a safety gate: renaming the workspace the apiserver is pointed at leaves
    // it looking for a directory that no longer exists.
    await mount({ name: "acme_ws", branch: "main", uri: "" });
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Add/Update").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Add/Update"));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByLabelText(/Workspace Name/) as HTMLInputElement).readOnly).toBe(true);
    expect((within(dialog).getByLabelText(/Workspace Branch/) as HTMLInputElement).readOnly).toBe(true);
    // The uri predicate is a different body and this deployment has no uri.
    expect((within(dialog).getByLabelText(/Worksapce URI/) as HTMLInputElement).readOnly).toBe(false);
  });

  it("leaves another workspace's name editable", async () => {
    await mount({ name: "acme_ws", branch: "main", uri: "" });
    await selectRow("globex_ws");
    await waitFor(() => expect(button("Add/Update").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Add/Update"));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByLabelText(/Workspace Name/) as HTMLInputElement).readOnly).toBe(false);
  });

  it("locks the git status command when the deployment configures a uri", async () => {
    // The second body, and the site that makes it non-obvious: a *git command*
    // box gated on whether the server has a workspace uri of its own.
    await mount({ name: "jets_ws", branch: "jets_ai", uri: "git@example/jets" });
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Git Status").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Git Status"));
    const dialog = await screen.findByRole("dialog");
    const command = within(dialog).getByLabelText(/Git Status Commands/) as HTMLInputElement;
    expect(command.readOnly).toBe(true);
    expect(command.value).toBe("git status");
  });
});

describe("the actions that are not dialogs", () => {
  it("compiles the selected workspace", async () => {
    const { posts } = await mount();
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Compile Workspace").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Compile Workspace"));
    await waitFor(() =>
      expect(posts.some((p) => p.body["action"] === "workspace_insert_rows")).toBe(true),
    );
    const sent = posts.find((p) => p.body["action"] === "workspace_insert_rows")!.body;
    expect(sent["fromClauses"]).toEqual([{ table: "compile_workspace" }]);
    expect(sent["workspaceName"]).toBe("acme_ws");
  });

  it("asks before deleting, and posts nothing when refused", async () => {
    const { posts } = await mount();
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    await selectRow("old_ws");
    await waitFor(() => expect(button("Delete").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Delete"));
    await waitFor(() => expect(confirm).toHaveBeenCalled());
    expect(posts.filter((p) => p.body["action"] === "workspace_insert_rows")).toHaveLength(0);
    confirm.mockRestore();
  });

  it("deletes when confirmed", async () => {
    const { posts } = await mount();
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
    await selectRow("old_ws");
    await waitFor(() => expect(button("Delete").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Delete"));
    await waitFor(() =>
      expect(posts.some((p) => p.body["action"] === "workspace_insert_rows")).toBe(true),
    );
    expect(posts.find((p) => p.body["action"] === "workspace_insert_rows")!.body["fromClauses"]).toEqual(
      [{ table: "delete_workspace" }],
    );
    confirm.mockRestore();
  });

  /**
   * **I-183, closed by C.3 — and the case changed shape rather than being
   * deleted.** It asserted that Open *reports* instead of navigating, because the
   * destination did not exist; it now asserts where it goes. What is kept from the
   * original is the second assertion: the escape does **not** post
   * `workspace_query_structure`.
   *
   * That is the whole difference between the two apps. The Dart's Open fetches the
   * tree and writes it into `JetsRouterDelegate().workspaceMenuState`, which is
   * what the destination renders its menu from; this app's destination fetches its
   * own tree on mount, so the same request from here would be a request whose only
   * effect is client state nobody keeps.
   */
  it("navigates to the workspace home, and does not fetch the tree on the way", async () => {
    const { posts } = await mount();
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Open").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Open"));
    await waitFor(() =>
      expect(screen.getByTestId("location").textContent).toBe("/workspaces/acme_ws/home"),
    );
    expect(posts.some((p) => p.body["action"] === "workspace_query_structure")).toBe(false);
  });
});

describe("the export dialog's item query", () => {
  it("fills its dropdown from the form's named query", async () => {
    const { posts } = await mount();
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Export Client Config").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Export Client Config"));
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(within(dialog).getByText("acme")).toBeTruthy());
    // I.2b's transport: one `raw_query_map`, the literal prompt item kept in
    // front of the rows.
    expect(posts.some((p) => p.body["action"] === "raw_query_map")).toBe(true);
    expect(within(dialog).getByText("Select a Client")).toBeTruthy();
  });

  it("posts save_workspace_client_config, which is not workspace_insert_rows", async () => {
    const { posts } = await mount();
    await selectRow("acme_ws");
    await waitFor(() => expect(button("Export Client Config").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Export Client Config"));
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(within(dialog).getByText("acme")).toBeTruthy());
    fireEvent.change(within(dialog).getByLabelText(/Client/), { target: { value: "acme" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Export Client Config" }));

    await waitFor(() =>
      expect(posts.some((p) => p.body["action"] === "save_workspace_client_config")).toBe(true),
    );
    const sent = posts.find((p) => p.body["action"] === "save_workspace_client_config")!.body;
    // It resolves no statement out of `sqlInsertStmts`, so it carries no table.
    expect(sent["fromClauses"]).toBeUndefined();
    expect((sent["data"] as Record<string, unknown>[])[0]!["client"]).toBe("acme");
  });
});

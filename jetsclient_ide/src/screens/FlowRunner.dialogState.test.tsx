/**
 * @vitest-environment jsdom
 *
 * **A flow's dialog has its own form state.** `jetstore_maintenance_02` Phase 2,
 * `I-28`, 2026-10-01.
 *
 * Until this file the flow runner wrote a dialog's parameters into the flow's
 * one form state and ran the dialog's actions against it. The Dart builds a new
 * state per dialog (`data_table.dart`, the `doActionShowDialog` case,
 * `makeFormState(parentFormState: formState)`), so what the port changed was
 * which keys a dialog could see — and `pipelineConfigUF`'s process-input dialog
 * decides insert or update on whether it can see `key`
 * (`addProcessInputOk`, `{ "op": "isNull", "key": "key" }`).
 *
 * `proofFlows.test.ts` drives `addProcessInputOk` alone with `key` set by hand,
 * which proves the action and cannot see this: the defect is in what the screen
 * puts in front of it. So everything here goes through `FlowRunner` with the
 * committed documents and a stubbed server that records what was posted.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { ApiClient } from "../api/client";
import type { JetsRow } from "../datatable/types";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { PromptsProvider } from "../shell/prompts";
import { answerConfirm } from "../shell/promptsTesting";
import { FlowRunner } from "./FlowRunner";

afterEach(cleanup);

/** The committed documents, which is what a workspace installs. */
const ASSETS = join(__dirname, "../../../jets/workspace_assets");
const files: Record<string, string> = {};
for (const dir of ["user_flows", "table_configs"]) {
  for (const name of readdirSync(join(ASSETS, dir))) {
    if (name.endsWith(".json")) files[`${dir}/${name}`] = readFileSync(join(ASSETS, dir, name), "utf8");
  }
}

interface Posted {
  path: string;
  body: Record<string, unknown>;
}

/**
 * The apiserver. Reads answer from fixed rows; every write is recorded and
 * answered 200, because what is asserted is the request, not its effect.
 *
 * **`pipeline_config` key 7 and `process_input` key 7 are both here on
 * purpose**: two serial columns collide as a matter of course, which is what made
 * `I-28` an overwrite of an unrelated row rather than a miss.
 */
function stubServer() {
  const posts: Posted[] = [];
  // `pcPipelineConfigTable`'s sixteen columns.
  const pipelineConfigs: JetsRow[] = [
    ["7", "ACME", "loadClaims", "11", "3", "{}", "claim", "file", "month_period", "0", "PIPE-DESC-7", "0", "{}", "[]", "rdf:Claim", "2026-09-30"],
  ];
  // `pcMainProcessInputKey`'s ten columns; the three process-input tables share them.
  const processInputs: JetsRow[] = [
    ["3", "ACME", "EAST", "claim", "rdf:Claim", "file", "TBL_MAIN_3", "0", "a@b", "2026-09-01"],
    ["7", "ACME", "WEST", "claim", "rdf:Claim", "file", "TBL_UNRELATED_7", "0", "a@b", "2026-09-02"],
  ];
  // `pcProcessInputRegistry`'s six; the key is the four-value concatenation
  // `pcSetProcessInputRegistryKey` derives.
  const registry: JetsRow[] = [
    ["loadClaimsclaimTBL_NEW_Rfile", "NORTH", "claim", "rdf:Claim", "file", "TBL_NEW_R"],
    ["loadClaimsclaimTBL_MAIN_3file", "EAST", "claim", "rdf:Claim", "file", "TBL_MAIN_3_REG"],
  ];
  // `fmInputSourceMappingUF`'s seven.
  const sourceConfigs: JetsRow[] = [["42", "ACME", "EAST", "claim", "ACME_EAST_claim", "rdf:Claim", "2026-09-30"]];

  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ path, body });
    const ok = (payload: unknown) => new Response(JSON.stringify(payload), { status: 200 });
    if (path === "/login") {
      return ok({
        token: "t0",
        name: "Michel",
        user_email: "michel@artisoft.io",
        is_admin: false,
        capabilities: ["workspace_ide", "client_config", "run_pipelines"],
      });
    }
    const table = ((body["fromClauses"] as { table: string }[] | undefined) ?? [])[0]?.table;
    const rows = (r: JetsRow[]) => ok({ rows: r, totalRowCount: r.length });
    switch (body["action"]) {
      case "get_workspace_uri":
        return ok({ workspace_name: "jets_ws", workspace_uri: "git@example", workspace_branch: "jets_ai", workspace_file_key_label_re: "" });
      case "get_workspace_document": {
        const name = (body["data"] as { file_name: string }[])[0]!.file_name;
        const text = files[decodeURIComponent(name.replace(/\+/g, " "))];
        return text === undefined
          ? new Response(JSON.stringify({ error: `no such file: ${name}` }), { status: 404 })
          : ok({ file_content: text });
      }
      case "read":
        if (table === "pipeline_config") return rows(pipelineConfigs);
        if (table === "process_input") return rows(processInputs);
        if (table === "process_input_registry") return rows(registry);
        if (table === "client_registry") return rows([["ACME", "the client", "2026-09-30"]]);
        if (table === "client_org_registry") return rows([["ACME", "EASTORG", "east details"]]);
        if (table === "source_config") return rows(sourceConfigs);
        return rows([]);
      case "raw_query_map":
        return ok({ result_map: {} });
      case "insert_rows":
      case "insert_raw_rows":
        return ok({});
      default:
        return new Response(JSON.stringify({ error: `unexpected action ${String(body["action"])}` }), { status: 422 });
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

async function mount(flowKey: string) {
  const server = stubServer();
  const api = new ApiClient("", server.fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <PromptsProvider>
          <Banners />
          <MemoryRouter initialEntries={[`/flow/${flowKey}`]}>
            <Routes>
              <Route path="/flow/:key" element={<FlowRunner api={api} />} />
              <Route path="/home" element={<p>the home screen</p>} />
            </Routes>
          </MemoryRouter>
        </PromptsProvider>
      </NotificationsProvider>
    </ApiProvider>,
  );
  return server;
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;

/** The checkbox of the row whose cells include `text`, within `scope`. */
const box = (text: string, scope: HTMLElement = document.body) =>
  within(within(scope).getByText(text).closest("tr")!).getByRole("checkbox") as HTMLInputElement;

const tick = (text: string, scope?: HTMLElement) => {
  if (!box(text, scope).checked) fireEvent.click(box(text, scope));
};
const untick = (text: string, scope?: HTMLElement) => {
  if (box(text, scope).checked) fireEvent.click(box(text, scope));
};

/** The writes, in order: the table each names and its one row. */
const writes = (posts: Posted[]) =>
  posts
    .filter((p) => p.body["action"] === "insert_rows" || p.body["action"] === "insert_raw_rows")
    .map((p) => ({
      table: (p.body["fromClauses"] as { table: string }[])[0]!.table,
      row: (p.body["data"] as Record<string, unknown>[])[0]!,
    }));

const stateLine = () => document.querySelector(".uf-runner__state")?.textContent;

/** Opens the flow, ticks the one pipeline configuration and edits it. */
async function editPipeline() {
  const server = await mount("pipelineConfigUF");
  await screen.findByText("PIPE-DESC-7");
  tick("PIPE-DESC-7");
  fireEvent.click(button("Edit"));
  await screen.findByText("TBL_MAIN_3");
  // The edit path restores the main input from the record, so it arrives ticked.
  await waitFor(() => expect(box("TBL_MAIN_3").checked).toBe(true));
  return server;
}

async function openProcessInputDialog() {
  fireEvent.click(button("Add/Update Data Source Configuration"));
  const dialog = await screen.findByRole("dialog");
  await within(dialog).findByText("TBL_NEW_R");
  return dialog;
}

async function saveDialog(dialog: HTMLElement, label: string, posts: Posted[]) {
  const before = writes(posts).length;
  fireEvent.click(within(dialog).getByRole("button", { name: label }));
  await waitFor(() => expect(writes(posts).length).toBe(before + 1));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  return writes(posts)[before]!;
}

async function cancelDialog(dialog: HTMLElement) {
  fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
}

describe("Pipeline Configuration: the process-input dialog decides insert or update on its own key", () => {
  /**
   * The I-28 sequence: *Add/Update* on the main page with the main input ticked,
   * then Next, *Add Data Source to Merge*, and *Add/Update* with nothing ticked.
   * The first open left `key` = "3" in the one shared state, so the add went to
   * `update2/process_input` with that key and overwrote the main input.
   */
  async function addOnTheMergePage(posts: Posted[]) {
    // Idempotent here; under the shared state a cancelled dialog dropped the
    // selection (the next case), and ticking it again is what a user did.
    await screen.findByText("TBL_MAIN_3");
    tick("TBL_MAIN_3");
    fireEvent.click(button("Next"));
    fireEvent.click(await screen.findByRole("button", { name: "Add Data Source to Merge" }));
    await screen.findByText("TBL_UNRELATED_7");
    expect(box("TBL_MAIN_3").checked || box("TBL_UNRELATED_7").checked).toBe(false);
    const dialog = await openProcessInputDialog();
    tick("TBL_NEW_R", dialog);
    return saveDialog(dialog, "Save", posts);
  }

  it("adds on the merge page after an Add/Update of the main input was saved", async () => {
    const { posts } = await editPipeline();
    const dialog = await openProcessInputDialog();
    tick("TBL_MAIN_3_REG", dialog);
    const updated = await saveDialog(dialog, "Save", posts);
    // Add/Update of the ticked row is an update of that row, as the Dart's is.
    expect(updated.table).toBe("update2/process_input");
    expect(updated.row["key"]).toBe("3");

    // A dialog that saved refreshes the flow's tables, which clears their
    // selection — `_refreshTable` (`binding.ts`, `clearPublishedSelection`), the
    // Dart's behaviour and unchanged here — so the helper ticks it again.
    await screen.findByText("TBL_MAIN_3");
    await waitFor(() => expect(box("TBL_MAIN_3").checked).toBe(false));

    const added = await addOnTheMergePage(posts);
    expect(added.table).toBe("process_input");
    expect(added.row["key"]).toBeUndefined();
    expect(added.row).toMatchObject({ client: "ACME", org: "NORTH", table_name: "TBL_NEW_R", source_type: "file" });
  });

  it("adds on the merge page after an Add/Update of the main input was cancelled", async () => {
    // **Proved by mutation**, and this is the case that shows it end to end:
    // restore the pre-2026-10-01 `FlowRunner.tsx` and the assertion below fails
    // with `update2/process_input` — key "3", the main input, overwritten with
    // the merge choice. (The saved variant above goes red earlier under the same
    // mutation: the shared state's refresh also emptied the main table.)
    const { posts } = await editPipeline();
    await cancelDialog(await openProcessInputDialog());
    const added = await addOnTheMergePage(posts);
    expect(added.table).toBe("process_input");
    expect(added.row["key"]).toBeUndefined();
  });

  it("adds while editing with no row selected, and sends no list-valued key", async () => {
    // The case I-28 suspected. It posted `update2/process_input` with the
    // pipeline's key as `["7"]`, which pgx cannot encode into an int4: a 500.
    const { posts } = await editPipeline();
    untick("TBL_MAIN_3");
    const dialog = await openProcessInputDialog();
    tick("TBL_NEW_R", dialog);
    const added = await saveDialog(dialog, "Save", posts);
    expect(added.table).toBe("process_input");
    expect(added.row["key"]).toBeUndefined();
    // The dialog's state is its parameters and its own fields — none of the
    // pipeline's record, which is the Dart's payload too.
    expect(added.row["pcPipelineConfigTable"]).toBeUndefined();
    expect(added.row["description"]).toBeUndefined();
    expect(added.row).toMatchObject({
      client: "ACME",
      process_name: "loadClaims",
      org: "NORTH",
      object_type: "claim",
      entity_rdf_type: "rdf:Claim",
      table_name: "TBL_NEW_R",
      source_type: "file",
      lookback_periods: "0",
      user_email: "michel@artisoft.io",
    });
  });

  it("keeps the main selection through a cancelled dialog, and Next goes on", async () => {
    // With one state, the parameters rewrote `client` and `entity_rdf_type`,
    // which filter this page's table; it re-read, dropped the selection, and
    // Next stopped on "Please select an option."
    const { posts } = await editPipeline();
    const reads = () => posts.filter((p) => p.body["action"] === "read").length;
    const before = reads();
    await cancelDialog(await openProcessInputDialog());
    expect(box("TBL_MAIN_3").checked).toBe(true);
    // The dialog's own registry table is the one read; the page's is not.
    expect(reads()).toBe(before + 1);
    fireEvent.click(button("Next"));
    await waitFor(() => expect(stateLine()).toBe("View the merge process inputs"));
    expect(screen.queryByText("Please select an option.")).toBeNull();
  });

  it("pre-selects the registry row of the input selected now, not of the last open", async () => {
    // `pcSetProcessInputRegistryKey` derives the key from the parameters, so it
    // has to run after they are seeded — the Dart's order. Run first, it read the
    // previous open's: nothing on the first, the main input's on the next.
    await editPipeline();
    let dialog = await openProcessInputDialog();
    // The table restores its selection from form state after its rows arrive.
    await waitFor(() => expect(box("TBL_MAIN_3_REG", dialog).checked).toBe(true));
    expect(box("TBL_NEW_R", dialog).checked).toBe(false);
    await cancelDialog(dialog);

    untick("TBL_MAIN_3");
    dialog = await openProcessInputDialog();
    // A negative, so give the restore the same chance to happen first.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(box("TBL_MAIN_3_REG", dialog).checked).toBe(false);
    expect(box("TBL_NEW_R", dialog).checked).toBe(false);
  });
});

describe("the other flow dialogs, on their own state", () => {
  it("opens the vendor dialog (+ Add on the org table) empty after an org was ticked, with the client passed in", async () => {
    const { posts } = await mount("clientRegistryUF");
    await screen.findByText("the client");
    tick("the client");
    fireEvent.click(button("Next"));
    await screen.findByText("EASTORG");
    // The org table draws no checkboxes until asked.
    fireEvent.click(button("Show/Hide Select Row"));
    tick("EASTORG");
    fireEvent.click(button("+ Add"));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByLabelText("Client Name") as HTMLInputElement).value).toBe("ACME");
    expect((within(dialog).getByLabelText("Vendor/Org Name") as HTMLInputElement).value).toBe("");
    fireEvent.change(within(dialog).getByLabelText("Vendor/Org Name"), { target: { value: "WESTORG" } });
    const added = await saveDialog(dialog, "Add", posts);
    expect(added.table).toBe("client_org_registry");
    expect(added.row).toMatchObject({ client: "ACME", org: "WESTORG" });
  });

  it("posts the pasted mapping and the user, and nothing of the flow's selection", async () => {
    // `insert_raw_rows` reads `raw_rows` and `user_email` from the row and
    // replaces the rest with the parsed rows (`InsertRawRows`,
    // `jets/datatable/data_table_action.go`), so the flow's `client`, `org` and
    // `table_name` it used to carry were sent and ignored.
    const { posts } = await mount("fileMappingUF");
    await screen.findByText("ACME_EAST_claim");
    tick("ACME_EAST_claim");
    fireEvent.click(button("Next"));
    fireEvent.click(await screen.findByRole("button", { name: "Paste File Mapping" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("File Mapping (csv/tsv)"), {
      target: { value: "client,object_type,data_property\nACME,claim,p1" },
    });
    const loaded = await saveDialog(dialog, "Save", posts);
    expect(loaded.table).toBe("raw_rows/process_mapping");
    expect(loaded.row).toEqual({
      raw_rows: "client,object_type,data_property\nACME,claim,p1",
      user_email: "michel@artisoft.io",
    });
  });
});

describe("Delete Client", () => {
  it("deletes the selected client and says so in the status banner", async () => {
    // Michel, 2026-10-02: a successful delete showed nothing, so it read as
    // the button not working. The banner comes after the post, which stops the
    // action on a failure, so it only ever reports a delete that happened.
    const { posts } = await mount("clientRegistryUF");
    await screen.findByText("the client");
    tick("the client");
    fireEvent.click(button("Delete Client"));
    // Asked in the app's own dialog (jetstore_maintenance_02 `I-39`): the
    // embedded browser pane never shows `window.confirm`, which answered false
    // there, so the button did nothing. Nothing is posted until *OK*.
    await screen.findByRole("dialog");
    expect(writes(posts)).toEqual([]);
    await answerConfirm("OK");
    await waitFor(() => expect(writes(posts)).toEqual([{ table: "delete/client", row: { client: "ACME" } }]));
    expect((await screen.findByRole("status")).textContent).toBe("Client deleted");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("posts nothing when the confirmation is cancelled", async () => {
    const { posts } = await mount("clientRegistryUF");
    await screen.findByText("the client");
    tick("the client");
    fireEvent.click(button("Delete Client"));
    await answerConfirm("Cancel");
    expect(writes(posts)).toEqual([]);
    expect(screen.queryByRole("status")).toBeNull();
  });
});

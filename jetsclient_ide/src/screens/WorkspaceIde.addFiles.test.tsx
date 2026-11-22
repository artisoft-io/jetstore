/**
 * @vitest-environment jsdom
 *
 * **Adding files from the IDE.** *New file* opens an empty tab and its save adds
 * the file, refusing one that is there; *Upload files* and *Upload folder* ask
 * the server what the upload would do, show it, and add every file or none. An
 * added file goes through the server's save checks
 * (`jets/datatable/workspace_add_files.go`), so a refusal is the server's
 * message, shown in the error banner. Adding wants `workspace_ide`, as saving
 * does.
 *
 * The editor is replaced by a text box, as in `WorkspaceIde.readOnly.test.tsx`.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { answerConfirm, answerPrompt } from "../shell/promptsTesting";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());

vi.mock("../editor/Editor", () => ({
  Editor: ({ fileName, content, onChange }: EditorProps) => (
    <textarea
      aria-label={`Edit ${fileName}`}
      defaultValue={content}
      onChange={(event) => onChange?.(event.target.value)}
    />
  ),
}));

afterEach(cleanup);

const EXISTING = "runbooks/health_check/count.sql";

/** A request the stub received: its action and its data rows. */
type Posted = { action: string; data: Array<Record<string, unknown>> };

/** What the stub answers the add and check actions with, per call. */
interface Answers {
  check?: (data: Array<Record<string, unknown>>) => { status: number; body: unknown };
  add?: (data: Array<Record<string, unknown>>) => { status: number; body: unknown };
}

function fileNode(name: string) {
  return {
    key: name,
    pageMatchKey: name,
    type: "file",
    size: 8,
    label: name.split("/").pop(),
    route_params: { workspace_name: "ws", file_name: encodeURIComponent(name) },
    children: null,
  };
}

function stubServer(posted: Posted[], answers: Answers, capabilities: string[]) {
  return vi.fn(async (url: string | URL, init?: RequestInit) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({ token: "t0", name: "Ada", user_email: "ada@example.com", is_admin: false, capabilities }),
        { status: 200 },
      );
    }
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    const action = String(body["action"]);
    const data = (body["data"] ?? []) as Array<Record<string, unknown>>;
    posted.push({ action, data });
    const answer = (a: { status: number; body: unknown }) =>
      new Response(JSON.stringify(a.body), { status: a.status });
    switch (action) {
      case "raw_query":
        return new Response(JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"]] }), {
          status: 200,
        });
      case "workspace_query_structure":
        return new Response(
          JSON.stringify({ result_type: "workspace_file_structure", result_data: [fileNode(EXISTING)] }),
          { status: 200 },
        );
      case "check_workspace_files":
        return answer(answers.check!(data));
      case "add_workspace_files":
        return answer(answers.add!(data));
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

async function mount(answers: Answers, capabilities: string[] = [WORKSPACE_IDE]) {
  const posted: Posted[] = [];
  const api = new ApiClient("", stubServer(posted, answers, capabilities));
  await api.login("ada@example.com", "pw");
  render(
    <MemoryRouter initialEntries={["/workspace"]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByTitle("count.sql");
  return posted;
}

const of = (posted: Posted[], action: string) => posted.filter((p) => p.action === action);
const treeReads = (posted: Posted[]) => of(posted, "workspace_query_structure").length;

/** The result rows the server answers for `data`, every file with `status`. */
function results(data: Array<Record<string, unknown>>, status: (path: string) => string) {
  return data.map((row) => {
    const path = decodeURIComponent(String(row["file_name"]).replace(/\+/g, " "));
    return { file_name: row["file_name"], path, status: status(path), warnings: [] };
  });
}

describe("New file", () => {
  it("opens an empty tab for the path, and its save adds the file and re-reads the tree", async () => {
    const posted = await mount({
      add: (data) => ({ status: 200, body: { files: results(data, () => "new") } }),
    });
    fireEvent.click(screen.getByRole("button", { name: "New file" }));
    expect(await answerPrompt("runbooks/health_check.rb.json")).toContain("New file in ws");

    const editor = await screen.findByLabelText("Edit runbooks/health_check.rb.json");
    expect(screen.getByRole("tab", { name: /health_check\.rb\.json/ })).toBeTruthy();
    // New and unsaved, even empty: the tab shows the dot and Save is offered.
    expect(screen.getByLabelText("unsaved")).toBeTruthy();
    fireEvent.change(editor, { target: { value: '{"format": 1}' } });

    const readsBefore = treeReads(posted);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(of(posted, "add_workspace_files")).toHaveLength(1));
    expect(of(posted, "add_workspace_files")[0]!.data).toEqual([
      { file_name: "runbooks%2Fhealth_check.rb.json", file_content: '{"format": 1}', replace: false },
    ]);
    expect(of(posted, "save_workspace_file_content")).toEqual([]);
    expect((await screen.findByRole("status")).textContent).toContain("Saved runbooks/health_check.rb.json");
    await waitFor(() => expect(treeReads(posted)).toBeGreaterThan(readsBefore));
    expect(screen.queryByLabelText("unsaved")).toBeNull();
    expect((screen.getByRole("button", { name: "Save" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("starts at the root, or in the folder of the file open, and refuses a path the tree has", async () => {
    const posted = await mount({});
    fireEvent.click(screen.getByRole("button", { name: "New file" }));
    const dialog = await screen.findByRole("dialog");
    expect((within(dialog).getByRole("textbox") as HTMLInputElement).value).toBe("");
    await answerPrompt(EXISTING);
    expect((await screen.findByRole("alert")).textContent).toContain(
      `${EXISTING} is already in the workspace: open it from the tree.`,
    );
    expect(screen.queryByLabelText(`Edit ${EXISTING}`)).toBeNull();

    // A typed path's mistake is caught before a tab opens.
    fireEvent.click(screen.getByRole("button", { name: "New file" }));
    await answerPrompt("runbooks/../outside.sql");
    expect((await screen.findByRole("alert")).textContent).toContain("may not use . or ..");
    expect(of(posted, "add_workspace_files")).toEqual([]);

    // With a file open, the path starts in its folder.
    fireEvent.click(screen.getByTitle("count.sql"));
    await screen.findByLabelText(`Edit ${EXISTING}`);
    fireEvent.click(screen.getByRole("button", { name: "New file" }));
    const second = await screen.findByRole("dialog");
    expect((within(second).getByRole("textbox") as HTMLInputElement).value).toBe("runbooks/health_check/");
    await answerPrompt(null);
  });

  it("shows the server's refusal and keeps the tab new", async () => {
    await mount({
      add: () => ({
        status: 400,
        // As the apiserver writes an error (ERROR, jets/apiserver/utils.go).
        body: {
          error:
            "nothing was added:\nrunbooks/a.sql cannot be saved:\n" +
            "  line 1: a runbook step may not run this statement: BEGIN is transaction control",
        },
      }),
    });
    fireEvent.click(screen.getByRole("button", { name: "New file" }));
    await answerPrompt("runbooks/a.sql");
    fireEvent.change(await screen.findByLabelText("Edit runbooks/a.sql"), { target: { value: "BEGIN;" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect((await screen.findByRole("alert")).textContent).toContain("BEGIN is transaction control");
    expect(screen.getByLabelText("unsaved")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Save" }) as HTMLButtonElement).disabled).toBe(false);
  });
});

/** Picks files in the hidden input named `label`. */
function pick(label: string, files: File[]) {
  const input = screen.getByLabelText(label) as HTMLInputElement;
  Object.defineProperty(input, "files", { value: files, configurable: true });
  fireEvent.change(input);
}

function inFolder(relative: string, content: string | Uint8Array): File {
  const file = new File([content as BlobPart], relative.split("/").pop()!);
  Object.defineProperty(file, "webkitRelativePath", { value: relative });
  return file;
}

describe("Upload", () => {
  it("uploads a folder under the folder named, after showing what it does", async () => {
    const posted = await mount({
      check: (data) => ({
        status: 200,
        body: {
          files: results(data, (p) =>
            p === EXISTING ? "replaces" : p.endsWith(".txt") ? "not_listed" : "new",
          ),
        },
      }),
      add: (data) => ({ status: 200, body: { files: results(data, () => "new") } }),
    });
    const input = screen.getByLabelText("Folder to upload");
    expect(input.hasAttribute("webkitdirectory")).toBe(true);
    pick("Folder to upload", [
      inFolder("health_check/count.sql", "SELECT 1"),
      inFolder("health_check/more.sql", "SELECT 2"),
      inFolder("health_check/notes.txt", "notes"),
    ]);
    expect(await answerPrompt("runbooks")).toContain("Upload 3 files into which folder of ws?");

    await waitFor(() => expect(of(posted, "check_workspace_files")).toHaveLength(1));
    expect(of(posted, "check_workspace_files")[0]!.data.map((r) => r["file_name"])).toEqual([
      "runbooks%2Fhealth_check%2Fcount.sql",
      "runbooks%2Fhealth_check%2Fmore.sql",
      "runbooks%2Fhealth_check%2Fnotes.txt",
    ]);
    const detail = (await screen.findByTestId("prompt-detail")).textContent ?? "";
    expect(detail).toContain("New:\n  runbooks/health_check/more.sql");
    expect(detail).toContain("Replaces the file there:\n  runbooks/health_check/count.sql");
    expect(detail).toContain("Left out, a file the IDE does not list:\n  runbooks/health_check/notes.txt");
    expect(of(posted, "add_workspace_files")).toEqual([]);
    expect(await answerConfirm("OK")).toBe("Upload 2 files to ws?");

    await waitFor(() => expect(of(posted, "add_workspace_files")).toHaveLength(1));
    expect(of(posted, "add_workspace_files")[0]!.data).toEqual([
      { file_name: "runbooks%2Fhealth_check%2Fcount.sql", file_content: "SELECT 1", replace: true },
      { file_name: "runbooks%2Fhealth_check%2Fmore.sql", file_content: "SELECT 2", replace: false },
    ]);
    expect((await screen.findByRole("status")).textContent).toContain("Uploaded 2 files to ws.");
  });

  it("adds nothing when the check refuses a file, and says why for every one", async () => {
    const posted = await mount({
      check: (data) => ({
        status: 200,
        body: {
          files: results(data, (p) => (p.endsWith("bad.sql") ? "refused" : "new")).map((r) =>
            r.status === "refused"
              ? { ...r, message: `${r.path} cannot be saved:\n  line 1: a runbook step may not run this statement: COMMIT` }
              : r,
          ),
        },
      }),
    });
    pick("Files to upload", [new File(["SELECT 1"], "good.sql"), new File(["COMMIT"], "bad.sql")]);
    await answerPrompt("runbooks");
    expect((await screen.findByRole("alert")).textContent).toContain(
      "Nothing was uploaded:\nrunbooks/bad.sql cannot be saved:",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(of(posted, "add_workspace_files")).toEqual([]);
  });

  it("adds nothing when a file is not text, and asks the server nothing", async () => {
    const posted = await mount({});
    pick("Files to upload", [new File(["SELECT 1"], "a.sql"), new File([new Uint8Array([0, 1, 2])], "b.db")]);
    await answerPrompt("");
    expect((await screen.findByRole("alert")).textContent).toContain(
      "Nothing was uploaded: a workspace file is text, and these are not:",
    );
    expect(of(posted, "check_workspace_files")).toEqual([]);
  });

  it("asks nothing of the server when the folder prompt is cancelled", async () => {
    const posted = await mount({});
    pick("Files to upload", [new File(["SELECT 1"], "a.sql")]);
    await answerPrompt(null);
    expect(of(posted, "check_workspace_files")).toEqual([]);
  });
});

describe("without workspace_ide", () => {
  it("offers none of the three", async () => {
    await mount({}, []);
    for (const name of ["New file", "Upload files", "Upload folder"]) {
      expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
    }
  });
});

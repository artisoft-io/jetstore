/**
 * @vitest-environment jsdom
 *
 * **A workspace's README opens read-only.** jetstore_maintenance_02 `AL.1`,
 * 2026-10-27.
 *
 * The server lists a workspace's `.md` files and marks each node `read_only`
 * (`wsfile.ReadOnlySuffixes`), and refuses a save of one. This pins the
 * editor's half: the tab opens read-only, Save stays disabled however the
 * buffer changes, and no save request is sent. A writable file beside it is the
 * control, so a Save button that is simply always disabled does not pass.
 *
 * The editor is replaced by a text box, as in `WorkspaceIde.closeTab.test.tsx`:
 * what the screen hands it is what is under test.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());

vi.mock("../editor/Editor", () => ({
  Editor: ({ fileName, content, readOnly, onChange, onSave }: EditorProps) => (
    <textarea
      aria-label={`Edit ${fileName}`}
      defaultValue={content}
      readOnly={readOnly}
      onChange={(event) => onChange?.(event.target.value)}
      // The real editor's Mod-s, which calls the screen's save directly and so
      // does not go through the disabled button.
      onKeyDown={(event) => {
        if (event.ctrlKey && event.key === "s") onSave?.();
      }}
    />
  ),
}));

afterEach(cleanup);

const README = "README.md";
const RULES = "jet_rules/a.jr";

function fileNode(name: string, readOnly: boolean) {
  return {
    key: name,
    pageMatchKey: name,
    type: "file",
    size: 8,
    label: name,
    ...(readOnly ? { read_only: true } : {}),
    route_params: { workspace_name: "ws", file_name: encodeURIComponent(name) },
    children: null,
  };
}

function stubServer(saves: string[]) {
  return vi.fn(async (url: string | URL, init?: RequestInit) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Michel",
          user_email: "michel@artisoft.io",
          is_admin: false,
          capabilities: [WORKSPACE_IDE],
        }),
        { status: 200 },
      );
    }
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    switch (body["action"]) {
      case "raw_query":
        return new Response(JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"]] }), {
          status: 200,
        });
      case "workspace_query_structure":
        return new Response(
          JSON.stringify({
            result_type: "workspace_file_structure",
            result_data: [fileNode(RULES, false), fileNode(README, true)],
          }),
          { status: 200 },
        );
      case "get_workspace_file_content":
        return new Response(JSON.stringify({ file_content: "# text\n" }), { status: 200 });
      case "save_workspace_file_content":
        saves.push(JSON.stringify(body["data"]));
        return new Response(JSON.stringify({}), { status: 200 });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

async function mount(saves: string[]) {
  const api = new ApiClient("", stubServer(saves));
  await api.login("michel@artisoft.io", "pw");
  render(
    <MemoryRouter initialEntries={["/workspace"]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

async function open(name: string): Promise<HTMLTextAreaElement> {
  fireEvent.click(await screen.findByTitle(name));
  return (await screen.findByLabelText(`Edit ${name}`)) as HTMLTextAreaElement;
}

const saveButton = () => screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;

describe("a README in the Workspace IDE", () => {
  it("opens read-only, and Save stays disabled after a change", async () => {
    const saves: string[] = [];
    await mount(saves);
    const editor = await open(README);

    expect(editor.readOnly).toBe(true);
    expect(screen.getByText("Read only")).toBeTruthy();

    fireEvent.change(editor, { target: { value: "# edited\n" } });
    expect(saveButton().disabled).toBe(true);
    fireEvent.click(saveButton());
    fireEvent.keyDown(editor, { key: "s", ctrlKey: true });
    expect(saves).toEqual([]);
  });

  it("leaves a file the server does not mark writable, as before", async () => {
    const saves: string[] = [];
    await mount(saves);
    const editor = await open(RULES);

    expect(editor.readOnly).toBe(false);
    expect(screen.queryByText("Read only")).toBeNull();
    fireEvent.change(editor, { target: { value: "# edited\n" } });
    expect(saveButton().disabled).toBe(false);
    fireEvent.keyDown(editor, { key: "s", ctrlKey: true });
    await waitFor(() => expect(saves.length).toBe(1));
  });
});

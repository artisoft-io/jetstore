/**
 * @vitest-environment jsdom
 *
 * **Switching tabs keeps a tab's unsaved edits on screen.** `jetstore_maintenance_02`
 * `I-65`, 2026-10-27. The editor now makes a fresh state for each file it is shown,
 * so what it shows on a switch is the `content` the screen passes. That is the
 * tab's current text. It was its saved text, and the old editor's replace
 * transaction then reset the tab to it, so edits were lost by leaving a tab and
 * coming back.
 *
 * The editor is replaced by a text box that, like the real one, takes `content`
 * only when a different file (`docKey`) is shown, and reports edits through
 * `onChange`. `Editor.test.tsx` pins the real editor's side.
 */

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());
afterEach(cleanup);

vi.mock("../editor/Editor", () => ({
  Editor: ({ docKey, content, onChange }: EditorProps) => (
    <textarea
      key={docKey}
      aria-label="Editor"
      defaultValue={content}
      onChange={(event) => onChange?.(event.target.value)}
    />
  ),
}));

const FILE_A = "jet_rules/a.jr";
const FILE_B = "jet_rules/b.jr";
const SAVED: Record<string, string> = { [FILE_A]: "# a, saved\n", [FILE_B]: "# b, saved\n" };

function fileNode(name: string) {
  return {
    key: name,
    pageMatchKey: name,
    type: "file",
    size: 12,
    label: name,
    route_params: { workspace_name: "ws", file_name: encodeURIComponent(name) },
    children: null,
  };
}

function stubServer() {
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
            result_data: [
              {
                key: "jet_rules",
                pageMatchKey: "jet_rules",
                type: "dir",
                size: 0,
                label: "jet_rules",
                children: [fileNode(FILE_A), fileNode(FILE_B)],
              },
            ],
          }),
          { status: 200 },
        );
      case "get_workspace_file_content": {
        const data = (body["data"] as Array<Record<string, string>> | undefined)?.[0] ?? {};
        const name = decodeURIComponent(data["file_name"] ?? "");
        return new Response(JSON.stringify({ file_content: SAVED[name] ?? "" }), { status: 200 });
      }
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

async function mountIde() {
  const api = new ApiClient("", stubServer());
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

async function open(file: string) {
  fireEvent.click(await screen.findByTitle(file));
  await screen.findByRole("button", { name: `Close ${file}` });
}

function tab(file: string) {
  const name = file.split("/").pop()!;
  return screen.getAllByRole("tab").find((t) => within(t).queryByText(name) !== null)!;
}

const editor = () => screen.getByLabelText("Editor") as HTMLTextAreaElement;

describe("switching tabs in the Workspace IDE", () => {
  it("shows a tab's unsaved edits again when it is shown again", async () => {
    await mountIde();
    await open(FILE_A);
    expect(editor().value).toBe(SAVED[FILE_A]);
    fireEvent.change(editor(), { target: { value: "# a, edited\n" } });

    await open(FILE_B);
    expect(editor().value).toBe(SAVED[FILE_B]);

    fireEvent.click(within(tab(FILE_A)).getAllByRole("button")[0]!);
    expect(editor().value).toBe("# a, edited\n");
    expect(within(tab(FILE_A)).getByLabelText("unsaved")).toBeTruthy();
  });

  it("shows a clean tab's saved text", async () => {
    await mountIde();
    await open(FILE_A);
    await open(FILE_B);

    fireEvent.click(within(tab(FILE_A)).getAllByRole("button")[0]!);
    expect(editor().value).toBe(SAVED[FILE_A]);
    expect(within(tab(FILE_A)).queryByLabelText("unsaved")).toBeNull();
  });
});

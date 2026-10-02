/**
 * @vitest-environment jsdom
 *
 * **Closing a tab with unsaved changes asks first, in the app's own dialog.**
 * jetstore_maintenance_02 `I-39`, 2026-10-02.
 *
 * It asked with `window.confirm` until then, which the embedded browser pane
 * never shows — there it answered `false` at once, so a dirty tab could not be
 * closed at all. Nothing tested the question before; this pins both answers
 * and the case that does not ask.
 *
 * The editor is replaced by a text box: CodeMirror is not what is under test,
 * and what the screen needs from it is the one `onChange` that makes a tab dirty.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { answerConfirm } from "../shell/promptsTesting";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

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

const FILE = "jet_rules/a.jr";

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
                children: [
                  {
                    key: FILE,
                    pageMatchKey: FILE,
                    type: "file",
                    size: 12,
                    label: FILE,
                    route_params: { workspace_name: "ws", file_name: "jet_rules%2Fa.jr" },
                    children: null,
                  },
                ],
              },
            ],
          }),
          { status: 200 },
        );
      case "get_workspace_file_content":
        return new Response(JSON.stringify({ file_content: "# a rule file\n" }), { status: 200 });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

/** Mounts the IDE in the shell and opens the one file. */
async function openFile() {
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
  fireEvent.click(await screen.findByTitle(FILE));
  await screen.findByRole("button", { name: `Close ${FILE}` });
}

const closeButton = () => screen.queryByRole("button", { name: `Close ${FILE}` });

describe("closing a tab in the Workspace IDE", () => {
  it("closes a clean tab without asking", async () => {
    await openFile();
    fireEvent.click(closeButton()!);
    await waitFor(() => expect(closeButton()).toBeNull());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("asks about a dirty tab, and keeps it open on Cancel", async () => {
    await openFile();
    fireEvent.change(screen.getByLabelText(`Edit ${FILE}`), { target: { value: "# edited\n" } });
    fireEvent.click(closeButton()!);

    expect(await answerConfirm("Cancel")).toBe(`${FILE} has unsaved changes. Close it anyway?`);
    expect(closeButton()).not.toBeNull();
  });

  it("closes a dirty tab on OK", async () => {
    await openFile();
    fireEvent.change(screen.getByLabelText(`Edit ${FILE}`), { target: { value: "# edited\n" } });
    fireEvent.click(closeButton()!);

    await answerConfirm("OK");
    await waitFor(() => expect(closeButton()).toBeNull());
  });
});

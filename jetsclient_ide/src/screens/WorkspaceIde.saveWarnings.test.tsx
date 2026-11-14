/**
 * @vitest-environment jsdom
 *
 * **A save the server accepts with warnings shows them.** The save check's
 * validators report findings with a severity, and until November 2026 the
 * server dropped the warnings, so a misspelt pipeline configuration key saved
 * with nothing said. The response now carries them (`warnings`, see
 * `WorkspaceApi.saveFile`), and the editor shows them in the status banner in
 * the warning colours, each at its JSON Pointer or line, without blocking: the
 * tab is saved. A save with no warning keeps the plain confirmation, and a
 * refused one the error banner, as before.
 *
 * The editor is replaced by a text box, as in `WorkspaceIde.readOnly.test.tsx`.
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
  Editor: ({ fileName, content, onChange, onSave }: EditorProps) => (
    <textarea
      aria-label={`Edit ${fileName}`}
      defaultValue={content}
      onChange={(event) => onChange?.(event.target.value)}
      onKeyDown={(event) => {
        if (event.ctrlKey && event.key === "s") onSave?.();
      }}
    />
  ),
}));

afterEach(cleanup);

const CONFIG = "pipes_config/a.pc.json";

/** What the server answers a save with: a status and a body. */
type SaveAnswer = { status: number; body: unknown };

function stubServer(answer: SaveAnswer, saves: string[]) {
  return vi.fn(async (url: string | URL, init?: RequestInit) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Ada",
          user_email: "ada@example.com",
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
                key: CONFIG,
                pageMatchKey: CONFIG,
                type: "file",
                size: 8,
                label: CONFIG,
                route_params: { workspace_name: "ws", file_name: encodeURIComponent(CONFIG) },
                children: null,
              },
            ],
          }),
          { status: 200 },
        );
      case "get_workspace_file_content":
        return new Response(JSON.stringify({ file_content: '{"cluster_config": {}}' }), { status: 200 });
      case "save_workspace_file_content":
        saves.push(JSON.stringify(body["data"]));
        return new Response(JSON.stringify(answer.body), { status: answer.status });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

/** Mounts the IDE in the shell, opens the configuration, edits it and saves. */
async function editAndSave(answer: SaveAnswer, saves: string[]) {
  const api = new ApiClient("", stubServer(answer, saves));
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
  fireEvent.click(await screen.findByTitle(CONFIG));
  const editor = await screen.findByLabelText(`Edit ${CONFIG}`);
  fireEvent.change(editor, { target: { value: '{"cluster_config": {}, "output_table": []}' } });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(saves.length).toBe(1));
}

const saveButton = () => screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;

describe("saving a file the server warns about", () => {
  it("saves it and shows each warning at its coordinate, in the warning colours", async () => {
    const saves: string[] = [];
    await editAndSave(
      {
        status: 200,
        body: {
          file_name: CONFIG,
          warnings: [
            {
              severity: "warning",
              code: "cpipes_config_unknown_field",
              message: 'ignored when the pipeline runs: unknown field "output_table"',
              path: "/output_table",
            },
            { severity: "warning", code: "x", message: "a finding on a line", line: 4 },
          ],
        },
      },
      saves,
    );
    const banner = await screen.findByRole("status");
    expect(banner.className).toContain("banner-warn");
    expect(banner.textContent).toContain(`Saved ${CONFIG}, with 2 warnings:`);
    expect(banner.textContent).toContain('/output_table: ignored when the pipeline runs: unknown field "output_table"');
    expect(banner.textContent).toContain("line 4: a finding on a line");
    expect(screen.queryByRole("alert")).toBeNull();
    // Not blocking: the tab is saved, so Save has nothing left to do.
    await waitFor(() => expect(saveButton().disabled).toBe(true));
  });

  it("keeps the plain confirmation when there is no warning, and from a server that sends none", async () => {
    for (const body of [{ file_name: CONFIG, warnings: [] }, { file_name: CONFIG }]) {
      const saves: string[] = [];
      await editAndSave({ status: 200, body }, saves);
      const banner = await screen.findByRole("status");
      expect(banner.className).toContain("banner-ok");
      expect(banner.textContent).toContain(`Saved ${CONFIG}`);
      expect(banner.textContent).not.toContain("warning");
      cleanup();
    }
  });

  it("shows a refusal as an error, and leaves the tab unsaved", async () => {
    const saves: string[] = [];
    await editAndSave(
      {
        status: 400,
        body: {
          error: `${CONFIG} cannot be saved:\n  /cluster_config: the pipeline cannot start: cluster_config is required`,
        },
      },
      saves,
    );
    const alert = await screen.findByRole("alert");
    expect(alert.className).toContain("banner-error");
    expect(alert.textContent).toContain("cluster_config is required");
    expect(screen.queryByRole("status")).toBeNull();
    expect(saveButton().disabled).toBe(false);
  });
});

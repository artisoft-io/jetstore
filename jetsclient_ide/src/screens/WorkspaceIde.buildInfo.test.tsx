/**
 * @vitest-environment jsdom
 *
 * **The Workspace IDE shows the build information once.** jetstore_maintenance_02
 * Phase 1, task AC.3, for D05.
 *
 * The shell draws the build line in a footer at the bottom of every screen, and
 * this screen already has a bottom bar of its own — so here the footer stands
 * down and the line joins the status bar instead. The failure this guards is the
 * obvious one, two bars carrying the same text, and the less obvious one: the
 * status bar used to follow the editor only, so with no file open the line
 * would have been nowhere at all.
 *
 * jsdom has no layout, so *at the bottom* is a browser check (plan `AC.4`). What
 * is asserted here is *how many*, and *in which bar*.
 *
 * The shell is rendered rather than the screen alone, as in
 * `CompiledView.test.tsx`: the suppression is a conversation between the two.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import { AppShell } from "../shell/AppShell";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

afterEach(() => {
  cleanup();
  localStorage.clear();
});

const LINE = "jets_ai-6aeb790 · built 2025-10-01";

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
          jetstore_version: "1759338000",
          jetstore_git_sha: "jets_ai-6aeb790",
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
                    key: "jet_rules/a.jr",
                    pageMatchKey: "jet_rules/a.jr",
                    type: "file",
                    size: 12,
                    label: "jet_rules/a.jr",
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

async function mount() {
  const api = new ApiClient("", stubServer());
  await api.login("michel@artisoft.io", "pw");
  render(
    <MemoryRouter initialEntries={["/workspace"]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[{ to: "/other", label: "Other" }]} />}>
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
          <Route path="other" element={<p>another screen</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByTitle("jet_rules/a.jr");
}

/** Every element whose own text is the build line. */
const buildLines = () => screen.queryAllByText(LINE);

describe("the Workspace IDE and the build information", () => {
  it("shows it once, in the status bar, with no file open", async () => {
    await mount();
    await waitFor(() => expect(buildLines()).toHaveLength(1));
    expect(buildLines()[0]!.closest(".statusbar")).not.toBeNull();
    expect(screen.queryByRole("contentinfo", { name: "Build information" })).toBeNull();
  });

  it("shows it once, beside the file's own entries, with a file open", async () => {
    await mount();
    fireEvent.click(screen.getByTitle("jet_rules/a.jr"));
    await screen.findByRole("button", { name: "Close jet_rules/a.jr" });

    await waitFor(() => expect(buildLines()).toHaveLength(1));
    const bar = buildLines()[0]!.closest(".statusbar");
    expect(bar).not.toBeNull();
    expect(bar!.textContent).toContain("Saved");
    expect(document.querySelectorAll(".statusbar")).toHaveLength(1);
    expect(screen.queryByRole("contentinfo", { name: "Build information" })).toBeNull();
  });

  it("gives the line back to the shell's footer on leaving the IDE", async () => {
    await mount();
    fireEvent.click(screen.getByRole("link", { name: "Other" }));
    await screen.findByText("another screen");

    const footer = await screen.findByRole("contentinfo", { name: "Build information" });
    expect(footer.textContent).toBe(LINE);
    expect(buildLines()).toHaveLength(1);
  });
});

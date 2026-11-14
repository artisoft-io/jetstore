/**
 * @vitest-environment jsdom
 *
 * **The Workspace IDE's Documentation section** (2026-11-13): a workspace's
 * written pages under `docs/`, and the guide's settings file, opened from the
 * tree, edited and saved, and mapped by the *Workspace guide* button to the
 * page's own guide URL.
 *
 * The tree is the shape `wsfile.BuildWorkspaceFileStructure` emits for the
 * `docs` section (`jets/datatable/wsfile/sections.go`): a section node, a
 * subfolder as a `dir` node, and file nodes whose `file_name` is the escaped
 * workspace path and which carry no `read_only`, unlike a README elsewhere.
 * The editor is replaced by a text box, as in `WorkspaceIde.readOnly.test.tsx`.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient } from "../api/client";
import type { EditorProps } from "../editor/Editor";
import { AppShell } from "../shell/AppShell";
import { DOCS_TAB_NAME } from "../shell/helpTopics";
import { WORKSPACE_MANIFEST_PATH } from "../shell/workspaceGuide";
import { WorkspaceIde, WORKSPACE_IDE } from "./WorkspaceIde";

beforeEach(() => window.sessionStorage.clear());

vi.mock("../editor/Editor", () => ({
  Editor: ({ fileName, content, readOnly, onChange }: EditorProps) => (
    <textarea
      aria-label={`Edit ${fileName}`}
      defaultValue={content}
      readOnly={readOnly}
      onChange={(event) => onChange?.(event.target.value)}
    />
  ),
}));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

const PAGE = "docs/customisation/observe.md";
const SETTINGS = "docs/workspace_docs.json";

/** The guide's manifest, as docs/scripts/workspace_docs.mjs writes it: written pages keyed `docs/...`. */
const MANIFEST = {
  workspace: "ws",
  index: "/docs/workspace/",
  pages: { [PAGE]: "/docs/workspace/customisation/observe/" },
};

/** A file node as wsfile's visitDir writes it: label the base name, file_name the escaped path. */
function fileNode(path: string) {
  const name = path.slice(path.lastIndexOf("/") + 1);
  return {
    key: name,
    pageMatchKey: encodeURIComponent(path),
    type: "file",
    size: 40,
    label: name,
    route_path: "/workspace/:workspace_name/home",
    route_params: { workspace_name: "ws", file_name: encodeURIComponent(path), label: name },
    children: null,
  };
}

const DOCS_SECTION = {
  key: "docs",
  pageMatchKey: "docs",
  type: "section",
  size: 0,
  label: "Documentation",
  route_path: "/workspace/:workspace_name/home",
  route_params: { workspace_name: "ws", label: "Documentation" },
  children: [
    {
      key: "customisation",
      pageMatchKey: "customisation",
      type: "dir",
      size: 0,
      label: "customisation",
      route_params: { workspace_name: "ws", label: "customisation" },
      children: [fileNode(PAGE)],
    },
    fileNode(SETTINGS),
  ],
};

interface Saved {
  file_name: string;
  file_content: string;
}

function stubServer(saves: Saved[], reads: string[]) {
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
    if (String(url) === "/docsSession") return new Response(null, { status: 204 });
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    switch (body["action"]) {
      case "raw_query":
        return new Response(JSON.stringify({ rows: [["ws", "git@example.com:ws.git", "main"]] }), {
          status: 200,
        });
      case "workspace_query_structure":
        return new Response(
          JSON.stringify({ result_type: "workspace_file_structure", result_data: [DOCS_SECTION] }),
          { status: 200 },
        );
      case "get_workspace_file_content": {
        const [row] = body["data"] as { file_name: string }[];
        reads.push(row!.file_name);
        return new Response(JSON.stringify({ file_content: "---\ntitle: Observe\n---\n" }), { status: 200 });
      }
      case "save_workspace_file_content":
        saves.push(...(body["data"] as Saved[]));
        return new Response(JSON.stringify({}), { status: 200 });
      default:
        return new Response(JSON.stringify({ rows: [], totalRowCount: 0 }), { status: 200 });
    }
  }) as unknown as typeof fetch;
}

async function mount(saves: Saved[] = [], reads: string[] = []) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string | URL) =>
      String(url) === WORKSPACE_MANIFEST_PATH
        ? new Response(JSON.stringify(MANIFEST))
        : new Response("", { status: 404 }),
    ),
  );
  const api = new ApiClient("", stubServer(saves, reads));
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
  await screen.findByText("Documentation");
}

/** Expands the page's folder and opens it; returns its editor. */
async function openPage(): Promise<HTMLTextAreaElement> {
  fireEvent.click(await screen.findByRole("button", { name: "Expand customisation" }));
  fireEvent.click(await screen.findByTitle("observe.md"));
  return (await screen.findByLabelText(`Edit ${PAGE}`)) as HTMLTextAreaElement;
}

const saveButton = () => screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;

describe("the Documentation section", () => {
  it("opens a page in a subfolder by its workspace path, editable", async () => {
    const reads: string[] = [];
    await mount([], reads);
    const editor = await openPage();

    expect(reads).toEqual([encodeURIComponent(PAGE)]);
    expect(editor.readOnly).toBe(false);
    expect(screen.queryByText("Read only")).toBeNull();
  });

  it("saves an edited page under its workspace path", async () => {
    const saves: Saved[] = [];
    await mount(saves);
    const editor = await openPage();

    fireEvent.change(editor, { target: { value: "---\ntitle: Observe\n---\nedited\n" } });
    expect(saveButton().disabled).toBe(false);
    fireEvent.click(saveButton());

    await waitFor(() => expect(saves.length).toBe(1));
    expect(saves[0]).toEqual({
      file_name: encodeURIComponent(PAGE),
      file_content: "---\ntitle: Observe\n---\nedited\n",
    });
  });

  it("saves the guide's settings file", async () => {
    const saves: Saved[] = [];
    await mount(saves);
    fireEvent.click(await screen.findByTitle("workspace_docs.json"));
    const editor = (await screen.findByLabelText(`Edit ${SETTINGS}`)) as HTMLTextAreaElement;

    fireEvent.change(editor, { target: { value: '{ "rule_text": true }' } });
    fireEvent.click(saveButton());

    await waitFor(() => expect(saves.length).toBe(1));
    expect(saves[0]!.file_name).toBe(encodeURIComponent(SETTINGS));
  });

  it("opens the page's own guide URL from the Workspace guide button", async () => {
    await mount();
    const button = await screen.findByRole("button", { name: "Workspace guide" });
    await openPage();

    const tab = { opener: "the app" as unknown, location: { href: "about:blank" }, close: vi.fn(), focus: vi.fn() };
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    fireEvent.click(button);

    expect(open).toHaveBeenCalledWith("", DOCS_TAB_NAME);
    await waitFor(() => expect(tab.location.href).toBe("/docs/workspace/customisation/observe/"));
  });

  it("opens the guide's landing page for the settings file, which is not a page", async () => {
    await mount();
    const button = await screen.findByRole("button", { name: "Workspace guide" });
    fireEvent.click(await screen.findByTitle("workspace_docs.json"));
    await screen.findByLabelText(`Edit ${SETTINGS}`);

    const tab = { opener: "the app" as unknown, location: { href: "about:blank" }, close: vi.fn(), focus: vi.fn() };
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    fireEvent.click(button);

    await waitFor(() => expect(tab.location.href).toBe("/docs/workspace/"));
  });
});

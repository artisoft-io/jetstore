/**
 * The Workspace IDE, as a screen.
 *
 * This is Phase 1's `App.tsx` with three things lifted out into the shell — the
 * session gate, the theme toggle and the banner region — and nothing else
 * changed. The editor, the tab model, the dirty tracking and the save path are
 * the code that has been deployed since Phase 1; A.1 moved it, it did not
 * rewrite it.
 *
 * **Screens own their own toolbar.** The shell's top bar carries what is true of
 * the whole application — who is signed in, where they can go, the theme. The
 * workspace picker and the Save button belong to this screen and would be
 * meaningless on another, so they render in a second row here rather than being
 * pushed up through a slot. That keeps the shell's contract to one thing —
 * `<Outlet/>` plus notifications — which is what makes a second screen cheap.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import type { ApiClient } from "../api/client";
import { SessionExpiredError } from "../api/client";
import { WorkspaceApi, fileNameOf, type WorkspaceNode, type WorkspaceSummary } from "../api/workspace";
import { FileTree } from "../components/FileTree";
import { Editor } from "../editor/Editor";
import { isServerValidatedJson, languageNameFor } from "../editor/language";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import { CompiledView } from "./CompiledView";
import { compiledViewFor, compiledViews } from "./sectionContract";

/** The capability the server requires for every workspace action. */
export const WORKSPACE_IDE = "workspace_ide";

/**
 * An open tab. Task C.3 made this a union of two.
 *
 * **The Flutter screen has had two kinds since it was written** — its
 * `TabBarView` renders a `JetsForm` for a file and a `JetsFormWithTabs` for a
 * compiled view (`jetsclient/lib/screens/screen_tab_form.dart`,
 * `ScreenWithTabsWithForm`) — and this app carried only the first, because Phase
 * 1 ported the editor and nothing had ported the view. The discriminant is
 * explicit rather than inferred from which fields are present: a file tab is
 * identified by its escaped path and a view tab by the compiled view it shows,
 * and neither key can be mistaken for the other.
 */
type Tab =
  | {
      kind: "file";
      fileName: string;
      label: string;
      /** Text as last loaded or saved; the dirty check compares against this. */
      saved: string;
      current: string;
      size: number;
    }
  | {
      kind: "view";
      /** The `wsfile.CompiledView` value, which is also this tab's key. */
      view: string;
      label: string;
    };

/** The key a tab is addressed by, unique across both kinds. */
function tabKey(tab: Tab): string {
  return tab.kind === "file" ? `file:${tab.fileName}` : `view:${tab.view}`;
}

function isDirty(tab: Tab): boolean {
  return tab.kind === "file" && tab.current !== tab.saved;
}

export function WorkspaceIde({ api }: { api: ApiClient }) {
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const { setError, setStatus } = useNotifications();
  /**
   * The workspace named by the route, when there is one. Task C.3.
   *
   * **Two routes, one screen**: `/workspace` is Phase 1's, where the picker
   * chooses; `/workspaces/:workspace_name/home` is the Flutter path, where the
   * url chooses and the picker follows. The parameter is the source of truth when
   * it is present, so a back button and a bookmark both work — which is the
   * property the Flutter screen does not have, because its section list is client
   * state written by the button that navigates to it.
   */
  const routeWorkspace = useParams()["workspace_name"];
  const navigate = useNavigate();

  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [workspace, setWorkspace] = useState<string>(routeWorkspace ?? "");
  const [tree, setTree] = useState<WorkspaceNode[]>([]);
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const canEdit = api.can(WORKSPACE_IDE);
  const activeTab = useMemo(
    () => tabs.find((t) => tabKey(t) === activeKey) ?? null,
    [tabs, activeKey],
  );
  /** Only a file tab highlights a row in the tree; a view tab highlights none. */
  const activeFile = activeTab?.kind === "file" ? activeTab.fileName : null;
  const dirtyCount = tabs.filter(isDirty).length;

  /** Routes every failure to the banner, and a dead session back to the login screen. */
  const guard = useCallback(
    async (work: () => Promise<void>) => {
      try {
        await work();
      } catch (err) {
        if (err instanceof SessionExpiredError) {
          setTabs([]);
          setActiveKey(null);
          setTree([]);
        }
        setError(err instanceof Error ? err.message : String(err));
      }
    },
    [setError],
  );

  // Warn before discarding unsaved buffers on a reload or tab close.
  useEffect(() => {
    if (dirtyCount === 0) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirtyCount]);

  useEffect(() => {
    void guard(async () => {
      const list = await workspaceApi.listWorkspaces();
      setWorkspaces(list);
      // The route wins when it names one; otherwise the first is as good a
      // default as any, which is what Phase 1 has always done.
      if (list.length > 0 && workspace === "") setWorkspace(list[0]!.name);
    });
    // Once, on mount: the shell guarantees a session, and picking a workspace is
    // handled below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The route changing under the screen — a second `Open`, a back button — is a
  // workspace change like any other, and the effect below does the rest.
  useEffect(() => {
    if (routeWorkspace !== undefined && routeWorkspace !== workspace) {
      setWorkspace(routeWorkspace);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeWorkspace]);

  useEffect(() => {
    if (workspace === "") return;
    setTabs([]);
    setActiveKey(null);
    setBusy(true);
    void guard(async () => {
      setTree(await workspaceApi.fileTree(workspace));
    }).finally(() => setBusy(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace]);

  const openFile = useCallback(
    (node: WorkspaceNode) => {
      const fileName = fileNameOf(node);
      if (!fileName) return;
      if (tabs.some((t) => t.kind === "file" && t.fileName === fileName)) {
        setActiveKey(`file:${fileName}`);
        return;
      }
      setBusy(true);
      setStatus(null);
      void guard(async () => {
        const file = await workspaceApi.readFile(workspace, node);
        setTabs((prev) => [
          ...prev,
          {
            kind: "file",
            fileName: file.fileName,
            label: file.label,
            saved: file.content,
            current: file.content,
            size: node.size,
          },
        ]);
        setActiveKey(`file:${file.fileName}`);
      }).finally(() => setBusy(false));
    },
    [guard, setStatus, tabs, workspace, workspaceApi],
  );

  /**
   * Open a section's compiled view. Task C.3.
   *
   * **No fetch here, which is the difference from `openFile`.** A file tab has
   * to read the file before it can be drawn; a compiled view is a bundled
   * document and its tables fetch themselves once mounted. So this cannot fail,
   * and there is nothing to guard.
   */
  const openView = useCallback(
    (node: WorkspaceNode) => {
      const document = compiledViewFor(node.compiled_view);
      if (document === null) return;
      const key = `view:${document.view}`;
      if (!tabs.some((t) => tabKey(t) === key)) {
        setTabs((prev) => [...prev, { kind: "view", view: document.view, label: document.label }]);
      }
      setActiveKey(key);
    },
    [tabs],
  );

  /**
   * Change workspace, closing every tab in the same update. Task C.3.
   *
   * **The effect below already does this and doing it here as well is not
   * redundant**, which is worth a sentence because it looks it. An effect runs
   * *after* the render that changed the workspace, and a compiled view's tables
   * key their fetch on the workspace name — so for exactly one render the old
   * tabs are mounted with the new name, and every one of them issues a query
   * against a workspace the user is leaving. Measured: three wasted requests per
   * switch, in `CompiledView.test.tsx`. Clearing in the same batch as the change
   * means that render never happens.
   */
  const pickWorkspace = useCallback(
    (name: string) => {
      setTabs([]);
      setActiveKey(null);
      setWorkspace(name);
      // On the addressable route the url is the state, so the picker navigates
      // rather than diverging from it; on the bare route there is nothing to keep
      // in step. `replace`, because picking a workspace is not a place in the
      // history a back button should return to one entry at a time.
      if (routeWorkspace !== undefined) {
        navigate(`/workspaces/${encodeURIComponent(name)}/home`, { replace: true });
      }
    },
    [navigate, routeWorkspace],
  );

  /** Whether a tree node is a section this app renders a compiled view for. */
  const canOpenView = useCallback(
    (node: WorkspaceNode) => node.type === "section" && compiledViewFor(node.compiled_view) !== null,
    [],
  );

  const closeTab = useCallback(
    (key: string) => {
      const tab = tabs.find((t) => tabKey(t) === key);
      if (tab && isDirty(tab)) {
        const ok = window.confirm(`${tab.label} has unsaved changes. Close it anyway?`);
        if (!ok) return;
      }
      setTabs((prev) => {
        const next = prev.filter((t) => tabKey(t) !== key);
        setActiveKey((cur) =>
          cur === key ? (next.length > 0 ? tabKey(next[next.length - 1]!) : null) : cur,
        );
        return next;
      });
    },
    [tabs],
  );

  const save = useCallback(() => {
    const tab = tabs.find((t) => tabKey(t) === activeKey);
    // A compiled view is read-only by construction: it shows what the compiler
    // produced, and the way to change it is to edit the sources below the
    // heading. Save is disabled on it rather than silently doing nothing.
    if (!tab || tab.kind !== "file" || tab.current === tab.saved || !canEdit) return;

    // The server parses .json before writing, so catch it here and point at the
    // problem rather than surfacing a bare 400.
    if (isServerValidatedJson(tab.fileName)) {
      try {
        JSON.parse(tab.current);
      } catch (err) {
        setError(
          `${tab.label} is not valid JSON, so the server would reject it: ${
            err instanceof Error ? err.message : String(err)
          }`,
        );
        return;
      }
    }

    setBusy(true);
    setError(null);
    void guard(async () => {
      await workspaceApi.saveFile(workspace, tab.fileName, tab.current);
      setTabs((prev) =>
        prev.map((t) =>
          t.kind === "file" && t.fileName === tab.fileName ? { ...t, saved: t.current } : t,
        ),
      );
      setStatus(`Saved ${tab.label}`);
    }).finally(() => setBusy(false));
  }, [activeKey, canEdit, guard, setError, setStatus, tabs, workspace, workspaceApi]);

  const onChange = useCallback(
    (value: string) => {
      setTabs((prev) =>
        prev.map((t) =>
          t.kind === "file" && t.fileName === activeFile ? { ...t, current: value } : t,
        ),
      );
    },
    [activeFile],
  );

  return (
    <>
      <div className="screenbar">
        <label className="ws-picker">
          <span className="sr-only">Workspace</span>
          <select value={workspace} onChange={(e) => pickWorkspace(e.target.value)}>
            {workspaces.length === 0 && <option value="">No workspaces</option>}
            {workspaces.map((w) => (
              <option key={w.name} value={w.name}>
                {w.name}
                {w.branch ? ` (${w.branch})` : ""}
              </option>
            ))}
          </select>
        </label>

        <div className="spacer" />

        {busy && <span className="pill">Working…</span>}
        {!canEdit && <span className="pill pill-warn">Read only — no {WORKSPACE_IDE}</span>}

        <ActionButton
          capability={WORKSPACE_IDE}
          className="btn btn-primary"
          onClick={save}
          disabled={!activeTab || activeTab.kind !== "file" || activeTab.current === activeTab.saved || busy}
        >
          Save
        </ActionButton>
      </div>

      <div className="body">
        <aside className="sidebar">
          <FileTree
            nodes={tree}
            activeFile={activeFile}
            onOpen={openFile}
            canOpenView={canOpenView}
            onOpenView={openView}
          />
        </aside>

        <main className="main">
          <div className="tabbar" role="tablist">
            {tabs.map((t) => {
              const key = tabKey(t);
              return (
                <div
                  key={key}
                  className={`tab${key === activeKey ? " is-active" : ""}`}
                  role="tab"
                  aria-selected={key === activeKey}
                >
                  <button type="button" className="tab-label" onClick={() => setActiveKey(key)}>
                    {isDirty(t) && <span className="dot" aria-label="unsaved" />}
                    {t.kind === "file" ? t.label.split("/").pop() : t.label}
                  </button>
                  <button
                    type="button"
                    className="tab-close"
                    onClick={() => closeTab(key)}
                    aria-label={`Close ${t.label}`}
                  >
                    ×
                  </button>
                </div>
              );
            })}
          </div>

          {activeTab?.kind === "view" ? (
            <CompiledView api={api} document={compiledViews[activeTab.view]!} workspaceName={workspace} />
          ) : activeTab ? (
            <>
              <Editor
                docKey={activeTab.fileName}
                fileName={activeTab.label}
                content={activeTab.saved}
                readOnly={!canEdit}
                onChange={onChange}
                onSave={save}
              />
              <footer className="statusbar">
                <span title={activeTab.label}>{activeTab.label}</span>
                <span className="spacer" />
                <span>{languageNameFor(activeTab.label)}</span>
                <span>{activeTab.current.length.toLocaleString()} chars</span>
                <span>{activeTab.current !== activeTab.saved ? "Modified" : "Saved"}</span>
              </footer>
            </>
          ) : (
            <div className="empty">
              <p>Select a file to start editing.</p>
              <p className="empty-sub">
                Every file in the workspace opens here, whatever its size. A section heading whose
                files compile into the workspace database opens the compiled view instead.
              </p>
            </div>
          )}
        </main>
      </div>
    </>
  );
}

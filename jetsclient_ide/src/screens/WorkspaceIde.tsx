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

import type { ApiClient } from "../api/client";
import { SessionExpiredError } from "../api/client";
import { WorkspaceApi, fileNameOf, type WorkspaceNode, type WorkspaceSummary } from "../api/workspace";
import { FileTree } from "../components/FileTree";
import { Editor } from "../editor/Editor";
import { isServerValidatedJson, languageNameFor } from "../editor/language";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";

/** The capability the server requires for every workspace action. */
export const WORKSPACE_IDE = "workspace_ide";

interface Tab {
  fileName: string;
  label: string;
  /** Text as last loaded or saved; the dirty check compares against this. */
  saved: string;
  current: string;
  size: number;
}

export function WorkspaceIde({ api }: { api: ApiClient }) {
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const { setError, setStatus } = useNotifications();

  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [workspace, setWorkspace] = useState<string>("");
  const [tree, setTree] = useState<WorkspaceNode[]>([]);
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeFile, setActiveFile] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const canEdit = api.can(WORKSPACE_IDE);
  const activeTab = useMemo(
    () => tabs.find((t) => t.fileName === activeFile) ?? null,
    [tabs, activeFile],
  );
  const dirtyCount = tabs.filter((t) => t.current !== t.saved).length;

  /** Routes every failure to the banner, and a dead session back to the login screen. */
  const guard = useCallback(
    async (work: () => Promise<void>) => {
      try {
        await work();
      } catch (err) {
        if (err instanceof SessionExpiredError) {
          setTabs([]);
          setActiveFile(null);
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
      if (list.length > 0 && workspace === "") setWorkspace(list[0]!.name);
    });
    // Once, on mount: the shell guarantees a session, and picking a workspace is
    // handled below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (workspace === "") return;
    setTabs([]);
    setActiveFile(null);
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
      if (tabs.some((t) => t.fileName === fileName)) {
        setActiveFile(fileName);
        return;
      }
      setBusy(true);
      setStatus(null);
      void guard(async () => {
        const file = await workspaceApi.readFile(workspace, node);
        setTabs((prev) => [
          ...prev,
          {
            fileName: file.fileName,
            label: file.label,
            saved: file.content,
            current: file.content,
            size: node.size,
          },
        ]);
        setActiveFile(file.fileName);
      }).finally(() => setBusy(false));
    },
    [guard, setStatus, tabs, workspace, workspaceApi],
  );

  const closeTab = useCallback(
    (fileName: string) => {
      const tab = tabs.find((t) => t.fileName === fileName);
      if (tab && tab.current !== tab.saved) {
        const ok = window.confirm(`${tab.label} has unsaved changes. Close it anyway?`);
        if (!ok) return;
      }
      setTabs((prev) => {
        const next = prev.filter((t) => t.fileName !== fileName);
        setActiveFile((cur) =>
          cur === fileName ? (next.length > 0 ? next[next.length - 1]!.fileName : null) : cur,
        );
        return next;
      });
    },
    [tabs],
  );

  const save = useCallback(() => {
    const tab = tabs.find((t) => t.fileName === activeFile);
    if (!tab || tab.current === tab.saved || !canEdit) return;

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
        prev.map((t) => (t.fileName === tab.fileName ? { ...t, saved: t.current } : t)),
      );
      setStatus(`Saved ${tab.label}`);
    }).finally(() => setBusy(false));
  }, [activeFile, canEdit, guard, setError, setStatus, tabs, workspace, workspaceApi]);

  const onChange = useCallback(
    (value: string) => {
      setTabs((prev) => prev.map((t) => (t.fileName === activeFile ? { ...t, current: value } : t)));
    },
    [activeFile],
  );

  return (
    <>
      <div className="screenbar">
        <label className="ws-picker">
          <span className="sr-only">Workspace</span>
          <select value={workspace} onChange={(e) => setWorkspace(e.target.value)}>
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
          disabled={!activeTab || activeTab.current === activeTab.saved || busy}
        >
          Save
        </ActionButton>
      </div>

      <div className="body">
        <aside className="sidebar">
          <FileTree nodes={tree} activeFile={activeFile} onOpen={openFile} />
        </aside>

        <main className="main">
          <div className="tabbar" role="tablist">
            {tabs.map((t) => (
              <div
                key={t.fileName}
                className={`tab${t.fileName === activeFile ? " is-active" : ""}`}
                role="tab"
                aria-selected={t.fileName === activeFile}
              >
                <button type="button" className="tab-label" onClick={() => setActiveFile(t.fileName)}>
                  {t.current !== t.saved && <span className="dot" aria-label="unsaved" />}
                  {t.label.split("/").pop()}
                </button>
                <button
                  type="button"
                  className="tab-close"
                  onClick={() => closeTab(t.fileName)}
                  aria-label={`Close ${t.label}`}
                >
                  ×
                </button>
              </div>
            ))}
          </div>

          {activeTab ? (
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
              <p className="empty-sub">Every file in the workspace opens here, whatever its size.</p>
            </div>
          )}
        </main>
      </div>
    </>
  );
}

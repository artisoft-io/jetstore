import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiClient, SessionExpiredError, type User } from "./api/client";
import { WorkspaceApi, fileNameOf, type WorkspaceNode, type WorkspaceSummary } from "./api/workspace";
import { FileTree } from "./components/FileTree";
import { Login } from "./components/Login";
import { Editor } from "./editor/Editor";
import { isServerValidatedJson, languageNameFor } from "./editor/language";

const api = new ApiClient();
const workspaceApi = new WorkspaceApi(api);

/** The capability the server requires for every workspace action. */
const WORKSPACE_IDE = "workspace_ide";

interface Tab {
  fileName: string;
  label: string;
  /** Text as last loaded or saved; the dirty check compares against this. */
  saved: string;
  current: string;
  size: number;
}

type Theme = "light" | "dark";

export default function App() {
  const [user, setUser] = useState<User | null>(api.currentUser);
  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [workspace, setWorkspace] = useState<string>("");
  const [tree, setTree] = useState<WorkspaceNode[]>([]);
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeFile, setActiveFile] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem("jetstore-ide-theme") as Theme | null) ?? "light",
  );

  useEffect(() => api.subscribe(setUser), []);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("jetstore-ide-theme", theme);
  }, [theme]);

  const canEdit = api.can(WORKSPACE_IDE);
  const activeTab = useMemo(
    () => tabs.find((t) => t.fileName === activeFile) ?? null,
    [tabs, activeFile],
  );
  const dirtyCount = tabs.filter((t) => t.current !== t.saved).length;

  /** Routes every failure to the banner, and a dead session back to the login screen. */
  const guard = useCallback(async (work: () => Promise<void>) => {
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
  }, []);

  // Warn before discarding unsaved buffers on a reload or tab close.
  useEffect(() => {
    if (dirtyCount === 0) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirtyCount]);

  useEffect(() => {
    if (!user) return;
    void guard(async () => {
      const list = await workspaceApi.listWorkspaces();
      setWorkspaces(list);
      if (list.length > 0 && workspace === "") setWorkspace(list[0]!.name);
    });
    // Only when the session changes; picking a workspace is handled below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user]);

  useEffect(() => {
    if (!user || workspace === "") return;
    setTabs([]);
    setActiveFile(null);
    setBusy(true);
    void guard(async () => {
      setTree(await workspaceApi.fileTree(workspace));
    }).finally(() => setBusy(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user, workspace]);

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
    [guard, tabs, workspace],
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
        setError(`${tab.label} is not valid JSON, so the server would reject it: ${
          err instanceof Error ? err.message : String(err)
        }`);
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
  }, [activeFile, canEdit, guard, tabs, workspace]);

  const onChange = useCallback(
    (value: string) => {
      setTabs((prev) =>
        prev.map((t) => (t.fileName === activeFile ? { ...t, current: value } : t)),
      );
    },
    [activeFile],
  );

  if (!user) {
    return (
      <Login
        version=""
        onSubmit={async (email, password) => {
          await api.login(email, password);
          setError(null);
        }}
      />
    );
  }

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">JetStore <strong>Workspace IDE</strong></span>

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

        <button
          type="button"
          className="btn"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          aria-label="Toggle colour theme"
        >
          {theme === "dark" ? "Light" : "Dark"}
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={save}
          disabled={!activeTab || activeTab.current === activeTab.saved || !canEdit || busy}
        >
          Save
        </button>
        <button type="button" className="btn" onClick={() => api.logout()}>
          Sign out
        </button>
      </header>

      {error && (
        <div className="banner banner-error" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => setError(null)} aria-label="Dismiss">
            ×
          </button>
        </div>
      )}
      {status && !error && (
        <div className="banner banner-ok" role="status">
          <span>{status}</span>
          <button type="button" onClick={() => setStatus(null)} aria-label="Dismiss">
            ×
          </button>
        </div>
      )}

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
              <p className="empty-sub">
                Every file in the workspace opens here, whatever its size.
              </p>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

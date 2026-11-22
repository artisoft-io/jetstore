/**
 * The Workspace IDE, as a screen.
 *
 * This is the original editor app's `App.tsx` with three things lifted out into the shell — the
 * session gate, the theme toggle and the banner region — and nothing else
 * changed. The editor, the tab model, the dirty tracking and the save path are
 * the code that has been deployed since the editor was first ported; moving it
 * here did not rewrite it.
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
import {
  WorkspaceApi,
  describeSaveWarnings,
  fileNameOf,
  queryEscape,
  type AddedFile,
  type WorkspaceNode,
  type WorkspaceSummary,
} from "../api/workspace";
import { FileTree } from "../components/FileTree";
import { Editor } from "../editor/Editor";
import { isServerValidatedJson, languageNameFor } from "../editor/language";
import { BuildInfoText, useBuildInfoInScreenBar } from "../shell/buildInfoFooter";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import { usePrompts } from "../shell/prompts";
import {
  WORKSPACE_GUIDE_LABEL,
  describesWorkspace,
  openWorkspaceGuide,
  useWorkspaceManifest,
} from "../shell/workspaceGuide";
import {
  AddFilesControls,
  describeUploadPlan,
  folderOf,
  normaliseFolder,
  pathProblem,
  readText,
  treeFilePaths,
  uploadPath,
} from "./AddFiles";
import { CompiledView } from "./CompiledView";
import { WorkspaceChanges } from "./WorkspaceChanges";
import { WorkspaceToolbar } from "./WorkspaceToolbar";
import { compiledViewFor, compiledViews } from "./sectionContract";

/** The capability the server requires for every workspace action. */
export const WORKSPACE_IDE = "workspace_ide";

/**
 * The session key the picker's choice is kept under. 2026-10-11.
 *
 * **Session storage, so it is per tab and ends with it**: returning to the IDE
 * later in the session lands on the workspace last open, and a new session
 * starts again from the deployment's `WORKSPACE`. Every access is guarded,
 * because storage can be unavailable (a private window, blocked site data) and
 * the picker must still work without it.
 */
export const PICKED_WORKSPACE_KEY = "jetstore.workspaceIde.workspace";

export function rememberedWorkspace(): string | null {
  try {
    return window.sessionStorage.getItem(PICKED_WORKSPACE_KEY);
  } catch {
    return null;
  }
}

function rememberWorkspace(name: string): void {
  try {
    window.sessionStorage.setItem(PICKED_WORKSPACE_KEY, name);
  } catch {
    // Not remembered; the picker still works.
  }
}

/**
 * The workspace the picker opens on when the route names none: the one picked
 * earlier this session, else the deployment's `WORKSPACE`, else the first —
 * each only when it is in the list, so a removed workspace is never selected.
 */
export function defaultWorkspace(
  list: WorkspaceSummary[],
  remembered: string | null,
  active: string | null,
): string {
  const names = new Set(list.map((w) => w.name));
  if (remembered !== null && names.has(remembered)) return remembered;
  if (active !== null && names.has(active)) return active;
  return list[0]?.name ?? "";
}

/**
 * An open tab: a union of two kinds.
 *
 * **The Flutter screen has had two kinds since it was written** — its
 * `TabBarView` renders a `JetsForm` for a file and a `JetsFormWithTabs` for a
 * compiled view (`jetsclient/lib/screens/screen_tab_form.dart`,
 * `ScreenWithTabsWithForm`) — and this app carried only the first, because the
 * editor was ported first and nothing had ported the view. The discriminant is
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
      /**
       * The node's `read_only` (`wsfile.IsReadOnly`): shown, never saved. It is
       * set on a `.md` or `.mdx` file outside `docs/`, other than the
       * workspace's root README.
       */
      readOnly: boolean;
      /**
       * A file *New file* opened, which is not in the workspace until the tab is
       * saved (`AddFiles.tsx`). Its save adds the file rather than saving over
       * one, and it is unsaved until then, empty or not.
       */
      isNew?: boolean;
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
  return tab.kind === "file" && (tab.isNew === true || tab.current !== tab.saved);
}

export function WorkspaceIde({ api }: { api: ApiClient }) {
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const { setError, setStatus } = useNotifications();
  const prompts = usePrompts();
  /**
   * The workspace named by the route, when there is one.
   *
   * **Two routes, one screen**: `/workspace` is the original editor's, where the picker
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
  /** The shell's footer text, drawn in this screen's status bar instead. */
  const buildInfo = useBuildInfoInScreenBar();
  /**
   * The installation's workspace guide, when its image was built with one. The
   * *Workspace guide* button is shown only when it describes the workspace open
   * here (`workspaceGuide.ts`).
   */
  const guideManifest = useWorkspaceManifest(api, workspace);
  const hasGuide = describesWorkspace(guideManifest, workspace);

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
      // The route wins when it names one. Otherwise the workspace picked earlier
      // this session, then the deployment's — the first in the list was the
      // default until 2026-10-11, which put a user back on whichever workspace
      // sorts first every time they returned to the screen.
      if (list.length > 0 && workspace === "") {
        const remembered = rememberedWorkspace();
        let active: string | null = null;
        if (remembered === null) {
          try {
            active = (await workspaceApi.activeWorkspace()).name;
          } catch {
            // The deployment's workspace is a preference, not a requirement.
          }
        }
        setWorkspace(defaultWorkspace(list, remembered, active));
      }
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
    rememberWorkspace(workspace);
    setTabs([]);
    setActiveKey(null);
    setBusy(true);
    void guard(async () => {
      setTree(await workspaceApi.fileTree(workspace));
    }).finally(() => setBusy(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace]);

  /**
   * Re-read the tree without touching the tabs.
   *
   * **The effect above cannot serve this and the difference is the two lines it
   * opens with.** Changing workspace closes every tab, because the open ones
   * belong to the workspace being left; adding or deleting a file changes what
   * the tree holds and nothing about what is open — closing the view that just
   * added a file would be a worse answer than not refreshing at all.
   *
   * Both server actions answer with the recomputed structure and this throws that
   * answer away, which is deliberate and argued in `CompiledView.tsx`: an action
   * document has no channel for a response body, and one extra round trip is the
   * cheaper of the two prices.
   */
  const refreshTree = useCallback(() => {
    if (workspace === "") return;
    void guard(async () => {
      setTree(await workspaceApi.fileTree(workspace));
    });
  }, [guard, workspace, workspaceApi]);

  /**
   * The toolbar's Refresh: the file tree, and the changes table remounted so it
   * reads again, for what changed outside this screen -- a pull, a compile, a new
   * unit test directory. Open tabs are left alone, as refreshTree leaves them.
   * The toolbar re-reads its own row (WorkspaceToolbar's onRefresh).
   */
  const [changesGeneration, setChangesGeneration] = useState(0);
  const refresh = useCallback(() => {
    refreshTree();
    setChangesGeneration((n) => n + 1);
  }, [refreshTree]);

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
            readOnly: node.read_only === true,
          },
        ]);
        setActiveKey(`file:${file.fileName}`);
      }).finally(() => setBusy(false));
    },
    [guard, setStatus, tabs, workspace, workspaceApi],
  );

  /**
   * Open a section's compiled view.
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
   * Change workspace, closing every tab in the same update.
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
    async (key: string) => {
      const tab = tabs.find((t) => tabKey(t) === key);
      if (tab && isDirty(tab)) {
        // The app's own confirmation (2026-10-02): the browser's is never shown in the embedded pane.
        const ok = await prompts.confirm(`${tab.label} has unsaved changes. Close it anyway?`);
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
    [prompts, tabs],
  );

  const save = useCallback(() => {
    const tab = tabs.find((t) => tabKey(t) === activeKey);
    // A compiled view is read-only by construction: it shows what the compiler
    // produced, and the way to change it is to edit the sources below the
    // heading. Save is disabled on it rather than silently doing nothing.
    if (!tab || tab.kind !== "file" || !isDirty(tab) || !canEdit || tab.readOnly) return;

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
      // A new file is added: the same checks and the same write as a save, and
      // refused if a file of its name is there by now (`AddFiles.tsx`).
      const { warnings } = tab.isNew
        ? {
            warnings:
              (await workspaceApi.addFiles(workspace, [{ path: tab.label, content: tab.current }]))[0]
                ?.warnings ?? [],
          }
        : await workspaceApi.saveFile(workspace, tab.fileName, tab.current);
      setTabs((prev) =>
        prev.map((t) =>
          t.kind === "file" && t.fileName === tab.fileName ? { ...t, saved: t.current, isNew: false } : t,
        ),
      );
      if (tab.isNew) refresh();
      // The file is saved either way; warnings are what its validator found
      // and did not refuse, shown in the warning colours until dismissed.
      if (warnings.length > 0) {
        setStatus(describeSaveWarnings(tab.label, warnings), "warn");
      } else {
        setStatus(`Saved ${tab.label}`);
      }
    }).finally(() => setBusy(false));
  }, [activeKey, canEdit, guard, refresh, setError, setStatus, tabs, workspace, workspaceApi]);

  /** The folder of the file open in the editor, where an added file goes unless told otherwise. */
  const currentFolder = activeTab?.kind === "file" ? folderOf(activeTab.label) : "";

  /**
   * *New file*: ask for a path, then open an empty tab for it. Nothing is
   * written until the tab is saved.
   */
  const newFile = useCallback(() => {
    void (async () => {
      const typed = await prompts.prompt(
        `New file in ${workspace}: its path in the workspace, such as runbooks/my_runbook.rb.json`,
        currentFolder === "" ? "" : `${currentFolder}/`,
      );
      if (typed === null) return;
      const path = typed.trim();
      const problem = pathProblem(path);
      if (problem !== null) {
        setError(`${path === "" ? "The new file" : path} cannot be added: ${problem}`);
        return;
      }
      const fileName = queryEscape(path);
      if (tabs.some((t) => t.kind === "file" && t.fileName === fileName)) {
        setActiveKey(`file:${fileName}`);
        return;
      }
      if (treeFilePaths(tree).has(path)) {
        setError(`${path} is already in the workspace: open it from the tree.`);
        return;
      }
      setError(null);
      setStatus(`${path} is new: write it, then Save to add it to the workspace.`);
      setTabs((prev) => [
        ...prev,
        {
          kind: "file",
          fileName,
          label: path,
          saved: "",
          current: "",
          size: 0,
          readOnly: false,
          isNew: true,
        },
      ]);
      setActiveKey(`file:${fileName}`);
    })();
  }, [currentFolder, prompts, setError, setStatus, tabs, tree, workspace]);

  /**
   * *Upload files* and *Upload folder*: ask which folder the files go to, read
   * them, ask the server what adding them would do, show it, and add them all,
   * or none when any is refused.
   */
  const upload = useCallback(
    (files: File[]) => {
      void (async () => {
        const count = files.length === 1 ? "1 file" : `${files.length} files`;
        const typed = await prompts.prompt(
          `Upload ${count} into which folder of ${workspace}? Leave it empty for the workspace's root.`,
          currentFolder,
        );
        if (typed === null) return;
        const folder = normaliseFolder(typed);
        const folderProblem = folder === "" ? null : pathProblem(folder);
        if (folderProblem !== null) {
          setError(`Nothing was uploaded: the folder ${folder}: ${folderProblem}`);
          return;
        }
        setBusy(true);
        setError(null);
        setStatus(null);
        await guard(async () => {
          const read: AddedFile[] = [];
          const notText: string[] = [];
          for (const file of files) {
            const path = uploadPath(folder, file);
            const content = await readText(file);
            if (content === null) notText.push(path);
            else read.push({ path, content });
          }
          if (notText.length > 0) {
            setError(
              [
                "Nothing was uploaded: a workspace file is text, and these are not:",
                ...notText.map((p) => `  ${p}`),
              ].join("\n"),
            );
            return;
          }
          const plan = await workspaceApi.checkFiles(workspace, read);
          // A file the IDE does not list is left out, and the confirmation says
          // so: a folder picked whole may hold a note or a hidden file. Any other
          // refusal stops the upload, so that a runbook never arrives without
          // its SQL.
          const refused = plan.filter(
            (r) => r.status !== "new" && r.status !== "replaces" && r.status !== "not_listed",
          );
          if (refused.length > 0) {
            setError(["Nothing was uploaded:", ...refused.map((r) => r.message ?? r.path)].join("\n"));
            return;
          }
          const adding = plan.filter((r) => r.status === "new" || r.status === "replaces");
          if (adding.length === 0) {
            setError(
              "Nothing was uploaded: the IDE lists none of these files. A file is added under a section of the " +
                "file tree, with a file type the section lists.",
            );
            return;
          }
          const ok = await prompts.confirm(
            adding.length === 1
              ? `Upload 1 file to ${workspace}?`
              : `Upload ${adding.length} files to ${workspace}?`,
            describeUploadPlan(plan),
          );
          if (!ok) return;
          const content = new Map(read.map((f) => [f.path, f.content]));
          const results = await workspaceApi.addFiles(
            workspace,
            adding.map((r) => ({
              path: r.path,
              content: content.get(r.path) ?? "",
              replace: r.status === "replaces",
            })),
          );
          // A replaced file open in a tab takes the uploaded text as its saved
          // text; an edit not yet saved there is kept, and now differs from it.
          setTabs((prev) =>
            prev.map((t) => {
              if (t.kind !== "file" || !content.has(t.label) || t.isNew) return t;
              const text = content.get(t.label)!;
              return { ...t, saved: text, current: t.current === t.saved ? text : t.current };
            }),
          );
          refresh();
          // Every file is added either way; the warnings are its validator's,
          // as a save shows them, one file after another.
          const warned = results.filter((r) => r.warnings.length > 0);
          const done =
            adding.length === 1
              ? `Uploaded 1 file to ${workspace}.`
              : `Uploaded ${adding.length} files to ${workspace}.`;
          if (warned.length > 0) {
            setStatus(
              [done, ...warned.map((r) => describeSaveWarnings(r.path, r.warnings))].join("\n"),
              "warn",
            );
          } else {
            setStatus(done);
          }
        });
        setBusy(false);
      })();
    },
    [currentFolder, guard, prompts, refresh, setError, setStatus, workspace, workspaceApi],
  );

  /**
   * Opens the guide's page for the file in the editor, else its landing page, in
   * the documentation tab *Help* uses. A failure goes to the banner, as *Help*'s
   * does.
   */
  const openGuide = useCallback(() => {
    void openWorkspaceGuide({
      workspace,
      fileName: activeFile,
      openDocsSession: () => api.openDocsSession(),
    }).then((message) => {
      if (message !== null) setError(message);
    });
  }, [activeFile, api, setError, workspace]);

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

        {/*
          Beside Save rather than in the toolbar below: the toolbar's buttons are
          registry actions authored in a table document (WorkspaceToolbar.tsx),
          and this one is not; and it belongs to what this bar already holds, the
          workspace picked and the file open. Not gated on workspace_ide, since
          the guide is readable by every signed-in user.
        */}
        {hasGuide && (
          <button
            type="button"
            className="btn btn-secondary"
            onClick={openGuide}
            title={
              activeFile !== null
                ? "Open this file's page in the workspace's guide"
                : "Open the workspace's guide"
            }
          >
            {WORKSPACE_GUIDE_LABEL}
          </button>
        )}

        <ActionButton
          capability={WORKSPACE_IDE}
          className="btn btn-primary"
          onClick={save}
          disabled={
            !activeTab ||
            activeTab.kind !== "file" ||
            activeTab.readOnly ||
            !isDirty(activeTab) ||
            busy
          }
        >
          Save
        </ActionButton>
      </div>

      {workspace !== "" && <WorkspaceToolbar api={api} workspace={workspace} onRefresh={refresh} />}

      <div className="body">
        <aside className="sidebar">
          <AddFilesControls
            capability={WORKSPACE_IDE}
            disabled={busy || workspace === ""}
            onNewFile={newFile}
            onUpload={upload}
          />
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
                    onClick={() => void closeTab(key)}
                    aria-label={`Close ${t.label}`}
                  >
                    ×
                  </button>
                </div>
              );
            })}
          </div>

          {activeTab?.kind === "view" ? (
            <CompiledView
              api={api}
              document={compiledViews[activeTab.view]!}
              workspaceName={workspace}
              onFilesChanged={refreshTree}
            />
          ) : activeTab ? (
            /*
              `content` is the tab's current text, not its saved text: the editor
              makes a fresh state for each file it is shown (2026-10-27),
              so switching back to a tab must show the edits it still holds.
              Until then it was `saved`, and the switch's replace transaction
              reset `current` to it through `onChange`, so unsaved edits were
              dropped silently by leaving the tab and coming back.
            */
            <Editor
              docKey={activeTab.fileName}
              fileName={activeTab.label}
              content={activeTab.current}
              readOnly={!canEdit || activeTab.readOnly}
              onChange={onChange}
              onSave={save}
            />
          ) : (
            /*
              **The base content, not a placeholder.** The Flutter workspace home
              is `ScreenWithTabsWithForm` — tabs *and* a form — and its form is
              action-less with one full-height field, the `workspaceChangesTable`
              (`workspace_ide/form_config.dart`, `FormKeys.workspaceHome`). So the
              changes table is what this screen shows when no tab is open, and the
              "Select a file" text that stood here was the half of that screen the
              port had not reached.

              It needs a workspace to filter on, so the bare `/workspace` route —
              where a user has not picked one yet — keeps the placeholder.
            */
            workspace ? (
              <WorkspaceChanges
                key={changesGeneration}
                api={api}
                workspace={workspace}
                onFilesChanged={refreshTree}
              />
            ) : (
              <div className="empty">
                <p>Select a workspace to see its changes, or a file to start editing.</p>
                <p className="empty-sub">
                  Every file in the workspace opens here, whatever its size. A section heading whose
                  files compile into the workspace database opens the compiled view instead.
                </p>
              </div>
            )
          )}

          {/*
            The status bar, and since 2026-10-01 the
            build information as its last entry: this screen claims the shell's
            footer, so it has one bottom bar rather than two. **It is drawn
            whatever the tab**, where it used to follow the editor only, because
            the build line has to be somewhere on a compiled view and on the
            changes table too — the file's own entries are what stay conditional.
          */}
          {(activeTab?.kind === "file" || buildInfo !== null) && (
            <footer className="statusbar">
              {activeTab?.kind === "file" ? (
                <>
                  <span title={activeTab.label}>{activeTab.label}</span>
                  <span className="spacer" />
                  <span>{languageNameFor(activeTab.label)}</span>
                  {activeTab.readOnly && <span>Read only</span>}
                  <span>{activeTab.current.length.toLocaleString()} chars</span>
                  <span>{activeTab.current !== activeTab.saved ? "Modified" : "Saved"}</span>
                </>
              ) : (
                <span className="spacer" />
              )}
              {buildInfo !== null && <BuildInfoText info={buildInfo} />}
            </footer>
          )}
        </main>
      </div>
    </>
  );
}

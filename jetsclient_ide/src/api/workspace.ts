/**
 * Typed wrappers over the workspace actions of /dataTable.
 *
 * The wire shapes here are not invented; they mirror the Go side:
 *   - the tree node is `wsfile.WorkspaceNode` (jets/datatable/wsfile/visitor.go)
 *   - the actions are the `workspace_*` arms of the switch in
 *     jets/apiserver/api_tables.go
 *   - `file_name` is **url-escaped by the server** when it builds the tree and
 *     url-unescaped again when it reads the file, so it is passed back exactly as
 *     received and never decoded on the way through.
 *
 * Note what is deliberately absent: a size limit. The Flutter client refused to
 * open anything at or above 250,000 bytes because its editor laid the whole
 * document out per frame. CodeMirror virtualises by viewport, so the limit has no
 * reason to exist here and the largest rule file in the corpus (1.18 MB) is
 * unremarkable.
 */

import type { ApiClient } from "./client";

/** Mirrors wsfile.WorkspaceNode. */
export interface WorkspaceNode {
  key: string;
  pageMatchKey: string;
  /** "dir" | "file" | "section" */
  type: string;
  size: number;
  label: string;
  route_path: string;
  route_params: Record<string, string> | null;
  children: WorkspaceNode[] | null;
}

export interface WorkspaceSummary {
  name: string;
  uri: string;
  branch: string;
}

/**
 * The workspace this apiserver is *running*, as opposed to the ones it can edit.
 *
 * `listWorkspaces` reads `jetsapi.workspace_registry` — every workspace a user may
 * open in the IDE. That is the wrong question for a flow: a flow's documents live
 * in the deployment's active workspace, which is the `WORKSPACE` environment
 * variable and is answered by the `get_workspace_uri` action
 * (`jets/apiserver/api_tables.go:183`).
 *
 * `fileKeyLabelRe` comes back in the same response and is the pattern the
 * `fileKeyLabel` cell filter uses (`actions/registry.ts`). The Flutter app reads
 * it here too, at sign-in (`modules/actions/user_delegates.dart:107`,
 * `globalWorkspaceFileKeyLabelRe`); this app had no equivalent until F.0a needed
 * the workspace name from the same call.
 */
export interface ActiveWorkspace {
  name: string;
  uri: string;
  branch: string;
  /** Empty when the deployment sets no `WORKSPACE_FILE_KEY_LABEL_RE`. */
  fileKeyLabelRe: string;
}

/** A file opened for editing. */
export interface WorkspaceFile {
  /** Url-escaped relative path, exactly as the server issued it. */
  fileName: string;
  /** Human-readable path, for tab labels and titles. */
  label: string;
  content: string;
}

export class WorkspaceApi {
  constructor(private readonly api: ApiClient) {}

  /** The workspaces this user may open, from jetsapi.workspace_registry. */
  async listWorkspaces(): Promise<WorkspaceSummary[]> {
    const body = await this.api.dataTable<{ rows?: unknown[][]; columns?: unknown }>({
      action: "raw_query",
      query:
        "SELECT workspace_name, workspace_uri, workspace_branch " +
        "FROM jetsapi.workspace_registry ORDER BY workspace_name ASC LIMIT 200",
    });
    const rows = Array.isArray(body.rows) ? body.rows : [];
    return rows.map((r) => ({
      name: String(r?.[0] ?? ""),
      uri: String(r?.[1] ?? ""),
      branch: String(r?.[2] ?? ""),
    }));
  }

  /** The workspace this deployment runs — see `ActiveWorkspace`. */
  async activeWorkspace(): Promise<ActiveWorkspace> {
    const body = await this.api.dataTable<{
      workspace_uri?: string;
      workspace_name?: string;
      workspace_branch?: string;
      workspace_file_key_label_re?: string;
    }>({ action: "get_workspace_uri" });
    return {
      name: body.workspace_name ?? "",
      uri: body.workspace_uri ?? "",
      branch: body.workspace_branch ?? "",
      fileKeyLabelRe: body.workspace_file_key_label_re ?? "",
    };
  }

  /** The file tree for a workspace. */
  async fileTree(workspaceName: string): Promise<WorkspaceNode[]> {
    const body = await this.api.dataTable<{
      result_type?: string;
      result_data?: WorkspaceNode[];
    }>({
      action: "workspace_query_structure",
      fromClauses: [{ table: "workspace_file_structure" }],
      workspaceName,
      data: [{ workspace_name: workspaceName }],
    });
    if (body.result_type !== "workspace_file_structure" || !Array.isArray(body.result_data)) {
      return [];
    }
    return body.result_data;
  }

  async readFile(workspaceName: string, node: WorkspaceNode): Promise<WorkspaceFile> {
    const fileName = fileNameOf(node);
    if (!fileName) throw new Error(`${node.label} is not a file`);
    const body = await this.api.dataTable<{ file_content?: string }>({
      action: "get_workspace_file_content",
      workspaceName,
      data: [{ ...(node.route_params ?? {}), file_name: fileName }],
    });
    return {
      fileName,
      label: decodeLabel(fileName),
      content: body.file_content ?? "",
    };
  }

  /**
   * Reads a file by its path, without a tree node.
   *
   * **`readFile` above cannot serve a caller that knows the path**, which is not
   * obvious and cost a defect: it takes a `WorkspaceNode` and starts by asking
   * `fileNameOf`, which returns null for anything whose `type` is not `"file"`.
   * `FlowStore.readText` was constructing `{ label, key } as never` and passing
   * it in — a shape with no `type` — so every load through it would have thrown
   * *"…is not a file"* against the real API. Nothing said so because the store's
   * tests stub `WorkspaceApi` and their stub keys off `node.key` (I-65).
   *
   * A flow's documents are addressed by path and not by browsing, so the honest
   * fix is this method rather than a synthetic node. The tree is still how the
   * *editor* opens a file; it is not how a runner resolves one.
   *
   * The server url-unescapes what it receives (`GetWorkspaceFileContent`,
   * `jets/datatable/workspace_data_table_action.go:907`), so the path is escaped
   * on the way out — see `queryEscape`.
   */
  async readWorkspaceFile(workspaceName: string, path: string): Promise<WorkspaceFile> {
    const fileName = queryEscape(path);
    const body = await this.api.dataTable<{ file_content?: string }>({
      action: "get_workspace_file_content",
      workspaceName,
      data: [{ workspace_name: workspaceName, file_name: fileName }],
    });
    return { fileName, label: path, content: body.file_content ?? "" };
  }

  /**
   * Save. The server validates any `.json` file before writing (it refuses to
   * persist one that will not parse), so a 400 here is frequently a real syntax
   * error in the buffer rather than a transport problem — worth surfacing verbatim.
   */
  async saveFile(workspaceName: string, fileName: string, content: string): Promise<void> {
    await this.api.dataTable({
      action: "save_workspace_file_content",
      workspaceName,
      data: [{ file_name: fileName, file_content: content }],
    });
  }
}

/** The escaped relative path for a file node, or null for anything else. */
export function fileNameOf(node: WorkspaceNode): string | null {
  if (node.type !== "file") return null;
  const fromParams = node.route_params?.["file_name"];
  if (typeof fromParams === "string" && fromParams !== "") return fromParams;
  return node.pageMatchKey !== "" ? node.pageMatchKey : null;
}

/**
 * Go's `url.QueryEscape`, which is what the server's tree walk emits and what its
 * read and save paths unescape (`net/url`, and `wsfile/visitor.go`'s
 * `relativeFileName`).
 *
 * `encodeURIComponent` is close and not the same on two characters that matter
 * here: it leaves `/` alone where Go writes `%2F`, and it leaves `+` alone where
 * Go writes `%2B` — and Go's *unescape* reads a bare `+` as a space. A path with
 * a `+` in it would therefore come back naming a different file. Neither
 * character appears in a flow key, which is exactly why this would have been
 * found late rather than never.
 */
export function queryEscape(value: string): string {
  return encodeURIComponent(value)
    .replace(/[!'()*]/g, (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`)
    .replace(/\//g, "%2F")
    .replace(/%20/g, "+");
}

/** Turn the escaped wire path back into something readable for a tab label. */
export function decodeLabel(escaped: string): string {
  try {
    return decodeURIComponent(escaped.replace(/\+/g, " "));
  } catch {
    return escaped;
  }
}

/** Depth-first walk, used by the tree filter and by the tests. */
export function* walk(nodes: WorkspaceNode[]): Generator<WorkspaceNode> {
  for (const n of nodes) {
    yield n;
    if (n.children) yield* walk(n.children);
  }
}

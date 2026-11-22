/**
 * Adding files to a workspace from the IDE: *New file*, *Upload files* and
 * *Upload folder*, above the file tree.
 *
 * **An added file is saved exactly as an edit is.** The server checks it with
 * the same validators and writes it with the same function
 * (`jets/datatable/workspace_add_files.go`), so it becomes an override, listed
 * in *Workspace Changes*, kept by every start of the apiserver, and deleted by
 * its revert, since the workspace as deployed has no copy of it. Adding asks
 * for `workspace_ide`, as saving does, and is no wider than editing: only a file
 * the tree would list can be added, so whatever is added can be opened here.
 *
 * - *New file* asks for a path, then opens an empty tab for it. Nothing is
 *   written until the tab is saved, and that save refuses a file that is there,
 *   so a new file never overwrites one.
 * - *Upload files* and *Upload folder* read the files picked on this computer,
 *   ask which workspace folder to put them in (a folder upload keeps its
 *   subfolders), ask the server what the upload would do, show it, and add them
 *   all or none.
 *
 * This file holds the controls and the pure helpers; `WorkspaceIde.tsx` holds
 * the flow, because it owns the tabs, the tree and the notifications.
 */

import { useEffect, useRef, type ChangeEvent } from "react";

import { decodeLabel, walk, type AddedFileResult, type WorkspaceNode } from "../api/workspace";
import { ActionButton } from "../shell/capabilities";

/** The folder a path is in, `""` for the workspace's root. */
export function folderOf(path: string): string {
  const at = path.lastIndexOf("/");
  return at < 0 ? "" : path.slice(0, at);
}

/** A folder as typed, without surrounding spaces or slashes; `""` is the root. */
export function normaliseFolder(input: string): string {
  return input.trim().replace(/^\/+/, "").replace(/\/+$/, "");
}

/** `folder/name`, or `name` at the root. */
export function joinPath(folder: string, name: string): string {
  const f = normaliseFolder(folder);
  return f === "" ? name : `${f}/${name}`;
}

/**
 * What is wrong with a workspace path as typed, or null. The server checks the
 * name again, and decides whether the IDE lists such a file; this only saves a
 * round trip for the mistakes a person makes typing one.
 */
export function pathProblem(path: string): string | null {
  if (path.trim() === "") return "Give the file a name.";
  if (path.includes("\\")) return "Write the path with forward slashes.";
  if (path.startsWith("/")) return "The path is relative to the workspace: leave out the leading /.";
  if (path.endsWith("/")) return "The path names a folder: end it with the file's name.";
  for (const segment of path.split("/")) {
    if (segment === "") return "The path has an empty folder name (//).";
    if (segment === "." || segment === "..") return "The path may not use . or .. as a folder.";
    if (segment.trim() !== segment) return "A folder or file name may not start or end with a space.";
  }
  return null;
}

/** The decoded paths of the tree's files. */
export function treeFilePaths(nodes: WorkspaceNode[]): Set<string> {
  const paths = new Set<string>();
  for (const node of walk(nodes)) {
    if (node.type !== "file") continue;
    const escaped = node.route_params?.["file_name"] ?? node.pageMatchKey;
    if (typeof escaped === "string" && escaped !== "") paths.add(decodeLabel(escaped));
  }
  return paths;
}

/**
 * The path a picked file goes to under `folder`: its path within the folder
 * picked, which names that folder first, for *Upload folder*; its name for
 * *Upload files*.
 */
export function uploadPath(folder: string, file: File): string {
  const relative = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
  return joinPath(folder, relative !== undefined && relative !== "" ? relative : file.name);
}

/**
 * The file's text, or null when it is not text: a workspace file is saved as
 * text, so a binary one would arrive corrupted. A NUL, or a byte sequence that
 * is not UTF-8 (read as U+FFFD), marks it.
 */
export async function readText(file: File): Promise<string | null> {
  const text = await file.text();
  return text.includes("\u0000") || text.includes("�") ? null : text;
}

/** One line per file of a check's answer, as the upload's confirmation lists them. */
export function describeUploadPlan(results: AddedFileResult[]): string {
  const added = results.filter((r) => r.status === "new");
  const replaced = results.filter((r) => r.status === "replaces");
  const left = results.filter((r) => r.status === "not_listed");
  const sections: string[] = [];
  if (added.length > 0) sections.push(["New:", ...added.map((r) => `  ${r.path}`)].join("\n"));
  if (replaced.length > 0) {
    sections.push(["Replaces the file there:", ...replaced.map((r) => `  ${r.path}`)].join("\n"));
  }
  if (left.length > 0) {
    sections.push(
      ["Left out, a file the IDE does not list:", ...left.map((r) => `  ${r.path}`)].join("\n"),
    );
  }
  const warned = results.filter((r) => r.warnings.length > 0);
  if (warned.length > 0) {
    sections.push(
      [
        "Saved with warnings:",
        ...warned.flatMap((r) => [
          `  ${r.path}`,
          ...r.warnings.map((w) => {
            let where = w.path ?? "";
            if (w.line && w.line > 0) where = where !== "" ? `${where}:${w.line}` : `line ${w.line}`;
            return where !== "" ? `    ${where}: ${w.message}` : `    ${w.message}`;
          }),
        ]),
      ].join("\n"),
    );
  }
  return sections.join("\n\n");
}

/**
 * The three controls above the tree. Each wants `capability`, as *Save* does,
 * and is disabled while the screen is busy or has no workspace.
 *
 * The file pickers are `<input type="file">` elements kept out of sight; a
 * button opens one. *Upload folder* sets `webkitdirectory`, which every
 * current browser supports under that name, on its element directly, since it
 * is not a React attribute.
 */
export function AddFilesControls({
  capability,
  disabled,
  onNewFile,
  onUpload,
}: {
  capability: string;
  disabled: boolean;
  onNewFile(): void;
  onUpload(files: File[]): void;
}) {
  const filesInput = useRef<HTMLInputElement>(null);
  const folderInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    folderInput.current?.setAttribute("webkitdirectory", "");
  }, []);

  const picked = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    // Cleared, so that picking the same files again is a change.
    event.target.value = "";
    if (files.length > 0) onUpload(files);
  };

  return (
    <div className="tree-actions">
      <ActionButton capability={capability} className="btn btn-small" disabled={disabled} onClick={onNewFile}>
        New file
      </ActionButton>
      <ActionButton
        capability={capability}
        className="btn btn-small"
        disabled={disabled}
        onClick={() => filesInput.current?.click()}
      >
        Upload files
      </ActionButton>
      <ActionButton
        capability={capability}
        className="btn btn-small"
        disabled={disabled}
        onClick={() => folderInput.current?.click()}
      >
        Upload folder
      </ActionButton>
      <input ref={filesInput} type="file" multiple hidden aria-label="Files to upload" onChange={picked} />
      <input ref={folderInput} type="file" multiple hidden aria-label="Folder to upload" onChange={picked} />
    </div>
  );
}

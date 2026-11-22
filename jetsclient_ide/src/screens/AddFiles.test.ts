/**
 * @vitest-environment jsdom
 *
 * The pure half of adding files from the IDE (`AddFiles.tsx`): the paths a new
 * file and an upload go to, what is refused before asking the server, and how
 * an upload's plan is shown.
 */

import { describe, expect, it } from "vitest";

import type { AddedFileResult, WorkspaceNode } from "../api/workspace";
import {
  describeUploadPlan,
  folderOf,
  joinPath,
  normaliseFolder,
  pathProblem,
  readText,
  treeFilePaths,
  uploadPath,
} from "./AddFiles";

describe("paths", () => {
  it("joins a folder and a name, the root being the empty folder", () => {
    expect(joinPath("runbooks", "a.sql")).toBe("runbooks/a.sql");
    expect(joinPath(" /runbooks/ ", "a.sql")).toBe("runbooks/a.sql");
    expect(joinPath("", "README.md")).toBe("README.md");
    expect(normaliseFolder("  //runbooks/health_check// ")).toBe("runbooks/health_check");
    expect(folderOf("runbooks/health_check/count.sql")).toBe("runbooks/health_check");
    expect(folderOf("workspace_control.json")).toBe("");
  });

  it("refuses the mistakes a typed path makes, and nothing else", () => {
    expect(pathProblem("runbooks/health_check/count.sql")).toBeNull();
    expect(pathProblem("workspace_control.json")).toBeNull();
    for (const [path, says] of [
      ["", "name"],
      ["   ", "name"],
      ["/runbooks/a.sql", "leading /"],
      ["runbooks/", "folder"],
      ["runbooks//a.sql", "empty folder"],
      ["runbooks/../a.sql", ".."],
      ["./runbooks/a.sql", ".."],
      ["runbooks\\a.sql", "forward slashes"],
      ["runbooks/ a.sql", "space"],
    ] as const) {
      expect(pathProblem(path), path).toContain(says);
    }
  });

  it("puts a picked file under the folder, keeping a picked folder's own path", () => {
    const plain = new File(["x"], "count.sql");
    expect(uploadPath("runbooks", plain)).toBe("runbooks/count.sql");
    const inFolder = new File(["x"], "count.sql");
    Object.defineProperty(inFolder, "webkitRelativePath", { value: "health_check/steps/count.sql" });
    expect(uploadPath("runbooks", inFolder)).toBe("runbooks/health_check/steps/count.sql");
    expect(uploadPath("", inFolder)).toBe("health_check/steps/count.sql");
  });

  it("lists the tree's files by their decoded paths", () => {
    const tree = [
      {
        key: "runbooks",
        pageMatchKey: "runbooks",
        type: "section",
        size: 0,
        label: "Runbooks",
        route_path: "",
        route_params: null,
        children: [
          {
            key: "a b.sql",
            pageMatchKey: "runbooks%2Fa+b.sql",
            type: "file",
            size: 1,
            label: "a b.sql",
            route_path: "",
            route_params: { file_name: "runbooks%2Fa+b.sql" },
            children: null,
          },
        ],
      },
    ] as WorkspaceNode[];
    expect([...treeFilePaths(tree)]).toEqual(["runbooks/a b.sql"]);
  });
});

describe("readText", () => {
  it("reads a text file and refuses one that is not text", async () => {
    expect(await readText(new File(["SELECT 1\n"], "a.sql"))).toBe("SELECT 1\n");
    expect(await readText(new File([new Uint8Array([0x50, 0x4b, 0x03, 0x04, 0x00])], "a.zip"))).toBeNull();
    expect(await readText(new File([new Uint8Array([0x41, 0xff, 0xfe, 0x42])], "a.bin"))).toBeNull();
  });
});

describe("describeUploadPlan", () => {
  it("lists the new files, the replaced ones, the ones left out and the warnings", () => {
    const result = (path: string, status: AddedFileResult["status"], warnings: AddedFileResult["warnings"] = []) =>
      ({ file_name: path, path, status, warnings }) as AddedFileResult;
    const text = describeUploadPlan([
      result("runbooks/rb.rb.json", "new"),
      result("runbooks/rb/a.sql", "replaces"),
      result("runbooks/rb/notes.txt", "not_listed"),
      result("runbooks/rb/b.sql", "new", [{ severity: "warning", code: "x", message: "a finding", line: 3 }]),
    ]);
    expect(text).toBe(
      [
        "New:",
        "  runbooks/rb.rb.json",
        "  runbooks/rb/b.sql",
        "",
        "Replaces the file there:",
        "  runbooks/rb/a.sql",
        "",
        "Left out, a file the IDE does not list:",
        "  runbooks/rb/notes.txt",
        "",
        "Saved with warnings:",
        "  runbooks/rb/b.sql",
        "    line 3: a finding",
      ].join("\n"),
    );
  });
});

/**
 * The workspace validator and its Node entry. `jetstore_maintenance_02` AG.2.
 *
 * Three groups. The shipping assets pass with the production registry — the
 * claim that matters, since `install_workspace_assets` puts exactly these into
 * every workspace and the compile now refuses to proceed past an error. Each
 * check fires, at the file and pointer an author would edit. And the command's
 * exit status is the contract the Go step reads: 0, 1, or 2, never 0 for a
 * workspace it could not see.
 */

import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";

import { emptyRegistry } from "../actions/escapes";
import { productionRegistry } from "../actions/registry";
import { directoryReader, main } from "./validateWorkspaceCli";
import {
  formatFindings,
  validateWorkspace,
  type WorkspaceFinding,
  type WorkspaceReader,
} from "./validateWorkspace";

const assetsDir = fileURLToPath(new URL("../../../jets/workspace_assets/", import.meta.url));

/** An in-memory workspace: path → text. */
function memoryReader(files: Record<string, string>): WorkspaceReader {
  return {
    list(dir) {
      const prefix = `${dir}/`;
      const names = Object.keys(files)
        .filter((p) => p.startsWith(prefix) && !p.slice(prefix.length).includes("/"))
        .map((p) => p.slice(prefix.length));
      return names.length > 0 ? names : null;
    },
    read: (path) => files[path] ?? null,
  };
}

/** The shipping documents of one flow and every table, as a mutable tree. */
function shippingTree(flowKey: string): Record<string, string> {
  const reader = directoryReader(assetsDir);
  const files: Record<string, string> = {};
  for (const suffix of ["uf", "ua", "form"]) {
    const path = `user_flows/${flowKey}.${suffix}.json`;
    files[path] = reader.read(path)!;
  }
  for (const name of reader.list("table_configs")!) {
    files[`table_configs/${name}`] = reader.read(`table_configs/${name}`)!;
  }
  return files;
}

const edit = (files: Record<string, string>, path: string, change: (doc: any) => void): void => {
  const doc = JSON.parse(files[path]!);
  change(doc);
  files[path] = JSON.stringify(doc);
};

const errors = (findings: WorkspaceFinding[]) => findings.filter((f) => f.severity === "error");

describe("the shipping assets", () => {
  it("validate with no error against the production registry", () => {
    const findings = validateWorkspace(directoryReader(assetsDir), { registry: productionRegistry });
    expect(formatFindings(errors(findings))).toEqual([]);
  });

  it("are actually walked: an empty registry finds the escapes they name", () => {
    // The guard on the test above: if the reader found nothing, it would pass
    // vacuously. The shipping flows name escapes, so the empty registry must
    // refuse some of them — in flows and in tables both.
    const findings = errors(validateWorkspace(directoryReader(assetsDir), { registry: emptyRegistry }));
    expect(findings.some((f) => f.file.startsWith("user_flows/"))).toBe(true);
    expect(findings.some((f) => f.file.startsWith("table_configs/"))).toBe(true);
  });
});

describe("each check fires at the file an author would edit", () => {
  const run = (files: Record<string, string>) =>
    errors(validateWorkspace(memoryReader(files), { registry: productionRegistry }));

  it("passes the unmutated tree", () => {
    expect(run(shippingTree("clientRegistryUF"))).toEqual([]);
  });

  it("a document that is not JSON", () => {
    const files = shippingTree("clientRegistryUF");
    files["user_flows/clientRegistryUF.form.json"] = "{ nope";
    const [f, ...rest] = run(files);
    expect(rest).toEqual([]);
    expect(f).toMatchObject({ file: "user_flows/clientRegistryUF.form.json", pointer: "" });
    expect(f!.message).toContain("not valid JSON");
  });

  it("a document its schema refuses, with the Zod path as a pointer", () => {
    const files = shippingTree("clientRegistryUF");
    edit(files, "user_flows/clientRegistryUF.uf.json", (d) => {
      d.startAtKey = 42;
    });
    expect(run(files)).toContainEqual(
      expect.objectContaining({ file: "user_flows/clientRegistryUF.uf.json", pointer: "/startAtKey" }),
    );
  });

  it("a missing sibling is an error on the flow", () => {
    const files = shippingTree("clientRegistryUF");
    delete files["user_flows/clientRegistryUF.ua.json"];
    expect(run(files)).toEqual([
      expect.objectContaining({
        file: "user_flows/clientRegistryUF.uf.json",
        message: expect.stringContaining("clientRegistryUF.ua.json is missing"),
      }),
    ]);
  });

  it("a sibling with no flow is a warning, not an error", () => {
    const files = shippingTree("clientRegistryUF");
    files["user_flows/orphanUF.ua.json"] = files["user_flows/clientRegistryUF.ua.json"]!;
    const findings = validateWorkspace(memoryReader(files), { registry: productionRegistry });
    expect(errors(findings)).toEqual([]);
    expect(findings).toContainEqual(
      expect.objectContaining({ severity: "warning", file: "user_flows/orphanUF.ua.json" }),
    );
  });

  it("a transition to a state that does not exist (validateFlow)", () => {
    const files = shippingTree("clientRegistryUF");
    edit(files, "user_flows/clientRegistryUF.uf.json", (d) => {
      d.startAtKey = "noSuchState";
    });
    expect(run(files)).toContainEqual(expect.objectContaining({ file: "user_flows/clientRegistryUF.uf.json" }));
  });

  it("a state naming a form the form document lacks (validateDocumentSet)", () => {
    const files = shippingTree("clientRegistryUF");
    edit(files, "user_flows/clientRegistryUF.uf.json", (d) => {
      const first = Object.keys(d.states)[0]!;
      d.states[first].formConfig = "noSuchForm";
    });
    const found = run(files);
    expect(found).toHaveLength(1);
    expect(found[0]).toMatchObject({ file: "user_flows/clientRegistryUF.uf.json" });
    expect(found[0]!.pointer).toMatch(/^\/states\/.+\/formConfig$/);
  });

  it("a table action naming an action the flow lacks lands on the table file", () => {
    const files = shippingTree("clientRegistryUF");
    edit(files, "user_flows/clientRegistryUF.ua.json", (d) => {
      delete d.actions.deleteClientAction;
    });
    expect(run(files)).toEqual([
      expect.objectContaining({
        file: "table_configs/client.tc.json",
        pointer: "/actions/0/actionName",
        message: expect.stringContaining('(in flow "clientRegistryUF")'),
      }),
    ]);
  });

  it("a form drawing a table that is not on disk", () => {
    const files = shippingTree("clientRegistryUF");
    delete files["table_configs/client.tc.json"];
    expect(run(files)).toContainEqual(
      expect.objectContaining({
        file: "user_flows/clientRegistryUF.form.json",
        message: expect.stringContaining("table_configs/client.tc.json does not exist"),
      }),
    );
  });

  it("an escape the build does not have, in a form, an action and a table", () => {
    const files = shippingTree("homeFiltersUF");
    edit(files, "user_flows/homeFiltersUF.ua.json", (d) => {
      for (const action of Object.values<any>(d.actions)) {
        for (const step of action.steps) if (step.do === "escape") step.name = "noSuchEscape";
      }
    });
    edit(files, "table_configs/client.tc.json", (d) => {
      d.columns[0].cellFilter = "noSuchFilter";
    });
    const found = run(files);
    expect(found).toContainEqual(
      expect.objectContaining({
        file: "user_flows/homeFiltersUF.ua.json",
        pointer: expect.stringMatching(/^\/actions\/.+\/steps\/\d+$/),
        message: expect.stringContaining('"noSuchEscape"'),
      }),
    );
    expect(found).toContainEqual(
      expect.objectContaining({ file: "table_configs/client.tc.json", pointer: "/columns/0/cellFilter" }),
    );
  });

  it("an escape a form names, with the pointer relative to the form file", () => {
    const files = shippingTree("homeFiltersUF");
    edit(files, "user_flows/homeFiltersUF.form.json", (d) => {
      const first = Object.keys(d.forms)[0]!;
      d.forms[first].validator = "noSuchValidator";
    });
    const found = run(files);
    expect(found).toHaveLength(1);
    expect(found[0]).toMatchObject({ file: "user_flows/homeFiltersUF.form.json" });
    expect(found[0]!.pointer).toMatch(/^\/forms\/.+\/validator$/);
  });
});

describe("the command", () => {
  const dirs: string[] = [];
  afterEach(() => {
    for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
  });
  const capture = () => {
    const out: string[] = [];
    const err: string[] = [];
    return { out, err, o: (l: string) => out.push(l), e: (l: string) => err.push(l) };
  };
  const copyOfAssets = (): string => {
    const dir = mkdtempSync(join(tmpdir(), "jm2-ag-"));
    dirs.push(dir);
    cpSync(join(assetsDir, "user_flows"), join(dir, "user_flows"), { recursive: true });
    cpSync(join(assetsDir, "table_configs"), join(dir, "table_configs"), { recursive: true });
    return dir;
  };

  it("exits 0 over the shipping assets and says how many it found", () => {
    const c = capture();
    expect(main([assetsDir], {}, c.o, c.e)).toBe(0);
    expect(c.out.at(-1)).toContain("0 error(s)");
  });

  it("exits 1 on an error finding, and prints the file and pointer", () => {
    const dir = copyOfAssets();
    const path = join(dir, "user_flows", "clientRegistryUF.ua.json");
    const doc = JSON.parse(readFileSync(path, "utf8"));
    delete doc.actions.deleteClientAction;
    writeFileSync(path, JSON.stringify(doc));
    const c = capture();
    expect(main([dir], {}, c.o, c.e)).toBe(1);
    expect(c.out).toContain(
      'ERROR table_configs/client.tc.json#/actions/0/actionName: table "client" action "deleteClient" runs ' +
        '"deleteClientAction", which the action document does not define (in flow "clientRegistryUF")',
    );
  });

  it("exits 0 on warnings alone", () => {
    const dir = copyOfAssets();
    cpSync(join(dir, "user_flows", "clientRegistryUF.ua.json"), join(dir, "user_flows", "orphanUF.ua.json"));
    const c = capture();
    expect(main([dir], {}, c.o, c.e)).toBe(0);
    expect(c.out.some((l) => l.startsWith("WARNING user_flows/orphanUF.ua.json"))).toBe(true);
  });

  it("exits 2, never 0, when it cannot see a workspace", () => {
    const c = capture();
    expect(main([], {}, c.o, c.e)).toBe(2);
    expect(main([join(tmpdir(), "jm2-ag-does-not-exist")], {}, c.o, c.e)).toBe(2);
    expect(c.err).toHaveLength(2);
  });

  it("--json prints the findings as data", () => {
    const dir = copyOfAssets();
    writeFileSync(join(dir, "table_configs", "client.tc.json"), "{");
    const c = capture();
    expect(main([dir, "--json"], {}, c.o, c.e)).toBe(1);
    const findings = JSON.parse(c.out.join("\n")) as WorkspaceFinding[];
    expect(findings).toContainEqual(
      expect.objectContaining({ severity: "error", file: "table_configs/client.tc.json", pointer: "" }),
    );
  });

  it("a workspace with neither directory has nothing to refuse", () => {
    // A workspace that never had assets installed is not invalid, it is empty —
    // which is different from a directory that is not there, refused above.
    const dir = mkdtempSync(join(tmpdir(), "jm2-ag-"));
    dirs.push(dir);
    const c = capture();
    expect(main([dir], {}, c.o, c.e)).toBe(0);
  });
});

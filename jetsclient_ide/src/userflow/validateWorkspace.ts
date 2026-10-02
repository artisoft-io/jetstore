/**
 * Every user-flow and table document in a workspace, validated as the app would
 * load them. `jetstore_maintenance_02` task AG.2 (`R12`).
 *
 * ## Why this exists
 *
 * These documents are going to be written by a model (Michel, 2026-10-01), so a
 * validator is the whole of their quality control. Two halves existed and
 * neither was reachable by a generator or a build:
 *
 * - **per document** — the JSON Schemas, enforced in Go on every save and over
 *   the embedded corpus by test;
 * - **between documents** — `documentSet.ts` and escape resolution, which run in
 *   the browser at `FlowStore.load` and in vitest, and nowhere else.
 *
 * This function runs both over a directory tree, collecting every finding with
 * the file it is in, rather than stopping at the first as `FlowStore.load` must.
 * `validateWorkspaceCli.ts` is the Node entry the workspace compile runs
 * (`jets/workspace/asset_validation.go`); a generator runs the same script.
 *
 * ## What is checked, and in what order
 *
 * 1. Every `table_configs/*.tc.json` parses, and every escape it names (cell
 *    filters, `isEnabled` predicates) is in the registry.
 * 2. Every `user_flows/*.uf.json`, `*.ua.json` and `*.form.json` parses.
 * 3. For each flow: both siblings exist; `validateFlow`'s reference checks;
 *    `validateDocumentSet`; every table its forms name exists; then
 *    `validateTableActions`; then every escape the flow, its actions and its
 *    forms name is in the registry.
 *
 * Step 3 mirrors `FlowStore.load` (`store.ts`) check for check. **It is a second
 * reading of that sequence rather than a call to it**, because `load` throws at
 * the first failing stage and reports pointers without files, which is right for
 * a screen and wrong for a build log. A new check added to `load` must be added
 * here too; the test beside this file runs both over the shipping assets.
 *
 * `.apply.json` (a projected flow's template plan, read by `src/cpipes/` alone)
 * is not one of the four kinds and is not checked here.
 *
 * ## Pure on purpose
 *
 * No `node:fs` and no registry import: the caller supplies a reader and the
 * registry. That keeps this testable with an in-memory tree and `emptyRegistry`,
 * and keeps the Node-only half in the CLI.
 */

import { resolveEscapes, type EscapeReferences, type EscapeRegistry } from "../actions/escapes";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import {
  TableConfigDocumentSchema,
  tableEscapeReferences,
  type TableConfigDocument,
} from "../datatable/table";
import { validateDocumentSet, validateTableActions, type SetFinding } from "./documentSet";
import { FormDocumentSchema, fieldsOf, type FormDocument } from "./form";
import { UserFlowSchema, type UserFlow } from "./schema";
import { actionPath, escapeReferences, flowPath, formEscapeReferences, formPath, tableKeysOf } from "./store";
import { defaultPolicy, validateFlow, type Policy, type Severity } from "./validate";

export const TABLE_DIR = "table_configs";
export const FLOW_DIR = "user_flows";

/** One problem, located by file and by JSON Pointer within it. */
export interface WorkspaceFinding {
  severity: Severity;
  /** Relative to the workspace root, `/`-separated. */
  file: string;
  /** RFC 6901, into `file`. Empty when the finding is about the whole file. */
  pointer: string;
  message: string;
}

/** What the validator needs from a filesystem, and nothing else. */
export interface WorkspaceReader {
  /** The file names directly inside `dir` (relative to the root), or null when it does not exist. */
  list(dir: string): string[] | null;
  /** The file's text, or null when it does not exist. */
  read(path: string): string | null;
}

export interface ValidateWorkspaceOptions {
  registry: EscapeRegistry;
  policy?: Policy;
}

const tableFile = (key: string): string => `${TABLE_DIR}/${key}.tc.json`;

type Parser = { safeParse(v: unknown): { success: boolean; data?: unknown; error?: unknown } };

const escapePointer = (segment: PropertyKey): string =>
  String(segment).replace(/~/g, "~0").replace(/\//g, "~1");

/** Parses one file, or reports why it does not. */
function parseFile<T>(
  reader: WorkspaceReader,
  file: string,
  schema: Parser,
  findings: WorkspaceFinding[],
): T | undefined {
  const text = reader.read(file);
  if (text === null) return undefined;
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch (error) {
    findings.push({ severity: "error", file, pointer: "", message: `not valid JSON: ${(error as Error).message}` });
    return undefined;
  }
  const result = schema.safeParse(value);
  if (result.success) return result.data as T;
  const issues = (result.error as { issues: { path: PropertyKey[]; message: string }[] }).issues;
  for (const issue of issues) {
    findings.push({
      severity: "error",
      file,
      pointer: issue.path.length === 0 ? "" : `/${issue.path.map(escapePointer).join("/")}`,
      message: issue.message,
    });
  }
  return undefined;
}

const keysWithSuffix = (names: string[] | null, suffix: string): string[] =>
  (names ?? [])
    .filter((n) => n.endsWith(suffix))
    .map((n) => n.slice(0, -suffix.length))
    .sort();

/** Which file a set finding is about, given the flow it was raised for. */
function setFindingFile(key: string, finding: SetFinding): { file: string; pointer: string } {
  switch (finding.document) {
    case "flow":
      return { file: flowPath(key), pointer: finding.path };
    case "actions":
      return { file: actionPath(key), pointer: finding.path };
    case "forms":
      return { file: formPath(key), pointer: finding.path };
    case "tables": {
      // `validateTableActions` writes `/<tableKey>/<row>/<index>/…`, the table
      // key standing in for the file. Split it back out.
      const [, tableKey = "", ...rest] = finding.path.split("/");
      return { file: tableFile(tableKey), pointer: rest.length > 0 ? `/${rest.join("/")}` : "" };
    }
  }
}

function unresolvedFindings(
  references: { file: string; ref: EscapeReferences }[],
  registry: EscapeRegistry,
): WorkspaceFinding[] {
  const findings: WorkspaceFinding[] = [];
  for (const { file, ref } of references) {
    if (resolveEscapes([ref], registry).length === 0) continue;
    findings.push({
      severity: "error",
      file,
      pointer: ref.at,
      message: `no ${ref.kind === "queries" ? "registered query" : `${ref.kind} escape`} named "${ref.name}" in this build`,
    });
  }
  return findings;
}

/** Every finding over the workspace a reader exposes, errors and warnings both. */
export function validateWorkspace(reader: WorkspaceReader, options: ValidateWorkspaceOptions): WorkspaceFinding[] {
  const { registry } = options;
  const policy = options.policy ?? defaultPolicy;
  const findings: WorkspaceFinding[] = [];

  // 1. Tables, alone. Every table on disk rather than only the ones a flow
  //    names: an unreferenced table with a bad document is still a bad document,
  //    and the next flow to name it would inherit the failure.
  const tables: Record<string, TableConfigDocument> = {};
  for (const key of keysWithSuffix(reader.list(TABLE_DIR), ".tc.json")) {
    const table = parseFile<TableConfigDocument>(reader, tableFile(key), TableConfigDocumentSchema, findings);
    if (table === undefined) continue;
    tables[key] = table;
    findings.push(
      ...unresolvedFindings(
        tableEscapeReferences(table).map((ref) => ({ file: tableFile(key), ref })),
        registry,
      ),
    );
  }

  // 2. Every flow document, alone.
  const names = reader.list(FLOW_DIR);
  const flowKeys = keysWithSuffix(names, ".uf.json");
  const actionKeys = keysWithSuffix(names, ".ua.json");
  const formKeys = keysWithSuffix(names, ".form.json");
  const flows = new Map<string, UserFlow>();
  const actionDocs = new Map<string, ActionDocument>();
  const formDocs = new Map<string, FormDocument>();
  for (const key of flowKeys) {
    const doc = parseFile<UserFlow>(reader, flowPath(key), UserFlowSchema, findings);
    if (doc !== undefined) flows.set(key, doc);
  }
  for (const key of actionKeys) {
    const doc = parseFile<ActionDocument>(reader, actionPath(key), ActionDocumentSchema, findings);
    if (doc !== undefined) actionDocs.set(key, doc);
  }
  for (const key of formKeys) {
    const doc = parseFile<FormDocument>(reader, formPath(key), FormDocumentSchema, findings);
    if (doc !== undefined) formDocs.set(key, doc);
  }

  // A sibling with no flow is loaded by nothing — `FlowStore.list` reads the
  // `.uf.json` names — so it is a warning rather than an error: harmless today,
  // and most likely a flow whose `.uf.json` was renamed or not yet written.
  for (const [keys, path] of [
    [actionKeys, actionPath],
    [formKeys, formPath],
  ] as const) {
    for (const key of keys) {
      if (flowKeys.includes(key)) continue;
      findings.push({
        severity: "warning",
        file: path(key),
        pointer: "",
        message: `no ${flowPath(key)} beside it, so no flow loads this document`,
      });
    }
  }

  // 3. Each flow as a set, in `FlowStore.load`'s order.
  for (const key of flowKeys) {
    for (const [keys, path] of [
      [actionKeys, actionPath],
      [formKeys, formPath],
    ] as const) {
      if (!keys.includes(key)) {
        findings.push({
          severity: "error",
          file: flowPath(key),
          pointer: "",
          message: `${path(key)} is missing, and the flow cannot load without it`,
        });
      }
    }
    const flow = flows.get(key);
    const actions = actionDocs.get(key);
    const forms = formDocs.get(key);
    // A document that did not parse has already been reported, and checking
    // references over it would report missing forms that are merely unreadable.
    if (flow === undefined || actions === undefined || forms === undefined) continue;

    for (const f of validateFlow(flow, policy)) {
      findings.push({ severity: f.severity, file: flowPath(key), pointer: f.path, message: f.message });
    }

    for (const f of validateDocumentSet({ flow, actions, forms })) {
      findings.push({ severity: f.severity, ...setFindingFile(key, f), message: f.message });
    }

    // Every table the forms name is on disk and parsed. Which form names it is
    // the pointer, so an author lands on the field rather than on the file.
    const named: Record<string, TableConfigDocument> = {};
    for (const tableKey of tableKeysOf(forms)) {
      const table = tables[tableKey];
      if (table !== undefined) {
        named[tableKey] = table;
        continue;
      }
      if (reader.read(tableFile(tableKey)) !== null) continue; // present, did not parse: reported in step 1
      for (const [formKey, form] of Object.entries(forms.forms)) {
        if (!fieldsOf(form).some((field) => field.field === "dataTable" && field.table === tableKey)) continue;
        findings.push({
          severity: "error",
          file: formPath(key),
          pointer: `/forms/${escapePointer(formKey)}`,
          message: `form "${formKey}" draws table "${tableKey}", and ${tableFile(tableKey)} does not exist`,
        });
      }
    }

    for (const f of validateTableActions(actions, forms, named)) {
      const located = setFindingFile(key, f);
      findings.push({ severity: f.severity, ...located, message: `${f.message} (in flow "${key}")` });
    }

    // Escapes. `escapeReferences` writes pointers without a file — the
    // initializer is the flow's, a step is the action document's — and
    // `formEscapeReferences` prefixes its own; both are split back here.
    const references: { file: string; ref: EscapeReferences }[] = [];
    for (const ref of escapeReferences(flow, actions)) {
      references.push({ file: ref.at.startsWith("/actions/") ? actionPath(key) : flowPath(key), ref });
    }
    for (const ref of formEscapeReferences(forms, key)) {
      const prefix = formPath(key);
      references.push({
        file: prefix,
        ref: { ...ref, at: ref.at.startsWith(prefix) ? ref.at.slice(prefix.length) : ref.at },
      });
    }
    findings.push(...unresolvedFindings(references, registry));
  }

  return findings;
}

/** `file#pointer`, the form a finding is printed in. */
export const locationOf = (f: WorkspaceFinding): string => (f.pointer === "" ? f.file : `${f.file}#${f.pointer}`);

/** One line per finding, errors first, in a stable order. */
export function formatFindings(findings: readonly WorkspaceFinding[]): string[] {
  const rank = (s: Severity) => (s === "error" ? 0 : 1);
  return [...findings]
    .sort(
      (a, b) =>
        rank(a.severity) - rank(b.severity) ||
        a.file.localeCompare(b.file) ||
        a.pointer.localeCompare(b.pointer),
    )
    .map((f) => `${f.severity.toUpperCase()} ${locationOf(f)}: ${f.message}`);
}

/**
 * Every user-flow and table document in a workspace, validated as the app would
 * load them.
 *
 * ## Why this exists
 *
 * These documents are going to be written by a model (2026-10-01), so a
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
 * 1. Every `table_configs/*.tc.json` parses, every escape it names (cell
 *    filters, `isEnabled` predicates) is in the registry, and every authored
 *    `isEnabled` is the predicate the runtime derives for that table and action
 *    (`isEnabledFindings`).
 * 2. Every `user_flows/*.uf.json`, `*.ua.json` and `*.form.json` parses.
 * 3. For each flow: both siblings exist; every table its forms name exists;
 *    then `checkFlowSet` (`store.ts`), the checks `FlowStore.load` makes:
 *    `validateFlow`'s reference checks, `validateDocumentSet`,
 *    `validateTableActions`, and every escape the flow, its actions and its
 *    forms name is in the registry.
 *
 * ~~Step 3 mirrors `FlowStore.load` check for check, as a second reading of
 * that sequence rather than a call to it.~~ **It calls it, since
 * 2026-10-27.** The restatement had begun to drift. `load` throws at the first failing stage and this
 * reports every finding with its file, and that difference is now in what each
 * caller does with `checkFlowSet`'s findings rather than in two copies of the
 * checks.
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
  actionRowsOf,
  tableEscapeReferences,
  type TableConfigDocument,
} from "../datatable/table";
import { isEnabledEscapeFor } from "../datatable/tableTranslate";
import type { SetFinding } from "./documentSet";
import { FormDocumentSchema, fieldsOf, type FormDocument } from "./form";
import { UserFlowSchema, type UserFlow } from "./schema";
import { actionPath, checkFlowSet, flowPath, formPath, tableKeysOf } from "./store";
import { defaultPolicy, type Policy, type Severity } from "./validate";

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

type Parser = {
  safeParse(v: unknown): { success: boolean; data?: unknown; error?: unknown };
  /** Zod's: whether `v` parses, stopping at the first problem rather than collecting every one. */
  validate(v: unknown): boolean;
};

const escapePointer = (segment: PropertyKey): string =>
  String(segment).replace(/~/g, "~0").replace(/\//g, "~1");

/**
 * The most findings listed for one file; the rest are counted in one more.
 *
 * A file with this many findings is not fixed one finding at a time, so the
 * first ones and a count say all an author can use, and a compile log is not
 * buried under one document's problems.
 */
export const MAX_FINDINGS_PER_FILE = 100;

/**
 * The most JSON values a document may hold and still have its problems listed.
 *
 * **The cap above does not bound memory on its own.** The schema records an
 * issue for every element that fails before any finding is built, so a
 * `.tc.json` whose `columns` holds a million numbers ran a 1 GB Node heap out
 * of memory inside the schema library, with or without the cap. A document over
 * this size is checked with `validate`, which stops at its first problem: a
 * valid one is accepted as before, and an invalid one gets one finding saying
 * so, without the list. The largest shipped document holds about 7,700 values
 * (a projected form document, 2026-11-14), so this is several times any real
 * one, and at this size the schema's issues fit in a few hundred megabytes even
 * for a form document whose every field fails all its kinds.
 */
export const MAX_VALUES_LISTED = 50_000;

/** Whether `value` holds more than `limit` JSON values, counting containers. Stops counting there. */
function holdsMoreThan(value: unknown, limit: number): boolean {
  let count = 0;
  const stack: unknown[] = [value];
  while (stack.length > 0) {
    const v = stack.pop();
    count += 1;
    if (count > limit) return true;
    if (Array.isArray(v)) stack.push(...v.slice(0, limit + 1 - count));
    else if (v !== null && typeof v === "object") stack.push(...Object.values(v as object).slice(0, limit + 1 - count));
  }
  return false;
}

/**
 * The findings of one run, at most `MAX_FINDINGS_PER_FILE` per file.
 *
 * `push` takes findings as an array's `push` does and drops those past the cap,
 * counting them; `list` adds, for each file that had more, one finding saying
 * how many were not listed, an error if any of them was.
 */
class CappedFindings {
  private readonly listed: WorkspaceFinding[] = [];
  private readonly perFile = new Map<string, { count: number; omitted: number; omittedError: boolean }>();

  private entry(file: string) {
    let e = this.perFile.get(file);
    if (e === undefined) {
      e = { count: 0, omitted: 0, omittedError: false };
      this.perFile.set(file, e);
    }
    return e;
  }

  /** Whether `file` already has as many findings listed as it may. */
  full(file: string): boolean {
    return this.entry(file).count >= MAX_FINDINGS_PER_FILE;
  }

  push(...findings: WorkspaceFinding[]): void {
    for (const f of findings) {
      const e = this.entry(f.file);
      if (e.count < MAX_FINDINGS_PER_FILE) {
        e.count += 1;
        this.listed.push(f);
      } else {
        this.omit(f.file, 1, f.severity);
      }
    }
  }

  /** Counts `n` findings of `file` that were never built. */
  omit(file: string, n: number, severity: Severity): void {
    const e = this.entry(file);
    e.omitted += n;
    if (severity === "error") e.omittedError = true;
  }

  list(): WorkspaceFinding[] {
    const out = [...this.listed];
    for (const [file, e] of this.perFile) {
      if (e.omitted === 0) continue;
      out.push({
        severity: e.omittedError ? "error" : "warning",
        file,
        pointer: "",
        message:
          `${e.omitted} more finding(s) in this file are not listed: ` +
          `the validator lists the first ${MAX_FINDINGS_PER_FILE} of each file`,
      });
    }
    return out;
  }
}

/** Parses one file, or reports why it does not. */
function parseFile<T>(
  reader: WorkspaceReader,
  file: string,
  schema: Parser,
  findings: CappedFindings,
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
  if (holdsMoreThan(value, MAX_VALUES_LISTED) && !schema.validate(value)) {
    findings.push({
      severity: "error",
      file,
      pointer: "",
      message:
        `does not match its schema, and holds more than ${MAX_VALUES_LISTED} values, too many for ` +
        "its problems to be listed: check a smaller copy of it to see them",
    });
    return undefined;
  }
  const result = schema.safeParse(value);
  if (result.success) return result.data as T;
  const issues = (result.error as { issues: { path: PropertyKey[]; message: string }[] }).issues;
  for (const [index, issue] of issues.entries()) {
    // Stop building findings at the cap rather than building them all and
    // dropping most: building them is the cost the cap is for.
    if (findings.full(file)) {
      findings.omit(file, issues.length - index, "error");
      break;
    }
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
function setFindingFile(
  key: string,
  finding: Pick<SetFinding, "document" | "path">,
): { file: string; pointer: string } {
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

/**
 * An authored `isEnabled` the runtime would not use.
 *
 * **The action bar does not read the document's `isEnabled`.** It asks
 * `isEnabledEscapeFor(tableKey, actionKey)` (`actionBarModel.ts`), because
 * `fromDocument` keeps only that the action has a predicate, not its name
 * (a choice that keeps `ActionConfig` free of a field the
 * corpus cannot express). So a document naming another predicate is valid,
 * resolves, and is silently gated on the derived one instead. Every committed
 * document agrees with the derivation, which `clearFiltersGate.test.tsx`
 * pins; this is the check for a document written in a workspace, by hand or by
 * a model.
 *
 * The table key is the file's name, which is the key the runtime is given.
 */
export function isEnabledFindings(tableKey: string, table: TableConfigDocument): WorkspaceFinding[] {
  const findings: WorkspaceFinding[] = [];
  for (const [row, actions] of actionRowsOf(table)) {
    actions.forEach((action, index) => {
      if (action.isEnabled === undefined) return;
      const derived = isEnabledEscapeFor(tableKey, action.key);
      if (action.isEnabled === derived) return;
      findings.push({
        severity: "error",
        file: tableFile(tableKey),
        pointer: `/${row}/${index}/isEnabled`,
        message:
          `action "${action.key}" names predicate "${action.isEnabled}", and the runtime gates it on ` +
          `"${derived}" for this table and action, so "${action.isEnabled}" would be ignored`,
      });
    });
  }
  return findings;
}

/**
 * Every finding over the workspace a reader exposes, errors and warnings both,
 * at most `MAX_FINDINGS_PER_FILE` of them per file and a count of the rest.
 */
export function validateWorkspace(reader: WorkspaceReader, options: ValidateWorkspaceOptions): WorkspaceFinding[] {
  const { registry } = options;
  const policy = options.policy ?? defaultPolicy;
  const findings = new CappedFindings();

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
    findings.push(...isEnabledFindings(key, table));
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

    // Every table the forms name is on disk and parsed. Which form names it is
    // the pointer, so an author lands on the field rather than on the file.
    // This is the validator's half of what `FlowStore.load` meets as a read
    // failure, so it stays here rather than in `checkFlowSet`.
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

    // `load`'s checks, by calling them. Every stage is reported rather
    // than the first that fails, each finding at the file it is about.
    for (const f of checkFlowSet({ flow, actions, forms }, named, { registry, policy }, key)) {
      // A table's escapes were reported in step 1 for every table on disk, so
      // the copy each flow naming the table would add is dropped.
      if (f.stage === "escapes" && f.document === "tables") continue;
      const located = setFindingFile(key, f);
      const message = f.stage === "tables" ? `${f.message} (in flow "${key}")` : f.message;
      findings.push({ severity: f.severity, ...located, message });
    }
  }

  return findings.list();
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

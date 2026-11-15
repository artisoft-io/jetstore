/**
 * Loading and saving user flows from a workspace.
 *
 * **No new plumbing, as was predicted.** The Workspace IDE
 * already reads and writes workspace files through `/dataTable`
 * (`api/workspace.ts`), the server already persists them through
 * `workspace_changes`, and it already refuses to save a `.json` file that will
 * not parse. This is a store on top of that, not a transport.
 *
 * ## Where the files live, and why there
 *
 * `user_flows/<key>.uf.json` and `user_flows/<key>.ua.json`, one directory per
 * file type — the convention the workspaces already use: `pipes_config/` holds
 * `*.pc.json`, `jet_rules/` holds the rules, `lookups/` the lookup tables. A
 * flow's actions sit beside it rather than inside it for a reason settled when
 * the action grammar was sized: one action is reachable from a state and from
 * a table's action bar, and a sibling document lets one entry serve both.
 *
 * **The key is the file name and nothing else.** The flow schema settled that a flow carries
 * no self-key, because `FormConfig.key` had drifted from its registry key twice
 * in fifty. The same reasoning applies to the file: two names for one
 * thing is one name too many.
 *
 * ## Loading validates, and a flow that does not validate does not load
 *
 * Three checks, in an order chosen so the first failure is the useful one:
 * the document's shape, then its internal references, then whether the escape
 * names it uses exist in this build. **The third can only happen here** — the
 * registry is compiled into the bundle and the server cannot enumerate it
 * (`actions/escapes.ts`), so a green save does not mean a loadable flow.
 *
 * Findings carry a JSON Pointer, so an editor can put the cursor on the offence
 * rather than on the file.
 *
 * ## Four documents, not two
 *
 * The first store loaded `.uf.json` and `.ua.json`. It did not load the `.form.json`, and
 * `FormDocumentSchema` had no consumer outside three test files, so a form could
 * be authored, saved, validated in Go and rejected properly when malformed —
 * and nothing in the client would ever open it. The `.tc.json` a
 * `dataTable` field names was in the same position once the table document existed.
 *
 * Both are loaded here now, and **the set is validated as a set**
 * (`documentSet.ts`, `validateDocumentSet`). That check could not run
 * anywhere else: it needs three documents at once, and the Go save path sees one
 * file at a time — legitimately, since a `.uf.json` is saved before its
 * `.form.json` exists.
 *
 * **Table documents are keyed by table and not by flow**, which is why they live
 * in `table_configs/` rather than beside the flow: two flows may name the same
 * table. So the set of tables to fetch is not knowable from the flow document —
 * it is read off the *form* document, after it parses.
 */

import {
  describeUnresolved,
  resolveEscapes,
  type EscapeReferences,
  type EscapeRegistry,
  type UnresolvedEscape,
} from "../actions/escapes";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { describeSaveWarnings, type SaveFinding, type WorkspaceApi } from "../api/workspace";
import {
  TableConfigDocumentSchema,
  tableEscapeReferences,
  tablePath,
  type TableConfigDocument,
} from "../datatable/table";
import { fromDocument } from "../datatable/tableTranslate";
import type { TableConfig } from "../datatable/types";
import {
  validateAddClearsState,
  validateDocumentSet,
  validateTableActions,
  type SetFinding,
  type SetFindingCode,
} from "./documentSet";
import { FormDocumentSchema, fieldsOf, type FormDocument } from "./form";
import { UserFlowSchema, type UserFlow } from "./schema";
import { validateFlow, type Finding, type Policy, type Severity } from "./validate";

/** The directory a workspace keeps its flows in. */
export const FLOW_DIR = "user_flows";

export const flowPath = (key: string): string => `${FLOW_DIR}/${key}.uf.json`;
export const actionPath = (key: string): string => `${FLOW_DIR}/${key}.ua.json`;
export const formPath = (key: string): string => `${FLOW_DIR}/${key}.form.json`;

/** A flow and everything it needs to run, because it runs with none of it missing. */
export interface LoadedFlow {
  key: string;
  flow: UserFlow;
  actions: ActionDocument;
  forms: FormDocument;
  /** Keyed by table key, as the `dataTable` fields name them. */
  tables: Record<string, TableConfigDocument>;
}

/** Every table key the forms of this set name, in document order, deduplicated. */
export function tableKeysOf(forms: FormDocument): string[] {
  const keys = new Set<string>();
  for (const form of Object.values(forms.forms)) {
    for (const field of fieldsOf(form)) {
      if (field.field === "dataTable") keys.add(field.table);
    }
  }
  return [...keys];
}

/**
 * A loaded table document, as the runtime configuration the widget consumes.
 *
 * `fromDocument` was written for the table document's round-trip proof — *the translation loses
 * nothing* — and this is its first production caller, which is the outcome that
 * test was arguing for: the inverse being exercised rather than merely asserted.
 */
export function tableConfigOf(loaded: LoadedFlow, key: string): TableConfig {
  const document = loaded.tables[key];
  if (document === undefined) {
    throw new FlowLoadError(loaded.key, [
      {
        severity: "error",
        code: "unknownTarget",
        message: `no table configuration loaded for "${key}"`,
        path: "",
      },
    ]);
  }
  return fromDocument(key, document);
}

export interface LoadFailure {
  key: string;
  findings: Finding[];
}

export class FlowLoadError extends Error {
  constructor(
    readonly key: string,
    readonly findings: Finding[],
  ) {
    super(`user flow "${key}" cannot be loaded:\n${findings.map((f) => `  ${f.path} ${f.message}`).join("\n")}`);
    this.name = "FlowLoadError";
  }
}

/**
 * Turns a Zod failure into the same `Finding` shape the reference checks emit.
 *
 * One shape for both is what lets an editor treat "this is not a valid document"
 * and "this transition goes nowhere" the same way — and Zod's issue path is
 * already the segments a JSON Pointer needs.
 */
function findingsFromParse(error: { issues: ParseIssue[] }): Finding[] {
  return error.issues.map((issue) => ({
    severity: "error" as const,
    code: "unknownTarget" as const,
    message: isUnknownEnumeratedValue(issue) ? `${issue.message}. ${RELOAD_HINT}` : issue.message,
    // RFC 6901: "~" then "/", in that order, or the escaping is wrong.
    path: `/${issue.path.map((p) => String(p).replace(/~/g, "~0").replace(/\//g, "~1")).join("/")}`,
  }));
}

/** The part of a Zod issue `findingsFromParse` reads. */
interface ParseIssue {
  path: PropertyKey[];
  message: string;
  code?: string;
  note?: string;
  /** On `invalid_union`, each alternative's own issues. */
  errors?: { code?: string; path: PropertyKey[] }[][];
}

/**
 * What a refusal adds when the document may simply be newer than this page.
 *
 * The schemas are compiled into the bundle, so a tab opened before a deployment
 * keeps the old ones: a document that uses a value the deployment added to an
 * enumerated field (a new button style, a new step kind) is refused until the
 * page is reloaded. The refusal cannot tell that from a typo, so it says both.
 */
export const RELOAD_HINT =
  "If JetStore was updated since this page was opened, reload the page: it may not know this value yet";

/**
 * Whether a Zod issue is an enumerated field holding a value the schema does not list.
 *
 * Three shapes: an enum or literal that does not match (`invalid_value`); a
 * discriminated union whose discriminator matches no member; and a plain union
 * every member of which refuses the same field's value, which is how a step's
 * `do` or a field's `field` is checked. Any other refusal is about the document.
 */
export function isUnknownEnumeratedValue(issue: ParseIssue): boolean {
  if (issue.code === "invalid_value") return true;
  if (issue.code !== "invalid_union") return false;
  if (issue.note === "No matching discriminator") return true;
  const members = issue.errors ?? [];
  if (members.length === 0) return false;
  const refusedValueAt = (member: { code?: string; path: PropertyKey[] }[]): Set<string> =>
    new Set(member.filter((i) => i.code === "invalid_value").map((i) => i.path.map(String).join("/")));
  const [first, ...rest] = members.map(refusedValueAt);
  return [...first!].some((path) => rest.every((paths) => paths.has(path)));
}

/** One file read, with the failure carried as a value so a batch can report all of them. */
interface ReadResult {
  path: string;
  text?: string;
  error?: Error;
}

export interface FlowStoreOptions {
  workspaceName: string;
  registry: EscapeRegistry;
  policy?: Policy;
}

export class FlowStore {
  constructor(
    private readonly api: WorkspaceApi,
    private readonly options: FlowStoreOptions,
  ) {}

  /**
   * The flow keys this workspace defines.
   *
   * Read from the file tree rather than from a manifest: a manifest is a second
   * place the set of flows is written down, and the flow schema spent a finding on what
   * happens when one name is recorded twice.
   */
  async list(): Promise<string[]> {
    const tree = await this.api.fileTree(this.options.workspaceName);
    const keys = new Set<string>();
    const walk = (nodes: { label: string; children?: unknown }[]): void => {
      for (const node of nodes) {
        if (node.label.endsWith(".uf.json")) keys.add(node.label.slice(0, -".uf.json".length));
        if (Array.isArray(node.children)) walk(node.children as typeof nodes);
      }
    };
    walk(tree as unknown as { label: string; children?: unknown }[]);
    return [...keys].sort();
  }

  /**
   * Reads, parses and validates. Throws `FlowLoadError` rather than returning
   * half a flow.
   *
   * **Six checks, in an order chosen so the first failure is the useful one**:
   * shape, then the flow's internal references, then the *set*, then the tables'
   * shape, then the tables' cross-document references, then whether every
   * escape name exists in this build. Each rests on the one before — a set check
   * over a document that did not parse would report missing forms that are merely
   * unreadable.
   */
  async load(key: string): Promise<LoadedFlow> {
    const reads = await this.readAll([flowPath(key), actionPath(key), formPath(key)]);
    // The checks after parsing are `checkFlowSet`'s, which the workspace
    // validator also calls. This method's
    // part is the reading and the order in which a failing stage is thrown.
    const unreadable = reads.filter((r) => r.error !== undefined);
    if (unreadable.length > 0) {
      // **A missing document is a load error with the file's name in it**, not a
      // transport exception escaping the store. Before the flow runner a flow was two files
      // and both were fetched with a bare `Promise.all`; a third that a workspace
      // may legitimately not have authored yet makes the difference visible.
      throw new FlowLoadError(
        key,
        unreadable.map((r) => ({
          severity: "error" as const,
          code: "unknownTarget" as const,
          message: `${r.path} could not be read: ${r.error!.message}`,
          path: "",
        })),
      );
    }
    const [flowText, actionText, formText] = reads.map((r) => r.text!) as [string, string, string];

    const findings = [
      ...this.parseFindings(flowText, flowPath(key), UserFlowSchema),
      ...this.parseFindings(actionText, actionPath(key), ActionDocumentSchema),
      ...this.parseFindings(formText, formPath(key), FormDocumentSchema),
    ];
    if (findings.length > 0) throw new FlowLoadError(key, findings);

    const flow = UserFlowSchema.parse(JSON.parse(flowText)) as UserFlow;
    const actions = ActionDocumentSchema.parse(JSON.parse(actionText)) as ActionDocument;
    const forms = FormDocumentSchema.parse(JSON.parse(formText)) as FormDocument;

    // The two stages that do not read the tables run first, over no tables, so a
    // flow they refuse costs no table reads. Their findings do not depend on the
    // tables, so they are the ones the full run below would give.
    const early = checkFlowSet({ flow, actions, forms }, {}, this.options, key);
    const earlyStage = (name: FlowCheckStage) => early.filter((f) => f.stage === name);

    // Warnings, such as an unreachable state, do not refuse a flow.
    const references = earlyStage("references").filter((f) => f.severity === "error");
    if (references.length > 0) throw new FlowLoadError(key, references.map(asLoadFinding));

    // The set findings carry their own code union — deliberately not
    // `validate.ts`'s, because Go can never raise these — so they are mapped
    // rather than concatenated, and the document they name goes into the pointer
    // so a message spanning files still says which file.
    const set = earlyStage("set");
    if (set.length > 0) throw new FlowLoadError(key, set.map(asLoadFinding));

    const { tables, findings: tableReadFindings } = await this.loadTables(tableKeysOf(forms));
    if (tableReadFindings.length > 0) throw new FlowLoadError(key, tableReadFindings);

    const checks = checkFlowSet({ flow, actions, forms }, tables, this.options, key);
    const stage = (name: FlowCheckStage) => checks.filter((f) => f.stage === name);

    const tableActions = stage("tables");
    if (tableActions.length > 0) throw new FlowLoadError(key, tableActions.map(asLoadFinding));

    const unresolved = stage("escapes").map((f) => f.unresolved!);
    if (unresolved.length > 0) {
      throw new FlowLoadError(key, [
        {
          severity: "error",
          code: "unknownTarget",
          message: describeUnresolved(unresolved)!,
          path: "",
        },
      ]);
    }

    return { key, flow, actions, forms, tables };
  }

  /**
   * Reads and parses the table documents the forms name.
   *
   * **Every failure is collected rather than thrown at the first**, and that is
   * not symmetry with the rest of the file for its own sake: a flow with four
   * tables is normally missing all four or none — the directory has not been
   * authored yet — and reporting them one reload at a time is the friction
   * `resolveEscapes` avoids for the same reason. **Returned rather than thrown
   * since the checks were shared with the workspace validator**, so that `load`
   * decides when to throw them. `load` reads the tables only once the checks that do not need them have passed.
   */
  private async loadTables(
    keys: string[],
  ): Promise<{ tables: Record<string, TableConfigDocument>; findings: Finding[] }> {
    const reads = await this.readAll(keys.map(tablePath));

    const findings: Finding[] = [];
    const tables: Record<string, TableConfigDocument> = {};
    reads.forEach((entry, index) => {
      const tableKey = keys[index]!;
      if (entry.error !== undefined) {
        findings.push({
          severity: "error",
          code: "unknownTarget",
          message: `${entry.path} could not be read: ${entry.error.message}`,
          path: "",
        });
        return;
      }
      const parse = this.parseFindings(entry.text!, entry.path, TableConfigDocumentSchema);
      if (parse.length > 0) {
        findings.push(...parse);
        return;
      }
      tables[tableKey] = TableConfigDocumentSchema.parse(
        JSON.parse(entry.text!),
      ) as TableConfigDocument;
    });
    return { tables, findings };
  }

  /** Reads several files at once, turning a rejection into a value. */
  private readAll(paths: string[]): Promise<ReadResult[]> {
    return Promise.all(
      paths.map(async (path): Promise<ReadResult> => {
        try {
          return { path, text: await this.readText(path) };
        } catch (error) {
          return { path, error: error as Error };
        }
      }),
    );
  }

  /**
   * Writes the three documents, and answers what the server's save check found
   * in each without refusing it.
   *
   * **Not atomic, and it cannot be**: the save endpoint takes one file. A flow
   * whose actions fail to write is a flow referencing actions that are not
   * there, which is why the *flow* is written last — a stale flow with new
   * actions still runs, and the reverse does not.
   *
   * **A save with warnings is a save** (`WorkspaceApi.saveFile`): every document
   * is written, and the warnings come back for the screen that asked to show,
   * as `describeFlowSaveWarnings` words them. A refused document still throws,
   * and the ones after it are not written, as before.
   */
  async save(loaded: LoadedFlow): Promise<FlowSaveResult> {
    const { workspaceName } = this.options;
    const documents: [string, unknown][] = [
      [actionPath(loaded.key), loaded.actions],
      [formPath(loaded.key), loaded.forms],
      [flowPath(loaded.key), loaded.flow],
    ];
    const files: SavedDocument[] = [];
    for (const [path, document] of documents) {
      const { warnings } = await this.api.saveFile(workspaceName, path, serialise(document));
      files.push({ path, warnings });
    }
    return { files };
  }

  /**
   * Reads one workspace file by path.
   *
   * **This went through `readFile` and could not have worked against the real
   * API**. `readFile` takes a tree node and asks `fileNameOf` for the
   * path, which returns null unless `node.type === "file"`; the caller here
   * synthesised `{ label, key } as never`, which has no `type`, so the real
   * method would have thrown *"…is not a file"* on every load. The store's tests
   * stub `WorkspaceApi` with a map keyed on `node.key`, so the stub accepted what
   * the implementation would refuse — and the `as never` was the cast that made
   * the type checker agree.
   */
  private async readText(path: string): Promise<string> {
    // `readWorkspaceDocument`, not `readWorkspaceFile`: this is the running app
    // reading a document, and the difference is the capability. See the method.
    const file = await this.api.readWorkspaceDocument(this.options.workspaceName, path);
    return file.content;
  }

  private parseFindings(
    text: string,
    path: string,
    schema: { safeParse(v: unknown): { success: boolean; error?: unknown } },
  ): Finding[] {
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch (error) {
      return [
        {
          severity: "error",
          code: "unknownTarget",
          message: `${path} is not valid JSON: ${(error as Error).message}`,
          path: "",
        },
      ];
    }
    const result = schema.safeParse(parsed);
    if (result.success) return [];
    return findingsFromParse(result.error as { issues: ParseIssue[] });
  }
}

/** One document `FlowStore.save` wrote, and the save check's warnings about it. */
export interface SavedDocument {
  path: string;
  warnings: SaveFinding[];
}

/** What `FlowStore.save` answers: each document written, in the order written. */
export interface FlowSaveResult {
  files: SavedDocument[];
}

/**
 * The text a flow save's warnings are shown with, or null when there are none:
 * `describeSaveWarnings`'s, once per document that has any, so each warning is
 * said in the form the Workspace IDE says it and under the file it is in.
 */
export function describeFlowSaveWarnings(result: FlowSaveResult): string | null {
  const parts = result.files
    .filter((file) => file.warnings.length > 0)
    .map((file) => describeSaveWarnings(file.path, file.warnings));
  return parts.length > 0 ? parts.join("\n") : null;
}

/** Two spaces and a trailing newline, matching every fixture this port emits. */
export const serialise = (document: unknown): string => `${JSON.stringify(document, null, 2)}\n`;

/**
 * Every escape name a *form* document references.
 *
 * The third document to name one, after the flow (`formStateInitializer`) and
 * the action document (`escape` steps). A form names a `validator` and, when it
 * repeats, a `rowInitializers` entry for its rows — and both must be refused at
 * load rather than at the moment a user presses Save, which is the whole reason
 * `resolveEscapes` runs where it does.
 */
export function formEscapeReferences(forms: FormDocument, key = ""): EscapeReferences[] {
  const references: EscapeReferences[] = [];
  for (const [formKey, form] of Object.entries(forms.forms)) {
    if (form.validator !== undefined) {
      references.push({
        kind: "validators",
        name: form.validator,
        at: `${formPath(key)}/forms/${formKey}/validator`,
      });
    }
    if (form.repeat !== undefined) {
      references.push({
        kind: "rowInitializers",
        name: form.repeat.seed,
        at: `${formPath(key)}/forms/${formKey}/repeat/seed`,
      });
    }
    // **A field's `isReadOnlyFrom`.** Third kind a form can name, and
    // it is here for the reason the two above are: an unresolved name must refuse
    // the set at load, not leave a field that silently stops being protected. The
    // walk is over `fieldsOf` rather than `valueFieldsOf` because a `button` field
    // is in neither union arm that carries this and the narrowing costs nothing.
    fieldsOf(form).forEach((field, index) => {
      if ("isReadOnlyFrom" in field && field.isReadOnlyFrom !== undefined) {
        references.push({
          kind: "predicates",
          name: field.isReadOnlyFrom,
          at: `${formPath(key)}/forms/${formKey}/fields/${index}/isReadOnlyFrom`,
        });
      }
    });
  }
  return references;
}

/** Every escape name the pair references, with where it was found. */
export function escapeReferences(flow: UserFlow, actions: ActionDocument): EscapeReferences[] {
  const references: EscapeReferences[] = [];
  if (flow.formStateInitializer !== undefined) {
    references.push({
      kind: "initializers",
      name: flow.formStateInitializer,
      at: "/formStateInitializer",
    });
  }
  for (const [name, action] of Object.entries(actions.actions)) {
    action.steps.forEach((step, index) => {
      if (step.do === "escape") {
        references.push({ kind: "actions", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
      // **A `query` step's name resolves out of the build too.**
      // `schema.ts` says so — *"`name` is a registered query, not SQL"* — and
      // nothing enforced it, so a document naming a query no build has would load
      // cleanly and fail at the press of a button. That is precisely the failure
      // `escapes.ts` refuses for the other six namespaces, and it was live: the
      // coverage document's `into` named two columns the statement does not
      // return, which a load-time check on the *name* would not have caught but
      // which is the same class of gap one field over.
      if (step.do === "query") {
        references.push({ kind: "queries", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
    });
  }
  return references;
}

/** The stages of `checkFlowSet`, in the order `load` throws them. */
export type FlowCheckStage = "references" | "set" | "tables" | "escapes";

/**
 * One finding of `checkFlowSet`.
 *
 * `document` and `path` locate it as a `SetFinding` does: `path` is a JSON
 * Pointer into that document, and for `tables` its first segment is the table
 * key, standing in for the file.
 */
export interface FlowCheckFinding {
  stage: FlowCheckStage;
  severity: Severity;
  /** `validate.ts`'s code on a `references` finding; the set's code otherwise, or `unresolvedEscape`. */
  code: Finding["code"] | SetFindingCode | "unresolvedEscape";
  document: SetFinding["document"];
  path: string;
  message: string;
  /** On an `escapes` finding, the reference as `describeUnresolved` prints it. */
  unresolved?: UnresolvedEscape;
}

/**
 * Every check a parsed flow, its siblings and its tables must pass to load.
 *
 * **One function for `FlowStore.load` and for the workspace validator**
 * (`validateWorkspace.ts`). The validator used to restate `load`'s sequence check
 * for check, with a comment asking that a check added to one be added to the
 * other. Now each caller does only what differs between them: `load` reads the
 * files and throws the first stage that fails, keeping errors only; the
 * validator reads a directory, reports every stage with its file, and reports
 * the parse and missing-file failures `load` meets as read errors.
 *
 * The stages, in `load`'s order, each resting on the one before:
 *
 * 1. `references`: `validateFlow`, warnings included;
 * 2. `set`: `validateDocumentSet`;
 * 3. `tables`: `validateTableActions` over the tables given, and
 *    `validateAddClearsState`;
 * 4. `escapes`: every escape the flow, its actions, its forms and the given
 *    tables name is in the registry.
 *
 * Every stage runs, whatever the one before found; stopping is the caller's
 * choice. `tables` holds the tables that were read and parsed, which may be
 * fewer than the forms name.
 */
export function checkFlowSet(
  set: { flow: UserFlow; actions: ActionDocument; forms: FormDocument },
  tables: Record<string, TableConfigDocument>,
  options: { registry: EscapeRegistry; policy?: Policy },
  key = "",
): FlowCheckFinding[] {
  const { flow, actions, forms } = set;
  const findings: FlowCheckFinding[] = [];

  for (const f of validateFlow(flow, options.policy)) {
    findings.push({ stage: "references", ...f, document: "flow" });
  }
  for (const f of validateDocumentSet(set)) findings.push({ stage: "set", ...f });
  for (const f of validateTableActions(actions, forms, tables)) findings.push({ stage: "tables", ...f });
  for (const f of validateAddClearsState(set, tables)) findings.push({ stage: "tables", ...f });

  // Each reference keeps the `at` that `load`'s message has always printed, and
  // is also located in its own document for the validator.
  const references: { document: SetFinding["document"]; path: string; ref: EscapeReferences }[] = [
    ...escapeReferences(flow, actions).map((ref) => ({
      document: ref.at.startsWith("/actions/") ? ("actions" as const) : ("flow" as const),
      path: ref.at,
      ref,
    })),
    ...formEscapeReferences(forms, key).map((ref) => ({
      document: "forms" as const,
      path: ref.at.slice(formPath(key).length),
      ref,
    })),
    ...Object.entries(tables).flatMap(([tableKey, table]) =>
      tableEscapeReferences(table).map((ref) => ({
        document: "tables" as const,
        path: `/${tableKey}${ref.at}`,
        ref: { ...ref, at: `${tablePath(tableKey)}${ref.at}` },
      })),
    ),
  ];
  for (const { document, path, ref } of references) {
    const [unresolved] = resolveEscapes([ref], options.registry);
    if (unresolved === undefined) continue;
    findings.push({
      stage: "escapes",
      severity: "error",
      code: "unresolvedEscape",
      document,
      path,
      message: `no ${ref.kind === "queries" ? "registered query" : `${ref.kind} escape`} named "${ref.name}" in this build`,
      unresolved,
    });
  }
  return findings;
}

/** A `checkFlowSet` finding as `FlowLoadError` carries it. */
function asLoadFinding(f: FlowCheckFinding): Finding {
  if (f.stage === "references") {
    return { severity: f.severity, code: f.code as Finding["code"], message: f.message, path: f.path };
  }
  return { severity: "error", code: "unknownTarget", message: f.message, path: `${f.document}${f.path}` };
}

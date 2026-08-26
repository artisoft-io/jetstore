/**
 * Loading and saving user flows from a workspace. Task S.3.
 *
 * **No new plumbing, which is what §2 of the plan predicted.** The Workspace IDE
 * already reads and writes workspace files through `/dataTable`
 * (`api/workspace.ts`), the server already persists them through
 * `workspace_changes`, and it already refuses to save a `.json` file that will
 * not parse. S.3 is a store on top of that, not a transport.
 *
 * ## Where the files live, and why there
 *
 * `user_flows/<key>.uf.json` and `user_flows/<key>.ua.json`, one directory per
 * file type — the convention the workspaces already use: `pipes_config/` holds
 * `*.pc.json`, `jet_rules/` holds the rules, `lookups/` the lookup tables. A
 * flow's actions sit beside it rather than inside it for the reason S.2a chose
 * (`sizing_action_grammar.md` R5): one action is reachable from a state and from
 * a table's action bar, and a sibling document lets one entry serve both.
 *
 * **The key is the file name and nothing else.** S.1 settled that a flow carries
 * no self-key, because `FormConfig.key` had drifted from its registry key twice
 * in fifty (I-14). The same reasoning applies to the file: two names for one
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
 * ## Four documents, not two — tasks F.0a and I.3b
 *
 * S.3 loaded `.uf.json` and `.ua.json`. It did not load the `.form.json`, and
 * `FormDocumentSchema` had no consumer outside three test files, so a form could
 * be authored, saved, validated in Go and rejected properly when malformed —
 * and nothing in the client would ever open it (**I-51**). The `.tc.json` a
 * `dataTable` field names was in the same position after I.3a.
 *
 * Both are loaded here now, and **the set is validated as a set**
 * (`documentSet.ts`, `validateDocumentSet`, **I-57**). That check could not run
 * anywhere else: it needs three documents at once, and the Go save path sees one
 * file at a time — legitimately, since a `.uf.json` is saved before its
 * `.form.json` exists.
 *
 * **Table documents are keyed by table and not by flow**, which is why they live
 * in `table_configs/` rather than beside the flow: two flows may name the same
 * table. So the set of tables to fetch is not knowable from the flow document —
 * it is read off the *form* document, after it parses.
 */

import { describeUnresolved, resolveEscapes, type EscapeReferences, type EscapeRegistry } from "../actions/escapes";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import type { WorkspaceApi } from "../api/workspace";
import {
  TableConfigDocumentSchema,
  tableEscapeReferences,
  tablePath,
  type TableConfigDocument,
} from "../datatable/table";
import { fromDocument } from "../datatable/tableTranslate";
import type { TableConfig } from "../datatable/types";
import { validateDocumentSet, validateTableActions } from "./documentSet";
import { FormDocumentSchema, fieldsOf, type FormDocument } from "./form";
import { UserFlowSchema, type UserFlow } from "./schema";
import { validateFlow, type Finding, type Policy } from "./validate";

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
 * `fromDocument` was written for I.3a's round-trip proof — *the translation loses
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
function findingsFromParse(error: { issues: { path: PropertyKey[]; message: string }[] }): Finding[] {
  return error.issues.map((issue) => ({
    severity: "error" as const,
    code: "unknownTarget" as const,
    message: issue.message,
    // RFC 6901: "~" then "/", in that order, or the escaping is wrong.
    path: `/${issue.path.map((p) => String(p).replace(/~/g, "~0").replace(/\//g, "~1")).join("/")}`,
  }));
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
   * place the set of flows is written down, and S.1 spent a finding on what
   * happens when one name is recorded twice (I-14).
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
   * shape, then the tables' cross-document references (F.3), then whether every
   * escape name exists in this build. Each rests on the one before — a set check
   * over a document that did not parse would report missing forms that are merely
   * unreadable.
   */
  async load(key: string): Promise<LoadedFlow> {
    const reads = await this.readAll([flowPath(key), actionPath(key), formPath(key)]);
    const unreadable = reads.filter((r) => r.error !== undefined);
    if (unreadable.length > 0) {
      // **A missing document is a load error with the file's name in it**, not a
      // transport exception escaping the store. Before F.0a a flow was two files
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

    const reference = validateFlow(flow, this.options.policy);
    const errors = reference.filter((f) => f.severity === "error");
    if (errors.length > 0) throw new FlowLoadError(key, errors);

    // I-57. The set findings carry their own code union — deliberately not
    // `validate.ts`'s, because Go can never raise these — so they are mapped
    // rather than concatenated, and the document they name goes into the pointer
    // so a message spanning files still says which file.
    const setFindings = validateDocumentSet({ flow, actions, forms });
    if (setFindings.length > 0) {
      throw new FlowLoadError(
        key,
        setFindings.map((f) => ({
          severity: "error" as const,
          code: "unknownTarget" as const,
          message: f.message,
          path: `${f.document}${f.path}`,
        })),
      );
    }

    const tables = await this.loadTables(key, tableKeysOf(forms));

    // F.3, and the same layer one document further out: a table's `doAction`
    // names an entry in *this flow's* action document and its `showDialog`
    // names a form in *this flow's* form document, and neither reference had a
    // check (**I-88**). It runs here rather than inside `validateDocumentSet`
    // because the tables are not knowable until the form document has parsed —
    // which is the reason `loadTables` is below that call and not above it.
    const tableFindings = validateTableActions(actions, forms, tables);
    if (tableFindings.length > 0) {
      throw new FlowLoadError(
        key,
        tableFindings.map((f) => ({
          severity: "error" as const,
          code: "unknownTarget" as const,
          message: f.message,
          path: `${f.document}${f.path}`,
        })),
      );
    }

    const references = [
      ...escapeReferences(flow, actions),
      ...formEscapeReferences(forms, key),
      ...Object.entries(tables).flatMap(([tableKey, table]) =>
        tableEscapeReferences(table).map((ref) => ({
          ...ref,
          at: `${tablePath(tableKey)}${ref.at}`,
        })),
      ),
    ];
    const unresolved = resolveEscapes(references, this.options.registry);
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
   * `resolveEscapes` avoids for the same reason.
   */
  private async loadTables(
    key: string,
    keys: string[],
  ): Promise<Record<string, TableConfigDocument>> {
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
    if (findings.length > 0) throw new FlowLoadError(key, findings);
    return tables;
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
   * Writes both documents.
   *
   * **Not atomic, and it cannot be**: the save endpoint takes one file. A flow
   * whose actions fail to write is a flow referencing actions that are not
   * there, which is why the *flow* is written second — a stale flow with new
   * actions still runs, and the reverse does not.
   */
  async save(loaded: LoadedFlow): Promise<void> {
    const { workspaceName } = this.options;
    await this.api.saveFile(workspaceName, actionPath(loaded.key), serialise(loaded.actions));
    await this.api.saveFile(workspaceName, formPath(loaded.key), serialise(loaded.forms));
    await this.api.saveFile(workspaceName, flowPath(loaded.key), serialise(loaded.flow));
  }

  /**
   * Reads one workspace file by path.
   *
   * **This went through `readFile` and could not have worked against the real
   * API** (I-65). `readFile` takes a tree node and asks `fileNameOf` for the
   * path, which returns null unless `node.type === "file"`; the caller here
   * synthesised `{ label, key } as never`, which has no `type`, so the real
   * method would have thrown *"…is not a file"* on every load. The store's tests
   * stub `WorkspaceApi` with a map keyed on `node.key`, so the stub accepted what
   * the implementation would refuse — and the `as never` was the cast that made
   * the type checker agree.
   */
  private async readText(path: string): Promise<string> {
    const file = await this.api.readWorkspaceFile(this.options.workspaceName, path);
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
    return findingsFromParse(result.error as { issues: { path: PropertyKey[]; message: string }[] });
  }
}

/** Two spaces and a trailing newline, matching every fixture this port emits. */
export const serialise = (document: unknown): string => `${JSON.stringify(document, null, 2)}\n`;

/**
 * Every escape name a *form* document references. Task F.1.
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
    // **A field's `isReadOnlyFrom`. Task C.2b.** Third kind a form can name, and
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
      // **A `query` step's name resolves out of the build too. Task F.6.**
      // `schema.ts` says so — *"`name` is a registered query, not SQL"* — and
      // nothing enforced it, so a document naming a query no build has would load
      // cleanly and fail at the press of a button. That is precisely the failure
      // `escapes.ts` refuses for the other six namespaces, and it was live: the
      // coverage document's `into` named two columns the statement does not
      // return, which a load-time check on the *name* would not have caught but
      // which is the same class of gap one field over (I-113).
      if (step.do === "query") {
        references.push({ kind: "queries", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
    });
  }
  return references;
}

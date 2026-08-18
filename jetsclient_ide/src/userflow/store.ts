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
 */

import { describeUnresolved, resolveEscapes, type EscapeReferences, type EscapeRegistry } from "../actions/escapes";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import type { WorkspaceApi } from "../api/workspace";
import { UserFlowSchema, type UserFlow } from "./schema";
import { validateFlow, type Finding, type Policy } from "./validate";

/** The directory a workspace keeps its flows in. */
export const FLOW_DIR = "user_flows";

export const flowPath = (key: string): string => `${FLOW_DIR}/${key}.uf.json`;
export const actionPath = (key: string): string => `${FLOW_DIR}/${key}.ua.json`;

/** A flow and its actions, loaded together because neither runs without both. */
export interface LoadedFlow {
  key: string;
  flow: UserFlow;
  actions: ActionDocument;
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

  /** Reads, parses and validates. Throws `FlowLoadError` rather than returning half a flow. */
  async load(key: string): Promise<LoadedFlow> {
    const [flowText, actionText] = await Promise.all([
      this.readText(flowPath(key)),
      this.readText(actionPath(key)),
    ]);

    const findings = [
      ...this.parseFindings(flowText, flowPath(key), UserFlowSchema),
      ...this.parseFindings(actionText, actionPath(key), ActionDocumentSchema),
    ];
    if (findings.length > 0) throw new FlowLoadError(key, findings);

    const flow = UserFlowSchema.parse(JSON.parse(flowText)) as UserFlow;
    const actions = ActionDocumentSchema.parse(JSON.parse(actionText)) as ActionDocument;

    const reference = validateFlow(flow, this.options.policy);
    const errors = reference.filter((f) => f.severity === "error");
    if (errors.length > 0) throw new FlowLoadError(key, errors);

    const unresolved = resolveEscapes(escapeReferences(flow, actions), this.options.registry);
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

    return { key, flow, actions };
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
    await this.api.saveFile(workspaceName, flowPath(loaded.key), serialise(loaded.flow));
  }

  private async readText(path: string): Promise<string> {
    const file = await this.api.readFile(this.options.workspaceName, {
      label: path,
      key: path,
    } as never);
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
    });
  }
  return references;
}

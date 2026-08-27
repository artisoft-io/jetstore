/**
 * The `cpipesTemplateApply` escape: what a projected template flow does at its end.
 *
 * **Task M.5 of the agentic_ai project, living in this repository by agreement rather
 * than by drift.** The registry slot and the registration site belong to `ui_refresh`;
 * the body belongs to the project whose contract it assembles, because it changes when
 * that contract changes and neither side wants the other's pull request in the way.
 * That is the division `userflow_template_sharing_assessment.md` set for the schema,
 * pointed the other way — and it is the reason this file imports `actions/escapes` for
 * a type and nothing else of `jetsclient_ide`'s.
 *
 * ## What it does, and the one thing it deliberately does not
 *
 * A projected flow collects two kinds of value: the bindings a template is
 * parameterised by, and one fill per hole occurrence. This turns those back into a
 * `.pc.json`.
 *
 * **It does not expand the template.** Expansion is author-time and lives in
 * `tools/cpipes_contract`; there is no expander in Go and none in TypeScript, and
 * writing a second one here is exactly the drift M.4 refused when it made the
 * projection a *consumer* of `expand`'s traversal rather than a second traversal beside
 * it. Instead the generator emits a **skeleton** — the expanded config with a marker
 * wherever a collected value belongs — and this file substitutes into it. So the
 * wizard's output is the expander's output by construction. A second implementation
 * that merely agreed with the first on three templates would be a worse guarantee, and
 * it is the kind that stops agreeing quietly.
 *
 * ## Why the form keys need a plan to be read back
 *
 * `FormStateValue` is `string | string[]`, so a nested value is carried as leaves whose
 * keys spell the path back — `metric_columns.0.select.column` and so on. Two things
 * cannot be recovered from that spelling alone, and the plan carries both:
 *
 * * **Which keys choose a variant.** A chooser's key is spelled exactly like a field's,
 *   and a contract type may declare a property named `type` of its own. Guessing would
 *   be right until it silently was not.
 * * **Which values are lists.** A `json`-ruled field holds `["a", "b"]` as text, and
 *   nothing in the key says so.
 * * **Which properties the schema fixes.** A `const` is never a field — asking an author
 *   to retype what the contract already decided is how a form and a chooser come to
 *   disagree — so the config's value for it has to come from the plan.
 *
 * ## Failure is a message, not a throw
 *
 * An `ActionEscape` returns `string | null`, and a returned string stops the action with
 * that text in front of the author. Everything here that can fail is something an author
 * can act on — a missing plan, an unparseable list, a variant not chosen — so it is
 * reported that way rather than thrown, and the collected form state survives for them
 * to fix and retry.
 */

import type { ActionEscape, EscapeContext } from "../actions/escapes";
import { queryEscape, type WorkspaceApi } from "../api/workspace";

/** Where a workspace keeps the projected documents. Beside the flows they belong to. */
export const APPLY_DIR = "user_flows";

export const applyPlanPath = (flowKey: string): string => `${APPLY_DIR}/${flowKey}.apply.json`;

/** Where the assembled config lands. `pipes_config` is what JetStore loads. */
export const configPath = (template: string): string => `pipes_config/${template}.pc.json`;

/** One variant choice: the key that selects, and what it may select. */
export interface Chooser {
  property: string;
  literals: string[];
}

/** The generator's output. `tools/cpipes_contract/cpipes_contract/project.py` writes it. */
export interface ApplyPlan {
  schemaVersion: number;
  template: string;
  fillMarker: string;
  bindingMarker: string;
  choosers: Record<string, Chooser>;
  constants: Record<string, Record<string, unknown>>;
  jsonFields: string[];
  skeleton: unknown;
}

export interface TemplateApplyDeps {
  workspaceName: string;
  api: Pick<WorkspaceApi, "readWorkspaceDocument" | "saveFile">;
}

/** What form state a key holds, as a plain string. */
type Read = (key: string) => string | undefined;

/**
 * Assembles the object a fill collected, from the leaves whose keys start with `at`.
 *
 * Recursive on two rules and no others: a key in `choosers` is a variant, so its value
 * is `{[property]: literal}` plus whatever the chosen branch collected; anything else
 * nests on `.`. A `json` field is parsed. That is the whole grammar `project.py` emits,
 * and the two files are the two halves of it.
 */
export function assembleAt(
  at: string,
  plan: ApplyPlan,
  read: Read,
  keys: readonly string[],
): { value: unknown; problems: string[] } {
  const problems: string[] = [];
  const jsonFields = new Set(plan.jsonFields);

  const build = (prefix: string, root = false): unknown => {
    const chooser = plan.choosers[prefix];
    if (chooser !== undefined) {
      const literal = read(`${prefix}.${chooser.property}`);
      if (literal === undefined || literal === "") {
        problems.push(`${prefix}: no ${chooser.property} was chosen`);
        return undefined;
      }
      if (!chooser.literals.includes(literal)) {
        problems.push(`${prefix}: ${literal} is not one of ${chooser.literals.join(", ")}`);
        return undefined;
      }
      const chosen = build(`${prefix}.${literal}`, root);
      return { [chooser.property]: literal, ...(chosen as object | undefined) };
    }

    const out: Record<string, unknown> = {};
    const seen = new Set<string>();
    let authored = false;
    for (const key of keys) {
      if (!key.startsWith(`${prefix}.`)) continue;
      const rest = key.slice(prefix.length + 1);
      const head = rest.split(".")[0]!;
      if (seen.has(head)) continue;
      seen.add(head);
      const child = `${prefix}.${head}`;
      if (child === key) {
        // A leaf. **Empty is absent, not empty string** — a projected form offers every
        // declarable property (I-80) and a config that carried all 43 of
        // `InputChannelConfigStage`'s as `""` would be a different config from the one
        // the author meant, and would fail the contract rather than the wizard.
        const text = read(key);
        if (text === undefined || text === "") continue;
        authored = true;
        if (jsonFields.has(key)) {
          try {
            out[head] = JSON.parse(text);
          } catch {
            problems.push(`${key}: not a JSON list — ${text}`);
          }
          continue;
        }
        out[head] = text;
        continue;
      }
      const nested = build(child);
      if (nested !== undefined && Object.keys(nested as object).length > 0) {
        out[head] = nested;
        authored = true;
      }
    }
    // **Constants come last and only when something else is here.** An optional nested
    // object the author left alone must stay absent: emitting `{"type": "select"}` for
    // it would put a half-built operator into the config where the template meant
    // nothing at all. At the root the caller decides, because a fill is always emitted —
    // an empty one is a gap the contract should report, not one to paper over.
    const fixed = plan.constants[prefix] ?? {};
    if (!authored && !root) return out;
    return { ...fixed, ...out };
  };

  return { value: build(at, true), problems };
}

/**
 * Substitutes the collected values into the skeleton.
 *
 * Exported separately from the escape because it is the part worth testing: it is pure,
 * it is where every decision above lands, and a test of it is a test of the generator's
 * output as much as of this file.
 */
export function applyPlan(
  plan: ApplyPlan,
  read: Read,
  keys: readonly string[],
): { config: unknown; problems: string[] } {
  const problems: string[] = [];

  const substitute = (node: unknown): unknown => {
    if (Array.isArray(node)) return node.map(substitute);
    if (node !== null && typeof node === "object") {
      const entries = Object.entries(node as Record<string, unknown>);
      if (entries.length === 1 && entries[0]![0] === plan.fillMarker) {
        const at = String(entries[0]![1]);
        const { value, problems: found } = assembleAt(at, plan, read, keys);
        problems.push(...found);
        return value;
      }
      return Object.fromEntries(entries.map(([k, v]) => [k, substitute(v)]));
    }
    if (typeof node === "string" && node.startsWith(plan.bindingMarker)) {
      const key = node.slice(plan.bindingMarker.length);
      const text = read(key);
      if (text === undefined) {
        problems.push(`${key}: the flow collected no value for this binding`);
        return node;
      }
      return plan.jsonFields.includes(key) ? safeParse(key, text, problems) : text;
    }
    return node;
  };

  return { config: substitute(plan.skeleton), problems };
}

function safeParse(key: string, text: string, problems: string[]): unknown {
  try {
    return JSON.parse(text);
  } catch {
    problems.push(`${key}: not a JSON list — ${text}`);
    return text;
  }
}

/** Two spaces and a trailing newline, matching everything else this app writes. */
const serialise = (document: unknown): string => `${JSON.stringify(document, null, 2)}\n`;

/**
 * The escape itself, behind the dependencies `EscapeContext` deliberately does not carry.
 *
 * `EscapeContext` is `{formState, group, flowKey}` and nothing else — narrow on purpose,
 * per `escapes.ts` — so an escape that needs the workspace has to be closed over it. That
 * is why this is a factory rather than the function itself: `ui_refresh` registers
 * `cpipesTemplateApply: createCpipesTemplateApply({workspaceName, api})`, and the name in
 * the registry is the name the projected `.ua.json` escapes to.
 */
export function createCpipesTemplateApply(deps: TemplateApplyDeps): ActionEscape {
  return async ({ formState, group, flowKey }: EscapeContext): Promise<string | null> => {
    let plan: ApplyPlan;
    try {
      // **`readWorkspaceFile` rather than `readFile`, and the difference is not
      // cosmetic** (I-147, 2026-08-25). `readFile` takes a tree node and asks
      // `fileNameOf` for the path, which returns null unless `node.type` is
      // `"file"`; this line used to synthesise `{label, key} as never` — a shape
      // with no `type` — so the escape would have thrown *"…is not a file"*
      // against the real `WorkspaceApi` every time. Nothing said so because the
      // test beside this file stubs `readFile` and its stub matches on `label`.
      //
      // **The same defect, in the same shape, had already been found and fixed
      // one directory over**: `FlowStore.readText` carried it, `ui_refresh` found
      // it as their I-65 and added `readWorkspaceFile` for exactly this caller.
      // The fix did not reach here because it was made in the caller rather than
      // by removing the shape, and a second copy of a defect is invisible to the
      // party who fixed the first.
      //
      // **`readWorkspaceDocument` since 2026-08-26**, changed here by a
      // `ui_refresh` session rather than handed over, on the convention that the
      // editor fixes the call site: `readWorkspaceFile` gates on `workspace_ide`,
      // so leaving this line would have left a projected flow openable by an
      // `ops_user` and unable to apply. The `.apply.json` is one of the four
      // suffixes `documentPathOK` serves for exactly this call.
      const file = await deps.api.readWorkspaceDocument(deps.workspaceName, applyPlanPath(flowKey));
      plan = JSON.parse(file.content) as ApplyPlan;
    } catch (error) {
      return `Cannot read ${applyPlanPath(flowKey)}: ${(error as Error).message}`;
    }

    // **One snapshot rather than a lookup per key.** The assembly walks the collected
    // keys and reads most of them, and `snapshot` is what the app's own `wholeState`
    // request already uses — so this reads form state the way everything else does.
    const collected = formState.snapshot(group);
    const keys = Object.keys(collected);
    const read: Read = (key) => {
      const value = collected[key];
      if (value === undefined) return undefined;
      return Array.isArray(value) ? value[0] : value;
    };

    const { config, problems } = applyPlan(plan, read, keys);
    if (problems.length > 0) {
      return `This configuration is not complete:\n${problems.map((p) => `  ${p}`).join("\n")}`;
    }

    try {
      // `saveFile` takes the *escaped* path — the editor hands it a node's
      // `file_name`, which the server emitted escaped — and the server
      // url-unescapes it again (`SaveWorkspaceFileContent`,
      // `jets/datatable/workspace_data_table_action.go:963`). A raw path
      // survives that round trip today because no character in a template name
      // is touched by it; escaping is what keeps that a fact about the code
      // rather than about the three template names currently shipped.
      await deps.api.saveFile(
        deps.workspaceName,
        queryEscape(configPath(plan.template)),
        serialise(config),
      );
    } catch (error) {
      return `Cannot save ${configPath(plan.template)}: ${(error as Error).message}`;
    }
    return null;
  };
}

/**
 * The deps the registered escape closes over, and why they are a module variable.
 *
 * **Task U.3, 2026-08-25.** `productionRegistry` is a constant — one build, one registry,
 * for the reason `registry.ts` gives: the *documents* are checked against it at load, so a
 * registry assembled per screen would make "does this flow load?" depend on which screen
 * asked. That rules out registering `createCpipesTemplateApply({workspaceName, api})`
 * directly, because neither dep exists at module scope: the workspace comes from
 * `get_workspace_uri` and the api client is `App.tsx`'s.
 *
 * So the value is stable and its dependencies arrive later, which is exactly the shape
 * `registry.ts` already uses for `fileKeyLabelRe` and `setFileKeyLabelPattern` — set from
 * `FlowRunner` in the same `await` that reads the active workspace. **Copying the file's
 * own idiom rather than inventing a second one** is what keeps the cross-project surface
 * to one import and one line.
 *
 * The alternative considered and rejected was widening `EscapeContext`, which is
 * `{formState, group, flowKey}` and narrow on purpose. An escape that needs the workspace
 * is not evidence that every escape does.
 */
let currentDeps: TemplateApplyDeps | null = null;

/** Supplies the workspace the escape writes into. `null` clears it. */
export function setCpipesWorkspace(deps: TemplateApplyDeps | null): void {
  currentDeps = deps;
}

/**
 * The escape as `productionRegistry.actions` holds it.
 *
 * **Unset deps are a message rather than a throw**, on this file's rule: an escape that
 * ran before the workspace was known would be a bug in the host, and the author reading
 * the message is the one person who can report it. It is not reachable from `FlowRunner`,
 * which sets the deps before `FlowStore.load` and therefore before any button exists to
 * press — but "not reachable today" is a property of one caller.
 */
export const cpipesTemplateApply: ActionEscape = async (context, host) => {
  if (currentDeps === null) {
    return "The active workspace is not known yet, so there is nowhere to write the configuration.";
  }
  return createCpipesTemplateApply(currentDeps)(context, host);
};

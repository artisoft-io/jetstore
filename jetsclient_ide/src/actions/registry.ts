/**
 * The escape registry this build actually ships. Tasks F.0a and I.3b.
 *
 * `escapes.ts` designed the mechanism and shipped exactly one **value** —
 * `emptyRegistry` — so until now every escape name in every authored document
 * resolved to nothing outside a test. That was I-50's fourth row: *no production
 * escape registry*.
 *
 * ## Why this is a module rather than a constant in the screen
 *
 * Two reasons, and the second is the one that will matter next.
 *
 * The registry is what the *documents* are checked against at load
 * (`userflow/store.ts`, `resolveEscapes`), so a screen holding its own would make
 * "does this flow load?" depend on which screen asked. There is one build, so
 * there is one registry.
 *
 * And **the registration site is ui_refresh's while some of the bodies are not**.
 * The repository `CLAUDE.md` records the arrangement agreed on 2026-08-23: the
 * agentic_ai project owns `src/cpipes/`, and this file owns the slot its escape
 * is registered into. Their domain logic should not need a pull request here, and
 * this wiring should not need one there. Nothing of theirs is registered yet —
 * `templateApply.ts` is reached by a projected flow, and no projected flow is in
 * a workspace this app reads.
 *
 * ## What is here, and what is deliberately not
 *
 * I-54's two bodies, plus F.1's two — a row seeder and a form validator, both in
 * `fileMapping.ts` because they are one flow's and belong together.
 *
 * **And `homeFilters.ts`'s six, as of F.5, which fills the last empty namespace.**
 * `actions` was `{}` through four migrated flows and this comment used to explain
 * why: *the remaining action escapes belong to flows track F has not migrated
 * (`homeFiltersUF` carries one)*. That flow is migrated, so the entry is here —
 * `updateHomeFilters`, which compiles the flow's answers into `WhereClause`
 * objects, and `clearHomeFilters`, which `actionDispatch` names for every
 * `clearHomeFilters` button. `seedFromHomeFilters` fills `initializers`, which
 * was likewise empty and is the only `formStateInitializer` in the corpus.
 *
 * **The `predicates` namespace gained two and one of them corrects an assumption.**
 * `hasDataRegistryFilters` was the *only* predicate because `tableTranslate.ts`
 * sent every closure to it — true of the 37 flow tables and false of all three on
 * `pipelineExecStatusTable`. See that file's header for the mapping.
 *
 * **And `queries`, F.6's seventh namespace, holds the corpus's one registered
 * statement** — `processInputRdfTypes`, in `queries.ts` because it is one flow's
 * and belongs beside nothing else. It is data rather than a body, which is the
 * one way it differs from the six above and is argued in `escapes.ts`.
 */

import type { FormState } from "../datatable/formState";
import {
  downloadMapping,
  loadRawRows,
  mappingFormValidator,
  seedMappingRow,
} from "./fileMapping";
import {
  alwaysEnabled,
  clearHomeFilters,
  hasHomeFilters,
  homeFiltersFormValidator,
  seedFromHomeFilters,
  updateHomeFilters,
} from "./homeFilters";
import { productionQueries } from "./queries";
import type { EscapeRegistry } from "./escapes";

/**
 * The server's `WORKSPACE_FILE_KEY_LABEL_RE`, once something fetches it.
 *
 * **Nothing does yet, and that is carried rather than fixed.** The Flutter app
 * reads it out of the `get_workspace_uri` response at sign-in
 * (`jetsclient/lib/modules/actions/user_delegates.dart:107`, `globalWorkspaceFileKeyLabelRe`)
 * and this app has no equivalent bootstrap. The fallback below is the same branch
 * the Dart takes when the variable is unset or the pattern does not compile, so
 * an unconfigured deployment is *identical* and a configured one loses the
 * client-specific label until the bootstrap exists. Recorded as I-67.
 */
let fileKeyLabelRe: RegExp | null = null;

/** Sets the pattern the `fileKeyLabel` filter uses. Null restores the fallback. */
export function setFileKeyLabelPattern(pattern: string | null): void {
  if (pattern === null || pattern === "") {
    fileKeyLabelRe = null;
    return;
  }
  try {
    fileKeyLabelRe = new RegExp(pattern);
  } catch {
    // The Dart does the same and keeps going: a bad pattern in an environment
    // variable must not stop the table rendering.
    fileKeyLabelRe = null;
  }
}

/**
 * Shortens a file key for display.
 *
 * A port of the closure at
 * `jetsclient/lib/modules/user_flows/start_pipeline/data_table_config.dart:86`,
 * which I-54 found written three times — the other two are
 * `modules/data_table_config_impl.dart:127` and `:379`. All three agree, which is
 * the argument for one name rather than three ports.
 *
 * **The ellipsis is part of the output, not decoration.** The Dart returns
 * `'...' + text.substring(start)` where `start` is the index *of* the last `/`,
 * so the slash is kept: `a/b/c.csv` renders as `.../c.csv`. A key with no `/` is
 * returned whole.
 */
export const fileKeyLabel = (value: string | null): string | null => {
  if (value === null) return null;
  if (fileKeyLabelRe !== null) {
    const match = fileKeyLabelRe.exec(value);
    // Group 1, as the Dart's `match[1]` does — the pattern is expected to capture
    // the label, and a pattern that matches with no group yields undefined there.
    if (match !== null && match[1] !== undefined) return match[1];
  }
  const start = value.lastIndexOf("/");
  return start >= 0 ? `...${value.slice(start)}` : value;
};

/**
 * Whether the data-registry filters are set.
 *
 * `JetsRouterDelegate().dataRegistryFilters != null && …isNotEmpty`, the gate on
 * the three `clearFilters` buttons (I-54). **In this app the answer is always
 * false, and the honest thing is to say so here rather than to leave the name
 * unregistered.**
 *
 * The filters are router state in the Flutter app —
 * `jets_router_delegate.dart:36` declares `List<WhereClause>? dataRegistryFilters`
 * — set by the data-registry screens, which are track C's and not ported. So the
 * button correctly renders disabled: there is nothing to clear. When C ports
 * those screens the filters acquire a home and this body reads it; until then a
 * registered `false` is a working button in its empty state, and an unregistered
 * name is a flow that will not load.
 */
export const hasDataRegistryFilters = (_formState: FormState, _group: number): boolean => false;

/**
 * The registry the application runs with.
 *
 * **Every namespace has an entry as of F.5**, and the last two to fill were the
 * two `escapes.ts` was designed around: `actions`, which stayed empty through four
 * migrated flows because the grammar kept swallowing what the coverage pass had
 * transcribed as escapes (I-74), and `initializers`, which has exactly one member
 * in the whole corpus.
 *
 * **`actions` filling is not evidence the grammar fell short.** F.1's finding was
 * that a transcription says what the grammar could express when it was written;
 * this one survived the same question — a step that builds `LIKE` patterns and
 * `now() - interval '…'` bounds for another screen's `WHERE` is not a missing
 * primitive. See `homeFilters.ts`.
 *
 * **`actions` has four as of F.8, and the two it gained answer that question two
 * different ways.** `downloadMapping` is an escape because no step can express a
 * two-table joined read whose thousand rows become a file the browser saves.
 * `loadRawRows` is an escape although the grammar *can* express it — a `set` and
 * a `post` — because S.7's allowlist refuses the target: `insert_raw_rows`
 * deletes before it authorises (I-121). **So the escape count is an upper bound
 * on what the grammar cannot say and not on what stays a body**, which is a
 * narrowing of I-74 rather than an instance of it.
 */
export const productionRegistry: EscapeRegistry = {
  actions: { updateHomeFilters, clearHomeFilters, downloadMapping, loadRawRows },
  initializers: { seedFromHomeFilters },
  rowInitializers: { seedMappingRow },
  validators: { mappingFormValidator, homeFiltersFormValidator },
  cellFilters: { fileKeyLabel },
  predicates: { hasDataRegistryFilters, hasHomeFilters, alwaysEnabled },
  queries: productionQueries,
};

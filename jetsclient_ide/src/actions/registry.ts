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
 * this wiring should not need one there.
 *
 * **`cpipesTemplateApply` is registered as of 2026-08-25 — agentic_ai's task U.3,
 * and the first exercise of that arrangement in this direction.** This paragraph
 * used to end *"Nothing of theirs is registered yet — `templateApply.ts` is reached
 * by a projected flow, and no projected flow is in a workspace this app reads"*.
 * Their U.2 made the second clause false: `cpipes-contract` now writes its
 * projected document sets into `jets/workspace_assets/user_flows/`, a third
 * `AssetGroup`, so `install_workspace_assets` puts them in every workspace.
 *
 * The escape is a stable value whose workspace arrives later, set from
 * `FlowRunner` beside `setFileKeyLabelPattern` — the same shape `fileKeyLabelRe`
 * below uses, and chosen because this registry is a constant on purpose.
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
import { cpipesTemplateApply } from "../cpipes/templateApply";
// C.5's two, and the first escape bodies this project owns that live outside
// this directory. The screen's domain module is beside its screen, on the same
// terms as `src/cpipes/` above: **the registration site is this file's and the
// body is the screen's.** A registry that imported a React component would be
// the alternative, and it is why `inferServer.ts` is split from the `.tsx`.
import { inferServerNotRunning, inferServerNotStopped } from "../screens/inferServer";
import { productionQueries } from "./queries";
import {
  readXlsxSheetOption,
  saveSourceConfigForFileType,
  sourceConfigFormValidator,
} from "./sourceConfig";
import { openWorkspace } from "./workspaceRegistry";
import { loadReteSession, seedInputRecordsRow } from "./processErrors";
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
 * Strips the prefix a recovered load error carries. Task C.6.
 *
 * `text?.replaceFirst('File contains 0 bad rows,recovered error: ', '')`
 * (`jetsclient/lib/modules/data_table_config_impl.dart`, the `error_message`
 * column of `DTKeys.inputLoaderStatusTable`) — one site, and the only
 * `cellFilter` in either corpus that is not `fileKeyLabel`'s body.
 *
 * **It is here because the translation named the wrong one for it.**
 * `translateColumn` mapped every `hasCellFilter` to `fileKeyLabel`, which is
 * right for five of the six sites and would have rendered a stack trace as a
 * file-key label on the sixth. See `tableTranslate.ts`'s `cellFilterEscapeFor`.
 *
 * `replaceFirst` rather than a global replace, and the comma with no space after
 * it is the message the loader actually writes — both are copied rather than
 * tidied, because a prefix that no longer matches is a filter that does nothing
 * and says nothing.
 */
export const errorMessageLabel = (value: string | null): string | null =>
  value === null ? null : value.replace("File contains 0 bad rows,recovered error: ", "");

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
 * The deployment's active workspace, once the screen that needs it has asked.
 *
 * The Flutter app keeps this in three top-level variables set from the
 * `get_workspace_uri` response at sign-in
 * (`jetsclient/lib/utils/constants.dart`, `globalWorkspaceUri`; written at
 * `jetsclient/lib/modules/actions/user_delegates.dart:107`). This app has no
 * sign-in bootstrap — the same gap `setFileKeyLabelPattern` above records — so
 * the screens that care fetch it themselves and set it here. `WorkspaceApi`'s
 * `activeWorkspace()` returns all three in one call, which is the same response
 * the Dart reads.
 *
 * **Empty is the honest default and it is also the permissive one, which is why
 * the predicates below are written the way they are.** A screen that has not
 * fetched yet must not report *this is the active workspace* about a row it knows
 * nothing about.
 */
let activeWorkspace: { name: string; branch: string; uri: string } = { name: "", branch: "", uri: "" };

/** Records the deployment's workspace. Task C.2b. */
export function setActiveWorkspace(next: { name: string; branch: string; uri: string }): void {
  activeWorkspace = next;
}

/**
 * Whether the row being edited is the workspace this apiserver is running.
 *
 * `addWorkspace`'s name and branch fields are read-only when it is
 * (`jetsclient/lib/modules/workspace_ide/form_config.dart`, the `isReadOnlyEval`
 * on `FSK.wsName` and `FSK.wsBranch`) — **both fields share one closure body**,
 * which is why one name serves two sites. Renaming the workspace the deployment
 * is pointed at would leave the server looking for a directory that no longer
 * exists, so this is a safety gate rather than a convenience.
 *
 * Both halves must match, and both must be non-empty: the Dart's `b != null && w
 * != null` guard means a form with nothing typed yet is editable, and an unset
 * `activeWorkspace` must not make every row look active.
 */
export const isActiveWorkspace = (formState: FormState, group: number): boolean => {
  if (activeWorkspace.name === "" || activeWorkspace.branch === "") return false;
  const name = scalar(formState.getValue(group, "workspace_name"));
  const branch = scalar(formState.getValue(group, "workspace_branch"));
  return name === activeWorkspace.name && branch === activeWorkspace.branch;
};

/**
 * Whether the deployment configures a workspace uri.
 *
 * `globalWorkspaceUri.isNotEmpty`, and **two fields share it** — `addWorkspace`'s
 * uri and `doGitStatusWorkspaceDialog`'s command. That the second is a *git
 * command* box gated on the same fact is not obvious and is the Dart's: when the
 * server has a uri of its own, the status command is fixed at `git status` and
 * the user may not substitute one.
 *
 * So the four `isReadOnlyEval` sites in either corpus are two bodies, which is
 * I-54's finding on a third surface.
 */
export const hasWorkspaceUri = (_formState: FormState, _group: number): boolean =>
  activeWorkspace.uri !== "";

/** A form-state value as a scalar; a selection arrives as a one-element array. */
const scalar = (value: unknown): string | null => {
  if (typeof value === "string") return value;
  if (Array.isArray(value) && typeof value[0] === "string") return value[0];
  return null;
};

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
 *
 * **Six as of F.7, and one of the two it adds is a *replacement* for a bigger one.**
 * The coverage fixture transcribed the whole of `scSelectSourceConfigUF` as
 * `loadSourceConfigWithFileTypeInference`; F.2's `when` guard expresses all but one
 * step of it, so what is registered is `readXlsxSheetOption` — a `JSON.parse` of a
 * value held in form state, which no value form can say. `saveSourceConfigForFileType`
 * stays whole for a third reason again: the payload projection `wholeState` offers
 * carries no guard. See `sourceConfig.ts`.
 */
export const productionRegistry: EscapeRegistry = {
  actions: {
    updateHomeFilters,
    clearHomeFilters,
    downloadMapping,
    loadRawRows,
    readXlsxSheetOption,
    saveSourceConfigForFileType,
    // agentic_ai's, per the arrangement in this file's header. The body is in
    // `src/cpipes/`; this line is the whole of the wiring (their U.3).
    cpipesTemplateApply,
    // **The first escape a non-flow screen registers.** C.2b's, and it is an
    // escape for the third reason the header lists rather than either of the
    // first two: the grammar could express what the Dart does, and its
    // destination screen does not exist yet. See `workspaceRegistry.ts`.
    openWorkspace,
    // **Nine as of C.9, and the count moved by one where the Dart had two arms.**
    // `/processErrors` has three live delegate arms; `reteSession.VisitEntity` is
    // a guarded `set` in its action document and the deleted v1 arm is nothing at
    // all, so only the rete-session load needs a body — it decodes a JSON document
    // two levels deep, which no step reaches into. See `processErrors.ts`.
    loadReteSession,
  },
  initializers: { seedFromHomeFilters },
  // **Two as of C.9, and the second is the first outside a flow.** F.1 built the
  // repeating form for `fmMappingFormUF`; `viewInputRecordsDialog` is the same
  // construct on a screen's dialog, needing nothing that was not already here —
  // which is the only evidence available that F.1's mechanism generalised.
  rowInitializers: { seedMappingRow, seedInputRecordsRow },
  validators: { mappingFormValidator, homeFiltersFormValidator, sourceConfigFormValidator },
  // **Two as of C.6, and the second is why the mapping is a lookup.** I-103 moved
  // the *`isEnabled`* mapping out of a constant and stated the lesson; the cell
  // filter beside it in the same function stayed a constant, sending every
  // `hasCellFilter` to `fileKeyLabel`. Five of the six sites agree and the sixth
  // is `inputLoaderStatusTable`'s `error_message`, which would have rendered a
  // load error as a file-key label. `cellFilterEscapeFor` is keyed by table and
  // column now (**I-219**).
  cellFilters: { fileKeyLabel, errorMessageLabel },
  // **Seven as of C.5, and they no longer divide by consumer.** Five arrived with
  // C.2b, two of them read by a *form field* rather than by a table action —
  // `isReadOnlyFrom` resolves out of this namespace because the signature is the
  // same `(formState, group) => boolean`, and a second namespace holding
  // functions of one type would be a distinction nothing draws. C.5 adds
  // `inferServerNotRunning` and `inferServerNotStopped`, **the first entries here
  // that no flow reaches**, and the first users of `FormActionSchema`'s
  // `enabledWhen`. The namespace was built for a table action's `isEnabled` and
  // takes a form action's `isEnabledEval` unchanged, which is the argument that
  // made naming the predicate cheaper than inventing an expression for it.
  predicates: {
    hasDataRegistryFilters,
    hasHomeFilters,
    alwaysEnabled,
    isActiveWorkspace,
    hasWorkspaceUri,
    inferServerNotRunning,
    inferServerNotStopped,
  },
  queries: productionQueries,
};

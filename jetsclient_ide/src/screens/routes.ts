/**
 * Which screens this app serves, for a table action that names one. Task C.6.
 *
 * ## The problem, which arrives with the first screen that links to another
 *
 * A table action of kind `showScreen` carries a `configScreenPath`, and that
 * string is a **Flutter route template** — `/executionStatusDetails/:session_id`,
 * `/domainTableViewer/:table_name/:session_id`. Every screen so far sent it to
 * `window.location.href = "/#" + path`, which is right when the other app is the
 * only one that serves it, and is what `FlowRunner` and `WorkspaceRegistry` do.
 *
 * **It stops being right the moment this app serves the same screen.** C.7 and
 * C.8 ported `/executionStatusDetails` and `/executionStatsDetails` and their
 * handoff records the consequence plainly — *the screens run, and nothing links
 * to them*, because the only thing that links to either is the home screen's
 * `pipelineExecStatusTable` and that screen was C.6's. Without this map, C.6
 * would ship a button that leaves the React app for a screen React had ported
 * hours earlier.
 *
 * ## It is the mirror of `userFlowRoutes`, deliberately
 *
 * The Flutter app has the same problem in the other direction and F.10 solved it
 * the same way: `userFlowRoutes` maps a route template to what React serves
 * (`jetsclient/lib/routes/migrated_user_flows.dart`, `userFlowRoutes`). This is
 * that table for *screens*, on this side. Each screen adds its own row when it
 * lands, which is the whole of registering it — the same seam shape as
 * `NON_FLOW_KEYS`.
 *
 * **Templates, not paths.** `requestFor` has already substituted the parameters
 * by the time a request reaches a screen, so the lookup cannot be on the filled
 * string — it would have to match `/executionStatusDetails/1724…` against a
 * template. The caller also holds the `ActionConfig`, so the template is
 * `action.configScreenPath` and nothing in `actionDispatch` changes. The React
 * path is refilled from the same `params`, which is what lets the two templates
 * name their parameters differently without either side knowing.
 *
 * **A row is a claim that this app serves the screen**, so every one below is
 * checked against `App.tsx` at the commit that adds it: C.7 and C.8's two, C.11
 * and C.12's two here, C.2b's `/workspaces`, C.4's `/queryTool` — the one row
 * whose React path is not the Flutter one, renamed by that task — and C.5's
 * `/inferServerAdmin`. A row for a screen this app does not serve is worse than a
 * missing row: the miss is a working link into Flutter, and the wrong row is a
 * 404 (C.16).
 *
 * ## What is *not* here, and why that is not an omission
 *
 * **Flows.** `/startPipelineUF` and `/configureHomeFiltersUF` are user flows, and
 * which app owns a flow is not a compiled list — it is whether the workspace
 * holds `user_flows/<key>.uf.json` (`userflow/routing.ts`, `appForFlow`). That is
 * the one fact this table must not duplicate, because a second place to update is
 * the failure `routing.ts`'s header exists to prevent. No workspace holds those
 * documents today (I-87), so both flows fall through to Flutter, which is correct
 * rather than a stub.
 *
 * ~~**Screens track C has not ported.**~~ **They landed, and the rows arrived three
 * tasks late — corrected 2026-08-25.** This paragraph read *"`/processErrors/:session_id`
 * is C.9's and `/ruleConfig` is C.10's; both fall through to Flutter until they
 * land"*, and C.13's `/userAdmin` was never mentioned because it did not exist when
 * this was written. All three are served now.
 *
 * **What kept it wrong is that the check could not see it.** `routes.test.ts`
 * asserted that every row names a route `App.tsx` serves — rows to routes — and the
 * missing direction is routes to rows, which has no symptom: `reactScreenPath`
 * returns `null` for an absent row, the test that asserts `null` keeps passing, and
 * the home screen quietly hands a user to Flutter for a screen this app had ported
 * hours earlier. **A one-directional check meeting the direction it cannot see**,
 * which is the shape Phase 3 has now recorded five times.
 *
 * The reverse check is in `routes.test.ts` as of this change, and it is *derived*
 * rather than listed: it reads the Flutter route table and this app's route table
 * and requires a row wherever both serve the same screen, with the exclusions named
 * and justified rather than assumed. A row is still added by the task that lands the
 * screen; what changed is that forgetting now fails.
 */

import { fillPath } from "../datatable/actionDispatch";

/** A Flutter route template this app serves, and the React path for it. */
interface ServedScreen {
  /** The path under the router's basename, with `:name` segments. */
  reactPath: string;
}

/**
 * The screens this app serves, by the Flutter template a document names.
 *
 * Keys are verbatim from `jetsclient/lib/routes/jets_routes_app.dart`. The React
 * paths are the same strings minus the leading slash that `basename` supplies,
 * which is a property worth keeping: a handoff is then a prefix change rather
 * than a translation, and a template that has to be *renamed* is a decision
 * somebody takes rather than a difference that accumulates.
 */
export const SERVED_SCREENS: Readonly<Record<string, ServedScreen>> = {
  "/executionStatusDetails/:session_id": { reactPath: "/executionStatusDetails/:session_id" },
  "/executionStatsDetails/:session_id": { reactPath: "/executionStatsDetails/:session_id" },
  "/domainTableViewer/:table_name/:session_id": {
    reactPath: "/domainTableViewer/:table_name/:session_id",
  },
  "/filePreviewPath/:file_key": { reactPath: "/filePreviewPath/:file_key" },
  "/workspaces": { reactPath: "/workspaces" },
  "/queryTool": { reactPath: "/query-tool" },
  "/inferServerAdmin": { reactPath: "/inferServerAdmin" },
  "/processErrors/:session_id": { reactPath: "/processErrors/:session_id" },
  "/userAdmin": { reactPath: "/userAdmin" },
  "/ruleConfig": { reactPath: "/ruleConfig" },
  /**
   * **The one key with no Flutter predecessor**, and the paragraph above is
   * corrected by it rather than around it: keys are verbatim from
   * `jets_routes_app.dart` for the ten screens that were routed there, and this
   * one names a screen that was a *tab* on the home screen and never had a route
   * at all. D.10 gives it one (**I-260**), so the string is chosen here rather
   * than inherited — which is why the template and the React path are trivially
   * the same and always will be.
   */
  "/fileLoaderStatus": { reactPath: "/fileLoaderStatus" },
};

/**
 * Where a `configScreenPath` should go, given the parameters the action resolved.
 *
 * Returns the React path to navigate to in-app, or `null` when this app does not
 * serve the screen — in which case the caller does the full page load into
 * Flutter that every screen has done until now.
 */
export function reactScreenPath(
  template: string | undefined,
  params: Record<string, string>,
): string | null {
  if (template === undefined) return null;
  const served = SERVED_SCREENS[template];
  return served === undefined ? null : fillPath(served.reactPath, params);
}

/**
 * What to say when a table action names a screen this app does not serve.
 *
 * **The destination for this case used to exist.** Every `navigate` arm did
 * `window.location.href = "/#" + path`, handing the user to the Flutter app,
 * which was right while there were two apps and is a navigation to nowhere now
 * that X.1 has retired the other one — the fragment is not sent to the server,
 * so it would land on this app's own root with a dead `#` and look like the
 * button did nothing much.
 *
 * Reporting is the convention this codebase already follows for the same shape
 * of problem: `escapes.ts` refuses an unresolved name rather than silently
 * skipping it, `FlowRunner` says which dialog kind it cannot open, and C.16's
 * `NotFound` reports an unmatched path instead of redirecting. A `configScreenPath`
 * naming an unserved screen is now a configuration error, and configuration
 * errors are worth a sentence rather than a redirect.
 */
export function unservedScreenMessage(label: string, path: string): string {
  return `"${label}" goes to ${path}, which this app does not serve`;
}

/**
 * Where a `configScreenPath` goes in this app, or null.
 *
 * **Takes the template, not the resolved path**, and that distinction cost a test
 * failure while X.1 was being written: `SERVED_SCREENS` and `FLOW_ROUTES` are both
 * keyed by the Flutter route *template*, while an `ActionRequest`'s `path` has the
 * parameters already substituted — so looking up `/executionStatusDetails/sess-1`
 * finds nothing and reports a screen the app serves as unserved. The resolved path
 * is for the message; the template is for the lookup.
 */
export function inAppPath(
  template: string | undefined,
  params: Record<string, string>,
): string | null {
  if (template === undefined || template === "") return null;
  return (
    reactScreenPath(template, params) ??
    reactFlowRoute(template, params) ??
    reactFlowEntry(template, params)
  );
}

/**
 * Where a flow goes when it ends. Task D.8, from **I-265**.
 *
 * ## What the port lost
 *
 * A Flutter flow that set no `exitScreenPath` **popped the navigator** — it
 * returned to whatever screen had pushed it. `FlowRunner`'s `exit` replaced the
 * pop with a constant, `navigate("/workspace")`, and that was defensible when
 * F.0a wrote it: `/` redirected to the editor then, so `/workspace` *was* this
 * app's index. **X.1 moved the index to `/home` and nothing revisited the flow
 * exit**, so every flow with no authored destination has landed on the Workspace
 * IDE since — a `workspace_ide` screen, for a population that mostly holds
 * `jetstore_read` and nothing else.
 *
 * ## The origin travels in the url
 *
 * `returnTo` is a query parameter on `/flow/:key`, written by whoever navigates
 * to the flow. Three candidates were weighed and this is why it is not the other
 * two:
 *
 * - **`navigate(-1)`, the literal pop.** It is what the Dart did and it cannot
 *   be checked: a test can assert that history went back one, not that the user
 *   is on the screen that launched the flow. It is also wrong in a case that
 *   already exists — a flow's table action navigates out to
 *   `/filePreviewPath/:file_key` and the user comes back with the browser's Back
 *   button, after which the previous entry is the file preview rather than the
 *   origin. And it has no answer at all when the flow was opened in a fresh tab,
 *   which is the case that has to degrade.
 * - **A module-level store written by the shell.** No launch site would need
 *   changing, and it answers a subtly different question: *where was the user
 *   last*, not *what invoked this flow*. Those diverge for the same reason
 *   `navigate(-1)` does, and the divergence is silent.
 *
 * A parameter is written by the party that knows the answer, is visible in the
 * url, survives a reload, and a test asserts the destination rather than a
 * history offset. The cost is that every launch site has to pass it, which is
 * `withReturnTo` below and is one line each.
 *
 * **It is stripped before the flow's parameters are seeded.** `FlowRunner` writes
 * every query parameter into form-state group 0 by name; this one is the
 * runner's own and no form declares it, so it is excluded there rather than left
 * to be harmless.
 */
export const RETURN_TO = "returnTo";

/**
 * Whether a string may be navigated to inside this app.
 *
 * **The whole of the check is that it is a path and not a url.** `returnTo`
 * arrives from the query string, so it arrives from anyone who can get a user to
 * click a link — and `navigate()` will happily take `//evil.example.com`, which
 * a browser reads as protocol-relative and follows off-site. A single leading
 * slash, and no backslash anywhere, is what separates the two.
 */
export function isInAppPath(path: string): boolean {
  return (
    path.startsWith("/") &&
    !path.startsWith("//") &&
    !path.includes("\\") &&
    // eslint-disable-next-line no-control-regex
    !/[\u0000-\u001f\u007f]/.test(path)
  );
}

/**
 * A flow path carrying where to return to, or the path unchanged.
 *
 * **Only flow paths get one**, because only the flow runner reads it: the same
 * `inAppPath` result may be an ordinary screen, and a `returnTo` on one of those
 * would be a parameter nothing consumes. An existing `returnTo` is left alone so
 * that a launcher which has already decided beats this default.
 */
export function withReturnTo(to: string, from: string): string {
  if (!to.startsWith("/flow/") || !isInAppPath(from)) return to;
  const [path, query = ""] = to.split("?", 2);
  const search = new URLSearchParams(query);
  if (search.has(RETURN_TO)) return to;
  search.set(RETURN_TO, from);
  return `${path}?${search.toString()}`;
}

/** The origin a flow url carries, or null when it carries none this app may follow. */
export function returnToPath(search: URLSearchParams): string | null {
  const value = search.get(RETURN_TO);
  if (value === null || value === "" || !isInAppPath(value)) return null;
  return value;
}

/**
 * Where a flow goes when neither its document nor its url says.
 *
 * **`/home` rather than `/workspace`, and rather than nothing.** It is the app's
 * index since X.1, it is the one screen no capability gates, and it is the
 * destination the reporter offered as the acceptable second best. The case is
 * real rather than theoretical: a bookmarked flow, a flow opened in a new tab,
 * and a flow reached from the Flutter-era route templates before F.10's map was
 * asked for one.
 */
export const FLOW_EXIT_FALLBACK = "/home";

/**
 * The legacy route template of each flow, and what it carries.
 *
 * **A port of `userFlowRoutes` out of `jetsclient/lib/routes/migrated_user_flows.dart`,
 * done at X.1 because that file was about to be deleted with the app.** It is the
 * one piece of knowledge in there that outlives the Flutter client: a table
 * document's `configScreenPath` still names a flow by its *Flutter route*, and
 * those documents are workspace configuration rather than code — `workspaceRegistryTable`'s
 * *Load Client Config* and *Pull Workspace* buttons name two of these.
 *
 * **Four of the eleven have a route that disagrees with their key**, which is the
 * whole reason this is a map rather than a string operation: `mapFileUF`'s leading
 * segment is `fileMappingUF`, `homeFiltersUF`'s is `configureHomeFiltersUF`,
 * `workspacePullUF`'s is `pullWorkspaceUF`, and `loadConfigUF`'s is `workspaces` —
 * which is the workspace registry's own route. That was the ui refresh project's
 * I-75, found when a key derived from the first path segment would have handed the
 * registry screen to a flow.
 *
 * The parameters travel as a query string because the React route is `/flow/:key`
 * with nothing positional after it; `FlowRunner` seeds every query parameter into
 * form-state group 0 by name before the flow loads.
 */
export const FLOW_ROUTES: Readonly<Record<string, { flowKey: string; parameters: string[] }>> = {
  "/clientRegistryUF/:startAtKey": { flowKey: "clientRegistryUF", parameters: ["startAtKey"] },
  "/configureHomeFiltersUF": { flowKey: "homeFiltersUF", parameters: [] },
  "/fileMappingUF": { flowKey: "fileMappingUF", parameters: [] },
  "/fileMappingUF/mapping/:table_name/:object_type": { flowKey: "mapFileUF", parameters: ["table_name", "object_type"] },
  "/loadFilesUF": { flowKey: "loadFilesUF", parameters: [] },
  "/pipelineConfigUF": { flowKey: "pipelineConfigUF", parameters: [] },
  "/pullWorkspaceUF/:key/:workspace_name/:workspace_branch/:feature_branch/:workspace_uri": { flowKey: "workspacePullUF", parameters: ["key", "workspace_name", "workspace_branch", "feature_branch", "workspace_uri"] },
  "/registerFileKeyUF": { flowKey: "registerFileKeyUF", parameters: [] },
  "/sourceConfigUF/:startAtKey": { flowKey: "sourceConfigUF", parameters: ["startAtKey"] },
  "/startPipelineUF": { flowKey: "startPipelineUF", parameters: [] },
  "/workspaces/loadConfigUF/:workspace_name": { flowKey: "loadConfigUF", parameters: ["workspace_name"] },
};

/**
 * Where a flow's legacy route goes in this app, or null when the template names
 * no flow or an argument is missing.
 *
 * **Missing arguments fall through to null rather than opening the flow anyway**,
 * which is F.10's decision kept: `mapFileUF` without `table_name` has no rows to
 * draw and `loadConfigUF` without `workspace_name` has no workspace, so the runner
 * would render an empty worksheet and say nothing about why. In the Flutter app
 * the fallback was the old screen; here it is a reported error, which is worse for
 * the user and better than a blank page they cannot explain.
 */
export function reactFlowRoute(
  template: string,
  params: Record<string, string>,
): string | null {
  const route = FLOW_ROUTES[template];
  if (route === undefined) return null;
  const query = new URLSearchParams();
  for (const name of route.parameters) {
    const value = params[name];
    if (value === undefined || value === "") return null;
    query.set(name, value);
  }
  const suffix = query.toString();
  return `/flow/${route.flowKey}${suffix === "" ? "" : `?${suffix}`}`;
}


/**
 * Where a flow begins, when a button says somewhere other than its first state.
 * Task D.10, from **I-260**.
 *
 * ## What the report asks for
 *
 * *"Load Data starts `LoadFilesUF` skipping the first step, taking the selected
 * data source."* `loadFilesUF` has two states — `select_source_config` and
 * `select_file_keys` — and the source has already been chosen on the screen the
 * button is on, so the flow should open on its second page with that choice
 * already in form state.
 *
 * **Half of that already worked.** `FlowRunner` seeds every query parameter into
 * form-state group 0 before the load, which is how five flows carry their
 * arguments, so `?client=…&org=…&object_type=…&table_name=…` is the whole of
 * *taking the selected data source*. What had no mechanism at all is *skipping
 * the first step*: `startAt(flow)` reads `startAtKey` off the document, and a
 * document has one start.
 *
 * ## Why a second map rather than a row in `FLOW_ROUTES`
 *
 * `FLOW_ROUTES` is the port of `userFlowRoutes` and its rows are **the eleven
 * Flutter route templates** — `routes.test.ts` asserts the count for that reason.
 * A row here is not one of those: Flutter had no way to enter a flow partway, so
 * `/loadFilesUF/select_file_keys` is a template this app invents. Putting it in
 * the legacy map would make that map's own header false, and the header is the
 * thing that tells the next reader they may not add to it freely.
 *
 * **The parameters are declared rather than passed through, for F.10's reason one
 * step further on.** The obvious cheaper design is to let `reactFlowRoute` append
 * whatever `resolveParams` resolved; that would seed *every* action's
 * `navigationParams` into the flow's form state, silently, on the four existing
 * flow routes that carry them. An entry point says what it carries, refuses when
 * a name is missing, and carries nothing else.
 */
export interface FlowEntryPoint {
  /** The flow to open. */
  flowKey: string;
  /** The state to open it on, which the document must declare. */
  startAt: string;
  /** Names that must all resolve, or the button reports rather than opening. */
  parameters: string[];
}

/**
 * The parameter naming the state a flow opens on.
 *
 * **The runner's own, like `RETURN_TO`**, and excluded from the form-state seed
 * in the same `continue`: no form declares it, and seeding it would put a state
 * key into form state where a value belongs.
 */
export const START_AT = "startAt";

/**
 * A flow path opening on a named state, or the path unchanged.
 *
 * The shape is `withReturnTo`'s and so is the reasoning: only a `/flow/` path
 * gets one, because only the runner reads it, and an entry a caller has already
 * chosen is left alone.
 */
export function withStartAt(to: string, stateKey: string): string {
  if (!to.startsWith("/flow/") || stateKey === "") return to;
  const [path, query = ""] = to.split("?", 2);
  const search = new URLSearchParams(query);
  if (search.has(START_AT)) return to;
  search.set(START_AT, stateKey);
  return `${path}?${search.toString()}`;
}

/** The state a flow url asks to open on, or null. */
export function startAtState(search: URLSearchParams): string | null {
  const value = search.get(START_AT);
  return value === null || value === "" ? null : value;
}

/**
 * The entry points, by the `configScreenPath` a table document names.
 *
 * One row, and the shape of it is the point: the template reads as
 * *flow route, then state*, so a second entry point is a line rather than a
 * mechanism. `fmInputSourceMappingUF`'s *Load Data* button is the only site
 * today (`jets/workspace_assets/table_configs/fmInputSourceMappingUF.tc.json`).
 *
 * **The four parameters are `lfLoadFilesUF`'s, not the table's.** They are what
 * the second state needs and nothing more: `lfFileKeyStagingTable` filters on
 * `table_name` alone (`where`, `formStateKey`), and the action posts `client`,
 * `org`, `object_type` and `table_name` as `fromKey`
 * (`jets/workspace_assets/user_flows/loadFilesUF.ua.json`). Reading the list off
 * what consumes it rather than off what the first state's `formStateBinding`
 * happens to publish is what keeps the two from drifting silently — they agree
 * today, and only one of them is a requirement.
 */
export const FLOW_ENTRY_POINTS: Readonly<Record<string, FlowEntryPoint>> = {
  "/loadFilesUF/select_file_keys": {
    flowKey: "loadFilesUF",
    startAt: "select_file_keys",
    parameters: ["client", "org", "object_type", "table_name"],
  },
};

/**
 * Where an entry-point template goes in this app, or null.
 *
 * Null for a template that is not one, and null for one whose arguments are
 * incomplete — F.10's decision, and it bites harder here: a `loadFilesUF` opened
 * on its second page with no `table_name` would draw an empty file list and say
 * nothing, having skipped the page that would have set it.
 */
export function reactFlowEntry(
  template: string,
  params: Record<string, string>,
): string | null {
  const entry = FLOW_ENTRY_POINTS[template];
  if (entry === undefined) return null;
  const query = new URLSearchParams();
  for (const name of entry.parameters) {
    const value = params[name];
    if (value === undefined || value === "") return null;
    query.set(name, value);
  }
  return withStartAt(`/flow/${entry.flowKey}?${query.toString()}`, entry.startAt);
}

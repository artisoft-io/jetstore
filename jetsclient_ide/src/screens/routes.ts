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
  return reactScreenPath(template, params) ?? reactFlowRoute(template, params);
}

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

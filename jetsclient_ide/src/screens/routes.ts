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
 * **Screens track C has not ported.** `/processErrors/:session_id` is C.9's and
 * `/ruleConfig` is C.10's; both fall through to Flutter until they land. A miss
 * here is the honest answer, not a broken link.
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

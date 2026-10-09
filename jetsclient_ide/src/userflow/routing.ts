/**
 * Which app serves which flow, while both exist. Task S.8.
 *
 * ## The finding that decides this, and it was not what the plan assumed
 *
 * §2.6 anticipated deciding "which app serves which flow, per route", implying a
 * dispatch somewhere on the request path. **There is nothing to dispatch: the two
 * apps' URL spaces cannot collide.**
 *
 * `jetsclient` never calls `setPathUrlStrategy`. `url_strategy` is a dependency
 * at `pubspec.yaml:47` and nothing imports it, so Flutter web keeps its default
 * — **hash routing**. Every Flutter route lives after the `#`:
 *
 *     /#/loadFilesUF            the Flutter flow
 *     /ide/flow/loadFilesUF     the React flow
 *
 * A fragment is never sent to the server, which is why `server.go` has no
 * catch-all and needs none: the only path Flutter ever requests is `/`. So a
 * per-route decision at the apiserver is not merely unnecessary, it is
 * impossible for Flutter's routes — the server cannot see them.
 *
 * **A half-migrated corpus is therefore already a supported state at the routing
 * level.** What was missing is not dispatch but a *handoff*: something that sends
 * a user who asked Flutter for a migrated flow to the app that now owns it.
 *
 * ## The workspace is the switch
 *
 * A flow belongs to React **iff its documents exist in the workspace** —
 * `user_flows/<key>.uf.json` and its siblings. Not an environment variable, not
 * a list compiled into either app.
 *
 * That matters more than it sounds. Migrating a flow is already an act of
 * authoring: someone writes the documents and saves them, which S.3 stores and
 * S.4 validates. Making the routing follow the same fact means there is no
 * second place to update and no window in which the two disagree — the failure
 * that a compiled list produces is a flow that is migrated but still opens the
 * Flutter version, or worse the reverse.
 *
 * It also means migration is reversible: delete the documents and the flow is
 * Flutter's again. A cutover has no such property, which is the whole reason
 * §2.6 asked for this.
 */

/** The two apps, as a value rather than a boolean, so a third is expressible. */
export type FlowApp = "react" | "flutter";

/** Where React serves a flow. Relative to the router's basename — decision 4. */
export const reactFlowPath = (flowKey: string): string => `/flow/${flowKey}`;

/** Where Flutter serves one. The `#` is the whole point; see the header. */
export const flutterFlowPath = (flowKey: string): string => `/#/${flowKey}`;

/**
 * Which app owns a flow, given what the workspace holds.
 *
 * `migrated` is the set `FlowStore.list()` returns — the flow keys with a
 * `.uf.json` in the workspace.
 */
export function appForFlow(flowKey: string, migrated: ReadonlySet<string>): FlowApp {
  return migrated.has(flowKey) ? "react" : "flutter";
}

/**
 * The absolute URL to send a browser to, or null when the current app keeps it.
 *
 * Absolute rather than router-relative because a handoff crosses apps: React's
 * router cannot navigate into Flutter and Flutter's cannot navigate into React.
 * Both hand-offs are a full page load, which is correct — the two bundles share
 * an origin and a token but not a heap.
 */
export function handoffFor(
  flowKey: string,
  from: FlowApp,
  migrated: ReadonlySet<string>,
  ideBase = "/ide",
): string | null {
  const owner = appForFlow(flowKey, migrated);
  if (owner === from) return null;
  return owner === "react" ? `${ideBase}${reactFlowPath(flowKey)}` : flutterFlowPath(flowKey);
}

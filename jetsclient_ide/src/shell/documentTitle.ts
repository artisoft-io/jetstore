/**
 * What the browser tab is called. Task D.10, from **I-272**.
 *
 * ## The report, and why restoring the old string is not the answer
 *
 * `index.html` declared `<title>JetStore Workspace IDE</title>` and nothing in
 * the bundle wrote `document.title`, so the tab of every screen named one screen
 * of an application with twenty-two routes. That is the favicon defect of
 * **I-267** in the element four lines above, and it was raised rather than fixed
 * at D.9 because D.9's remit was the icon.
 *
 * **It differs from the favicon in the one way that matters: there is nothing to
 * restore.** The predecessor is `JetStore Platform`
 * (`jetsclient/web/index.html`, at `e4672bfa^`) and it was *also* static on every
 * Flutter page — so the port did not regress this, it narrowed a string that was
 * already flat from the product's name to one screen's. Putting *JetStore
 * Platform* back would leave the reporter looking at the same undifferentiated
 * tab they have now. **Taken 2026-08-27: a per-route title.**
 *
 * ## The static document names the default, which is D.9's argument reused
 *
 * `index.html` now says `JetStore`. Three consequences, and they are the three
 * that decided the favicon the same way round:
 *
 *  - it is what a tab shows before the bundle hydrates, and what it shows if the
 *    bundle never runs — right for the product, wrong for no route;
 *  - it fails safe: delete this module and every tab reads *JetStore*, which is
 *    true everywhere rather than false everywhere;
 *  - **`/register` is outside the shell** (`App.tsx`, the `path="/register"`
 *    route), so no effect mounted in `AppShell` reaches it. Under a document that
 *    named a *screen*, that route would carry the wrong one permanently.
 *
 * ## The table is checked against the route table rather than kept by hand
 *
 * A row is a claim that a route exists, and `documentTitle.test.tsx` reads
 * `App.tsx` and requires the two to agree in **both** directions — a title for a
 * route that has gone, and a route with no title, both fail. That is deliberately
 * the check `routes.test.ts` was missing until C.6's *"a one-directional check
 * meeting the direction it cannot see"*, which is the shape Phase 3 recorded five
 * times; there is no reason to reintroduce it here.
 *
 * ## `/flow/:key` has two writers, and they do not race
 *
 * A flow's name is in its document, not in its url, so the table can only say
 * *Flow* for `/flow/:key`. `FlowRunner` refines it to the flow's own `title` once
 * the document loads — the same string D.7 put in the heading. The two never
 * collide because `RouteTitle`'s effect is keyed on `pathname`, which does not
 * change while a flow loads: the runner's write is the last one until the user
 * navigates, and navigating is exactly when `RouteTitle` should win.
 */

import { useEffect } from "react";
import { matchPath, useLocation } from "react-router-dom";

/** The suffix every title carries, and the whole title when nothing is known. */
export const PRODUCT = "JetStore";

/**
 * The name of each route, as route templates rather than paths.
 *
 * **In `App.tsx`'s order**, which is a readability property rather than a
 * correctness one and is asserted as such. `matchPath` with a string pattern
 * matches to the end, so `/workspaces` does not match `/workspaces/cgt/home` and
 * no row can swallow another — a first draft of this comment claimed it could,
 * and the test that would have caught the swallowing passes either way, which is
 * exactly why the order is checked against `App.tsx` instead of argued for.
 *
 * Names are the nav's where the nav has one (`NAV` and `FLOW_MENU` in `App.tsx`)
 * and the screen's own heading otherwise. They are not derived from `NAV`:
 * `AppShell` is imported by `App.tsx`, so a module the shell imports may not
 * import back, and twelve of these twenty are in no menu at all.
 */
export const ROUTE_TITLES: ReadonlyArray<readonly [pattern: string, title: string]> = [
  ["/register", "Register"],
  ["/login", "Sign in"],
  ["/home", "Home"],
  ["/workspace", "Workspace IDE"],
  ["/workspaces/:workspace_name/home", "Workspace IDE"],
  ["/workspaces", "Workspaces"],
  ["/inferServerAdmin", "Infer Server Admin"],
  ["/proposals", "Proposals"],
  ["/proposals/:proposalId", "Proposal"],
  // agentic_ai's incident screens (AE.1). Added here rather than left to their
  // owner because this table's own test refuses a route with no title, and it
  // fails on the route rather than on the screen — the seam rule the repository
  // CLAUDE.md states for a break in the other direction, applied in this one.
  ["/incidents", "Incidents"],
  ["/incidents/:incidentId", "Incident"],
  // The flow's own title replaces this as soon as the document loads; see above.
  ["/flow/:key", "Flow"],
  ["/git-profile", "Git Profile"],
  ["/domainTableViewer/:table_name/:session_id", "Table View"],
  ["/filePreviewPath/:file_key", "Input File Preview"],
  ["/executionStatusDetails/:session_id", "Pipeline Execution Details"],
  ["/executionStatsDetails/:session_id", "CPIPES Execution Details"],
  ["/query-tool", "Query Tool"],
  ["/processErrors/:session_id", "Process Errors"],
  ["/userAdmin", "User Administration"],
  ["/ruleConfig", "Rules Config"],
  ["/fileLoaderStatus", "File Loader Status"],
];

/** `"<screen> — JetStore"`, or the product alone for a path no route matches. */
export function documentTitle(name: string | null): string {
  return name === null || name === "" ? PRODUCT : `${name} — ${PRODUCT}`;
}

/**
 * The name of the route serving this path, or null.
 *
 * **Null rather than a guess for an unmatched path**, which is `/` before its
 * redirect and anything `App.tsx` answers with `NotFound`. The product alone is
 * the honest title for both: one is a route the user is passing through and the
 * other is a page that is not a screen.
 */
export function titleNameFor(pathname: string): string | null {
  for (const [pattern, title] of ROUTE_TITLES) {
    if (matchPath(pattern, pathname)) return title;
  }
  return null;
}

/** Exported for the test; the component below is what the app mounts. */
export function titleFor(pathname: string): string {
  return documentTitle(titleNameFor(pathname));
}

/**
 * Keeps `document.title` in step with the route. Renders nothing.
 *
 * A component rather than a hook called from `ShellChrome`, for `RouteFavicon`'s
 * reason: it sits above the session gate, and a signed-out visitor to
 * `/workspace` is still looking at the editor's url.
 */
export function RouteTitle(): null {
  const { pathname } = useLocation();
  useEffect(() => {
    document.title = titleFor(pathname);
  }, [pathname]);
  return null;
}

/**
 * A screen naming itself, once it knows what it is called.
 *
 * One caller: `FlowRunner`, whose title is in the document rather than in the
 * url. `null` leaves the route's own answer alone, which is what it passes while
 * the flow is loading and after a load failure.
 */
export function useDocumentTitleDetail(name: string | null): void {
  useEffect(() => {
    if (name === null || name === "") return;
    document.title = documentTitle(name);
  }, [name]);
}

/**
 * The 404. Task C.16, `/404` in the Flutter app.
 *
 * `sizing_screen_migration.md` §7 calls this "a four-line widget", and as a
 * *widget* it is: the Flutter side is a `MessageScreen` saying `"Opps 404!"`
 * (`jets_routes_app.dart`, `pageNotFoundPath`). The work in this task is not the
 * widget. It is that **the React app already answered every unmatched path, by
 * redirecting**, and deciding whether to keep doing that is the whole of C.16.
 *
 * ## The decision: report, do not redirect
 *
 * `App.tsx` carried `<Route path="*" element={<Navigate to="/workspace" replace />} />`
 * since A.1. That is not the Flutter behaviour — `jetsRoutesParser` falls through
 * to `pageNotFoundPath` when no route template matches
 * (`jets_route_information_parser.dart`) — and the difference is a decision
 * rather than a gap: **a redirect hides a bad link and a 404 reports it.**
 *
 * **The evidence against the redirect is this repository's own, and it is not an
 * argument from taste.** I-50: the Flutter app had been handing users to
 * `/ide/flow/<key>` since S.8, nothing registered that route, and the catch-all
 * silently landed them in the editor. The failure did not look like a missing
 * route — it looked like a working application on the wrong screen — and it
 * survived long enough that the Phase 3 plan was written assuming a flow runner
 * existed. **The redirect was the mechanism by which a missing route stayed
 * invisible**, not merely an unlucky bystander.
 *
 * **And the exposure is about to grow rather than shrink.** Track C is a list of
 * handoff points: every screen it ports adds a url the Flutter app may send a
 * user to, and every one of those is a chance to name it wrong on one side. With
 * the redirect, each such mistake renders the editor and looks fine. This route
 * is also the *only* thing that can report it — the apiserver's `/ide/` handler
 * falls back to `index.html` for any path under the prefix
 * (`jets/apiserver/static_ide.go`), so a mistyped deep link never reaches a
 * server 404 and this app is the only party that knows.
 *
 * **What would have to be true to go back to the redirect**, since the prompt
 * asked and a decision without a reversal condition is a preference: that a real
 * user, rather than a developer or another app, routinely arrives at an unmatched
 * path through no error of anyone's — a stale bookmark from a url space this app
 * has renamed, most plausibly. Decision 4 defers exactly such a rename (`/ide/`
 * → something else), so **if that rename happens, redirect the old prefix
 * explicitly and keep this screen for everything else.** A catch-all is the wrong
 * tool for a known rename either way: it cannot tell the two cases apart, which
 * is the same reason it could not tell I-50 apart from a working app.
 *
 * ## What it shows, and one divergence
 *
 * It names the path that failed. That is the entire practical difference from the
 * Flutter screen, which says `"Opps 404!"` and nothing else — a message that
 * tells a user they are lost and does not tell whoever they report it to which
 * link was wrong. The typo is not carried across.
 *
 * **This screen sits inside `AppShell`, so it is behind the session gate, and the
 * Flutter 404 is not** — `noAuthRequiredPaths` includes `pageNotFoundPath`
 * (`jets_routes_app.dart`). An unauthenticated user reaching a bad `/ide/` url
 * therefore meets the login screen here rather than a 404. That follows from the
 * shell owning the session rather than from a choice made here, and it is the
 * better answer of the two: what a signed-out user needs first is to sign in, and
 * the bad path is still in their address bar.
 */

import { Link, useLocation } from "react-router-dom";

export function NotFound() {
  const { pathname, search } = useLocation();
  return (
    <main className="screen">
      <div className="empty" role="status">
        <p>Page not found</p>
        <p className="empty-sub">
          Nothing is routed at <code>{`${pathname}${search}`}</code>.
        </p>
        <p className="empty-sub">
          <Link to="/workspace">Go to the Workspace IDE</Link>
        </p>
      </div>
    </main>
  );
}

/// Which user flows the React app now serves, and how to get there.
///
/// **Task S.8 of the UI refresh.** While the port runs, a flow lives in exactly
/// one of the two apps, and a user who navigates here for a migrated flow has to
/// be handed over rather than shown a stale screen.
///
/// ## Why a handoff rather than a route the server decides
///
/// This app never calls `setPathUrlStrategy` — `url_strategy` is a dependency at
/// `pubspec.yaml:47` and nothing imports it — so Flutter web keeps its default
/// hash routing and every route here lives after the `#`. A fragment is never
/// sent to the server, which is why `jets/apiserver/server.go` has no catch-all
/// and needs none: the only path this app ever requests is `/`.
///
/// So the apiserver cannot dispatch on a Flutter route, because it never sees
/// one. The two apps' URL spaces cannot collide, and the handoff has to happen
/// here, on the client, as a full page load. That is correct rather than
/// unfortunate: the bundles share an origin and a token, not a heap.
///
/// ## The list, and why it is here rather than in the workspace
///
/// The React side decides ownership from the workspace — a flow is its own iff
/// `user_flows/<key>.uf.json` exists (`jetsclient_ide/src/userflow/routing.ts`).
/// This app cannot ask that question cheaply: it would be a request on every
/// navigation, in an app that is being replaced.
///
/// So this list is the one place the fact is duplicated, and it is **debt with a
/// short life**: it needs one line per flow as each is migrated, and the whole
/// file disappears when the Flutter app does. Recorded as I-30. The failure it
/// can produce is bounded and visible — a migrated flow that still opens the
/// Flutter version — rather than silent.
library;

/// Flow keys the React app serves. One line per migrated flow.
///
/// Keep in step with the documents in the workspace: a key here whose documents
/// are absent sends users to a React screen that cannot load, which is the worse
/// of the two directions.
///
/// **Empty, and it was wrongly non-empty for five days.** `loadFilesUF` and
/// `registerFileKeyUF` were listed here from S.8 on the strength of their
/// documents existing, and the React app had no `/flow/:key` route to receive
/// either — so `handoffUrlFor` returned a URL that landed on `App.tsx`'s
/// `path="*"` and redirected to the Workspace IDE. The user asked for a flow and
/// got the editor. Found 2026-08-23 as the ui_refresh project's **I-50**;
/// emptied by task F.0a, which builds the runner.
///
/// **The comment above anticipated the opposite failure and that is the lesson.**
/// It warns about a key whose *documents* are absent. The documents were present;
/// what was absent was the route. S.8 tested this file's output —
/// `handoffUrlFor(loadFilesUF) == '/ide/flow/loadFilesUF'` — and nothing tested
/// that the other app accepts it, because the contract between two apps is a
/// string and each has its own suite. **A handoff tested from one end is tested
/// on the half that cannot fail.**
///
/// So a key goes back here only once the React runner serves that flow end to
/// end, which is the ordering rule the phase 3 plan records as F10: adding a key
/// is a *consequence* of migrating a flow, never a step in it.
const migratedUserFlows = <String>{};

/// Where the React app is mounted. `/ide/` is a misnomer for a whole
/// application; see the UI refresh project's I-26.
const reactAppBase = '/ide';

/// The URL to hand off to, or null when this app keeps the flow.
///
/// [flowKey] is the leading path segment of a flow route — `loadFilesUF` for
/// `/#/loadFilesUF`.
String? handoffUrlFor(String flowKey) {
  if (!migratedUserFlows.contains(flowKey)) return null;
  return '$reactAppBase/flow/$flowKey';
}

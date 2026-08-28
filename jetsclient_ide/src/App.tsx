/**
 * The application's routes.
 *
 * Task A.1. Phase 1's `App.tsx` was the editor; it is now the route table, and
 * the editor is `screens/WorkspaceIde.tsx`.
 *
 * **`basename` is the only place the mount prefix appears in this app**, and
 * that is what keeps the decision recorded in `shell/AppShell.tsx` cheap to
 * revisit: every route and every `NavLink` below is relative to it, so moving
 * the bundle is this line plus vite's `base` plus one constant in the apiserver,
 * however many screens Phase 3 adds.
 *
 * It must agree with `base` in `vite.config.ts` and with `ideAssetPrefix` in
 * `jets/apiserver/static_ide.go`. Vite bakes its `base` into the asset urls at
 * build time, so a disagreement is not a redirect — it is a bundle whose assets
 * 404.
 */

import { BrowserRouter, Navigate, Route, Routes, useNavigate } from "react-router-dom";

import { ApiClient } from "./api/client";
import { Register } from "./components/Register";
import { AGENT_SUPERVISION } from "./proposals/api";
import { ProposalScreen } from "./proposals/ProposalScreen";
import { ProposalsScreen } from "./proposals/ProposalsScreen";
import { FlowRunner } from "./screens/FlowRunner";
import { GitProfileScreen } from "./screens/GitProfile";
import { TableScreen } from "./screens/TableScreen";
import { QueryTool, QUERY_TOOL } from "./screens/QueryTool";
import { Home } from "./screens/Home";
import { InferServerAdmin } from "./screens/InferServerAdmin";
import { NotFound } from "./screens/NotFound";
import { INFER_SERVER_ADMIN } from "./screens/inferServer";
import { ProcessErrors } from "./screens/ProcessErrors";
import { RuleConfig } from "./screens/RuleConfig";
import { UserAdmin } from "./screens/UserAdmin";
import { WorkspaceIde, WORKSPACE_IDE } from "./screens/WorkspaceIde";
import { WorkspaceRegistry } from "./screens/WorkspaceRegistry";
import { AppShell, type FlowMenuItem, type NavEntry } from "./shell/AppShell";
import cpipesExecDetailsTable from "./datatable/tables/cpipesExecDetailsTable.tc.json";
import inputFileViewerTable from "./datatable/tables/inputFileViewerTable.tc.json";
import inputTable from "./datatable/tables/inputTable.tc.json";
import pipelineExecDetailsTable from "./datatable/tables/pipelineExecDetailsTable.tc.json";
import type { TableConfigDocument } from "./datatable/table";

const api = new ApiClient();

/** Trailing slash omitted: react-router wants the prefix without it. */
// Re-exported so nothing that imported it from here has to change; the value
// lives in `base.ts` to keep `AppShell` from importing this module. See there.
export { BASENAME } from "./base";
import { BASENAME } from "./base";

export const NAV: NavEntry[] = [
  /*
    Task C.6. Ungated, mirroring Flutter's `jetstoreHome` entry, which is the
    first row of `defaultMenuEntries` and declares no capability
    (`jetsclient/lib/modules/screen_config_impl.dart`, `defaultMenuEntries`).
    First in the list for the same reason it is first there.

    **The route is `/home` and not the index, and that is a decision rather than
    an accident of ordering.** Flutter serves this screen at `/`; this app's `/`
    keeps redirecting to the editor, because `/ide/` bare is reached from exactly
    one place — the *Code Editor ↗* menu entry, which opens it in a new tab and
    carries `capability: 'workspace_ide'`
    (`jetsclient/lib/modules/workspace_ide/screen_delegates_helpers.dart`,
    `openWorkspaceIdeApp`). Every arrival at the bare prefix is therefore a
    `workspace_ide` holder who asked for the editor, and answering that press
    with the home screen would be wrong for the one population that generates it.

    ~~**The reversal trigger is track X**, not a condition: when `jetsclient`
    retires, `/ide/` stops meaning "the editor" and the index should become this
    screen.~~ **Fired 2026-08-26 at X.1**: the index redirects here. Naming the
    task rather than the condition is what made that happen on the right day —
    the root `CLAUDE.md` records the opposite case, a note whose trigger passed
    unnoticed because it named a state instead of an owner.
  */
  { to: "/home", label: "Home" },
  /*
    Task D.3, from **I-259**, which specifies the row as
    *Home | Filter Client | Workspaces | Workspace IDE | Proposals | Query Tool |
    Infer Server Admin*. The order below is that order, and the client filter is
    an entry rather than a fixed position after the row — see `ClientFilterSlot`.

    **The order is the reporter's and no argument is made for it here.** What is
    worth recording is that it is not the Flutter order either: that app had no
    row to order, it had a menu tree in the left panel, and the row this replaces
    was assembled a screen at a time as track C ported them. So this is the first
    time anybody has chosen the order rather than accumulated it.
  */
  { kind: "client-filter" },
  // **Track C's first screen (C.2b).** Gated on the same capability the Flutter
  // menu entry is and the same one the server enforces on every write it makes —
  // `admin` bypasses both, in `permissionFor` here and in `HasCapability`
  // (`jets/user/user.go`) there.
  { to: "/workspaces", label: "Workspaces", capability: WORKSPACE_IDE },
  { to: "/workspace", label: "Workspace IDE", capability: WORKSPACE_IDE },
  // agentic_ai's screens (task K.3). The nav entry and the two routes below are
  // this file's whole knowledge of them; the screens are in `proposals/`.
  { to: "/proposals", label: "Proposals", capability: AGENT_SUPERVISION },
  /*
    Task C.4. The Flutter menu entry for `/queryTool` sits in
    `workspaceRegistryMenuEntries` and declares no capability of its own
    (`jetsclient/lib/modules/workspace_ide/screen_config.dart`) — it does not
    need one there, because that menu is only carried by screens already gated on
    `workspace_ide`. This shell has one flat nav, so the gate that was implicit in
    *where the menu appears* is named here instead. It is the same capability the
    two server actions behind the screen require.
  */
  { to: "/query-tool", label: "Query Tool", capability: QUERY_TOOL },
  /*
    Track C's first ported screen (C.5).

    **The Flutter reachability corpus records this route's access as
    `admin | (infer_server_admin AND workspace_ide)` and this entry names one
    capability, which is a divergence rather than an oversight.** The conjunction
    is an artefact of the Flutter navigation graph: the entry lives in
    `workspaceRegistryMenuEntries`, so a user had to already be on a
    workspace-IDE screen to see it, and this shell renders one flat nav list from
    every screen. The `workspace_ide` half has no server counterpart either —
    `api_infer_server.go` enforces `infer_server_admin` and nothing else — so
    reproducing it would gate a control on a capability the endpoint does not
    ask for.

    **It is the only conjunction in the corpus**, 1 route of 27
    (`screens/fixtures/screen_reachability.json`, `accessSummary`), which is why
    `NavItem.capability` is still one name. What would change that: a second
    route needing one, or the endpoint starting to require both.
  */
  { to: "/inferServerAdmin", label: "Infer Server Admin", capability: INFER_SERVER_ADMIN },
];

/**
 * The flows the brand opens. Task D.3, from **I-261**.
 *
 * **The Flutter left panel's menu tree is where these were started**, and X.1
 * retired it with the app; the flows kept running and lost their entry points,
 * which is what the report describes. I-266 settled that the panel does not come
 * back, so the launcher is a menu on the brand.
 *
 * **The order is the reporter's, and so are the labels** — they are not the flow
 * keys, which is the point: **I-263** is the same problem seen from inside a
 * flow, where the key is rendered where a title belongs, and these are the titles
 * it needs.
 *
 * **`/ruleConfig` is not a flow and is in the list**, because it is what the
 * report asked for. It was reported as *"I can't find it in the React version"*
 * and it was routed the whole time, named by no navigation (**I-268**) — so the
 * fix for that entry is this line rather than a screen.
 *
 * **No capability is declared on any entry.** The four flows run through
 * `FlowRunner`, whose route carries no guard and whose reads the server gates;
 * `/ruleConfig` is likewise gated where it acts. Declaring one here would be a
 * second, client-side answer to a question the server already answers — and
 * `FLOW_RUNNER` is exactly such an answer sitting unused two files away
 * (**I-269**).
 */
export const FLOW_MENU: FlowMenuItem[] = [
  { to: "/flow/clientRegistryUF", label: "Clients & Vendors" },
  { to: "/flow/sourceConfigUF", label: "Source Config" },
  { to: "/flow/fileMappingUF", label: "Source Mapping" },
  { to: "/ruleConfig", label: "Rules Config" },
  { to: "/flow/pipelineConfigUF", label: "Pipeline Config" },
];

/**
 * The declarative router rather than the data router, deliberately.
 *
 * No route here has a loader or an action — screens fetch through `ApiClient`,
 * which owns the token refresh — so `createBrowserRouter` would buy nothing, and
 * it costs something real: its navigations construct `Request` objects, which
 * fail under jsdom because the `AbortSignal` it supplies is not the one node's
 * fetch implementation expects. Choosing the router the app actually needs kept
 * the shell testable without shimming globals.
 */
/**
 * `Register`, wired to the api and to the router. Task X.4.
 *
 * A wrapper rather than props threaded through `App`, so that the component
 * itself takes two callbacks and knows about neither the client nor the router —
 * which is what lets `Register.test.tsx` drive the rules without either.
 */
function RegisterScreen({ api }: { api: ApiClient }) {
  const navigate = useNavigate();
  return (
    <Register
      onSubmit={(name, email, password) => api.register(name, email, password)}
      onSignIn={() => navigate("/", { replace: true })}
    />
  );
}

export default function App() {
  return (
    <BrowserRouter basename={BASENAME}>
      <Routes>
        {/*
          **Outside the shell, because the shell is the session gate.** `AppShell`
          renders `<Login>` for anyone without a user and everything nested under
          it may therefore assume one — which is right for every screen and wrong
          for this one: a person registering has no account yet, so a `/register`
          inside the shell would show them the sign-in form they cannot use.

          Task X.4. It is the only unauthenticated route in the app, and the
          reason it exists at all is that `/register` is the only path by which an
          account is created — `UserAdmin` updates and deletes users and inserts
          none.
        */}
        <Route path="/register" element={<RegisterScreen api={api} />} />
        <Route path="/" element={<AppShell api={api} nav={NAV} flowMenu={FLOW_MENU} />}>
          {/*
            **`/login` is a real path and not a screen**, which is the shape the
            gate already had: unauthenticated, any path renders `<Login>`, so a
            person sent to `/login` sees the sign-in form without this route
            existing. What the route adds is the *other* direction — a signed-in
            user following an old `/login` link would otherwise fall through to
            the 404. Flutter had `/login` as a route and this keeps the url
            working in both states.
          */}
          <Route path="login" element={<Navigate to="/" replace />} />
          {/*
            **The index is Home since X.1, and this is the reversal C.6 scheduled
            rather than a change of mind.** It redirected to the editor because
            `/ide/` bare was reached from exactly one place — the *Code Editor ↗*
            menu entry in the Flutter app, which carried `capability:
            'workspace_ide'` — so every arrival at the bare prefix was a knowledge
            engineer who had asked for the editor, and answering that press with
            the home screen would have been wrong for the one population that
            generated it.
            
            That population is now everybody: `/` is the front door of the whole
            product and most of the people arriving at it hold `jetstore_read` and
            nothing else. C.6's comment named track X as the trigger — *"when
            `jetsclient` retires, `/ide/` stops meaning 'the editor' and the index
            should become this screen"* — and this is it.
          */}
          <Route index element={<Navigate to="/home" replace />} />
          <Route path="home" element={<Home api={api} />} />
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
          {/*
            **The same screen, addressable.** Task C.3, and the destination C.2b's
            *Open* button named before it existed (`actions/workspaceRegistry.ts`,
            `WORKSPACE_HOME_ROUTE`). The path is the Flutter route verbatim
            (`jetsclient/lib/routes/jets_routes_app.dart`, `workspaceHomePath`),
            so handing a user over is a prefix change rather than a translation.

            **The Flutter screen is reachable by button and not by URL** — its
            section list comes from `workspaceMenuState`, written by the *Open*
            delegate and read by `base_screen.dart`, so typing the url there gives
            an empty menu. This app has no such precondition: `WorkspaceIde`
            fetches its own tree in an effect keyed on the workspace. The bare
            `/workspace` above stays, because a user arriving with no workspace in
            mind picks one, and that is Phase 1's screen unchanged.
          */}
          <Route path="workspaces/:workspace_name/home" element={<WorkspaceIde api={api} />} />
          {/* The Flutter route is `/workspaces` and this is the same path, now at
              the same address: X.1 retired the other app, so there is no second
              copy for a `configScreenPath` to reach. */}
          <Route path="workspaces" element={<WorkspaceRegistry api={api} />} />
          <Route path="inferServerAdmin" element={<InferServerAdmin api={api} />} />
          <Route path="proposals" element={<ProposalsScreen api={api} />} />
          <Route path="proposals/:proposalId" element={<ProposalScreen api={api} />} />
          {/*
            The route the Flutter app has been handing users to since S.8, and
            which nothing registered until F.0a — so the handoff fell through to
            `path="*"` below and redirected to the editor (I-50). `reactFlowPath`
            in `userflow/routing.ts` is the one definition of the shape, and
            `migrated_user_flows.dart` is the Flutter side of the same string.

            There is no nav entry: a flow is entered by key, from the other app or
            from a link, and a list of flows is a screen track F has not been
            asked for.
          */}
          <Route path="flow/:key" element={<FlowRunner api={api} />} />
          {/*
            Task C.14, and it has no nav entry for the same reason the flow
            runner does not: the Flutter screen is reached from the app bar's
            user button rather than from any menu, and its `reachedFrom` in
            `screens/fixtures/screen_reachability.json` is empty. The shell's
            user name is the link, which is where the Flutter app puts it.

            **The four route parameters are deliberately not reproduced** — see
            the header of `screens/GitProfile.tsx`.
          */}
          <Route path="git-profile" element={<GitProfileScreen api={api} />} />
          {/*
            C.11 and C.12, and they are C.7's component with a title. Both are
            `ScreenOne` — F68's four, now all four served here — and unlike the
            execution-detail pair both `ScreenConfig`s declare a `title:`
            (`jetsclient/lib/modules/screen_config_impl.dart`,
            `ScreenKeys.fileRegistryTable` and `ScreenKeys.filePreview`), so
            `TableScreen`'s optional prop gets its first consumer.

            **Both tables declare no columns**; the server describes the result
            and `columnsFromResponse` has consumed it since A.4b (F81). C.12's
            also names `preview_file`, the `apiAction` C.2a's enum carries and
            no other configuration uses.

            Both are entered from the home screen: `/domainTableViewer` from
            `inputLoaderStatusTable` and `inputRegistryTable`, `/filePreviewPath`
            from `loadFilesUF`'s staging table — which is a flow, so that one is
            still reached from Flutter until a workspace holds the documents.
          */}
          <Route
            path="domainTableViewer/:table_name/:session_id"
            element={
              <TableScreen
                api={api}
                tableKey="inputTable"
                document={inputTable as TableConfigDocument}
                title="Staging Table or Domain Table View"
              />
            }
          />
          <Route
            path="filePreviewPath/:file_key"
            element={
              <TableScreen
                api={api}
                tableKey="inputFileViewerTable"
                document={inputFileViewerTable as TableConfigDocument}
                title="Input File Preview"
              />
            }
          />
          {/*
            C.7 and C.8. **One screen, two routes** — see `TableScreen.tsx` for
            why that is a measurement rather than a convenience.

            The paths are the Flutter app's verbatim
            (`jetsclient/lib/routes/jets_routes_app.dart`,
            `executionStatusDetailsPath` and `executionStatsDetailsPath`), minus
            the leading slash that `basename` supplies, so handing a user over is
            a prefix change rather than a translation.

            **No nav entry, and no user reaches either yet.** Both are entered
            from the home screen's `pipelineExecStatusTable` — its
            `viewStatusDetails` and `viewExecStatsDetails` buttons — and that
            screen is C.6's; a table action in this app currently navigates by
            `window.location.href = "/#" + path`, into the Flutter route. This is
            the same state F.0a left the flow runner in and it is stated rather
            than implied: the screens run, and nothing links to them.
          */}
          <Route
            path="executionStatusDetails/:session_id"
            element={
              <TableScreen
                api={api}
                tableKey="pipelineExecDetailsTable"
                document={pipelineExecDetailsTable as TableConfigDocument}
              />
            }
          />
          <Route
            path="executionStatsDetails/:session_id"
            element={
              <TableScreen
                api={api}
                tableKey="cpipesExecDetailsTable"
                document={cpipesExecDetailsTable as TableConfigDocument}
              />
            }
          />
          <Route path="query-tool" element={<QueryTool api={api} />} />
          {/*
            C.9. **Five table configurations and one of them is on the screen** —
            the other four are inside its two dialogs, which is why this route
            needs no more markup than the two above it. See `ProcessErrors.tsx`.

            The path is the Flutter app's verbatim
            (`jetsclient/lib/routes/jets_routes_app.dart`, `processErrorsPath`)
            minus the leading slash that `basename` supplies.

            **No nav entry and no user reaches it yet**, the same state C.7 and
            C.8 are in: the only way in is the *View Process Errors* button on the
            home screen's `pipelineExecStatusTable`, and that screen is C.6's.
          */}
          <Route path="processErrors/:session_id" element={<ProcessErrors api={api} />} />
          {/*
            C.13. **No nav entry, and that is a divergence stated rather than an
            omission.** The Flutter menu puts this in `adminMenuEntries` and the
            shell here renders one flat list; `NavItem.capability` names a
            capability and `admin` is not one — it is an *account*
            (`jets/user/user.go`, `IsAdmin`). A nav entry gated on
            `user_profile` would offer the screen to three of the four seeded
            roles and the server would refuse every write they made, which is
            worse than no entry. Reached by link until the shell learns the
            distinction.
          */}
          <Route path="userAdmin" element={<UserAdmin api={api} />} />
          {/*
            C.10. Any authenticated user reaches it; the two writes it makes are
            gated on `client_config` server-side and both buttons name it.

            **No nav entry yet**, for the same reason the three routes above have
            none: the shell's nav list is `NAV` in this file and adding entries for
            track C's screens is one decision about the whole list rather than nine
            about individual screens. The Flutter menu calls this one
            `processConfig`, which is the v1 name for the v2 screen.
          */}
          <Route path="ruleConfig" element={<RuleConfig api={api} />} />
          {/*
            **The catch-all reports rather than redirects, as of C.16.** It was
            `<Navigate to="/workspace" replace />` from A.1 until now, and that is
            not the Flutter behaviour: `jetsRoutesParser` falls through to
            `pageNotFoundPath` when no template matches. The argument, and the
            condition under which it reverses, are in `screens/NotFound.tsx` —
            short version, this redirect is what made I-50 invisible, and track C
            is about to multiply the handoff urls it could hide a mistake in.

            The index redirect above stays: `/ide/` bare is a url the Flutter app
            deliberately links to, which is a known destination rather than an
            unmatched path.
          */}
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

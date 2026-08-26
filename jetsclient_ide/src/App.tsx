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

import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { ApiClient } from "./api/client";
import { AGENT_SUPERVISION } from "./proposals/api";
import { ProposalScreen } from "./proposals/ProposalScreen";
import { ProposalsScreen } from "./proposals/ProposalsScreen";
import { FlowRunner } from "./screens/FlowRunner";
import { WorkspaceIde, WORKSPACE_IDE } from "./screens/WorkspaceIde";
import { WorkspaceRegistry } from "./screens/WorkspaceRegistry";
import { AppShell, type NavItem } from "./shell/AppShell";

const api = new ApiClient();

/** Trailing slash omitted: react-router wants the prefix without it. */
export const BASENAME = "/ide";

export const NAV: NavItem[] = [
  { to: "/workspace", label: "Workspace IDE", capability: WORKSPACE_IDE },
  // agentic_ai's screens (task K.3). The nav entry and the two routes below are
  // this file's whole knowledge of them; the screens are in `proposals/`.
  { to: "/proposals", label: "Proposals", capability: AGENT_SUPERVISION },
  // **Track C's first screen (C.2b).** Gated on the same capability the Flutter
  // menu entry is and the same one the server enforces on every write it makes —
  // `admin` bypasses both, in `permissionFor` here and in `HasCapability`
  // (`jets/user/user.go`) there.
  { to: "/workspaces", label: "Workspaces", capability: WORKSPACE_IDE },
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
export default function App() {
  return (
    <BrowserRouter basename={BASENAME}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={NAV} />}>
          {/* The IDE keeps the bare prefix working: /ide/ is where the Flutter
              app links to, and a redirect is cheaper than teaching it a url. */}
          <Route index element={<Navigate to="/workspace" replace />} />
          <Route path="workspace" element={<WorkspaceIde api={api} />} />
          {/* The Flutter route is `/workspaces`, and this is the same path under
              the `/ide` basename. Track X decides when the other app stops
              serving its copy; until then the two coexist and the Flutter one is
              what a `configScreenPath` on any other screen still reaches. */}
          <Route path="workspaces" element={<WorkspaceRegistry api={api} />} />
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
          <Route path="*" element={<Navigate to="/workspace" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

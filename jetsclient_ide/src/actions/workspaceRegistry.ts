/**
 * The one escape `/workspaces` needs. Task C.2b.
 *
 * ## `openWorkspace` is the action C.2 cannot finish, and the reason is a fact
 * about the Flutter app rather than about the port
 *
 * The Dart's *Open* button posts `workspace_query_structure`, writes the reply
 * into `JetsRouterDelegate().workspaceMenuState`, and then navigates to
 * `/workspaces/:workspace_name/home`
 * (`jetsclient/lib/modules/workspace_ide/screen_delegates_helpers.dart`,
 * `openWorkspaceActions`). Two thirds of that is client state, and the state is
 * what the destination screen renders its section list from.
 *
 * **`workspaceMenuState` is written at exactly three sites and read from the
 * route at none** — `openWorkspaceActions`, and the add-file and delete-file arms
 * of `screen_delegates.dart`, all three of them button presses. So the workspace
 * home screen **cannot be reached by URL in either app**: a user typing
 * `/#/workspaces/acme/home` gets it with an empty section list. That is why this
 * escape does not hand the browser to the Flutter route the way `FlowRunner`'s
 * `navigate` and `exit` do for every other cross-app link — the link would work
 * and the screen would be wrong, which is the failure mode this project spends
 * most of its care avoiding.
 *
 * **So it reports.** `escapes.ts`'s rule is that a button which silently does
 * nothing is what makes people distrust authored configuration, and I-68 applied
 * it to the dialog host; this is the same rule for a destination that does not
 * exist yet. Posting the structure query and discarding the answer would be
 * worse than not posting it: a request whose only effect is client state nobody
 * keeps.
 *
 * **The document does not change when C.3 lands.** `workspaceRegistry.ua.json`
 * names this escape and C.3 replaces the body — with a React route, a store for
 * the tree, and `WorkspaceApi.fileTree`, which already exists and already returns
 * `WorkspaceNode[]`. Recorded as **I-183**.
 */

import type { EscapeContext, EscapeHost } from "./escapes";

/** The route C.3 will serve, named here so the two ends agree in advance. */
export const WORKSPACE_HOME_ROUTE = "/workspaces/:workspace_name/home";

export async function openWorkspace(
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const name = context.formState.getValue(context.group, "workspace_name");
  const workspace = Array.isArray(name) ? name[0] : name;
  host.notify(
    "error",
    `Opening ${typeof workspace === "string" && workspace !== "" ? workspace : "a workspace"} needs the Workspace home screen, which is task C.3 — see I-183.`,
  );
  // Null rather than the message: an `ActionEscape` returning a string is an
  // *outcome* the caller reports, and reporting it twice would put the same
  // sentence in the banner and in the caller's error path.
  return null;
}

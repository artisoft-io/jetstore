/**
 * The one escape `/workspaces` needs. Tasks C.2b and C.3.
 *
 * ## What the Dart's *Open* button does, and why the port is three lines
 *
 * It posts `workspace_query_structure`, writes the reply into
 * `JetsRouterDelegate().workspaceMenuState`, and navigates to
 * `/workspaces/:workspace_name/home`
 * (`jetsclient/lib/modules/workspace_ide/screen_delegates_helpers.dart`,
 * `openWorkspaceActions`). Two thirds of that is client state, and the state is
 * what the destination screen renders its section list from — `base_screen.dart`
 * builds a `ScreenType.workspace` screen's left menu out of `workspaceMenuState`
 * and out of nothing else (the `menuEntries` switch, and `removeTab` above it).
 *
 * **So the Flutter screen cannot be entered by URL**: a user typing
 * `/#/workspaces/acme/home` gets it with an empty menu, because the fetch is on
 * the button rather than on the screen. C.2b declined to link to it for that
 * reason and named C.3.
 *
 * **The defect does not survive the port, and that is why this is now a
 * navigation and not a report.** `WorkspaceIde` fetches its own tree on mount —
 * `WorkspaceApi.fileTree(workspace)`, in an effect keyed on the workspace — so
 * the React screen has no client-state precondition and is addressable by URL by
 * construction. **The fix was not a decision anyone took here; it fell out of the
 * screen fetching where the Flutter one was handed.**
 *
 * ## One correction to the paragraph this replaced, because it steered a task
 *
 * It said `workspaceMenuState` is *"written at exactly three sites and read from
 * the route at none"*, which is exact and true, and it was relayed onward as
 * *written by three button presses and read from nowhere*, which is not: it has
 * two readers in `base_screen.dart`. The difference matters, because *the screen
 * is unreachable* makes C.3 a design and *the screen is reachable by button and
 * not by URL* makes it a port with a known defect not to reproduce. **A true
 * sentence about one mechanism became a false sentence about the screen in one
 * hop.** Corrected 2026-08-25 by C.3, which read the Dart because it was about to
 * define the entry point.
 */

import type { EscapeContext, EscapeHost } from "./escapes";

/**
 * The route this escape sends the user to.
 *
 * **C.2b named it before it existed and C.3 served exactly it**, which is the
 * whole value of a constant here: the two ends agreed in advance rather than
 * discovering a disagreement at run time. It is the Flutter path verbatim, minus
 * nothing — `App.tsx` mounts it under the `/ide` basename, so handing a user over
 * from the other app is a prefix change rather than a translation, as it is for
 * C.7 and C.8.
 */
export const WORKSPACE_HOME_ROUTE = "/workspaces/:workspace_name/home";

export async function openWorkspace(
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const name = context.formState.getValue(context.group, "workspace_name");
  const workspace = Array.isArray(name) ? name[0] : name;
  if (typeof workspace !== "string" || workspace === "") {
    // The button is gated on a selected row (`workspaceRegistryTable`'s
    // `enableWhen`), so this is a configuration error rather than a user one —
    // and it is reported rather than navigated past, because a route with an
    // empty segment would 404 into the catch-all and look like a broken link.
    return "Open needs a workspace to be selected.";
  }
  if (host.navigate === undefined) {
    // A host outside the router — a dialog host, or a generator. Saying so is
    // `escapes.ts`'s rule that a button which silently does nothing is what makes
    // people distrust authored configuration.
    host.notify("error", `Cannot open ${workspace} from here: this host has no router.`);
    return null;
  }
  host.navigate(WORKSPACE_HOME_ROUTE.replace(":workspace_name", encodeURIComponent(workspace)));
  return null;
}

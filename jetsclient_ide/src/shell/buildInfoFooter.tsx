/**
 * Where the build information is drawn. jetstore_maintenance_02 Phase 1, task
 * AC.3, for defect D05.
 *
 * **One bottom bar per screen, not two.** The shell draws a footer at the
 * bottom left of its flex column on every screen — except one that has a bottom
 * bar of its own. The Workspace IDE already draws a status bar under its editor
 * (`WorkspaceIde.tsx`, the `statusbar` footer), and a footer beneath that would
 * stack two bars carrying unrelated things. So a screen with its own bar calls
 * `useBuildInfoInScreenBar`, renders what it returns, and the shell's footer
 * stands down for as long as that screen is mounted.
 *
 * **A claim rather than a route test.** The shell could have matched the IDE's
 * two routes by path; it would then be naming another screen's urls, and the
 * third route to that screen would quietly bring the second bar back. The screen
 * that has the bar is the one that knows it.
 */

import { createContext, useContext, useLayoutEffect, type ReactNode } from "react";

import type { BuildInfo } from "./buildInfo";

/** What the shell offers the screens below it. */
export interface BuildInfoSlot {
  /** The formatted build information, or `null` when the login carried none. */
  info: BuildInfo | null;
  /** Suppresses the shell's footer until the returned release is called. */
  claim(): () => void;
}

const BuildInfoContext = createContext<BuildInfoSlot | null>(null);

export function BuildInfoProvider({ slot, children }: { slot: BuildInfoSlot; children: ReactNode }) {
  return <BuildInfoContext.Provider value={slot}>{children}</BuildInfoContext.Provider>;
}

/**
 * The build information, for a screen that draws its own bottom bar and puts it
 * there. Calling this is what suppresses the shell's footer.
 *
 * `useLayoutEffect` rather than `useEffect`, so the claim lands before the
 * browser paints: with a plain effect the first frame of the screen would show
 * both bars. Outside the shell it returns `null` and claims nothing.
 */
export function useBuildInfoInScreenBar(): BuildInfo | null {
  const slot = useContext(BuildInfoContext);
  const claim = slot?.claim;
  useLayoutEffect(() => claim?.(), [claim]);
  return slot?.info ?? null;
}

/** The text itself, with the raw `JETS_VERSION` as its tooltip. */
export function BuildInfoText({ info }: { info: BuildInfo }) {
  return (
    <span className="build-info" {...(info.title !== "" ? { title: info.title } : {})}>
      {info.label}
    </span>
  );
}

/** The shell's footer: the bottom left of every screen that has no bar of its own. */
export function BuildInfoFooter({ info }: { info: BuildInfo }) {
  return (
    <footer className="app-footer" aria-label="Build information">
      <BuildInfoText info={info} />
    </footer>
  );
}

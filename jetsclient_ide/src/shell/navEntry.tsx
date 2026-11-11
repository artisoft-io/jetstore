/**
 * Which entry of the top bar a screen belongs to, when its address does not
 * say.
 *
 * **The top bar marks an entry by its address**: an entry is current when the
 * location is its `to` or under it. That is right for almost every screen and
 * wrong for one kind: a screen whose address sits under *another* entry's. The
 * Runbooks screen of a workspace is `/workspaces/<name>/runbooks`, under
 * *Workspaces*, so the bar marked *Workspaces* while the user was on the screen
 * they had reached from *Runbooks*.
 *
 * **A claim rather than a route table in the shell**, for the reason
 * `buildInfoFooter.tsx` gives for its own claim: the shell could match the
 * runbook routes by path, but it would then be naming another screen's
 * addresses, and the next route to that screen would quietly bring the defect
 * back. The screen knows which entry it is reached from, so it says so with
 * `useNavEntry`, and while it is mounted that entry is the current one and no
 * other is.
 */

import { createContext, useContext, useLayoutEffect, type ReactNode } from "react";

/** What the shell offers the screens below it. */
export interface NavEntrySlot {
  /** Marks the entry whose `to` is given as current until the release is called. */
  claim(to: string): () => void;
}

const NavEntryContext = createContext<NavEntrySlot | null>(null);

export function NavEntryProvider({ slot, children }: { slot: NavEntrySlot; children: ReactNode }) {
  return <NavEntryContext.Provider value={slot}>{children}</NavEntryContext.Provider>;
}

/**
 * Marks the top-bar entry whose `to` is given as the current one while the
 * calling screen is mounted, whatever its address.
 *
 * `useLayoutEffect` so the claim lands before the browser paints, and the first
 * frame does not mark the entry the address alone would. Outside the shell it
 * claims nothing.
 */
export function useNavEntry(to: string): void {
  const claim = useContext(NavEntryContext)?.claim;
  useLayoutEffect(() => claim?.(to), [claim, to]);
}

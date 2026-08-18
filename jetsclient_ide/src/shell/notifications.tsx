/**
 * The banner region, shared by every screen.
 *
 * Task A.1. Phase 1's editor kept `error` and `status` in its own state and
 * rendered its own banners; with more than one screen that becomes one banner
 * implementation per screen, each free to disagree about where messages appear
 * and how they are dismissed. So the shell owns the region and screens raise
 * into it.
 *
 * Deliberately two levels and no more. `error` is a failure the user must see;
 * `status` is a completed action worth confirming. A third level always turns
 * out to be one of those two with a different colour.
 */

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

export interface Notifications {
  error: string | null;
  status: string | null;
  setError(message: string | null): void;
  setStatus(message: string | null): void;
  /** Clears both, which is what a screen change should do. */
  clear(): void;
}

const NotificationsContext = createContext<Notifications | null>(null);

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [error, setErrorState] = useState<string | null>(null);
  const [status, setStatusState] = useState<string | null>(null);

  // An error supersedes a status: the success line from the previous action is
  // not what the user needs to read once something has failed.
  const setError = useCallback((message: string | null) => {
    setErrorState(message);
    if (message != null) setStatusState(null);
  }, []);

  const setStatus = useCallback((message: string | null) => setStatusState(message), []);
  const clear = useCallback(() => {
    setErrorState(null);
    setStatusState(null);
  }, []);

  const value = useMemo<Notifications>(
    () => ({ error, status, setError, setStatus, clear }),
    [error, status, setError, setStatus, clear],
  );

  return (
    <NotificationsContext.Provider value={value}>{children}</NotificationsContext.Provider>
  );
}

export function useNotifications(): Notifications {
  const value = useContext(NotificationsContext);
  if (!value) {
    // A screen rendered outside the shell is a wiring mistake, and silently
    // swallowing its error messages is the worst way to find out.
    throw new Error("useNotifications must be used inside the app shell");
  }
  return value;
}

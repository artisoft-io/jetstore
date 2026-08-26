/**
 * The application shell: session, chrome, navigation, and the banner region.
 *
 * Task A.1 — the piece that turns Phase 1's single-screen editor into an
 * application that can host screens. Everything a screen should not have to
 * reimplement lives here; everything specific to a screen stays in it.
 *
 * ## The `/ide/` question, settled here
 *
 * The plan required A.1 to decide the relationship between this bundle and the
 * IDE's url space rather than inherit it, because vite bakes `base` into every
 * asset url at build time and a bundle cannot be moved without rebuilding.
 *
 * **One bundle, still mounted at `/ide/`.** Two facts decided it, neither of them
 * taste:
 *
 *  - The apiserver's handler for the prefix is already a SPA handler: a request
 *    under `/ide/` that names no file falls back to `index.html`
 *    (`jets/apiserver/static_ide.go:63`). **Client-side routes under it already
 *    work**, so hosting more screens here needs no server change, no second
 *    build, and no second copy of the api client.
 *  - The cost of renaming the prefix later does **not** grow with the number of
 *    screens. Routes inside this app are relative to the router's `basename`, so
 *    Phase 3 adding twenty screens adds no new mentions of `/ide/`. Today the
 *    prefix is named in one Go constant, one Dart line that opens this app
 *    (`jetsclient/lib/modules/workspace_ide/screen_delegates_helpers.dart:94`),
 *    the vite `base`, and the `basename` below. It will be named in the same
 *    places after Phase 3.
 *
 * So the name is now inaccurate — this is the React application, not only the
 * IDE — and that is a cosmetic debt with a bounded, non-growing price. The
 * moment to pay it is when the Flutter app retires and the static routes are
 * consolidated, which `static_ide.go:19` already anticipates; doing it now would
 * spend a change to an app being replaced and break existing links to buy
 * nothing.
 */

import { useCallback, useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";

import { ApiClient, type User } from "../api/client";
import { Login } from "../components/Login";
import { ApiProvider, useCan } from "./capabilities";
import { NotificationsProvider, useNotifications } from "./notifications";

export type Theme = "light" | "dark";

/** Kept under the Phase 1 key so a returning user keeps their choice. */
const THEME_KEY = "jetstore-ide-theme";

export interface NavItem {
  to: string;
  label: string;
  /** When set, the item is hidden unless the user holds the capability. */
  capability?: string;
}

export interface AppShellProps {
  api: ApiClient;
  nav: NavItem[];
}

/**
 * Wraps the shell so screens can raise banners into it.
 *
 * The provider sits outside `ShellChrome` rather than inside it because the
 * chrome renders the banners and the screens fill them: one of the two has to be
 * the parent, and a screen that could not raise an error before the chrome
 * mounted would drop the first failure of every session.
 */
export function AppShell(props: AppShellProps) {
  return (
    <ApiProvider api={props.api}>
      <NotificationsProvider>
        <ShellChrome {...props} />
      </NotificationsProvider>
    </ApiProvider>
  );
}

/**
 * A nav item the user cannot reach: visible and inert, not absent.
 *
 * See `capabilities.tsx` — every gated control in the Flutter app disables
 * rather than hides, and A.1 diverged from that here without noticing.
 */
function DisabledNavItem({ label, reason }: { label: string; reason?: string }) {
  return (
    <span
      className="navlink is-disabled"
      aria-disabled="true"
      {...(reason ? { title: reason } : {})}
    >
      {label}
    </span>
  );
}

function ShellNavItem({ item }: { item: NavItem }) {
  const permission = useCan(item.capability);
  if (!permission.allowed) {
    return <DisabledNavItem label={item.label} {...(permission.reason ? { reason: permission.reason } : {})} />;
  }
  return (
    <NavLink to={item.to} className={({ isActive }) => `navlink${isActive ? " is-active" : ""}`}>
      {item.label}
    </NavLink>
  );
}

function ShellChrome({ api, nav }: AppShellProps) {
  const [user, setUser] = useState<User | null>(api.currentUser);
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem(THEME_KEY) as Theme | null) ?? "light",
  );
  const { error, status, setError, setStatus } = useNotifications();

  useEffect(() => api.subscribe(setUser), [api]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem(THEME_KEY, theme);
  }, [theme]);

  const signIn = useCallback(
    async (email: string, password: string) => {
      await api.login(email, password);
      setError(null);
    },
    [api, setError],
  );

  // The session gate. Everything below it may assume a user, which is what lets
  // screens call the api without each one re-checking.
  if (!user) return <Login version="" onSubmit={signIn} />;

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">
          JetStore <strong>Workspace</strong>
        </span>

        <nav className="mainnav" aria-label="Screens">
          {nav.map((item) => (
            <ShellNavItem key={item.to} item={item} />
          ))}
        </nav>

        <div className="spacer" />

        {/*
          The user's name is the way to the git profile screen, which is where
          the Flutter app bar puts it: an `ElevatedButton` labelled with the
          user's name that navigates to `userGitProfilePath`
          (`jetsclient/lib/components/app_bar.dart`). Task C.14 — it is a link
          rather than a nav item because no menu names that screen, in either
          app.
        */}
        <NavLink
          to="/git-profile"
          className={({ isActive }) => `user${isActive ? " is-active" : ""}`}
          title={`${user.email} — edit git profile`}
        >
          {user.name || user.email}
        </NavLink>
        <button
          type="button"
          className="btn"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          aria-label="Toggle colour theme"
        >
          {theme === "dark" ? "Light" : "Dark"}
        </button>
        <button type="button" className="btn" onClick={() => api.logout()}>
          Sign out
        </button>
      </header>

      {error != null && (
        <div className="banner banner-error" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => setError(null)} aria-label="Dismiss">
            ×
          </button>
        </div>
      )}
      {status != null && error == null && (
        <div className="banner banner-ok" role="status">
          <span>{status}</span>
          <button type="button" onClick={() => setStatus(null)} aria-label="Dismiss">
            ×
          </button>
        </div>
      )}

      <Outlet />
    </div>
  );
}

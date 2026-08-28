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

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";

import { ApiClient, type User } from "../api/client";
import { BASENAME } from "../base";
import { Login } from "../components/Login";
import { withReturnTo } from "../screens/routes";
import { ApiProvider, useCan } from "./capabilities";
import { RouteTitle } from "./documentTitle";
import { RouteFavicon } from "./favicon";
import { NotificationsProvider, useNotifications } from "./notifications";
import {
  CLIENT_LIST_QUERY,
  clientList,
  selectedClient,
  setClientList,
  setSelectedClient,
  subscribeToClient,
} from "./selectedClient";

export type Theme = "light" | "dark";

/** Kept under the Phase 1 key so a returning user keeps their choice. */
const THEME_KEY = "jetstore-ide-theme";

export interface NavItem {
  to: string;
  label: string;
  /** When set, the item is hidden unless the user holds the capability. */
  capability?: string;
}

/**
 * The client filter's slot in the row, as a nav entry rather than a fixed
 * position. Task D.3, from **I-259**, which specifies the order as
 * *Home | Filter Client | Workspaces | Workspace IDE | Proposals | Query Tool |
 * Infer Server Admin* — the filter sits second, between two links.
 *
 * **The array is the order**, which is why this is an entry and not a prop: the
 * alternative was rendering the picker after the first item by name, and a
 * position expressed as *after the item whose `to` is `/home`* is a rule nobody
 * can see from the array they are editing.
 */
export interface ClientFilterSlot {
  kind: "client-filter";
}

export type NavEntry = NavItem | ClientFilterSlot;

const isClientFilter = (e: NavEntry): e is ClientFilterSlot =>
  "kind" in e && e.kind === "client-filter";

/**
 * One entry of the flow menu the brand becomes. Task D.3, from **I-261**.
 *
 * `to` is a full route rather than a flow key, because **one of the five entries
 * is not a flow**: *Rules Config* is `screens/RuleConfig.tsx` at `/ruleConfig`,
 * which the report listed among the missing flows because nothing in the
 * navigation named it (**I-268**). Keying on flow ids would have forced that one
 * to be a special case.
 */
export interface FlowMenuItem {
  to: string;
  label: string;
}

export interface AppShellProps {
  api: ApiClient;
  nav: NavEntry[];
  /** When non-empty, the brand becomes the menu that opens these. */
  flowMenu?: FlowMenuItem[];
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
        {/* Task D.9 (I-267). Above `ShellChrome` rather than inside it because
            `ShellChrome` returns `<Login>` when there is no user, and the icon
            belongs to the url rather than to the session. See `favicon.ts`. */}
        <RouteFavicon />
        {/* D.10, from **I-272** — the same shape one element over. */}
        <RouteTitle />
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

/**
 * The client filter, which every screen's tables are queried against. Task C.6.
 *
 * **In the shell because it is the shell's in Flutter too** — a dropdown in the
 * left menu that `base_screen.dart` draws on every screen, writing to a router
 * field the query builder reads (`selectedClient.ts` has the whole argument).
 * Putting it on the home screen would have made a filter that narrows five other
 * screens' tables settable from one of them.
 *
 * `useSyncExternalStore` rather than `useState` plus an effect: the store is
 * module-level and mutable, and this is the hook that exists for exactly that —
 * it also lets a screen read `selectedClient()` directly without a context.
 *
 * The list is fetched once, when the shell mounts, and a failure is silent by
 * design — see the module header. Rendered before the spacer rather than beside
 * the user's name, so it sits on the navigation side as the Dart's does.
 */
function ClientPicker({ api }: { api: ApiClient }) {
  const clients = useSyncExternalStore(subscribeToClient, clientList);
  const selection = useSyncExternalStore(subscribeToClient, selectedClient);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const body = await api.dataTable<{ rows?: unknown }>({
          action: "raw_query",
          query: CLIENT_LIST_QUERY,
        });
        if (cancelled || !Array.isArray(body.rows)) return;
        setClientList(
          (body.rows as unknown[][]).map((row) => row[0]).filter((v): v is string => typeof v === "string"),
        );
      } catch {
        // Deliberately silent. The selection narrows what the user may already
        // see, so its absence shows more rows rather than fewer, and nothing in
        // the app depends on the list being populated.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [api]);

  return (
    <select
      className="clientpicker"
      aria-label="Filter Client"
      value={selection ?? ""}
      onChange={(event) => setSelectedClient(event.target.value)}
    >
      {/* The Dart's first entry is a label with no value, and choosing it clears
          the filter: `DropdownItemConfig(label: 'Filter Client')` with `value`
          left null (`jetsclient/lib/screens/base_screen.dart`). */}
      <option value="">Filter Client</option>
      {clients.map((client) => (
        <option key={client} value={client}>
          {client}
        </option>
      ))}
    </select>
  );
}

/**
 * The brand, as the way into the flows. Task D.3, from **I-261**.
 *
 * **The Flutter left panel's menu tree held twenty distinct labels and this row
 * holds six**, which is why the flows became unreachable when X.1 retired that
 * app: they were started from the tree, not from a screen. I-266 settled that
 * the panel does not come back, so the launcher lives here.
 *
 * **A `<details>` rather than a popover built out of state.** It closes on
 * `Escape` and on outside click for free, it is keyboard-reachable without a
 * roving tabindex, and it needs no effect that must be torn down — which is the
 * whole of what a menu this size owes. The click handler exists only to close it
 * after a choice; navigation is the `NavLink`'s.
 */
function FlowMenu({ items }: { items: FlowMenuItem[] }) {
  const ref = useRef<HTMLDetailsElement>(null);
  /**
   * **Where the user was when they opened the menu, so the flow can send them
   * back** (D.8, I-265). The menu is the launch point for four of the eleven
   * flows and it is reachable from every screen, so without this the four would
   * exit to the same place wherever they were started — which is the defect,
   * with `/home` in place of `/workspace`.
   *
   * `withReturnTo` leaves `/ruleConfig` alone: it is a screen rather than a flow
   * and nothing there reads the parameter.
   */
  const location = useLocation();
  const here = `${location.pathname}${location.search}`;
  return (
    <details className="flowmenu" ref={ref}>
      <summary className="brand" aria-label="Open a configuration flow">
        JetStore <strong>Workspace</strong>
      </summary>
      <nav className="flowmenu-items" aria-label="Configuration flows">
        {items.map((item) => (
          <NavLink
            key={item.to}
            to={withReturnTo(item.to, here)}
            className={({ isActive }) => `flowmenu-item${isActive ? " is-active" : ""}`}
            onClick={() => ref.current?.removeAttribute("open")}
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
    </details>
  );
}

function ShellChrome({ api, nav, flowMenu }: AppShellProps) {
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
  // `BASENAME` rather than a bare "/register": this is an href out of the
  // router's tree, so it carries the prefix the router would otherwise add.
  if (!user) return <Login version="" onSubmit={signIn} registerHref={`${BASENAME}/register`} />;

  return (
    <div className="app">
      <header className="topbar">
        {flowMenu && flowMenu.length > 0 ? (
          <FlowMenu items={flowMenu} />
        ) : (
          <span className="brand">
            JetStore <strong>Workspace</strong>
          </span>
        )}

        {/* The client filter is inside the row now, at the position I-259 gives
            it, rather than after the row. See `ClientFilterSlot`. */}
        <nav className="mainnav" aria-label="Screens">
          {nav.map((entry) =>
            isClientFilter(entry) ? (
              <ClientPicker key="client-filter" api={api} />
            ) : (
              <ShellNavItem key={entry.to} item={entry} />
            ),
          )}
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

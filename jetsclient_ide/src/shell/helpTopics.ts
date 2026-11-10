/**
 * The help map: which page of the documentation site the *Help* button opens
 * for the screen the user is on.
 *
 * The site (`docs/`) is served by the apiserver under `/docs/`, and a page's
 * address is its path under `docs/src/content/docs/` without the extension,
 * with a trailing slash; a folder's page is its `index.md`. So `user/` is
 * `docs/src/content/docs/user/index.md`, and `user/screens/home/` would be
 * `user/screens/home.md` or `user/screens/home/index.md`.
 *
 * **To give a screen its own page**, write the page and change the `page` of
 * the screen's route below. **To add a route**, add it to `App.tsx` and a row
 * here. `helpTopics.test.ts` fails when a route of `App.tsx` has no row, when a
 * row names a route `App.tsx` does not have, and when a `page` has no file
 * behind it — so renaming or moving a page the map names means updating the
 * map in the same change.
 *
 * Most routes point at the User Guide's landing page until their screen has a
 * page of its own: the table is complete in routes and sparse in pages.
 */

import { matchPath } from "react-router-dom";

export interface HelpTopic {
  /** The route pattern exactly as `App.tsx` writes it. */
  route: string;
  /** A page of the site, relative to `/docs/`, with a trailing slash. */
  page: string;
}

/** Where the documentation site is served. */
export const DOCS_PREFIX = "/docs/";

/** The page opened when no route matches: the User Guide's landing page. */
export const DEFAULT_HELP_PAGE = "user/";

/**
 * Every route of `App.tsx`, in the order matching is tried. The first match
 * wins, so `*` stays last.
 */
export const HELP_TOPICS: ReadonlyArray<HelpTopic> = [
  { route: "/register", page: "user/" },
  { route: "/", page: "user/" },
  { route: "login", page: "user/" },
  { route: "home", page: "user/" },
  { route: "workspace", page: "user/" },
  { route: "workspaces/:workspace_name/home", page: "user/" },
  { route: "workspaces", page: "user/" },
  { route: "inferServerAdmin", page: "user/" },
  { route: "proposals", page: "user/" },
  { route: "proposals/:proposalId", page: "user/" },
  { route: "incidents", page: "user/" },
  { route: "incidents/:incidentId", page: "user/" },
  { route: "detection", page: "user/" },
  { route: "runbooks", page: "user/" },
  { route: "workspaces/:workspace_name/runbooks", page: "user/" },
  { route: "runbooks/run/:run_key", page: "user/" },
  { route: "flow/:key", page: "user/" },
  { route: "git-profile", page: "user/" },
  { route: "domainTableViewer/:table_name/:session_id", page: "user/" },
  { route: "filePreviewPath/:file_key", page: "user/" },
  { route: "executionStatusDetails/:session_id", page: "user/" },
  { route: "executionStatsDetails/:session_id", page: "user/" },
  { route: "query-tool", page: "user/" },
  { route: "processErrors/:session_id", page: "user/" },
  { route: "userAdmin", page: "user/" },
  { route: "ruleConfig", page: "user/" },
  { route: "fileLoaderStatus", page: "user/" },
  { route: "*", page: "user/" },
];

/**
 * A user flow's page, by flow key (`flow/:key`). A flow not listed here gets
 * the `flow/:key` row of `HELP_TOPICS`.
 */
export const FLOW_TOPICS: Readonly<Record<string, string>> = {};

const FLOW_ROUTE = "/flow/:key";

/** The routes of `App.tsx` are relative to the shell at `/`. */
function absolute(route: string): string {
  return route.startsWith("/") ? route : `/${route}`;
}

/**
 * The address of the documentation page for `pathname`, a location inside the
 * router (the router's `basename` already removed).
 */
export function helpPathFor(pathname: string): string {
  const flow = matchPath(FLOW_ROUTE, pathname);
  const flowKey = flow?.params["key"];
  if (flowKey !== undefined) {
    const page = FLOW_TOPICS[flowKey];
    if (page !== undefined) return DOCS_PREFIX + page;
  }
  for (const topic of HELP_TOPICS) {
    if (matchPath(absolute(topic.route), pathname) !== null) return DOCS_PREFIX + topic.page;
  }
  return DOCS_PREFIX + DEFAULT_HELP_PAGE;
}

/** What `openHelp` needs of the tab `window.open` returns. */
export interface HelpTab {
  opener: unknown;
  location: { href: string };
  close(): void;
}

/**
 * Opens the documentation for `pathname` in a new tab. Resolves to an error
 * message for the banner, or `null` when the tab was pointed at the page.
 *
 * **The tab is opened before anything is awaited.** A browser allows a new tab
 * only while it is handling the user's click, and the session call has to
 * finish first — it sets the cookie the site requires — so the tab is opened
 * blank, synchronously, and given its address once the call has answered.
 * `opener` is cleared before the tab leaves this origin's blank page.
 */
export async function openHelp(
  pathname: string,
  openDocsSession: () => Promise<void>,
  openTab: () => HelpTab | null = () => window.open("", "_blank"),
): Promise<string | null> {
  const tab = openTab();
  if (tab === null) {
    return "The browser blocked the documentation tab. Allow pop-ups for this site and press Help again.";
  }
  try {
    await openDocsSession();
  } catch (err) {
    tab.close();
    return err instanceof Error && err.message !== ""
      ? err.message
      : "The documentation could not be opened.";
  }
  tab.opener = null;
  tab.location.href = helpPathFor(pathname);
  return null;
}

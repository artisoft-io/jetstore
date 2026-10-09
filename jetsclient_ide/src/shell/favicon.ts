/**
 * Which favicon a route gets. Task D.9, from **I-267**.
 *
 * **The report is that the Workspace IDE's icon is served everywhere**, and it
 * is: `index.html` declares one `<link rel="icon">` and a static document cannot
 * vary by route. So the fix is a second asset and a switch — and the interesting
 * half is which of the two the *static* document declares.
 *
 * ## The default is the JetStore icon, and the effect is the exception
 *
 * The obvious shape is the other way round: leave `index.html` naming
 * `favicon.svg` and swap it out on every route that is not the editor. Three
 * things argue against it, and the third is the one that decides it.
 *
 *  - **The static document is the pre-hydration answer.** Whatever it names is
 *    what a tab shows while the bundle loads, and what it shows if the bundle
 *    never runs. The right answer for all but two of the app's routes is the
 *    JetStore icon, so that is what the document should name.
 *  - **It fails safe.** If this module is never mounted — deleted, thrown from,
 *    or mounted below a component that threw — every page shows the JetStore
 *    icon, which is correct everywhere except the editor. The inverted design
 *    fails back to exactly the defect being fixed here.
 *  - **`/register` is outside the shell.** It is the app's only unauthenticated
 *    route and it is a sibling of `<AppShell>` rather than a child
 *    (`App.tsx`, the `path="/register"` route), so no effect mounted in the shell
 *    can reach it. Under the inverted design that route would keep the editor's
 *    icon permanently, and the fix would be a second mount point. Under this one
 *    it is already right and needs no code at all.
 *
 * The cost is a brief flash of the JetStore icon when the editor is loaded cold,
 * which is real and is the smaller of the two: it is one route rather than
 * nineteen, and it lasts as long as the bundle takes to hydrate.
 *
 * ## Why the url is built from `BASE_URL` and not from `BASENAME`
 *
 * They are the same string today — both root — which is precisely why the wrong
 * one would never be caught. `BASENAME` (`src/base.ts`) is the **router's**
 * prefix; the prefix an *asset* url carries is vite's `base`, which is what
 * `import.meta.env.BASE_URL` holds and what vite rewrites `index.html`'s
 * leading-slash `href` with at build time. `base.ts` requires the two to agree,
 * so either would work — but only one is right by definition, and it is the one
 * that survives the pair being separated.
 *
 * `useLocation().pathname` is already stripped of the router's basename, so the
 * patterns below are the route templates from `App.tsx` verbatim.
 */

import { useEffect } from "react";
import { matchPath, useLocation } from "react-router-dom";

/**
 * The routes that render `WorkspaceIde`, as templates rather than strings.
 *
 * Both of them, because both are the editor: `/workspace` is the bare screen and
 * the other is the same screen addressed at a workspace (`App.tsx`, task C.3).
 * `/workspaces` is **not** here — it is the registry, a different screen.
 *
 * Matched with `matchPath` rather than compared, because the second carries a
 * parameter. That is also what keeps this list readable as a copy of the route
 * table: if a third route starts rendering the editor, the line to add here is
 * the same line that was added there.
 */
export const IDE_ROUTES = ["/workspace", "/workspaces/:workspace_name/home"];

/**
 * The original JetStore favicon, recovered rather than redrawn.
 *
 * It is `jetsclient/web/favicon.ico` byte for byte, taken from the commit before
 * X.1 deleted the Flutter app. It was added deliberately in `5c94ff7c` (*"new
 * favicon Fixes #420"*) and never touched again — so it is the mark the product
 * shipped with, not the Flutter template's default. The template's default is
 * still in that history as `web/favicon.png`, unmodified and unused, which is
 * why the Flutter `index.html` names the `.ico` explicitly.
 */
const JETSTORE_ICON = { file: "favicon.ico", type: "image/x-icon" } as const;

/** The editor's own icon, unchanged: the report asks for it to stay. */
const WORKSPACE_IDE_ICON = { file: "favicon.svg", type: "image/svg+xml" } as const;

/** Exported for the test; the component below is what the app mounts. */
export function faviconFor(pathname: string) {
  return IDE_ROUTES.some((pattern) => matchPath(pattern, pathname))
    ? WORKSPACE_IDE_ICON
    : JETSTORE_ICON;
}

/**
 * Keeps `<link rel="icon">` in step with the route. Renders nothing.
 *
 * A component rather than a hook called from `ShellChrome`, so that it sits
 * above the session gate: `ShellChrome` returns `<Login>` when there is no user,
 * and the icon is chrome rather than session state — a signed-out visitor to
 * `/workspace` is still looking at the editor's url.
 *
 * It writes the element rather than replacing it, and creates one if the
 * document has none. The second case does not arise in the app — `index.html`
 * declares the link — but it is what lets a test mount this against a bare
 * `document`.
 */
export function RouteFavicon(): null {
  const { pathname } = useLocation();
  useEffect(() => {
    const icon = faviconFor(pathname);
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!link) {
      link = document.createElement("link");
      link.setAttribute("rel", "icon");
      document.head.appendChild(link);
    }
    link.setAttribute("type", icon.type);
    link.setAttribute("href", `${import.meta.env.BASE_URL}${icon.file}`);
  }, [pathname]);
  return null;
}

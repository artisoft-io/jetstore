/**
 * @vitest-environment jsdom
 *
 * The per-route tab title. Task D.10, from **I-272**.
 *
 * **The interesting assertion is the one that reads `App.tsx`, in both
 * directions.** `favicon.test.tsx` has the same shape for two routes; this table
 * claims to cover *every* route, so the check has to be able to see a route with
 * no title as well as a title with no route. A one-directional check meeting the
 * direction it cannot see is the failure C.6 recorded and `routes.test.ts` fixed
 * three tasks late; there is no reason to write a fresh one.
 */

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { PRODUCT, ROUTE_TITLES, RouteTitle, titleFor, titleNameFor } from "./documentTitle";

afterEach(cleanup);

/**
 * **From the working directory rather than from `import.meta.url`**, which
 * `routes.test.ts` and `table.test.ts` both use. Under `@vitest-environment
 * jsdom` that url is jsdom's `http://localhost/…` rather than a `file:` one and
 * `readFileSync` refuses it — the effect below needs a DOM, so this file is
 * jsdom and pays for it here. Vitest's root is the package directory.
 */
const appSource = readFileSync(resolve(process.cwd(), "src/App.tsx"), "utf8");

/**
 * Every path `App.tsx` routes, with the basename's leading slash back on.
 *
 * `path="*"` and `path="/"` are dropped: the first is `NotFound`'s catch-all and
 * the second is the shell's layout route, and neither is a screen. The `index`
 * route has no `path` at all and so is not in this list — it redirects to
 * `/home`, whose title it borrows for the instant it exists.
 */
const routedPaths = [...appSource.matchAll(/path="([^"]+)"/g)]
  .map((m) => m[1]!)
  .filter((p) => p !== "*" && p !== "/")
  .map((p) => (p.startsWith("/") ? p : `/${p}`));

describe("the route title table", () => {
  it("names every route App.tsx serves, and no route it does not", () => {
    const titled = new Set(ROUTE_TITLES.map(([pattern]) => pattern));
    const routed = new Set(routedPaths);
    expect(
      [...routed].filter((p) => !titled.has(p)).sort(),
      "this route has no title, so its tab falls back to the product name",
    ).toEqual([]);
    expect(
      [...titled].filter((p) => !routed.has(p)).sort(),
      "this title names a route that has gone",
    ).toEqual([]);
  });

  it("is in App.tsx's order, which is what makes the two readable against each other", () => {
    expect(ROUTE_TITLES.map(([pattern]) => pattern)).toEqual(routedPaths);
  });

  it("matches to the end, so a prefix route does not answer for a deeper one", () => {
    // The registry and the editor are `/workspaces` and
    // `/workspaces/:workspace_name/home`, and the first is a prefix of the
    // second. `matchPath` with a string pattern is exact, so list order does not
    // decide this — which is worth an assertion because the obvious reading of a
    // first-match loop is that it does, and a table ordered on that belief would
    // pass every other case in this file.
    expect(titleNameFor("/workspaces/cgt/home")).toBe("Workspace IDE");
    expect(titleNameFor("/workspaces")).toBe("Workspaces");
  });

  it("gives the product alone for a path no route matches", () => {
    // `/` before its redirect, and anything `NotFound` answers.
    expect(titleNameFor("/no/such/screen")).toBeNull();
    expect(titleFor("/no/such/screen")).toBe(PRODUCT);
  });

  it("suffixes the product, so a tab says which application it is", () => {
    expect(titleFor("/home")).toBe("Home — JetStore");
    expect(titleFor("/processErrors/sess-1")).toBe("Process Errors — JetStore");
  });
});

describe("the effect", () => {
  it("writes the title of the route it is mounted under", () => {
    render(
      <MemoryRouter initialEntries={["/query-tool"]}>
        <Routes>
          <Route path="*" element={<RouteTitle />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(document.title).toBe("Query Tool — JetStore");
  });

  it("says Flow for the runner's route, which the runner then replaces", () => {
    // A flow's name is in its document rather than in its url, so this table can
    // only say *Flow*; `FlowRunner` calls `useDocumentTitleDetail` with the
    // document's own `title` once it loads. The two do not race — see the
    // module's header.
    expect(titleNameFor("/flow/loadFilesUF")).toBe("Flow");
  });
});

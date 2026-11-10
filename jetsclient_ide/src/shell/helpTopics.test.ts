/**
 * The help map against the two things it points between: the routes of
 * `App.tsx` and the pages of the documentation site.
 *
 * Both are read from the file system, so this fails when a route is added
 * without a row, when a row outlives its route, and when a mapped page is
 * renamed or removed — the three ways the map goes stale without anything
 * else noticing.
 */

import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { DEFAULT_HELP_PAGE, FLOW_TOPICS, HELP_TOPICS, helpPathFor } from "./helpTopics";

const here = fileURLToPath(new URL("./", import.meta.url));
const contentRoot = `${here}../../../docs/src/content/docs/`;
const appSource = `${here}../App.tsx`;

/**
 * The file behind a page address, or null. `user/` is `user/index.md`; `a/b/`
 * may also be `a/b.md` or `a/b.mdx`.
 */
function pageFile(page: string): string | null {
  if (!page.endsWith("/")) return null;
  const candidates = [`${page}index.md`, `${page}index.mdx`];
  if (page !== "") {
    const bare = page.slice(0, -1);
    candidates.push(`${bare}.md`, `${bare}.mdx`);
  }
  return candidates.find((c) => existsSync(contentRoot + c)) ?? null;
}

/** Every `path="…"` of a `<Route>` element in `App.tsx`. */
function appRoutes(): string[] {
  const source = readFileSync(appSource, "utf8");
  return [...source.matchAll(/<Route\b[^>]*?\spath="([^"]+)"/g)].map((m) => m[1] ?? "");
}

describe("the help map's pages", () => {
  const pages = [
    DEFAULT_HELP_PAGE,
    ...HELP_TOPICS.map((t) => t.page),
    ...Object.values(FLOW_TOPICS),
  ];

  it("finds the site's content", () => {
    expect(existsSync(contentRoot)).toBe(true);
  });

  it.each([...new Set(pages)])("page %s has a file under docs/src/content/docs/", (page) => {
    expect(
      pageFile(page),
      `help page "${page}" has no index.md, .md or .mdx under docs/src/content/docs/`,
    ).not.toBeNull();
  });
});

describe("the help map's routes", () => {
  const routes = appRoutes();
  const mapped = HELP_TOPICS.map((t) => t.route);

  it("reads App.tsx's routes", () => {
    // A guard on the regular expression: if it stops matching, the two tests
    // below would pass on an empty list.
    expect(routes.length).toBeGreaterThan(20);
  });

  it("maps every route of App.tsx", () => {
    expect(routes.filter((r) => !mapped.includes(r))).toEqual([]);
  });

  it("maps no route App.tsx does not have", () => {
    expect(mapped.filter((r) => !routes.includes(r))).toEqual([]);
  });

  it("lists each route once, with the catch-all last", () => {
    expect(new Set(mapped).size).toBe(mapped.length);
    expect(mapped[mapped.length - 1]).toBe("*");
  });
});

describe("helpPathFor", () => {
  it("maps a known route", () => {
    expect(helpPathFor("/home")).toBe("/docs/user/");
  });

  it("maps a parameterised route", () => {
    expect(helpPathFor("/workspaces/ws1/home")).toBe("/docs/user/");
    expect(helpPathFor("/processErrors/1234")).toBe("/docs/user/");
  });

  it("maps an unknown path to the User Guide", () => {
    expect(helpPathFor("/no/such/screen")).toBe("/docs/user/");
  });

  it("takes the page of the first matching row", () => {
    // Read the table rather than assume today's pages, so this holds when the
    // screens get pages of their own.
    const row = HELP_TOPICS.find((t) => t.route === "workspaces/:workspace_name/runbooks");
    expect(row).toBeDefined();
    expect(helpPathFor("/workspaces/ws1/runbooks")).toBe(`/docs/${row?.page}`);
  });

  it("maps a listed flow to its page and an unlisted one to the flow route's", () => {
    const topics = FLOW_TOPICS as Record<string, string>;
    const flowRow = HELP_TOPICS.find((t) => t.route === "flow/:key");
    expect(helpPathFor("/flow/notAFlow")).toBe(`/docs/${flowRow?.page}`);

    // FLOW_TOPICS is empty today; list one for the duration of the case.
    topics["someFlow"] = "developer/";
    try {
      expect(helpPathFor("/flow/someFlow")).toBe("/docs/developer/");
      expect(helpPathFor("/flow/otherFlow")).toBe(`/docs/${flowRow?.page}`);
    } finally {
      delete topics["someFlow"];
    }
  });
});

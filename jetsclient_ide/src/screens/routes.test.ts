/**
 * The served-screen map. Task C.6.
 *
 * **The interesting assertion is the one that reads `App.tsx`.** A row here is a
 * claim that this app serves a screen, and the failure it is guarding against is
 * a row that outlives its route — a table action would then navigate in-app to a
 * path nothing matches, which C.16 answers with a 404 rather than with the
 * Flutter screen that would have worked. So the map is checked against the route
 * table rather than against a list somebody keeps by hand.
 */

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import {
  FLOW_EXIT_FALLBACK,
  FLOW_ROUTES,
  RETURN_TO,
  SERVED_SCREENS,
  inAppPath,
  isInAppPath,
  reactFlowRoute,
  reactScreenPath,
  returnToPath,
  withReturnTo,
} from "./routes";
import screenConfigs from "./fixtures/screen_configs.json";

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");



/** Every path this app routes, as `App.tsx` writes them, with the basename's slash back on. */
/**
 * Every `configScreenPath` any table document names, derived rather than listed.
 *
 * **This replaces `NOT_CLAIMED`, which listed Flutter templates this app declined
 * to serve and why.** That list was a decision register while there were two apps;
 * with one app there is nothing to decline, and what matters instead is that every
 * path a button carries resolves somewhere. Derived from the corpus so a new
 * button is covered on the day it is authored rather than when somebody remembers
 * to add a row.
 *
 * `screen_configs.json` is the screens' corpus; the flows' tables are the
 * installed workspace assets, and both are read because a `configScreenPath` can
 * sit on either.
 */
const CONFIG_SCREEN_PATHS: string[] = (() => {
  const found = new Set<string>();
  const walk = (node: unknown): void => {
    if (Array.isArray(node)) {
      for (const item of node) walk(item);
      return;
    }
    if (node === null || typeof node !== "object") return;
    const record = node as Record<string, unknown>;
    const path = record["configScreenPath"];
    if (typeof path === "string" && path.length > 0) found.add(path);
    for (const value of Object.values(record)) walk(value);
  };
  walk(screenConfigs);
  for (const document of Object.values(
    import.meta.glob("../../../jets/workspace_assets/table_configs/*.tc.json", {
      eager: true,
    }) as Record<string, unknown>,
  )) {
    walk(document);
  }
  for (const document of Object.values(
    import.meta.glob("../datatable/tables/*.tc.json", { eager: true }) as Record<string, unknown>,
  )) {
    walk(document);
  }
  return [...found].sort();
})();

describe("the served-screen map", () => {
  it("names only paths App.tsx actually routes", () => {
    // `<Route path="executionStatusDetails/:session_id"` — the router's paths are
    // relative to `basename`, so the leading slash is dropped.
    const missing = Object.values(SERVED_SCREENS)
      .map((s) => s.reactPath.replace(/^\//, ""))
      .filter((path) => !appSource.includes(`path="${path}"`));
    expect(
      missing,
      "a row here whose route has gone is a table button that navigates to the 404 instead of to Flutter",
    ).toEqual([]);
  });

  it("covers the four ScreenOne routes, which is F68's whole set", () => {
    // C.7, C.8, C.11 and C.12. Named as a set rather than counted, because the
    // claim is that this app now serves all four rather than that it serves four.
    for (const template of [
      "/executionStatusDetails/:session_id",
      "/executionStatsDetails/:session_id",
      "/domainTableViewer/:table_name/:session_id",
      "/filePreviewPath/:file_key",
    ]) {
      expect(SERVED_SCREENS[template]).toBeTruthy();
    }
  });

  it("holds no flow route, because which app owns a flow is not a compiled list", () => {
    // `routing.ts` decides that from what the workspace holds, and a second place
    // to record it is the failure its header exists to prevent. The home screen's
    // `startPipeline` and `setHomeFilters` buttons both name one of these.
    for (const template of ["/startPipelineUF", "/configureHomeFiltersUF"]) {
      expect(SERVED_SCREENS[template]).toBeUndefined();
      expect(reactScreenPath(template, {})).toBeNull();
    }
  });

  it("substitutes the parameters the action resolved", () => {
    expect(
      reactScreenPath("/executionStatusDetails/:session_id", { session_id: "sess-1" }),
    ).toBe("/executionStatusDetails/sess-1");
    expect(
      reactScreenPath("/domainTableViewer/:table_name/:session_id", {
        table_name: "claim_staging",
        session_id: "sess-1",
      }),
    ).toBe("/domainTableViewer/claim_staging/sess-1");
  });

  it("renames only where a task decided to, and says which", () => {
    // One row of the seven. C.4 serves the Query Tool at `/query-tool`; every
    // other React path is the Flutter template verbatim, which is what makes a
    // handoff a prefix change rather than a translation.
    const renamed = Object.entries(SERVED_SCREENS).filter(([template, s]) => template !== s.reactPath);
    expect(renamed.map(([template]) => template)).toEqual(["/queryTool"]);
  });

  /**
   * **Every navigation target in the corpus resolves in this app.**
   *
   * This replaces three checks that compared `SERVED_SCREENS` against the Flutter
   * route table, which X.1 deleted. Those asked *does this app claim a screen the
   * other one declares* — a question with no referent once there is one app — and
   * what replaces them is stronger rather than weaker: it asks the question the
   * user experiences, which is whether pressing a button goes anywhere.
   *
   * A `configScreenPath` that resolves to nothing is now a reported error rather
   * than a hand-off to Flutter (`unservedScreenMessage`), so this is the check
   * that keeps that message unreachable.
   */
  it("resolves every configScreenPath the table corpus names", () => {
    const params: Record<string, string> = {
      session_id: "s",
      table_name: "t",
      object_type: "o",
      file_key: "f",
      key: "k",
      workspace_name: "w",
      workspace_branch: "b",
      feature_branch: "fb",
      workspace_uri: "u",
      startAtKey: "a",
    };
    // **The list is derived, so its emptiness has to be asserted separately.** A
    // glob that matched nothing would make the filter below pass over zero paths
    // and report success — the vacuous-pass failure C.0 found in the corpus tests,
    // where a corpus that does not move is not evidence a deletion was inert.
    expect(CONFIG_SCREEN_PATHS.length).toBeGreaterThanOrEqual(10);

    const unresolved = CONFIG_SCREEN_PATHS.filter((p) => inAppPath(p, params) === null);
    expect(
      unresolved,
      "a table action names this path and nothing serves it, so pressing that button reports an error",
    ).toEqual([]);
  });

  it("routes flows through FLOW_ROUTES and screens through SERVED_SCREENS", () => {
    // The two maps are keyed the same way and answer different questions, so a
    // template in both would be ambiguous — and `inAppPath` asks the screen map
    // first, which would silently win.
    const inBoth = Object.keys(FLOW_ROUTES).filter((t) => SERVED_SCREENS[t] !== undefined);
    expect(inBoth).toEqual([]);

    // The eleven, which is the whole corpus (`user_flows/` holds eleven `.uf.json`
    // plus three projections, and a projection has no Flutter route).
    expect(Object.keys(FLOW_ROUTES)).toHaveLength(11);
  });

  it("carries a flow's parameters as a query string, and refuses a partial one", () => {
    expect(reactFlowRoute("/workspaces/loadConfigUF/:workspace_name", { workspace_name: "cgt" })).toBe(
      "/flow/loadConfigUF?workspace_name=cgt",
    );
    // F.10's decision, kept: a flow whose arguments are absent is not opened with
    // an empty worksheet.
    expect(reactFlowRoute("/fileMappingUF/mapping/:table_name/:object_type", { table_name: "t" })).toBeNull();
  });

  it("returns null for a template neither app routes, and for none at all", () => {
    expect(reactScreenPath("/noSuchScreen", {})).toBeNull();
    expect(reactScreenPath(undefined, {})).toBeNull();
  });
});

/**
 * Where a flow goes when it ends. Task D.8, from **I-265**.
 *
 * The end-to-end behaviour is `FlowRunner.test.tsx`'s — a flow finishing and the
 * app arriving somewhere. What is here is the part with a decision in it: which
 * strings this app is willing to navigate to on the say-so of a query parameter.
 */
describe("the origin a flow url carries", () => {
  it("marks a flow path and leaves a screen path alone", () => {
    expect(withReturnTo("/flow/loadFilesUF", "/home")).toBe("/flow/loadFilesUF?returnTo=%2Fhome");
    // `/ruleConfig` is in the launcher and is not a flow; nothing there reads it.
    expect(withReturnTo("/ruleConfig", "/home")).toBe("/ruleConfig");
  });

  it("keeps the flow's own arguments, which travel in the same query string", () => {
    const to = withReturnTo("/flow/loadConfigUF?workspace_name=cgt", "/workspaces");
    expect(new URLSearchParams(to.split("?")[1]).get("workspace_name")).toBe("cgt");
    expect(returnToPath(new URLSearchParams(to.split("?")[1]))).toBe("/workspaces");
  });

  it("does not overwrite an origin the caller already chose", () => {
    expect(withReturnTo("/flow/x?returnTo=%2Fworkspaces", "/home")).toBe(
      "/flow/x?returnTo=%2Fworkspaces",
    );
  });

  it("refuses anything that is not a path inside this app", () => {
    // A protocol-relative url passes `startsWith("/")` and is followed off-site
    // by a browser, which is the case worth naming: the parameter is written by
    // whoever composed the link rather than by this app.
    expect(isInAppPath("//evil.example.com")).toBe(false);
    expect(isInAppPath("https://evil.example.com")).toBe(false);
    expect(isInAppPath("/\\evil.example.com")).toBe(false);
    expect(isInAppPath("home")).toBe(false);
    expect(isInAppPath("/home")).toBe(true);
    expect(isInAppPath("/flow/loadFilesUF?a=b")).toBe(true);
  });

  it("reads nothing back from a url that carries a refused origin", () => {
    expect(returnToPath(new URLSearchParams(`${RETURN_TO}=//evil.example.com`))).toBeNull();
    expect(returnToPath(new URLSearchParams(`${RETURN_TO}=`))).toBeNull();
    expect(returnToPath(new URLSearchParams(""))).toBeNull();
  });

  it("falls back to a route the app serves and no capability gates", () => {
    // The value is asserted rather than the constant, because what makes `/home`
    // the right answer is that it is the index and is ungated — `/workspace`, the
    // fallback until D.8, is neither.
    expect(FLOW_EXIT_FALLBACK).toBe("/home");
  });
});

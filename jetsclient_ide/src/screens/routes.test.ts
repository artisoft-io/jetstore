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

import { SERVED_SCREENS, reactScreenPath } from "./routes";

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");

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

  it("returns null for a screen track C has not ported", () => {
    // C.9's and C.10's. A miss is a working link into Flutter, which is the right
    // answer until they land.
    expect(reactScreenPath("/processErrors/:session_id", { session_id: "s" })).toBeNull();
    expect(reactScreenPath("/ruleConfig", {})).toBeNull();
    expect(reactScreenPath(undefined, {})).toBeNull();
  });
});

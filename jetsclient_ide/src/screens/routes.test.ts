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
import screenConfigs from "./fixtures/screen_configs.json";

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const flutterRoutes = readFileSync(
  new URL("../../../jetsclient/lib/routes/jets_routes_app.dart", import.meta.url),
  "utf8",
);

/** Every route template the Flutter app declares, from its own constants. */
const flutterTemplates = [...flutterRoutes.matchAll(/^const \w+Path = '([^']+)';/gm)].map(
  (m) => m[1]!,
);

/** Every path this app routes, as `App.tsx` writes them, with the basename's slash back on. */
const reactPaths = [...appSource.matchAll(/<Route\s+path="([^"]+)"/g)]
  .map((m) => m[1]!)
  .filter((p) => p !== "*" && p !== "/")
  .map((p) => (p.startsWith("/") ? p : `/${p}`));

/**
 * Flutter templates this app deliberately does not claim, each with its reason.
 *
 * **The list is the point.** The check below requires a `SERVED_SCREENS` row for
 * every Flutter template this app also routes; an entry here is a statement that
 * the omission is a decision. Adding one to silence the check is the thing to
 * refuse — the reason has to be true.
 */
const NOT_CLAIMED: Readonly<Record<string, string>> = {
  // Which app owns a flow is what the workspace holds, not a compiled list
  // (`userflow/routing.ts`, `appForFlow`). Duplicating it here is the failure that
  // file's header exists to prevent.
  "/clientRegistryUF/:startAtKey": "flow",
  "/sourceConfigUF/:startAtKey": "flow",
  "/fileMappingUF": "flow",
  "/fileMappingUF/mapping/:table_name/:object_type": "flow",
  "/pipelineConfigUF": "flow",
  "/loadFilesUF": "flow",
  "/registerFileKeyUF": "flow",
  "/startPipelineUF": "flow",
  "/configureHomeFiltersUF": "flow",
  "/workspaces/loadConfigUF/:workspace_name": "flow",
  // Track X's, not track C's: porting these is the moment the Flutter app stops
  // being reachable, which is a retirement decision (`sizing_screen_migration.md`
  // section 7, C.15).
  "/login": "track X",
  "/register": "track X",
  // This app answers an unmatched path itself (C.16, `NotFound.tsx`); there is no
  // handoff to claim, and a row would make a table action navigate to the 404 on
  // purpose.
  "/404": "this app answers it directly",
  // `/` is the Flutter home. This app serves Home at `/home` and keeps its index
  // redirecting to the editor, because every arrival at `/ide/` is a
  // `workspace_ide` holder who pressed *Code Editor* (C.6, I-217).
  "/": "different entry point by decision",
  // Reached from code rather than from a `configScreenPath`, so no table action
  // can name it and a row would never be read.
  "/workspaces/:workspace_name/home": "code-reached, not named by any action",
};

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

  it("has a row for every Flutter screen this app also serves", () => {
    // **The direction the map was missing, and the reason three rows were three
    // tasks late.** The assertion above checks rows against routes; nothing checked
    // routes against rows, and that gap has no symptom — `reactScreenPath` returns
    // `null` for an absent row exactly as it does for a screen this app does not
    // serve, so the home screen hands a user to Flutter for a screen React ported
    // hours earlier and every test stays green.
    //
    // Derived rather than listed, so a screen that lands without its row fails here
    // instead of being noticed by somebody reading a handoff.
    const owed = flutterTemplates.filter(
      (template) =>
        NOT_CLAIMED[template] === undefined &&
        SERVED_SCREENS[template] === undefined &&
        reactPaths.some((p) => p === template),
    );
    expect(
      owed,
      "this app routes these Flutter templates and claims none of them, so a table action naming one leaves the app for a screen it serves",
    ).toEqual([]);
  });

  it("names a reason for every Flutter template it declines", () => {
    // An entry in `NOT_CLAIMED` is a decision, so it has to correspond to a real
    // template — a stale one would silence the check above for a screen that no
    // longer exists, which is the same failure one level up.
    const stale = Object.keys(NOT_CLAIMED).filter((t) => !flutterTemplates.includes(t));
    expect(stale, "NOT_CLAIMED names a template the Flutter app no longer declares").toEqual([]);
  });

  it("cannot be used to excuse a screen this app serves and an action can name", () => {
    // **The escape hatch, closed — and it was open until it was mutation-tested.**
    // The check above requires a row for every Flutter template this app routes,
    // and deleting a row *plus* adding a `NOT_CLAIMED` entry made it pass again:
    // the excuse silenced exactly the case the check exists for.
    //
    // An entry is legitimate on one of two grounds, and both are checkable. Either
    // this app does not route the screen at all, or it routes it and **no table
    // action can name it**, which is `/workspaces/:workspace_name/home` — reached
    // from code, so a `configScreenPath` never carries it and the row would never
    // be read. `screen_configs.json` is the measurement of the second, not a
    // reading of it.
    const named = new Set(
      Object.values(screenConfigs.tables).flatMap((t) =>
        [...(t.actions ?? []), ...(t.secondRowActions ?? [])]
          .map((a) => (a as { configScreenPath?: string }).configScreenPath)
          .filter((p): p is string => typeof p === "string" && p.length > 0),
      ),
    );
    const excused = Object.keys(NOT_CLAIMED).filter(
      (template) => reactPaths.includes(template) && named.has(template),
    );
    expect(
      excused,
      "this app routes the screen and a table action names it, so declining it is not a decision — it is the missing row the check above is for",
    ).toEqual([]);
  });

  it("returns null for a template neither app routes, and for none at all", () => {
    expect(reactScreenPath("/noSuchScreen", {})).toBeNull();
    expect(reactScreenPath(undefined, {})).toBeNull();
  });
});

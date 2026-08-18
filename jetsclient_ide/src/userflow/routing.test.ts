/**
 * Tests for the flow routing switch (task S.8).
 *
 * Small, because the decision is the deliverable and the decision is that there
 * is nothing to dispatch — see `routing.ts`. What is worth pinning is that the
 * switch follows the workspace and that a handoff is a full URL rather than a
 * router path, since crossing apps is the one navigation neither router can do.
 */

import { describe, expect, it } from "vitest";

import { appForFlow, flutterFlowPath, handoffFor, reactFlowPath } from "./routing";

const migrated = new Set(["loadFilesUF", "registerFileKeyUF"]);

describe("which app owns a flow", () => {
  it("follows the workspace, not a compiled list", () => {
    expect(appForFlow("loadFilesUF", migrated)).toBe("react");
    expect(appForFlow("pipelineConfigUF", migrated)).toBe("flutter");
    // Reversible: remove the documents and the flow is Flutter's again, which a
    // cutover cannot offer.
    expect(appForFlow("loadFilesUF", new Set())).toBe("flutter");
  });
});

describe("the handoff", () => {
  it("is null when the current app already owns the flow", () => {
    expect(handoffFor("loadFilesUF", "react", migrated)).toBeNull();
    expect(handoffFor("pipelineConfigUF", "flutter", migrated)).toBeNull();
  });

  it("sends Flutter to the React path, under the IDE base", () => {
    expect(handoffFor("loadFilesUF", "flutter", migrated)).toBe("/ide/flow/loadFilesUF");
  });

  it("sends React back to the Flutter path, which is a fragment", () => {
    // The `#` is why the two URL spaces cannot collide, and why the server has
    // no catch-all and needs none.
    expect(handoffFor("pipelineConfigUF", "react", migrated)).toBe("/#/pipelineConfigUF");
  });

  it("respects a different IDE base rather than hard-coding one", () => {
    // I-26 records that `/ide/` is a misnomer; when it moves, this moves with it.
    expect(handoffFor("loadFilesUF", "flutter", migrated, "/app")).toBe("/app/flow/loadFilesUF");
  });

  it("keeps the two path helpers distinguishable", () => {
    expect(reactFlowPath("x")).toBe("/flow/x");
    expect(flutterFlowPath("x")).toBe("/#/x");
  });
});

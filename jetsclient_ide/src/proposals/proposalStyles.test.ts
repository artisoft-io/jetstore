/**
 * The proposal page's layout rule that jsdom cannot see.
 *
 * "What the run produced" was drawn as one clipped line on any proposal taller
 * than the window: `.main` is a flex column (`styles.css`), its children may
 * shrink to fit it, and `.proposal-artifact` has `overflow-x: auto`, which takes
 * away the content minimum a flex item otherwise keeps. jsdom draws no layout,
 * so the rule is asserted where it lives, in the node environment, as
 * `dialogStyles.test.ts` asserts its own.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const css = readFileSync(fileURLToPath(new URL("./proposals.css", import.meta.url)), "utf8");

describe("the proposal page's stylesheet", () => {
  it("keeps every block of the proposal at its own height in the page's flex column", () => {
    expect(css).toMatch(/\.proposal-detail\s*>\s*\*\s*\{[^}]*flex-shrink:\s*0/);
  });

  it("keeps the artifact's line breaks", () => {
    expect(css).toMatch(/\.proposal-artifact[^{]*\{[^}]*white-space:\s*pre-wrap/);
  });
});

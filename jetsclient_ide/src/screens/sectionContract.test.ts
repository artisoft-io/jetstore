/**
 * The Workspace IDE's section contract, from a third end. Task C.3.
 *
 * **C.1 guarded this surface with two tests that check each other**, because the
 * section list is server data and no client-side corpus reaches it:
 * `jets/datatable/wsfile/sections_test.go` and
 * `jetsclient/test/workspace_section_contract_test.dart` carry the same
 * declaration string and the same checksum, so changing the Go table fails the
 * Go test and updating the Go constant alone fails the Dart one. Its note said a
 * third reader owes the same treatment; this app became one when it started
 * rendering compiled views, so this file is the third copy.
 *
 * What it forces at the moment a section changes is the same question, asked of
 * this app: **does this section's files compile into `workspace.db`, and if so
 * does this app render the view?**
 *
 * **The two clients answer it differently and that is the interesting part.**
 * `lookups` is in the Dart's `viewsNotBuiltInFlutter` permanently — track X
 * deletes that app, so a view built there is discarded by construction (I-45) —
 * and in this app's `viewsNotBuiltInReact` only until C.3a.
 */

import { describe, expect, it } from "vitest";

import {
  compiledViewFor,
  compiledViews,
  EXPECTED_DECLARATION_CHECKSUM,
  fnv1a32,
  parseDeclaration,
  SERVER_SECTION_DECLARATION,
  viewsNotBuiltInReact,
} from "./sectionContract";

describe("the section contract", () => {
  it("is the declaration the apiserver produces", () => {
    expect(fnv1a32(SERVER_SECTION_DECLARATION)).toBe(EXPECTED_DECLARATION_CHECKSUM);
  });

  it("hashes the way the Go and Dart sides do, on a value neither of them supplies", () => {
    // The checksum above passing proves the *declaration* matches; it would also
    // pass if this implementation of FNV-1a were wrong in a way that happened to
    // collide, and more usefully it would give no signal at all about the shift
    // arithmetic until somebody changed the declaration. `hello` is the standard
    // FNV-1a 32-bit vector.
    expect(fnv1a32("hello")).toBe("fnv1a32:4f9f2cab");
  });

  it("has eight sections, three of which compile into workspace.db", () => {
    const declared = parseDeclaration();
    expect(declared.length).toBe(8);
    expect(declared.filter((d) => d.view !== "").map((d) => d.dir)).toEqual([
      "data_model",
      "jet_rules",
      "lookups",
    ]);
  });

  it("renders all three declared views, so the unbuilt set is empty", () => {
    // **The state C.3a reached, asserted rather than left to be read off an empty
    // set.** An empty `viewsNotBuiltInReact` and an empty `compiledViews` would
    // both satisfy the case below; only one of them is this.
    expect(Object.keys(compiledViews).sort()).toEqual(["data_model", "jet_rules", "lookups"]);
    expect([...viewsNotBuiltInReact]).toEqual([]);
  });

  it("answers every declared view either by rendering it or by naming it unbuilt", () => {
    for (const { dir, view } of parseDeclaration()) {
      if (view === "") continue;
      const rendered = view in compiledViews;
      const named = viewsNotBuiltInReact.has(view);
      expect(
        rendered || named,
        `section ${dir} declares compiled view "${view}" and this app neither renders it ` +
          "nor names it in viewsNotBuiltInReact. Those are the only two honest answers.",
      ).toBe(true);
      expect(
        rendered && named,
        `compiled view "${view}" is both rendered and listed as not built; one is stale`,
      ).toBe(false);
    }
  });

  it("renders and names only views the apiserver still declares", () => {
    const declared = new Set(parseDeclaration().filter((d) => d.view !== "").map((d) => d.view));
    for (const view of Object.keys(compiledViews)) {
      expect(declared, `this app renders "${view}", which nothing will ever send`).toContain(view);
    }
    for (const view of viewsNotBuiltInReact) {
      expect(declared, `"${view}" is listed as not built and is not declared either`).toContain(
        view,
      );
    }
  });

  it("names its own view inside each document, so the two declarations are compared", () => {
    for (const [key, doc] of Object.entries(compiledViews)) {
      expect(doc.view).toBe(key);
    }
  });

  it("resolves a section node the way the tree asks it to", () => {
    expect(compiledViewFor("data_model")?.label).toBe("Data Model");
    expect(compiledViewFor("jet_rules")?.label).toBe("Jets Rules");
    // C.3a's, and the one view neither client had before. The label is the
    // section's own, from `wsfile.WorkspaceSections`.
    expect(compiledViewFor("lookups")?.label).toBe("Lookups");
    // A section with no compiled view, and a payload that omits the field
    // entirely, which is what the Go side sends for five of the eight.
    expect(compiledViewFor("")).toBeNull();
    expect(compiledViewFor(undefined)).toBeNull();
    // Belt and braces against the field being renamed on the Go side, where no
    // compiler here can see it: an unknown view must not throw and must not guess.
    expect(compiledViewFor("no_such_view")).toBeNull();
  });
});

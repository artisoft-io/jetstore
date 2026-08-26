/**
 * The two escape bodies I-54 named, and the registry that holds them.
 *
 * These are ports of Dart closures the corpus could not serialise, so there is no
 * fixture to check them against — a `hasCellFilter: true` says a closure existed
 * and nothing about what it did. The evidence is the Dart source, cited per case;
 * the tests are what stop the port drifting from it.
 */

import { afterEach, describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import { emptyRegistry, resolveEscapes } from "./escapes";
import {
  fileKeyLabel,
  hasDataRegistryFilters,
  productionRegistry,
  setFileKeyLabelPattern,
} from "./registry";

afterEach(() => setFileKeyLabelPattern(null));

describe("fileKeyLabel", () => {
  it("keeps the slash, because the Dart's substring starts at it", () => {
    // `'...' + text.substring(start)` where `start = text.lastIndexOf('/')`
    // (`user_flows/start_pipeline/data_table_config.dart:86`). An off-by-one here
    // would read as a cosmetic difference and be wrong in every table.
    expect(fileKeyLabel("s3://bucket/in/f10.csv")).toBe(".../f10.csv");
    expect(fileKeyLabel("a/b/c")).toBe(".../c");
  });

  it("returns a key with no separator whole", () => {
    expect(fileKeyLabel("f10.csv")).toBe("f10.csv");
  });

  it("passes null through, as the closure's first line does", () => {
    expect(fileKeyLabel(null)).toBeNull();
  });

  it("prefers the deployment's pattern, and reads group 1", () => {
    // `match[1]`, not `match[0]`: the pattern is expected to *capture* the label.
    setFileKeyLabelPattern("client=([a-z]+)/");
    expect(fileKeyLabel("s3://b/client=acme/f10.csv")).toBe("acme");
  });

  it("falls back when the pattern matches nothing", () => {
    setFileKeyLabelPattern("client=([a-z]+)/");
    expect(fileKeyLabel("s3://b/in/f10.csv")).toBe(".../f10.csv");
  });

  it("falls back when the pattern matches but captures nothing", () => {
    setFileKeyLabelPattern("bucket");
    expect(fileKeyLabel("s3://bucket/in/f10.csv")).toBe(".../f10.csv");
  });

  it("survives a pattern that will not compile, as the Dart does", () => {
    // The value is an environment variable on the server, so a bad one must not
    // stop tables rendering — `user_delegates.dart:116` catches and nulls it.
    setFileKeyLabelPattern("([unclosed");
    expect(fileKeyLabel("s3://b/in/f10.csv")).toBe(".../f10.csv");
  });
});

describe("hasDataRegistryFilters", () => {
  it("is false, because this app has nowhere to set the filters yet", () => {
    // `JetsRouterDelegate().dataRegistryFilters` is router state set by the data
    // registry screens, which are track C's. A registered `false` renders the
    // `clearFilters` button correctly disabled — *nothing to clear* — where an
    // unregistered name would refuse the whole flow at load.
    expect(hasDataRegistryFilters(new FormState(), 0)).toBe(false);
  });
});

describe("the production registry", () => {
  it("resolves the two names an authored table document can use", () => {
    const references = [
      { kind: "cellFilters" as const, name: "fileKeyLabel", at: "/columns/0/cellFilter" },
      { kind: "predicates" as const, name: "hasDataRegistryFilters", at: "/actions/0/isEnabled" },
    ];
    expect(resolveEscapes(references, productionRegistry)).toEqual([]);
    // The same references against the empty registry are what a build without
    // this file looks like, which is what every build looked like until F.0a.
    expect(resolveEscapes(references, emptyRegistry)).toHaveLength(2);
  });

  it("registers exactly the escapes whose flows are migrated, and no others", () => {
    // **This test used to assert `actions` and `initializers` were empty**, and
    // the reason it gave was the right one: registering a name whose body is not
    // ported gives a document that loads and does nothing, which is the failure
    // `escapes.ts` exists to prevent. F.5 ported `homeFiltersUF`, so its two
    // action escapes and the corpus's one initializer are here — and the rule the
    // old assertion stood for is now stated as what it always was, a *bound*
    // rather than a zero.
    //
    // **F.8 added `fileMappingUF`'s two, and they are the first bodies here that
    // talk to the server.** That is what `EscapeHost` is for — an escape gets the
    // host as a second parameter, which is additive for the five bodies that
    // preceded it and for the one in another project's directory (`escapes.ts`).
    //
    // **F.7 added `sourceConfigUF`'s, and the bound is now tight: every flow is
    // migrated, so every escape any document names is here and there is nothing
    // left to be absent.** The two names are not the two the coverage document
    // transcribed — `loadSourceConfigWithFileTypeInference` was that arm read whole
    // and F.2's `when` guard expresses all but one step of it, so what is
    // registered is `readXlsxSheetOption` (`sourceConfig.ts`).
    //
    // **`cpipesTemplateApply` is the seventh and it is not this project's** —
    // agentic_ai's U.3, 2026-08-25, registering the body that has lived in
    // `src/cpipes/` since their M.5. The list above is *this* project's bound and
    // is still tight; the seventh entry says the bound was never the whole rule.
    // It is here rather than absent because their U.2 put projected document sets
    // in `jets/workspace_assets/user_flows/`, so a document naming it now exists
    // in every workspace — which is the same test the six above pass, applied to
    // a flow nobody hand-wrote.
    //
    // **`openWorkspace` is the eighth and the first registered by a non-flow
    // screen** (C.2b). It passes the same test on a document that is bundled
    // rather than in a workspace: `screens/documents/workspaceRegistry.ua.json`
    // names it, so a body that were absent would be a screen that will not load —
    // which is what `documentFindings` checks and what
    // `WorkspaceRegistry.test.tsx` asserts is empty.
    //
    // **It is also the only body here that is deliberately incomplete**, and that
    // is not a violation of the bound. The bound is about names with no body; this
    // has one, and what the body does is report that its destination screen is
    // C.3's (I-183). A registered escape that says why it cannot finish is the
    // thing `escapes.ts` prefers to an unregistered name, one layer up.
    expect(Object.keys(productionRegistry.actions).sort()).toEqual([
      "clearHomeFilters",
      "cpipesTemplateApply",
      "downloadMapping",
      "loadRawRows",
      "openWorkspace",
      "readXlsxSheetOption",
      "saveSourceConfigForFileType",
      "updateHomeFilters",
    ]);
    expect(Object.keys(productionRegistry.initializers)).toEqual(["seedFromHomeFilters"]);
    // The transcribed name is gone rather than pending, which is the one thing a
    // reader of this file five tasks from now would otherwise assume.
    expect(productionRegistry.actions["loadSourceConfigWithFileTypeInference"]).toBeUndefined();
  });

  it("registers the seven predicates the flows, the screens and their dialogs name", () => {
    // C.2b. `isReadOnlyFrom` resolves out of `predicates`, the namespace a table
    // action's `isEnabled` already used — the signature is identical and a second
    // namespace of the same type would be a distinction nothing draws.
    //
    // **Two names for four sites**, which is I-54's shape a third time: the corpus
    // reports `hasIsReadOnlyEval: true` on `addWorkspace`'s name, uri and branch
    // and on `doGitStatusWorkspaceDialog`'s command, and reading the Dart shows
    // two bodies. Per I-103 the mapping is a lookup keyed by what it is knowledge
    // about, which is what naming it in the document makes it.
    //
    // **C.5 adds two more and they are the first here that no flow reaches** —
    // `inferServerNotRunning` and `inferServerNotStopped`, the first users of
    // `FormActionSchema`'s `enabledWhen`. The namespace was built for a table
    // action's `isEnabled` and takes a form action's `isEnabledEval` unchanged,
    // which is what made naming the predicate cheaper than inventing an
    // expression for it.
    //
    // **The exact set is the point rather than the count.** This assertion is
    // what makes a predicate that is registered and named by nothing, or named
    // and registered by nothing, fail here rather than at the call site — and it
    // fired on exactly that during the C.2b/C.5 merge, which is the only test of
    // an exact-set assertion its author cannot run.
    expect(Object.keys(productionRegistry.predicates).sort()).toEqual([
      "alwaysEnabled",
      "hasDataRegistryFilters",
      "hasHomeFilters",
      "hasWorkspaceUri",
      "inferServerNotRunning",
      "inferServerNotStopped",
      "isActiveWorkspace",
    ]);
  });

  it("resolves all three of pipelineExecStatusTable's predicates", () => {
    // F.5, and the correction it carries: the table has three `isEnabledFnc`
    // closures and none of them is `hasDataRegistryFilters`, which the translation
    // used to assume was the only one.
    const references = [
      { kind: "predicates" as const, name: "hasHomeFilters", at: "/actions/5/isEnabled" },
      { kind: "predicates" as const, name: "alwaysEnabled", at: "/actions/3/isEnabled" },
    ];
    expect(resolveEscapes(references, productionRegistry)).toEqual([]);
  });
});

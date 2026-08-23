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

  it("registers no action escape yet, and that is the point", () => {
    // Six action escapes exist in the Dart and every one belongs to a flow track
    // F has not migrated. Registering a name whose body is not ported would give
    // a document that loads and does nothing — the failure `escapes.ts` exists to
    // prevent. The flow refuses to load instead.
    expect(Object.keys(productionRegistry.actions)).toEqual([]);
    expect(Object.keys(productionRegistry.initializers)).toEqual([]);
  });
});

/**
 * Tests for the form-state binding (task A.4c).
 *
 * The interesting assertions are about *ordering* and about the difference
 * between "no rows yet" and "no matching rows", which is the distinction the
 * whole blocking mechanism exists to preserve.
 */

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import {
  clearPublishedSelection,
  hasBlockingFilter,
  publishSelection,
  resetSecondaryKeys,
  restoreSelection,
  selectedPrimaryKeys,
  watchedFormStateKeys,
} from "./binding";
import { FormState } from "./formState";
import type { DataTableFormStateConfig, TableConfig } from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;
const sourceConfig = tables["lfSourceConfigTable"]!;
const fileStaging = tables["lfFileKeyStagingTable"]!;
const field = { group: 0, key: "lfSourceConfigTable" };

/** `load_files` table 1: key at column 0, four secondary columns at 1–4. */
const fsc: DataTableFormStateConfig = sourceConfig.formStateConfig!;

const rowA = ["1", "acme", "vendorA", "claims", "staging_claims", "csv", "d"];
const rowB = ["2", "beta", "vendorB", "members", "staging_members", "csv", "d"];

describe("FormState", () => {
  it("removes the binding when set to null rather than storing one", () => {
    const fs = new FormState();
    fs.setValue(0, "k", "v");
    expect(fs.getValue(0, "k")).toBe("v");
    fs.setValue(0, "k", null);
    // Undefined, not null: A.4a's where-clause fallback tests for absence.
    expect(fs.getValue(0, "k")).toBeUndefined();
  });

  it("marks a key updated only when the value actually changes", () => {
    const fs = new FormState();
    fs.setValue(0, "k", "v");
    expect(fs.isKeyUpdated(0, "k")).toBe(true);
    fs.resetUpdatedKeys(0);

    fs.setValue(0, "k", "v");
    // Writing the same value again is not a change; if it marked the key, every
    // dependent table would refetch on every write.
    expect(fs.isKeyUpdated(0, "k")).toBe(false);

    fs.setValue(0, "k", "w");
    expect(fs.isKeyUpdated(0, "k")).toBe(true);
  });

  it("compares list values by content, not identity", () => {
    const fs = new FormState();
    fs.setValue(0, "k", ["a", "b"]);
    fs.resetUpdatedKeys(0);
    fs.setValue(0, "k", ["a", "b"]);
    expect(fs.isKeyUpdated(0, "k")).toBe(false);
    fs.setValue(0, "k", ["a", "c"]);
    expect(fs.isKeyUpdated(0, "k")).toBe(true);
  });

  it("retains whole rows against their primary key, in selection order", () => {
    const fs = new FormState();
    fs.addSelectedRow(0, "t", "2", rowB);
    fs.addSelectedRow(0, "t", "1", rowA);
    expect(fs.selectedRows(0, "t")).toEqual([rowB, rowA]);
    fs.removeSelectedRow(0, "t", "2");
    expect(fs.selectedRows(0, "t")).toEqual([rowA]);
  });

  it("throws on an unknown validation group instead of silently answering", () => {
    // The Dart prints and returns null, which turns a programming error into a
    // table that shows every row.
    expect(() => new FormState(1).getValue(3, "k")).toThrow(/validation group/);
  });
});

describe("publishSelection", () => {
  it("writes the primary keys under the table's own widget key", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    expect(fs.getValue(0, field.key)).toEqual(["1"]);
    publishSelection(fs, field, fsc, rowB, true);
    expect(fs.getValue(0, field.key)).toEqual(["1", "2"]);
  });

  it("writes every secondary column from the selected rows", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    expect(fs.snapshot(0)).toMatchObject({
      client: ["acme"],
      org: ["vendorA"],
      object_type: ["claims"],
      table_name: ["staging_claims"],
    });
  });

  it("accumulates secondary values across a multi-row selection", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    publishSelection(fs, field, fsc, rowB, true);
    expect(fs.getValue(0, "table_name")).toEqual(["staging_claims", "staging_members"]);
  });

  it("nulls a secondary key when the last row contributing to it goes", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    publishSelection(fs, field, fsc, rowA, false);
    expect(fs.getValue(0, "table_name")).toBeUndefined();
    expect(fs.getValue(0, field.key)).toEqual([]);
  });

  it("ignores a row whose key column is null", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, [null, "acme", "v", "t", "st", "c", "d"], true);
    expect(fs.snapshot(0)).toEqual({});
  });

  it("resets the updated marks before writing, so only the writes are marked", () => {
    // The reset is in the middle of `_updateFormState` on purpose: it is what
    // stops a selection from looking like a change to this table's own filter.
    const fs = new FormState();
    fs.setValue(0, "unrelated", "x");
    expect(fs.isKeyUpdated(0, "unrelated")).toBe(true);

    publishSelection(fs, field, fsc, rowA, true);
    expect(fs.isKeyUpdated(0, "unrelated")).toBe(false);
    expect(fs.isKeyUpdated(0, "table_name")).toBe(true);
  });

  it("recomputes secondary keys from retained rows, not from what was written", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    publishSelection(fs, field, fsc, rowB, true);
    publishSelection(fs, field, fsc, rowA, false);
    expect(fs.getValue(0, "table_name")).toEqual(["staging_members"]);
    expect(selectedPrimaryKeys(fs, field, fsc)).toEqual(["2"]);
  });
});

describe("resetSecondaryKeys", () => {
  it("skips null cells rather than writing holes", () => {
    const fs = new FormState();
    fs.addSelectedRow(0, field.key, "1", ["1", "acme", null, "claims", "st", "c", "d"]);
    resetSecondaryKeys(fs, field, fsc);
    expect(fs.getValue(0, "client")).toEqual(["acme"]);
    expect(fs.getValue(0, "org")).toBeUndefined();
  });
});

describe("restoreSelection", () => {
  it("selects the rows the form already names", () => {
    const fs = new FormState();
    fs.setValue(0, field.key, ["2"]);
    expect(restoreSelection(fs, field, fsc, [rowA, rowB])).toEqual([false, true]);
  });

  it("drops retained rows that are no longer in the page", () => {
    // A changed where clause means the previous selection may not be here any
    // more; the form must not keep claiming it is.
    const fs = new FormState();
    fs.setValue(0, field.key, ["1", "2"]);
    fs.addSelectedRow(0, field.key, "1", rowA);
    fs.addSelectedRow(0, field.key, "2", rowB);

    restoreSelection(fs, field, fsc, [rowB]);
    expect(fs.selectedRows(0, field.key)).toEqual([rowB]);
  });

  it("reads a PostgreSQL array literal, which is how a seeded form arrives", () => {
    const fs = new FormState();
    fs.setValue(0, field.key, "{1,2}");
    expect(restoreSelection(fs, field, fsc, [rowA, rowB])).toEqual([true, true]);
  });

  it("selects nothing when the form holds nothing", () => {
    const fs = new FormState();
    expect(restoreSelection(fs, field, fsc, [rowA, rowB])).toEqual([false, false]);
  });
});

describe("hasBlockingFilter", () => {
  const stagingField = { group: 0, key: "lfFileKeyStagingTable" };

  it("blocks when a filter's form-state key is empty", () => {
    // `load_files` table 2 filters on table_name, which table 1 publishes.
    expect(hasBlockingFilter(fileStaging, stagingField, new FormState())).toBe(true);
  });

  it("stops blocking once the key has a value", () => {
    const fs = new FormState();
    fs.setValue(0, "table_name", ["staging_claims"]);
    expect(hasBlockingFilter(fileStaging, stagingField, fs)).toBe(false);
  });

  it("treats an empty list as absent", () => {
    const fs = new FormState();
    fs.setValue(0, "table_name", []);
    expect(hasBlockingFilter(fileStaging, stagingField, fs)).toBe(true);
  });

  it("accepts a route parameter in place of form state", () => {
    expect(
      hasBlockingFilter(fileStaging, stagingField, new FormState(), {
        table_name: "staging_claims",
      }),
    ).toBe(false);
  });

  it("never blocks a table with no form field", () => {
    expect(hasBlockingFilter(fileStaging, undefined, new FormState())).toBe(false);
  });

  it("never blocks the nine static tables", () => {
    for (const [key, config] of Object.entries(tables)) {
      if (config.apiPath !== "") continue;
      expect(hasBlockingFilter(config, { group: 0, key }, new FormState())).toBe(false);
    }
  });

  it("blocks exactly the tables whose filters are all unsatisfiable at rest", () => {
    // A property over the corpus rather than a fixture: with an empty form, a
    // querying table blocks if and only if it has a formStateKey clause with no
    // default. 20 of the 28 do.
    const blocked = Object.entries(tables)
      .filter(([, c]) => c.apiPath === "/dataTable")
      .filter(([key, c]) => hasBlockingFilter(c, { group: 0, key }, new FormState()));
    expect(blocked.length).toBeGreaterThan(0);
    for (const [, config] of blocked) {
      expect(
        config.whereClauses.some((wc) => wc.formStateKey != null && wc.defaultValue.length === 0),
      ).toBe(true);
    }
  });
});

describe("watchedFormStateKeys", () => {
  it("watches the where clauses' keys and the explicit refresh keys", () => {
    expect(watchedFormStateKeys(fileStaging)).toContain("table_name");
  });

  it("does not descend into orWith, reproducing the Dart", () => {
    // `checkRebuildTableOnFormStateChange` only walks top-level clauses. Two
    // clauses in the corpus nest and neither carries a formStateKey, so nothing
    // is currently missed — this asserts that, so a nested key acquiring one is
    // a failing test rather than a table that quietly goes stale.
    const nested = Object.values(tables).flatMap((c) =>
      c.whereClauses.filter((wc) => wc.orWith != null).map((wc) => wc.orWith!),
    );
    expect(nested).toHaveLength(2);
    expect(nested.every((wc) => wc.formStateKey == null)).toBe(true);
  });
});

describe("clearPublishedSelection", () => {
  it("removes the primary keys, the retained rows and every secondary key", () => {
    const fs = new FormState();
    publishSelection(fs, field, fsc, rowA, true);
    clearPublishedSelection(fs, field, fsc);

    expect(fs.getValue(0, field.key)).toBeUndefined();
    expect(fs.selectedRows(0, field.key)).toEqual([]);
    expect(fs.getValue(0, "table_name")).toBeUndefined();
    expect(fs.getValue(0, "client")).toBeUndefined();
  });
});

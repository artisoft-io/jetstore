/**
 * The form state's group dimension. Task I.1, and the answer to I-31.
 *
 * `FormState` was always an array of groups — the right shape for a repeating
 * structure — and had no way to change how many. These are the two methods that
 * change it, and the cases that matter are the ones where the Dart's behaviour
 * is surprising enough to be worth pinning: **resize only ever grows**, and
 * **removal renumbers everything after it**.
 *
 * The three Dart forms that size from a query are `rulesConfigDialog`
 * (`form_config_impl.dart:399`), `viewInputRecordsDialog` (`:591`) and the file
 * mapping flow's (`user_flows/file_mapping/form_config.dart:139`). Only the
 * first adds or deletes rows, and only the first is dead configuration.
 */

import { describe, expect, it } from "vitest";

import { FormState } from "./formState";
import type { JetsRow } from "./types";

const row = (pk: string): JetsRow => [pk, "x"] as unknown as JetsRow;

describe("resizeFormState", () => {
  it("grows to the requested count, with the new groups empty", () => {
    const fs = new FormState();
    expect(fs.groupCount).toBe(1);
    fs.resizeFormState(4);
    expect(fs.groupCount).toBe(4);
    expect(fs.getValue(3, "k")).toBeUndefined();
    expect(fs.selectedRows(3, "t")).toEqual([]);
    expect(fs.updatedKeys(3).size).toBe(0);
  });

  it("leaves the groups it already had untouched", () => {
    const fs = new FormState(2);
    fs.setValue(0, "k", "kept");
    fs.addSelectedRow(1, "t", "1", row("1"));
    fs.resizeFormState(5);
    expect(fs.getValue(0, "k")).toBe("kept");
    expect(fs.selectedRows(1, "t")).toEqual([row("1")]);
  });

  it("never shrinks — the Dart computes n and does nothing unless n > 0", () => {
    // `jets_form_state.dart:139`. A form reloaded with fewer rows keeps the
    // groups it had; the shrinking case belongs to removeValidationGroup, which
    // is told *which* group to drop. A resize cannot know that, and truncating
    // the tail would discard whichever group happened to be last.
    const fs = new FormState(4);
    fs.setValue(3, "k", "still here");
    fs.resizeFormState(2);
    expect(fs.groupCount).toBe(4);
    expect(fs.getValue(3, "k")).toBe("still here");
    fs.resizeFormState(4);
    expect(fs.groupCount).toBe(4);
  });

  it("takes its count from a query result the way the Dart form does", () => {
    // `form.dart:210`: the length of `data[inputFieldsQuery]`, plus one spare
    // group when the form offers an "add another" affordance.
    const rowsFromQuery = [{}, {}, {}];
    const sizeFor = (n: number, dynamicRows: boolean) => n + (dynamicRows ? 1 : 0);

    const mapping = new FormState();
    mapping.resizeFormState(sizeFor(rowsFromQuery.length, false));
    expect(mapping.groupCount).toBe(3);

    const editable = new FormState();
    editable.resizeFormState(sizeFor(rowsFromQuery.length, true));
    expect(editable.groupCount).toBe(4);
  });

  it("imposes no maximum of its own — a cap is the caller's (I-44)", () => {
    const fs = new FormState();
    fs.resizeFormState(500);
    expect(fs.groupCount).toBe(500);
  });
});

describe("removeValidationGroup", () => {
  it("carries values, selections and updated keys down together", () => {
    // The Dart removes from all four parallel lists in one method
    // (`jets_form_state.dart:155`); splitting them is how they desynchronise.
    const fs = new FormState(3);
    fs.setValue(0, "k", "zero");
    fs.setValue(1, "k", "one");
    fs.setValue(2, "k", "two");
    fs.addSelectedRow(2, "t", "pk", row("pk"));

    fs.removeValidationGroup(1);

    expect(fs.groupCount).toBe(2);
    expect(fs.getValue(0, "k")).toBe("zero");
    // Group 2 is now group 1 — everything after the removal renumbers.
    expect(fs.getValue(1, "k")).toBe("two");
    expect(fs.selectedRows(1, "t")).toEqual([row("pk")]);
    expect(fs.isKeyUpdated(1, "k")).toBe(true);
  });

  it("renumbers, so an index held across a removal is stale", () => {
    // This is why the Dart's only caller rewrites the `group` field of every
    // widget after the removed index (`config_delegates.dart:310`) rather than
    // patching the one it deleted.
    const fs = new FormState(3);
    fs.setValue(2, "k", "last");
    fs.removeValidationGroup(0);
    expect(fs.getValue(1, "k")).toBe("last");
    expect(() => fs.getValue(2, "k")).toThrow(/validation group/);
  });

  it("throws on an index that is out of range or not an integer", () => {
    const fs = new FormState(2);
    expect(() => fs.removeValidationGroup(2)).toThrow(/no such validation group/);
    expect(() => fs.removeValidationGroup(-1)).toThrow(/no such validation group/);
    expect(() => fs.removeValidationGroup(0.5)).toThrow(/no such validation group/);
  });

  it("refuses to remove the last group, which the Dart would allow", () => {
    // A deliberate divergence. The constructor declares `Math.max(1, count)`, so
    // a zero-group state contradicts an invariant this class already states, and
    // every accessor on one fails. Failing here names the call that caused it.
    const fs = new FormState(2);
    fs.removeValidationGroup(0);
    expect(fs.groupCount).toBe(1);
    expect(() => fs.removeValidationGroup(0)).toThrow(/cannot remove the last/);
    expect(fs.groupCount).toBe(1);
  });

  it("composes with resize, which is how a row is added then dropped", () => {
    // `config_delegates.dart:335` grows by one for "add row"; `:327` removes the
    // index the user pointed at.
    const fs = new FormState();
    fs.resizeFormState(fs.groupCount + 1);
    fs.setValue(1, "k", "added");
    expect(fs.groupCount).toBe(2);
    fs.removeValidationGroup(1);
    expect(fs.groupCount).toBe(1);
    expect(fs.getValue(0, "k")).toBeUndefined();
  });
});

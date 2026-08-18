/**
 * @vitest-environment jsdom
 *
 * Tests for the table action bar (task S.2b).
 *
 * **Driven from the corpus, not from examples.** The 25 `ActionConfig`s are in
 * `fixtures/table_configs.json`, generated out of the running Flutter app, so
 * the tests can assert over all of them rather than over the ones someone
 * thought of. That is what catches an action type nobody remembered.
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiProvider } from "../shell/capabilities";
import { ActionBar } from "./ActionBar";
import { availability, enabledPredicateFor, type ActionContext } from "./actionBarModel";
import { UnsupportedActionType, fillPath, requestFor, resolveParams } from "./actionDispatch";
import corpus from "./fixtures/table_configs.json";
import { FormState } from "./formState";
import type { ActionConfig, TableConfig } from "./types";

afterEach(cleanup);

const tables = (corpus as unknown as { tables: Record<string, TableConfig> }).tables;
const allActions: ActionConfig[] = Object.values(tables).flatMap((t) => t.actions ?? []);
/** The bar's share: everything except the two A.5 returned to the widget. */
const widgetOwned = new Set(["toggleCheckboxVisible", "refreshTable"]);
const barActions = allActions.filter((a) => !widgetOwned.has(a.actionType));

function makeContext(overrides: Partial<ActionContext> = {}): ActionContext {
  return {
    selectedRowCount: 1,
    checkboxVisible: true,
    whereClauseSatisfied: true,
    formState: new FormState(),
    predicates: { hasActiveFilters: () => true },
    ...overrides,
  };
}

describe("the corpus this task owns", () => {
  it("is 21 of the 25 configurations, the other four being the widget's", () => {
    expect(allActions).toHaveLength(25);
    expect(barActions).toHaveLength(21);
    expect(allActions.length - barActions.length).toBe(4);
  });

  it("uses five action types, all of which dispatch", () => {
    const types = [...new Set(barActions.map((a) => a.actionType))].sort();
    expect(types).toEqual([
      "clearHomeFilters",
      "doAction",
      "doActionShowDialog",
      "showDialog",
      "showScreen",
    ]);
    for (const action of barActions) {
      expect(() => requestFor(action, new FormState(), ["a", "b"])).not.toThrow();
    }
  });

  it("refuses the two the widget owns, rather than half-handling them", () => {
    for (const action of allActions.filter((a) => widgetOwned.has(a.actionType))) {
      expect(() => requestFor(action, new FormState(), undefined)).toThrow(UnsupportedActionType);
    }
  });

  it("does not require a selection for every action that reads a column", () => {
    // **This assertion is the reverse of the one first written, and the reversal
    // is the finding.** It seemed obvious that an action reading `row[3]` must
    // be gated on having a row. Three are not: `addProcessInput`, in all three
    // of its configurations, reads six column indices with no selection gate.
    //
    // It is not a crash. The Dart guards on `row != null` (`data_table.dart:589`)
    // and simply omits the parameter, so the dialog opens with those fields
    // empty — which is a worse failure than a disabled button and a better one
    // than an exception. Reproduced rather than corrected; see I-24.
    const positional = barActions.filter((a) =>
      Object.values(a.navigationParams ?? {}).some((v) => typeof v === "number"),
    );
    const ungated = positional.filter((a) => a.isEnabledWhenHavingSelectedRows !== true);
    expect(ungated.map((a) => a.key)).toEqual([
      "addProcessInput",
      "addProcessInput",
      "addProcessInput",
    ]);

    // And the parameter is absent rather than wrong when nothing is selected.
    const params = resolveParams(ungated[0]!, new FormState(), undefined);
    expect(params["key"]).toBeUndefined();
  });

  it("names a predicate for every closure that is not a constant true", () => {
    // Three actions carry `hasIsEnabledFnc`; two of the Dart closures are
    // `(state) => true`, which is no gate. Only `clearFilters` has a real one.
    const withFnc = [...new Set(barActions.filter((a) => a.hasIsEnabledFnc).map((a) => a.key))];
    expect(withFnc).toEqual(["clearFilters"]);
    expect(enabledPredicateFor["clearFilters"]).toBe("hasActiveFilters");
  });
});

describe("availability", () => {
  const find = (key: string) => barActions.find((a) => a.key === key)!;

  it("disables a selection-gated action until a row is selected", () => {
    const action = find("deleteClient");
    expect(action.isEnabledWhenHavingSelectedRows).toBe(true);
    expect(availability(action, makeContext({ selectedRowCount: 0 })).enabled).toBe(false);
    expect(availability(action, makeContext()).enabled).toBe(true);
  });

  it("hides a visibility-gated action when the mode does not match", () => {
    const action = barActions.find((a) => a.isVisibleWhenCheckboxVisible !== undefined)!;
    const mode = action.isVisibleWhenCheckboxVisible!;
    expect(availability(action, makeContext({ checkboxVisible: mode })).visible).toBe(true);
    expect(availability(action, makeContext({ checkboxVisible: !mode })).visible).toBe(false);
  });

  it("disables until the named form-state keys are set, and says which are missing", () => {
    const action = find("configureMappingPage");
    expect(action.isEnabledWhenStateHasKeys).toEqual(["table_name", "object_type"]);
    const formState = new FormState();
    formState.setValue(0, "table_name", "t");
    const partial = availability(action, makeContext({ formState }));
    expect(partial.enabled).toBe(false);
    expect(partial.reason).toBe("Needs object_type");
    formState.setValue(0, "object_type", "o");
    expect(availability(action, makeContext({ formState })).enabled).toBe(true);
  });

  it("disables until the table's filters are satisfied", () => {
    const action = find("downloadMappingRows");
    expect(action.isEnabledWhenWhereClauseSatisfied).toBe(true);
    expect(availability(action, makeContext({ whereClauseSatisfied: false })).enabled).toBe(false);
  });

  it("leaves an action disabled when its predicate is not registered", () => {
    // The safe direction: a missing predicate must not silently open an action.
    const action = find("clearFilters");
    const state = availability(action, makeContext({ predicates: {} }));
    expect(state.enabled).toBe(false);
    expect(state.reason).toBe('Missing predicate "hasActiveFilters"');
  });

  it("composes gates as an and, not an or", () => {
    const action = find("deleteClient");
    const both = availability(
      action,
      makeContext({ selectedRowCount: 0, whereClauseSatisfied: false }),
    );
    expect(both.enabled).toBe(false);
  });
});

describe("navigation parameters", () => {
  it("reads form-state keys and column indices, and keeps them apart", () => {
    const action = barActions.find(
      (a) => a.stateFormNavigationParams && a.navigationParams,
    )!;
    // `addProcessInput` names two parameters in both maps; the dialog path lets
    // the column win, which is what `columnWins` reproduces.
    const collisions = Object.keys(action.stateFormNavigationParams!).filter(
      (k) => k in action.navigationParams!,
    );
    expect(collisions).toEqual(["object_type", "source_type"]);
    const formState = new FormState();
    for (const key of Object.values(action.stateFormNavigationParams!)) {
      formState.setValue(0, key, `state:${key}`);
    }
    const row = Array.from({ length: 12 }, (_, i) => `col${i}`);
    const params = resolveParams(action, formState, row);

    for (const [name, key] of Object.entries(action.stateFormNavigationParams!)) {
      if (collisions.includes(name)) continue;
      expect(params[name]).toBe(`state:${key}`);
    }
    for (const [name, source] of Object.entries(action.navigationParams!)) {
      expect(params[name]).toBe(typeof source === "number" ? `col${source}` : source);
    }
  });

  it("lets the two maps disagree the way each Dart path does", () => {
    // The wart no shipping configuration exposes: a dialog takes the column, a
    // screen takes the form-state key. See `ParamPrecedence`.
    const action = barActions.find((a) => a.key === "addProcessInput")!;
    const formState = new FormState();
    formState.setValue(0, "main_object_type", "fromState");
    const row = Array.from({ length: 12 }, (_, i) => `col${i}`);
    expect(resolveParams(action, formState, row, "columnWins")["object_type"]).toBe("col3");
    expect(resolveParams(action, formState, row, "formStateWins")["object_type"]).toBe("fromState");
  });

  it("wires each action type to the precedence its Dart path uses", () => {
    // **Synthetic, and it has to be**: no shipping `showScreen` action sets both
    // maps, so nothing in the corpus can distinguish the two orders through
    // `requestFor`. Without this the wiring could be flattened to one order and
    // every corpus-driven test would still pass — which is exactly what a
    // mutation run showed.
    const both = {
      actionType: "showScreen",
      key: "synthetic",
      label: "Synthetic",
      style: "primary",
      stateGroup: 0,
      hasIsEnabledFnc: false,
      hasActionDelegate: false,
      configScreenPath: "/x/:p",
      stateFormNavigationParams: { p: "fromStateKey" },
      navigationParams: { p: 1 },
    } as ActionConfig;
    const formState = new FormState();
    formState.setValue(0, "fromStateKey", "stateValue");

    const screen = requestFor(both, formState, ["c0", "c1"]);
    expect(screen).toEqual({ kind: "navigate", path: "/x/stateValue", params: { p: "stateValue" } });

    const dialog = requestFor(
      { ...both, actionType: "showDialog", configForm: "f" },
      formState,
      ["c0", "c1"],
    );
    expect(dialog).toEqual({ kind: "openDialog", form: "f", params: { p: "c1" } });
  });

  it("passes a literal navigationParam through untouched", () => {
    // One site sends a Postgres array literal as a where value rather than a
    // column: `{alias_domain_table}`.
    const literals = barActions.flatMap((a) =>
      Object.values(a.navigationParams ?? {}).filter((v) => typeof v === "string"),
    );
    expect(literals).toContain("{alias_domain_table}");
  });

  it("leaves an unfilled path segment visible rather than inserting undefined", () => {
    expect(fillPath("/a/:x/:y", { x: "1" })).toBe("/a/1/:y");
    expect(fillPath("/fileMappingUF/mapping/:table_name/:object_type", {
      table_name: "t", object_type: "o",
    })).toBe("/fileMappingUF/mapping/t/o");
  });

  it("takes the first element when a form-state value is a selection array", () => {
    const action = barActions.find((a) => a.stateFormNavigationParams)!;
    const [name, key] = Object.entries(action.stateFormNavigationParams!)[0]!;
    const formState = new FormState();
    formState.setValue(0, key, ["first", "second"]);
    expect(resolveParams(action, formState, undefined)[name]).toBe("first");
  });
});

describe("the rendered bar", () => {
  const api = {
    can: (c: string) => c === "client_config",
    isAuthenticated: () => true,
  } as never;

  const renderBar = (actions: ActionConfig[], context: ActionContext, onAction = vi.fn()) => {
    render(
      <ApiProvider api={api}>
        <ActionBar actions={actions} context={context} selectedRow={["a", "b"]} onAction={onAction} />
      </ApiProvider>,
    );
    return onAction;
  };

  it("renders a button per visible action and dispatches on click", () => {
    const action = barActions.find((a) => a.actionType === "doAction" && !a.capability)!;
    const onAction = renderBar([action], makeContext());
    fireEvent.click(screen.getByRole("button", { name: action.label }));
    expect(onAction).toHaveBeenCalledWith(
      { kind: "runAction", name: action.actionName },
      action,
    );
  });

  it("disables an action whose capability the user lacks, without hiding it", () => {
    // A.2's decision, and I-10's correction: gated sites disable, never hide.
    const action = barActions.find((a) => a.capability === "run_pipelines")!;
    renderBar([action], makeContext());
    const button = screen.getByRole("button", { name: action.label });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.title).toContain("run_pipelines");
  });

  it("renders nothing at all when every action is hidden", () => {
    const hidden = barActions.filter((a) => a.isVisibleWhenCheckboxVisible === true);
    expect(hidden.length).toBeGreaterThan(0);
    const { container } = render(
      <ApiProvider api={api}>
        <ActionBar
          actions={hidden}
          context={makeContext({ checkboxVisible: false })}
          onAction={vi.fn()}
        />
      </ApiProvider>,
    );
    expect(container.querySelector(".jets-datatable__actions")).toBeNull();
  });
});

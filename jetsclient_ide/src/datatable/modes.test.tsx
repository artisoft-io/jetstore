/**
 * @vitest-environment jsdom
 *
 * Tests for the table's own actions (task A.5).
 *
 * These four configurations — three `toggleCheckboxVisible` and one
 * `refreshTable` — sat in S.2's pile because they sit in the same `actions`
 * array as the ten that dispatch into a flow's delegate. They act on the table,
 * so what is asserted here is table behaviour: what the user sees change, and
 * what the server is asked for afterwards.
 */

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { DataTable } from "./DataTable";
import { FormState } from "./formState";
import { refreshTable, useTableModes } from "./modes";
import type { TableConfig } from "./types";
import { useTableBinding } from "./useTableBinding";

afterEach(cleanup);

const tables = (corpus as unknown as { tables: Record<string, TableConfig> }).tables;
const configOf = (key: string): TableConfig => structuredClone(tables[key]!);

function Harness({ config, ...rest }: { config: TableConfig; fetcher: any; formState: FormState }) {
  const binding = useTableBinding({
    config,
    field: { group: 0, key: config.key },
    formState: rest.formState,
    fetcher: rest.fetcher,
  });
  return (
    <>
      <DataTable config={config} state={binding} modes={binding.modes} />
      <button type="button" onClick={binding.modes.toggleCheckboxVisible}>
        Toggle Checkbox
      </button>
      <button type="button" onClick={binding.refresh}>
        Refresh
      </button>
    </>
  );
}

describe("useTableModes", () => {
  it("starts from the configuration", () => {
    // Ten of the 37 configurations set `isCheckboxVisible: false`, so neither
    // branch is hypothetical.
    const shown = { isCheckboxVisible: true } as TableConfig;
    const hidden = { isCheckboxVisible: false } as TableConfig;
    expect(runHook(shown).checkboxVisible).toBe(true);
    expect(runHook(hidden).checkboxVisible).toBe(false);
  });

  it("toggles the checkbox column and nothing else", () => {
    // The Dart flipped copy-on-click on every checkbox toggle
    // (`_noCopy2Clipboard = _checkboxVisible`, `data_table.dart:483`) because the
    // two modes competed for a click on a row. D.6 removed the copy mode, so a
    // configured `toggleCheckboxVisible` now does only what its name says.
    const hook = runHook({ isCheckboxVisible: true } as TableConfig);
    expect(Object.keys(hook).sort()).toEqual(["checkboxVisible", "toggleCheckboxVisible"]);
    act(() => hook.toggleCheckboxVisible());
    expect(hook.checkboxVisible).toBe(false);
    act(() => hook.toggleCheckboxVisible());
    expect(hook.checkboxVisible).toBe(true);
  });

  it("takes nothing from noCopy2Clipboard, which never decided behaviour in the corpus anyway", () => {
    // I-278, and the corpus is the argument rather than the code. All 11 of the
    // 37 configurations that set the field also set `isCheckboxVisible: true`,
    // where the Dart's own default was already suppression — so setting it
    // changed no load-time behaviour in any of them, and its whole observable
    // effect was to withhold the toggle button. Asserted here so that a later
    // reader does not restore a gate the corpus never exercised.
    const setters = Object.values(tables).filter((t) => t.noCopy2Clipboard === true);
    expect(setters).toHaveLength(11);
    expect(setters.every((t) => t.isCheckboxVisible)).toBe(true);
    expect(runHook({ isCheckboxVisible: true, noCopy2Clipboard: true } as TableConfig))
      .toMatchObject({ checkboxVisible: true });
  });
});

describe("copying a cell", () => {
  /** The widget's only clipboard path; jsdom supplies no `navigator.clipboard`. */
  function withClipboard(): { writeText: ReturnType<typeof vi.fn> } {
    const clipboard = { writeText: vi.fn() };
    Object.defineProperty(globalThis.navigator, "clipboard", {
      value: clipboard,
      configurable: true,
    });
    return clipboard;
  }

  it("copies the full cell text on click, with no toggle to arm first", async () => {
    // I-262: this is the behaviour the report asked to keep. It used to need
    // *Enable Copy Cell* pressed first on any table with checkboxes showing.
    const { config, formState, fetcher } = seededOrg();
    config.isCheckboxVisible = true;
    const clipboard = withClipboard();
    render(<Harness config={config} fetcher={fetcher} formState={formState} />);
    await act(async () => {});

    expect(screen.queryByText("Enable Copy Cell")).toBeNull();
    expect(screen.queryByText("Enable Select Row")).toBeNull();

    fireEvent.click(screen.getByText("c0"));
    expect(clipboard.writeText).toHaveBeenCalledWith("c0");
  });

  it("copies from a table that sets noCopy2Clipboard, which used to suppress it", async () => {
    const { config, formState, fetcher } = seededOrg();
    config.isCheckboxVisible = true;
    config.noCopy2Clipboard = true;
    const clipboard = withClipboard();
    render(<Harness config={config} fetcher={fetcher} formState={formState} />);
    await act(async () => {});

    fireEvent.click(screen.getByText("c1"));
    expect(clipboard.writeText).toHaveBeenCalledWith("c1");
  });
});

/** `org`, unblocked. All three toggling tables gate on a form-state key. */
function seededOrg() {
  const config = configOf("org");
  const formState = new FormState();
  formState.setValue(0, "client", "acme");
  const row = config.columns.map((_, i) => `c${i}`);
  const fetcher = vi.fn(async () => ({ rows: [row], totalRowCount: 1 }));
  return { config, formState, fetcher };
}

describe("the checkbox column", () => {
  it("appears and disappears with the mode, in one of the three tables that toggle it", async () => {
    // Exactly three configurations carry a `toggleCheckboxVisible` action, and
    // **all three set `isCheckboxVisible: false`** — so the button reveals the
    // column rather than hiding it, and a test that assumed the other direction
    // would pass against a table that never rendered a checkbox at all.
    const { config, formState, fetcher } = seededOrg();
    expect(config.actions?.some((a) => a.actionType === "toggleCheckboxVisible")).toBe(true);
    expect(config.isCheckboxVisible).toBe(false);

    render(<Harness config={config} fetcher={fetcher} formState={formState} />);
    await act(async () => {});
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);

    fireEvent.click(screen.getByText("Toggle Checkbox"));
    expect(screen.queryAllByRole("checkbox").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByText("Toggle Checkbox"));
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("no longer drags a copy button along with it", () => {
    // Until D.6 revealing the column also revealed an *Enable Copy Cell* button,
    // because `org` leaves `noCopy2Clipboard` unset and so qualified for one.
    // Both states are asserted: the button is absent either way, so a
    // reintroduction fails here rather than only in the browser.
    const { config, formState, fetcher } = seededOrg();
    render(<Harness config={config} fetcher={fetcher} formState={formState} />);
    expect(screen.queryByText("Enable Copy Cell")).toBeNull();
    fireEvent.click(screen.getByText("Toggle Checkbox"));
    expect(screen.queryByText("Enable Copy Cell")).toBeNull();
  });
});

describe("refreshTable", () => {
  it("resets the page size, which the watched-key path was not doing", () => {
    // The divergence A.5 found: `_refreshTable` sets
    // `rowsPerPage = availableRowsPerPage[0]`, and `[0]` is the configured size
    // (`data_table.dart:354`), so a user who chose 50 rows goes back to 10.
    const calls: string[] = [];
    const state = {
      setPage: (n: number) => calls.push(`setPage(${n})`),
      setRowsPerPage: (n: number) => calls.push(`setRowsPerPage(${n})`),
      clearSelection: () => calls.push("clearSelection"),
      refresh: () => calls.push("refresh"),
    };
    refreshTable(state, { rowsPerPage: 10 } as TableConfig, undefined, undefined, undefined);
    expect(calls).toEqual(["setPage(0)", "setRowsPerPage(10)", "clearSelection", "refresh"]);
  });

  it("clears the published selection before re-querying, not after", () => {
    // Order is the Dart's and it matters: anything downstream reading a
    // selection between the query and the clear would read one that belongs to
    // rows about to be replaced.
    const order: string[] = [];
    const formState = new FormState();
    formState.setValue(0, "t", "held");
    const state = {
      setPage: () => {},
      setRowsPerPage: () => {},
      clearSelection: () => {},
      refresh: () => order.push("refresh"),
    };
    const spy = vi.spyOn(formState, "setValue").mockImplementation(((...args: any[]) => {
      order.push("clear");
      return (FormState.prototype.setValue as any).apply(formState, args);
    }) as any);
    refreshTable(state, { rowsPerPage: 10 } as TableConfig, formState, { group: 0, key: "t" }, undefined);
    spy.mockRestore();
    expect(order.indexOf("clear")).toBeLessThan(order.indexOf("refresh"));
  });

  it("re-queries the server when the button is pressed", async () => {
    const { config, formState, fetcher } = seededOrg();
    render(<Harness config={config} fetcher={fetcher} formState={formState} />);
    await act(async () => {});
    const before = fetcher.mock.calls.length;
    expect(before).toBeGreaterThan(0);
    await act(async () => {
      fireEvent.click(screen.getByText("Refresh"));
    });
    expect(fetcher.mock.calls.length).toBeGreaterThan(before);
  });
});

/** Renders a component whose only job is to expose the hook's return value. */
function runHook(config: TableConfig) {
  const box = {} as ReturnType<typeof useTableModes>;
  function Probe() {
    Object.assign(box, useTableModes(config));
    return null;
  }
  render(<Probe />);
  return box;
}

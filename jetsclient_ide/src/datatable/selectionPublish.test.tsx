/**
 * @vitest-environment jsdom
 *
 * **A tick publishes once, from the event, and never while React renders.**
 *
 * `useDataTable` tells its caller about a selection change through
 * `onSelectionChange`, and the caller publishes the selection into form state,
 * which other components listen to. Until November 2026 the call was made inside
 * the `setSelection` updater. An updater runs while React renders, so a caller
 * that updated another component's state got React's warning, "Cannot update a
 * component while rendering a different component", and under StrictMode React
 * runs an updater twice, so the selection was published twice.
 *
 * The harness here is that caller in miniature: its `onSelectionChange` sets the
 * state of a parent component, as publishing into form state does on a screen,
 * and the whole tree renders under StrictMode, as the app does in development.
 */

import { StrictMode, useState } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { DataTable } from "./DataTable";
import { useDataTable, type DataTableFetcher, type DataTableResponse } from "./useDataTable";
import type { JetsRow, TableConfig } from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

type Published = { changedIndex: number; selected: boolean; deselected: number[]; changed?: number[] };

function Table({
  config,
  fetcher,
  onPublish,
}: {
  config: TableConfig;
  fetcher: DataTableFetcher;
  onPublish: (p: Published) => void;
}) {
  const state = useDataTable({
    config,
    context: { formField: { group: 0, key: config.key } },
    fetcher,
    onSelectionChange: ({ changedIndex, selected, deselected, changed }) =>
      onPublish({ changedIndex, selected, deselected, ...(changed ? { changed } : {}) }),
  });
  return <DataTable config={config} state={state} />;
}

/** Stands in for a screen whose form state the table publishes into. */
function Screen({
  config,
  fetcher,
  published,
}: {
  config: TableConfig;
  fetcher: DataTableFetcher;
  published: Published[];
}) {
  const [count, setCount] = useState(0);
  return (
    <>
      <output>published {count}</output>
      <Table
        config={config}
        fetcher={fetcher}
        onPublish={(p) => {
          published.push(p);
          setCount((c) => c + 1);
        }}
      />
    </>
  );
}

function respondWith(rows: JetsRow[]): DataTableFetcher {
  return vi.fn(async (): Promise<DataTableResponse> => ({ rows, totalRowCount: rows.length }));
}

function renderUpdateWarnings(spy: ReturnType<typeof vi.spyOn>): string[] {
  return spy.mock.calls
    .map((args: unknown[]) => args.map(String).join(" "))
    .filter((m: string) => m.includes("Cannot update a component"));
}

describe("a selection change is published", () => {
  it("once per tick, outside render, on a single-select table", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    const config = tables["lfSourceConfigTable"]!;
    const published: Published[] = [];
    render(
      <StrictMode>
        <Screen
          config={config}
          fetcher={respondWith([
            ["1", "acme", "vendorA", "claims", "staging_claims", "csv", "2026-08-01"],
            ["2", "acme", "vendorB", "members", "staging_members", "csv", "2026-08-02"],
          ])}
          published={published}
        />
      </StrictMode>,
    );
    await screen.findByText("staging_claims");

    fireEvent.click(screen.getByRole("checkbox", { name: "Select row 1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select row 2" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select row 2" }));

    expect(published).toEqual([
      { changedIndex: 0, selected: true, deselected: [] },
      // Single select: ticking the second row unpublishes the first.
      { changedIndex: 1, selected: true, deselected: [0] },
      // Unticking a row names it in `deselected` as well (`toggleSelection`).
      { changedIndex: 1, selected: false, deselected: [1] },
    ]);
    expect(screen.getByText("published 3")).toBeTruthy();
    expect(renderUpdateWarnings(errors)).toEqual([]);
  });

  it("once for the select-all box, naming every row it changed", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    const config = tables["lfFileKeyStagingTable"]!;
    const row = (k: string): JetsRow => Array.from({ length: config.columns.length }, (_, i) => `${k}${i}`);
    const published: Published[] = [];
    render(
      <StrictMode>
        <Screen config={config} fetcher={respondWith([row("a"), row("b"), row("c")])} published={published} />
      </StrictMode>,
    );
    await screen.findAllByRole("checkbox", { name: /^Select row / });

    fireEvent.click(screen.getByRole("checkbox", { name: "Select row 2" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all rows on this page" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all rows on this page" }));

    expect(published).toEqual([
      { changedIndex: 1, selected: true, deselected: [] },
      { changedIndex: 0, selected: true, deselected: [], changed: [0, 2] },
      { changedIndex: 0, selected: false, deselected: [], changed: [0, 1, 2] },
    ]);
    expect(renderUpdateWarnings(errors)).toEqual([]);
  });
});

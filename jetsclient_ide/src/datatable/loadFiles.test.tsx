/**
 * @vitest-environment jsdom
 *
 * **A.4c's exit condition: `load_files`' two tables behave as the Flutter app
 * does.**
 *
 * The flow is 28 lines of Dart and the cheapest one that uses a data table, and
 * it is also a complete specimen of the widget — single and multi selection,
 * publication into form state, a where clause reading the other table's
 * selection, a join clause, a default clause, hidden and numeric columns, server
 * sorting and paging. That is why the sizing nominated it, and why these tests
 * are the ones that matter: everything else in A.4a–A.4c is unit-tested against
 * a function, and this is the only place the three pieces are asked to behave
 * like an application.
 *
 * What is still not covered, and is not coverable here: the server. Both tables
 * talk to a stub. See I-7.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { DataTable } from "./DataTable";
import { FormState } from "./formState";
import { useTableBinding } from "./useTableBinding";
import type { DataTableFetcher, DataTableResponse } from "./useDataTable";
import type { JetsRow, TableConfig } from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;
const sourceConfig = tables["lfSourceConfigTable"]!;
const fileStaging = tables["lfFileKeyStagingTable"]!;

afterEach(cleanup);

/** Two source-config rows, distinguished by the staging table each names. */
const sourceRows: JetsRow[] = [
  ["1", "acme", "vendorA", "claims", "staging_claims", "csv", "2026-08-01"],
  ["2", "acme", "vendorB", "members", "staging_members", "csv", "2026-08-02"],
];

/** Input-registry rows, keyed to a staging table by column 9. */
function fileRow(key: string, tableName: string): JetsRow {
  return [key, "acme", "v", "claims", `f${key}.csv`, "2026", "8", "1", "p", tableName, "sess", "d", "spk"];
}

const allFiles = [
  fileRow("10", "staging_claims"),
  fileRow("11", "staging_claims"),
  fileRow("20", "staging_members"),
];

/**
 * A stub that answers as the server would: it reads the `table_name` where
 * clause out of the payload and filters. Without that, "table 2 refetched" and
 * "table 2 refetched *with the right filter*" would be the same assertion.
 */
function makeServer() {
  const calls: Record<string, unknown>[] = [];
  const fetcher: DataTableFetcher = vi.fn(async (payload): Promise<DataTableResponse> => {
    calls.push(payload);
    const from = (payload["fromClauses"] as { table: string }[])[0]!.table;
    if (from === "source_config") {
      return { rows: sourceRows, totalRowCount: sourceRows.length };
    }
    const clauses = (payload["whereClauses"] ?? []) as { column: string; values?: string[] }[];
    const wanted = clauses.find((c) => c.column === "table_name")?.values ?? [];
    const rows = allFiles.filter((r) => wanted.includes(r[9]!));
    return { rows, totalRowCount: rows.length };
  });
  return { fetcher, calls };
}

/** The two tables of `load_files`, sharing one form state, as the flow does. */
function LoadFilesFlow({ fetcher, formState }: { fetcher: DataTableFetcher; formState: FormState }) {
  const source = useTableBinding({
    config: sourceConfig,
    field: { group: 0, key: sourceConfig.key },
    formState,
    fetcher,
  });
  const files = useTableBinding({
    config: fileStaging,
    field: { group: 0, key: fileStaging.key },
    formState,
    fetcher,
  });
  return (
    <>
      <div data-testid="source">
        <DataTable config={sourceConfig} state={source} />
      </div>
      <div data-testid="files">
        <DataTable config={fileStaging} state={files} />
      </div>
    </>
  );
}

describe("load_files, end to end", () => {
  it("holds the second table back until the first has a selection", async () => {
    const { fetcher, calls } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={new FormState()} />);

    await screen.findByText("staging_claims");
    const files = within(screen.getByTestId("files"));
    expect(files.getByText(/Make a selection above/)).toBeTruthy();

    // And it did not ask: exactly one request, for the source config table.
    expect(calls).toHaveLength(1);
    expect((calls[0]!["fromClauses"] as { table: string }[])[0]!.table).toBe("source_config");
  });

  it("unblocks the second table with the first table's staging table name", async () => {
    const { fetcher, calls } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={new FormState()} />);
    await screen.findByText("staging_claims");

    const source = within(screen.getByTestId("source"));
    fireEvent.click(source.getAllByRole("checkbox")[0]!);

    // Both files under staging_claims arrive; the staging_members one does not.
    await screen.findByText("f10.csv");
    expect(screen.getByText("f11.csv")).toBeTruthy();
    expect(screen.queryByText("f20.csv")).toBeNull();

    const last = calls.at(-1)!;
    const clauses = last["whereClauses"] as { column: string; values?: string[] }[];
    expect(clauses).toContainEqual({
      table: "",
      column: "table_name",
      values: ["staging_claims"],
    });
  });

  it("publishes the first table's secondary columns into the form", async () => {
    const formState = new FormState();
    const { fetcher } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={formState} />);
    await screen.findByText("staging_claims");

    fireEvent.click(within(screen.getByTestId("source")).getAllByRole("checkbox")[0]!);

    await waitFor(() =>
      expect(formState.snapshot(0)).toMatchObject({
        lfSourceConfigTable: ["1"],
        client: ["acme"],
        org: ["vendorA"],
        object_type: ["claims"],
        table_name: ["staging_claims"],
      }),
    );
  });

  it("refilters the second table when the first table's selection changes", async () => {
    const { fetcher } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={new FormState()} />);
    await screen.findByText("staging_claims");

    const source = within(screen.getByTestId("source"));
    fireEvent.click(source.getAllByRole("checkbox")[0]!);
    await screen.findByText("f10.csv");

    // The source table is single-select: choosing the second row replaces the
    // first, which must republish table_name and refilter the second table.
    fireEvent.click(source.getAllByRole("checkbox")[1]!);
    await screen.findByText("f20.csv");
    expect(screen.queryByText("f10.csv")).toBeNull();
  });

  it("drops the second table's selection when the first table's changes", async () => {
    const formState = new FormState();
    const { fetcher } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={formState} />);
    await screen.findByText("staging_claims");

    const source = within(screen.getByTestId("source"));
    const files = () => within(screen.getByTestId("files"));

    fireEvent.click(source.getAllByRole("checkbox")[0]!);
    await screen.findByText("f10.csv");

    // Select two files, which the second table allows — it is multi-select.
    fireEvent.click(files().getAllByRole("checkbox")[0]!);
    fireEvent.click(files().getAllByRole("checkbox")[1]!);
    await waitFor(() =>
      expect(formState.getValue(0, fileStaging.key)).toEqual(["10", "11"]),
    );

    // Now change the source. The files those keys named are no longer on offer,
    // so keeping them would submit a selection the user cannot see.
    fireEvent.click(source.getAllByRole("checkbox")[1]!);
    await screen.findByText("f20.csv");
    await waitFor(() => {
      const keys = formState.getValue(0, fileStaging.key);
      expect(keys == null || (Array.isArray(keys) && !keys.includes("10"))).toBe(true);
    });
  });

  it("keeps the second table's selection across a page change", async () => {
    // The rows are retained in form state rather than in the page, which is what
    // makes this work at all.
    const formState = new FormState();
    const { fetcher } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={formState} />);
    await screen.findByText("staging_claims");

    fireEvent.click(within(screen.getByTestId("source")).getAllByRole("checkbox")[0]!);
    await screen.findByText("f10.csv");

    const files = () => within(screen.getByTestId("files"));
    fireEvent.click(files().getAllByRole("checkbox")[0]!);
    await waitFor(() => expect(formState.getValue(0, fileStaging.key)).toEqual(["10"]));

    // Sorting refetches the same rows here; the restore should re-tick the box.
    fireEvent.click(files().getAllByRole("button", { name: /File Key/ })[0]!);
    await waitFor(() => {
      const boxes = files().getAllByRole("checkbox") as HTMLInputElement[];
      expect(boxes[0]!.checked).toBe(true);
    });
  });

  it("publishes the second table's multi-selection as a list of keys", async () => {
    const formState = new FormState();
    const { fetcher } = makeServer();
    render(<LoadFilesFlow fetcher={fetcher} formState={formState} />);
    await screen.findByText("staging_claims");

    fireEvent.click(within(screen.getByTestId("source")).getAllByRole("checkbox")[0]!);
    await screen.findByText("f10.csv");

    const files = () => within(screen.getByTestId("files"));
    fireEvent.click(files().getAllByRole("checkbox")[0]!);
    fireEvent.click(files().getAllByRole("checkbox")[1]!);

    await waitFor(() =>
      expect(formState.snapshot(0)).toMatchObject({
        lfFileKeyStagingTable: ["10", "11"],
        // Its own secondary columns: file_key, session_id, source_period_key.
        file_key: ["f10.csv", "f11.csv"],
        source_period_key: ["spk", "spk"],
      }),
    );
  });
});

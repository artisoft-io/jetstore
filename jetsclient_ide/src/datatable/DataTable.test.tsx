/**
 * @vitest-environment jsdom
 *
 * Rendering tests for the data table (task A.4b).
 *
 * The environment is set per file rather than globally so the rest of the suite
 * keeps running in node, where it is faster and where a DOM would only hide a
 * dependency on one.
 *
 * **A.4b's exit condition is "renders any of the 37 configurations, driven by
 * A.4a, against a live apiserver."** The first two clauses are tested here: the
 * sweep at the bottom renders every configuration in the corpus, and the fetcher
 * asserts the payload it is handed is A.4a's. The third is not — there is no
 * apiserver in this environment. Issue I-7.
 */

import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { DataTable } from "./DataTable";
import { useDataTable, type DataTableFetcher, type DataTableResponse } from "./useDataTable";
import type { JetsRow, TableConfig } from "./types";

const tables = corpus.tables as unknown as Record<string, TableConfig>;

afterEach(cleanup);

/** Renders a table with a stubbed server, so no network and no apiserver. */
function Harness({
  config,
  fetcher,
  blocked,
}: {
  config: TableConfig;
  fetcher: DataTableFetcher;
  blocked?: boolean;
}) {
  const state = useDataTable({
    config,
    context: { formField: { group: 0, key: config.key } },
    fetcher,
    ...(blocked === undefined ? {} : { blocked }),
  });
  return <DataTable config={config} state={state} />;
}

function respondWith(rows: JetsRow[], totalRowCount?: number): DataTableFetcher {
  return vi.fn(async (): Promise<DataTableResponse> => ({
    rows,
    totalRowCount: totalRowCount ?? rows.length,
  }));
}

const sourceConfig = tables["lfSourceConfigTable"]!;

describe("DataTable", () => {
  it("renders the label, the visible columns and the rows", async () => {
    const rows: JetsRow[] = [
      ["1", "acme", "vendorA", "claims", "staging_claims", "csv", "2026-08-01"],
      ["2", "acme", "vendorB", "members", "staging_members", "csv", "2026-08-02"],
    ];
    render(<Harness config={sourceConfig} fetcher={respondWith(rows)} />);

    await screen.findByText("staging_claims");
    expect(screen.getByText("Select a File Data Source Configurations")).toBeTruthy();

    // Seven columns are configured, one of them hidden; six are rendered, plus
    // the checkbox column.
    const headers = screen.getAllByRole("columnheader");
    expect(headers).toHaveLength(7);
    expect(screen.queryByRole("button", { name: "Key" })).toBeNull();
    expect(screen.getByRole("button", { name: /Client/ })).toBeTruthy();
  });

  it("marks the sorted column and flips it on the second click", async () => {
    const fetcher = respondWith([["1", "acme", "v", "t", "st", "csv", "2026-08-01"]]);
    render(<Harness config={sourceConfig} fetcher={fetcher} />);
    await screen.findByText("acme");

    // The configured sort is last_update descending.
    const lastUpdate = screen.getByRole("columnheader", { name: /Last Updated/ });
    expect(lastUpdate.getAttribute("aria-sort")).toBe("descending");

    fireEvent.click(screen.getByRole("button", { name: /Last Updated/ }));
    await waitFor(() =>
      expect(
        screen.getByRole("columnheader", { name: /Last Updated/ }).getAttribute("aria-sort"),
      ).toBe("ascending"),
    );

    // And the request that followed the click carried the new direction — the
    // thing the Dart gets wrong by flipping after the fetch.
    const lastCall = (fetcher as unknown as { mock: { calls: [Record<string, unknown>][] } }).mock
      .calls.at(-1)![0];
    expect(lastCall["sortColumn"]).toBe("last_update");
    expect(lastCall["sortAscending"]).toBe(true);
  });

  it("hands the fetcher exactly what A.4a builds", async () => {
    const fetcher = respondWith([]);
    render(<Harness config={sourceConfig} fetcher={fetcher} />);
    await waitFor(() => expect(fetcher).toHaveBeenCalled());

    const payload = (fetcher as unknown as { mock: { calls: [Record<string, unknown>][] } }).mock
      .calls[0]![0];
    expect(payload["action"]).toBe("read");
    expect(payload["limit"]).toBe(20);
    expect(payload["offset"]).toBe(0);
    expect(payload["fromClauses"]).toEqual([{ schema: "jetsapi", table: "source_config" }]);
    // All seven columns, including the hidden one: hiding is presentation.
    expect(payload["columns"]).toHaveLength(7);
  });

  it("selects a single row at a time when the config says single-select", async () => {
    const rows: JetsRow[] = [
      ["1", "acme", "v", "t", "st1", "csv", "d"],
      ["2", "beta", "v", "t", "st2", "csv", "d"],
    ];
    render(<Harness config={sourceConfig} fetcher={respondWith(rows)} />);
    await screen.findByText("st1");

    const boxes = screen.getAllByRole("checkbox") as HTMLInputElement[];
    fireEvent.click(boxes[0]!);
    await waitFor(() => expect(boxes[0]!.checked).toBe(true));

    fireEvent.click(boxes[1]!);
    await waitFor(() => expect(boxes[1]!.checked).toBe(true));
    expect(boxes[0]!.checked).toBe(false);
  });

  it("keeps several rows when the config says multi-select", async () => {
    const config = tables["lfFileKeyStagingTable"]!;
    const row = (k: string): JetsRow => [k, "acme", "v", "t", "f", "2026", "8", "1", "p", "tbl", "s", "d", "spk"];
    render(<Harness config={config} fetcher={respondWith([row("1"), row("2")])} />);
    await waitFor(() => expect(screen.getAllByRole("checkbox").length).toBe(2));

    const boxes = screen.getAllByRole("checkbox") as HTMLInputElement[];
    fireEvent.click(boxes[0]!);
    fireEvent.click(boxes[1]!);
    await waitFor(() => expect(boxes[0]!.checked && boxes[1]!.checked).toBe(true));
  });

  it("pages through the server's rows, and lands on the real last page", async () => {
    // 50 rows at 20 per page: the Dart's last-page arithmetic gives page 1 here.
    const rows: JetsRow[] = [["1", "acme", "v", "t", "st", "csv", "d"]];
    const fetcher = respondWith(rows, 50);
    render(<Harness config={sourceConfig} fetcher={fetcher} />);
    await screen.findByText("acme");

    fireEvent.click(screen.getByRole("button", { name: "Last page" }));
    await waitFor(() => {
      const payload = (fetcher as unknown as { mock: { calls: [Record<string, unknown>][] } }).mock
        .calls.at(-1)![0];
      expect(payload["offset"]).toBe(40);
    });
  });

  it("disables the paging buttons at the ends", async () => {
    // **50 rows at 20 per page, where this asked for 5.** With 5 the table had
    // one page, so *both ends* were the same end and the assertion held for a
    // reason it was not testing — and since D.11 a one-page table draws no
    // footer at all, which is what turned a weak test into a failing one. Three
    // pages, on the first: the backward pair is disabled and the forward pair is
    // not, which is the claim the name makes.
    render(<Harness config={sourceConfig} fetcher={respondWith([["1", "a", "v", "t", "s", "c", "d"]], 50)} />);
    await screen.findByText("a");
    expect(screen.getByRole("button", { name: "First page" }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("button", { name: "Previous page" }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("button", { name: "Next page" }).hasAttribute("disabled")).toBe(false);
    expect(screen.getByRole("button", { name: "Last page" }).hasAttribute("disabled")).toBe(false);
  });

  it("draws no footer at all for a table that fits on one page", async () => {
    // D.11. The footer is paging apparatus; 5 rows at 20 per page have no pages
    // to move between, and the four buttons were rendered permanently disabled
    // beside a range that read `1–5 of 5`.
    render(<Harness config={sourceConfig} fetcher={respondWith([["1", "a", "v", "t", "s", "c", "d"]], 5)} />);
    await screen.findByText("a");
    expect(screen.queryByRole("button", { name: "First page" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Last page" })).toBeNull();
    expect(screen.queryByLabelText("Rows per page")).toBeNull();
  });

  it("blanks the column header of a one-column table that already has a caption", async () => {
    // **D.11.** `ufClientOrVendorOption` captions itself *Select one of the
    // following options:* and its single column header reads *Select one of the
    // following option* — the same sentence, one letter shorter. **9 of the 35
    // installed documents have exactly one visible column and the header is
    // redundant in all 9**; comparing the two strings catches only 5 of them,
    // which is why the test is structural.
    const config = tables["ufClientOrVendorOption"]!;
    render(<Harness config={config} fetcher={respondWith([["Create a client and add vendors"]], 1)} />);
    await screen.findByText("Create a client and add vendors");
    // The caption survives; the header does not repeat it.
    expect(screen.getByText("Select one of the following options:")).toBeTruthy();
    expect(screen.queryByText("Select one of the following option")).toBeNull();
    // The row is still there, so the cells still have a `<th scope="col">`.
    expect(document.querySelectorAll("thead th").length).toBeGreaterThan(0);
  });

  it("keeps the column headers of a table with more than one column", async () => {
    // The guard on the rule above. `org.tc.json` captions itself *Client
    // Organization Registry* and has a real *Client Organization* column among
    // three — a text comparison loose enough to catch all 9 single-column cases
    // reaches this one and blanks a header that names its column.
    render(<Harness config={sourceConfig} fetcher={respondWith([["1", "a", "v", "t", "s", "c", "d"]], 5)} />);
    await screen.findByText("a");
    expect(document.querySelectorAll("thead th button")[0]!.textContent).not.toBe("");
  });

  it("says why it is empty when a blocking filter is unsatisfied", async () => {
    const fetcher = respondWith([]);
    render(<Harness config={tables["lfFileKeyStagingTable"]!} fetcher={fetcher} blocked />);
    await screen.findByText(/Make a selection above/);
    // The point of blocking: the server is not asked at all.
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("surfaces a failed request instead of rendering an empty table", async () => {
    const fetcher: DataTableFetcher = vi.fn(async () => {
      throw new Error("The server rejected the request.");
    });
    render(<Harness config={sourceConfig} fetcher={fetcher} />);
    expect(await screen.findByRole("alert")).toHaveProperty(
      "textContent",
      "The server rejected the request.",
    );
  });

  it("renders a static table without asking a server", async () => {
    // Nine configurations carry their rows in the config itself. Note the corpus
    // is keyed by the *constant's value*, so `FSK.scFileTypeOption` appears as
    // `input_format` — the keys are not always the Dart identifier.
    const config = tables["otherWorkspaceActionOptions"]!;
    expect(config.staticTableModel).toBeTruthy();
    const fetcher = respondWith([]);
    render(<Harness config={config} fetcher={fetcher} />);

    await waitFor(() => expect(screen.getAllByRole("row").length).toBeGreaterThan(1));
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("hides the footer when the config says so", async () => {
    const config: TableConfig = { ...sourceConfig, noFooter: true };
    render(<Harness config={config} fetcher={respondWith([])} />);
    await waitFor(() => expect(screen.queryByLabelText("Rows per page")).toBeNull());
  });
});

describe("every configuration in the corpus renders", () => {
  const keys = Object.keys(tables);

  it.each(keys)("%s", async (key) => {
    const config = tables[key]!;
    // One row of the right width, so a table with 13 columns gets 13 cells.
    const width = config.columns.reduce((n, c) => Math.max(n, c.index + 1), 0);
    const rows: JetsRow[] = width > 0 ? [new Array<string>(width).fill("v")] : [];

    await act(async () => {
      render(<Harness config={config} fetcher={respondWith(rows)} />);
    });

    const table = await screen.findByRole("table");
    expect(table).toBeTruthy();
    // Header cells = visible columns, plus one for the checkbox column when the
    // configuration shows checkboxes.
    const expectedHeaders =
      config.columns.filter((c) => !c.isHidden).length + (config.isCheckboxVisible ? 1 : 0);
    expect(screen.getAllByRole("columnheader")).toHaveLength(expectedHeaders);
  });
});

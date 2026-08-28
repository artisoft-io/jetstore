/**
 * @vitest-environment jsdom
 *
 * The client filter reaches every table with a `client` column. Task D.3,
 * from **I-259**'s note.
 *
 * ## What this is actually testing, and why it is a whole file
 *
 * `makeQuery` has added the implicit `WHERE client = $n` since A.4a, gated on
 * `selectedClient` being set *and* the table declaring a `client` column. **Until
 * D.3 exactly one caller passed `selectedClient`** — `Home.tsx` — so the filter
 * narrowed the Home screen's tables and did nothing anywhere else, including in
 * every user flow. The report's wording is *"not only to tables on the Home
 * screen but also to all tables in user flows that have the column `client`"*.
 *
 * **Nothing failed, which is why this needed a test rather than a fix and a
 * glance.** A filter that is quietly off returns *more* rows, so the symptom is
 * data a user did not expect to see rather than an error, and no existing test
 * could see it: the query builder's own tests pass the context directly and were
 * right all along.
 *
 * So what is asserted here is the *wiring*, at the seam that was missing — a
 * table bound through `useTableBinding` with no context of its own, which is
 * what a flow's table is.
 */

import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { DataTable } from "./DataTable";
import { FormState } from "./formState";
import type { TableConfig } from "./types";
import { useTableBinding } from "./useTableBinding";
import { resetSelectedClient, setSelectedClient } from "../shell/selectedClient";

afterEach(() => {
  cleanup();
  resetSelectedClient();
});

const tables = (corpus as unknown as { tables: Record<string, TableConfig> }).tables;

/** A flow's table: bound with no context, because a flow supplies none. */
function FlowTable({ config, fetcher }: { config: TableConfig; fetcher: any }) {
  const formState = new FormState();
  const binding = useTableBinding({ config, field: { group: 0, key: config.key }, formState, fetcher });
  return <DataTable config={config} state={binding} modes={binding.modes} />;
}

function capture() {
  const sent: any[] = [];
  const fetcher = vi.fn(async (action: any) => {
    sent.push(action);
    return { rows: [], totalRowCount: 0 };
  });
  return { sent, fetcher };
}

describe("the client filter's reach", () => {
  it("applies to a flow's table, which supplies no context of its own", async () => {
    // `fmInputSourceMappingUF` is one of 22 fixture tables declaring a `client`
    // column, and it belongs to a flow — which is the case the report names.
    const config = structuredClone(tables["fmInputSourceMappingUF"]!);
    expect(config.columns.some((c) => c.name === "client")).toBe(true);

    setSelectedClient("acme");
    const { sent, fetcher } = capture();
    render(<FlowTable config={config} fetcher={fetcher} />);

    await waitFor(() => expect(sent.length).toBeGreaterThan(0));
    const where = JSON.stringify(sent[0]);
    expect(where).toContain("acme");
  });

  it("adds nothing when no client is chosen", async () => {
    // The filter's absence must show *more* rows rather than fail, which is the
    // property that made its absence invisible in the first place.
    const config = structuredClone(tables["fmInputSourceMappingUF"]!);
    const { sent, fetcher } = capture();
    render(<FlowTable config={config} fetcher={fetcher} />);

    await waitFor(() => expect(sent.length).toBeGreaterThan(0));
    expect(JSON.stringify(sent[0])).not.toContain("acme");
  });

  it("leaves a table without a `client` column alone", async () => {
    // `makeQuery`'s second gate, asserted here so that defaulting the context in
    // `useTableBinding` is not read as filtering everything.
    //
    // **The same table with its `client` column removed, rather than a different
    // table that happens to lack one.** Picking another config changes the
    // blocking filters and the form-state model with it, and the first attempt at
    // this test chose one that never queries at all — so it would have passed on
    // a table that sends nothing, which is not the claim.
    const config = structuredClone(tables["fmInputSourceMappingUF"]!);
    config.columns = config.columns.filter((c) => c.name !== "client");

    setSelectedClient("acme");
    const { sent, fetcher } = capture();
    render(<FlowTable config={config} fetcher={fetcher} />);

    await waitFor(() => expect(sent.length).toBeGreaterThan(0));
    expect(JSON.stringify(sent[0])).not.toContain("acme");
  });
});

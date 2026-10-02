/**
 * @vitest-environment jsdom
 *
 * The third action row, drawn. jetstore_maintenance_02, `D04`, task `AE.1`
 * (2026-10-01).
 *
 * `TableView` lays each entry of its `actionRows` out as a line of its own (`D03`,
 * `AA.4`), so a third row is a third line by construction — **this asserts that
 * construction, not the layout**: jsdom computes no boxes, so it catches the row
 * being dropped or merged into another and says nothing about where it lands on
 * screen. Criterion 11's *in the report's order* is the Home screen's test once
 * the row has its buttons (`AE.4`, `AE.5`).
 */

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { productionRegistry } from "../actions/registry";
import { ApiProvider } from "../shell/capabilities";
import pipelineExecStatusTable from "../../../jets/workspace_assets/table_configs/pipelineExecStatusTable.tc.json";
import { FormState } from "./formState";
import type { TableConfigDocument } from "./table";
import { TableView } from "./TableView";
import { fromDocument } from "./tableTranslate";

afterEach(cleanup);

describe("a table with three action rows", () => {
  it("draws the third row as a third line, below the other two", () => {
    const doc = {
      ...structuredClone(pipelineExecStatusTable),
      thirdRowActions: [
        { key: "thirdA", label: "Third A", action: "doAction", actionName: "a", style: "secondary" },
        { key: "thirdB", label: "Third B", action: "doAction", actionName: "b", style: "secondary" },
      ],
    } as unknown as TableConfigDocument;
    render(
      <ApiProvider api={{ can: () => true, isAuthenticated: () => true } as never}>
        <TableView
          config={fromDocument("pipelineExecStatusTable", doc)}
          field={{ group: 0, key: "pipelineExecStatusTable" }}
          formState={new FormState()}
          fetcher={vi.fn(async () => ({ rows: [], totalRowCount: 0 }))}
          predicates={productionRegistry.predicates}
          cellFilters={productionRegistry.cellFilters}
          onAction={vi.fn()}
        />
      </ApiProvider>,
    );
    const lineOf = (label: string) => screen.getByRole("button", { name: label }).closest(".jets-datatable__header-row");
    const lines = [...document.querySelectorAll(".jets-datatable__header-row")];
    expect(lines).toHaveLength(3);
    expect(lineOf("Start Pipeline")).toBe(lines[0]);
    expect(lineOf("Resubmit")).toBe(lines[1]);
    expect(lineOf("Third A")).toBe(lines[2]);
    expect(lineOf("Third B")).toBe(lines[2]);
  });
});

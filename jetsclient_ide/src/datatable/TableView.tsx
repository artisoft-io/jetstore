/**
 * A bound table with its two action rows. Task C.2b.
 *
 * **Extracted from `FormRenderer`'s `FormDataTable` rather than written beside
 * it.** That function was the only place `useTableBinding`, `DataTable` and the
 * two `ActionBar`s were composed, and it was reachable only from a `dataTable`
 * *field* — so the first non-flow screen would have copied it, and the eight
 * after that would have copied the copy. What each of them would have had to get
 * right independently is not the markup: it is that the selected row must reach
 * both the dispatcher and the enablement gate, and that the second action row
 * exists at all. Both are one-line omissions that render a plausible screen.
 *
 * `FormDataTable` now delegates here and keeps only what is form-specific — the
 * field key, the wrapper class and the field error.
 *
 * ## What a caller still owns
 *
 * The `field` — a group and a key, which is how a selection is published into
 * form state (`binding.ts`, `publishSelection`). A screen has no form field, and
 * that is not a reason to make this optional: the Flutter workspace registry
 * screen *is* a form with one data-table field, so a screen's table publishes its
 * selection under a key exactly as a flow's does, and every action on this table
 * reads that selection back. A screen picks the table's own key, which is what
 * the Dart's `FormDataTableFieldConfig(key: DTKeys.workspaceRegistryTable, …)`
 * does.
 */

import type { ReactNode } from "react";

import { ActionBar } from "./ActionBar";
import { DataTable } from "./DataTable";
import type { ActionRequest } from "./actionDispatch";
import type { FormState } from "./formState";
import { useTableBinding, type UseTableBindingOptions } from "./useTableBinding";
import type { DataTableFetcher } from "./useDataTable";
import type { ActionConfig, TableConfig } from "./types";

export interface TableViewProps {
  config: TableConfig;
  /** Group and key the selection is published under. */
  field: { group: number; key: string };
  formState: FormState;
  fetcher: DataTableFetcher;
  /** Route params, the selected client and the two filter lists — I-104. */
  context?: UseTableBindingOptions["context"];
  /** Named predicates a table action's `isEnabled` resolves through. */
  predicates: Readonly<Record<string, (formState: FormState, group: number) => boolean>>;
  /** Per-column display filters, by column name. */
  cellFilters: Record<string, (value: string | null) => string | null>;
  onAction(request: ActionRequest, action: ActionConfig): void;
}

export function TableView({
  config,
  field,
  formState,
  fetcher,
  context,
  predicates,
  cellFilters,
  onAction,
}: TableViewProps): ReactNode {
  const binding = useTableBinding({
    config,
    field,
    formState,
    fetcher,
    ...(context ? { context } : {}),
  });

  const selectedIndex = binding.selection.findIndex(Boolean);
  const selectedRow = selectedIndex === -1 ? undefined : binding.rows[selectedIndex];
  const selectedRowCount = binding.selection.filter(Boolean).length;

  /**
   * The two actions A.5 returned to the widget, handed to the bar. Task D.10.
   *
   * `binding` has held both since A.5 and nothing passed them on; see
   * `ActionBar`'s `WidgetActions` for what that cost.
   */
  const widget = {
    refresh: binding.refresh,
    toggleCheckboxVisible: binding.modes.toggleCheckboxVisible,
  };

  const barContext = {
    selectedRowCount,
    checkboxVisible: binding.modes.checkboxVisible,
    // `blocked` is `hasBlockingFilter`, which is this predicate inverted —
    // A.4c's answer reused rather than recomputed.
    whereClauseSatisfied: !binding.blocked,
    formState,
    predicates,
  };

  return (
    <DataTable
      config={config}
      state={binding}
      modes={binding.modes}
      cellFilters={cellFilters}
      actions={
        <>
          <ActionBar
            actions={config.actions}
            context={barContext}
            widget={widget}
            {...(selectedRow !== undefined ? { selectedRow } : {})}
            onAction={onAction}
          />
          {/* **The second row, F.5.** `ActionBar` renders nothing for an empty
              list, so this costs one element on the 37 flow tables and draws
              five buttons on `pipelineExecStatusTable` and eight on
              `workspaceRegistryTable`. Two bars rather than one concatenated
              list because the Dart draws two (`components/data_table.dart`), and
              because concatenating would make a `secondRowActions` index
              unreachable from a finding pointer that names the row. */}
          <ActionBar
            actions={config.secondRowActions}
            context={barContext}
            widget={widget}
            {...(selectedRow !== undefined ? { selectedRow } : {})}
            onAction={onAction}
          />
        </>
      }
    />
  );
}

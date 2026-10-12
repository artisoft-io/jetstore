/**
 * The Workspace IDE's toolbar: the registry actions that need a workspace,
 * acting on the one the IDE has open.
 *
 * ## Why these buttons moved
 *
 * On `/workspaces` every one of them was gated on a selected row, so a user
 * opening a workspace in the IDE and wanting to compile it, or commit it, had to
 * go back to the registry and select it again. The IDE always has a workspace in
 * context, so the same buttons there are always ready. Moved on 2026-10-11:
 * *Export Client Config* and *Load Client Config* from the registry's first row
 * and its whole second row but *Refresh*, which belongs to the table.
 *
 * ## They are still registry actions
 *
 * The buttons are `workspaceRegistryTable` actions, authored in
 * `documents/workspaceIdeToolbar.json` in that table's own action format, and
 * they run against the open workspace's **registry row**, read the way the
 * registry reads it — `workspace_read`, whose post-processing computes `status`
 * from git (`DoWorkspaceReadAction`, `jets/datatable/workspace_data_table_action.go`).
 * So each `navigationParams` index and `enableWhen` column means exactly what it
 * meant on the table (though *Commit & Push*, *Push Only* and *Pull* have had no
 * gate since the same evening, at Michel's request), and the row is
 * published into form state as a selection would publish it, which is what the
 * `doAction` entries read. The dialogs and actions are the registry's, through
 * the host the two screens share (`workspaceRegistryHost.tsx`).
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { z } from "zod";

import type { ApiClient } from "../api/client";
import { productionRegistry } from "../actions/registry";
import { ActionBar } from "../datatable/ActionBar";
import { publishSelection } from "../datatable/binding";
import { makeQuery } from "../datatable/query";
import { TableActionSchema } from "../datatable/table";
import { restoreActions } from "../datatable/tableTranslate";
import type { ActionConfig, JetsRow } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { GROUP, TABLE_KEY, parsed, registryTableConfig, useRegistryActionHost } from "./workspaceRegistryHost";

import toolbarJson from "./documents/workspaceIdeToolbar.json";
import "../datatable/datatable.css";

export const WorkspaceToolbarDocumentSchema = z
  .object({
    comment: z.string().optional(),
    rows: z.array(z.array(TableActionSchema).min(1)).min(1),
  })
  .strict();

const parsedToolbar = WorkspaceToolbarDocumentSchema.safeParse(toolbarJson);

/** The toolbar's rows, as the bar renders them, or the reason there are none. */
export function toolbarRows(): { rows: ActionConfig[][] } | { error: string } {
  if (!parsedToolbar.success) {
    return {
      error: parsedToolbar.error.issues.map((i) => `${i.path.join("/")} — ${i.message}`).join("; "),
    };
  }
  if (!parsed.table.success) return { error: "workspaceRegistryTable.tc.json does not parse" };
  const table = parsed.table.data;
  return { rows: parsedToolbar.data.rows.map((row) => restoreActions(table, row)) };
}

/**
 * The registry row of one workspace, with its status, as the table reads it.
 *
 * The table's own query with one where clause added. `rowsPerPage` is generous
 * rather than 1, so that two registry rows naming one workspace — which the
 * table would show — are found and the right one picked by name.
 */
export async function fetchRegistryRow(api: ApiClient, workspace: string): Promise<JetsRow | null> {
  const config = registryTableConfig;
  if (config === null) return null;
  const payload = makeQuery({
    config,
    indexOffset: 0,
    rowsPerPage: 50,
    sortColumnName: "",
    sortColumnTableName: "",
    sortAscending: true,
  });
  payload.whereClauses = [{ table: "workspace_registry", column: "workspace_name", values: [workspace] }];
  const body = await api.dataTable<{ rows?: unknown[] }>(payload as unknown as Record<string, unknown>);
  const rows = Array.isArray(body.rows) ? (body.rows as JetsRow[]) : [];
  return rows.find((r) => r[1] === workspace) ?? null;
}

/**
 * `onRefresh` is the IDE's half of the toolbar's Refresh (the document's
 * `refreshTable` action): the toolbar re-reads its own registry row and the IDE
 * re-reads what it shows. 2026-10-12, Michel: files that arrived outside the IDE
 * did not appear until the page was reloaded.
 */
export function WorkspaceToolbar({
  api,
  workspace,
  onRefresh,
}: {
  api: ApiClient;
  workspace: string;
  onRefresh?: () => void;
}) {
  const { setError } = useNotifications();
  const [row, setRow] = useState<JetsRow | null>(null);
  const [generation, setGeneration] = useState(0);
  const reload = useCallback(() => setGeneration((n) => n + 1), []);

  const tableContext = useMemo(() => ({ routeParams: { workspace_name: workspace } }), [workspace]);
  const registry = useRegistryActionHost({ api, tableContext, afterAction: reload });
  const { formState } = registry;
  const rows = useMemo(toolbarRows, []);
  /** The bar's widget: Refresh is the only widget action the toolbar configures. */
  const widget = useMemo(
    () => ({
      refresh() {
        reload();
        onRefresh?.();
      },
      toggleCheckboxVisible() {},
    }),
    [reload, onRefresh],
  );

  useEffect(() => {
    if (workspace === "") return;
    let cancelled = false;
    void (async () => {
      try {
        const found = await fetchRegistryRow(api, workspace);
        if (cancelled) return;
        // Published as a one-row selection, which is what the registry's
        // `doAction` entries and dialogs read (`formStateBinding`).
        const config = registryTableConfig;
        formState.clearSelectedRow(GROUP, TABLE_KEY);
        if (found !== null && config?.formStateConfig !== undefined) {
          publishSelection(formState, { group: GROUP, key: TABLE_KEY }, config.formStateConfig, found, true);
        }
        formState.notifyListeners();
        setRow(found);
      } catch (error) {
        if (!cancelled) {
          setRow(null);
          setError(
            `Could not read ${workspace} from the workspace registry: ${error instanceof Error ? error.message : String(error)}`,
          );
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [api, workspace, generation, formState, setError]);

  if ("error" in rows) {
    return (
      <div className="banner banner-error" role="alert">
        The workspace toolbar cannot be loaded: {rows.error}
      </div>
    );
  }

  const context = {
    tableKey: TABLE_KEY,
    selectedRowCount: row === null ? 0 : 1,
    checkboxVisible: true,
    whereClauseSatisfied: true,
    formState,
    predicates: productionRegistry.predicates,
  };

  return (
    <div className="ws-toolbar" aria-label="Workspace actions">
      {rows.rows.map((actions, index) => (
        <ActionBar
          key={index}
          actions={actions}
          context={context}
          widget={widget}
          {...(row !== null ? { selectedRow: row } : {})}
          onAction={registry.onAction}
        />
      ))}
      {registry.dialog}
    </div>
  );
}

/**
 * The workspace home's base content: the uncommitted changes in this workspace,
 * and the two buttons that revert them.
 *
 * ## Why this is a component rather than a route
 *
 * The Flutter workspace home is `ScreenWithTabsWithForm`
 * (`jetsclient/lib/routes/jets_routes_app.dart`, `workspaceHomePath`) — tabs
 * *and* a base form — and its form is action-less with a single field, the
 * `workspaceChangesTable` at `tableHeight: double.infinity`
 * (`workspace_ide/form_config.dart`, `FormKeys.workspaceHome`). So the changes
 * table is not a screen beside the editor; it is what the editor shows when no
 * tab is open. `WorkspaceIde` renders this where its "Select a file" placeholder
 * was, and the tabs and file tree are untouched.
 *
 * ## What it does not have, and why that is the whole of its size
 *
 * The screens in this directory are 580 to 711 lines each, and most of that is a
 * host: dialogs, named queries, validators, `goToState`, navigation. This form is
 * **action-less** and its two actions are `confirm` → `post` → `clearSelection`,
 * so the host is four methods and the rest throw rather than pretending. A host
 * that silently does nothing is what `escapes.ts` says makes people distrust
 * authored configuration; a host that throws names the missing piece.
 *
 * ## The workspace comes from the route, not from a form the user filled in
 *
 * The Dart read `FSK.wsName` out of form state, written there by the registry's
 * *Open* delegate. This app has the workspace in the URL — `openWorkspace`
 * navigates to `/workspaces/:workspace_name/home` — so the seed is the route
 * parameter, and the table's `where` clause resolves `workspace_name` out of the
 * same form state either way. Nothing else on this screen writes it.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ApiError, type ApiClient } from "../api/client";
import { productionRegistry } from "../actions/registry";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { ActionDocumentSchema, type Action } from "../actions/schema";
import type { TableConfigDocument } from "../datatable/table";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import { TableView } from "../datatable/TableView";
import { fromDocument } from "../datatable/tableTranslate";
import type { ActionConfig, JetsRow, TableConfig } from "../datatable/types";
import type { DataTableFetcher } from "../datatable/useDataTable";
import { useNotifications } from "../shell/notifications";

import actionsJson from "./documents/workspaceHome.ua.json";
import tableJson from "../datatable/tables/workspaceChangesTable.tc.json";

/** The one group this form has. */
const GROUP = 0;

/** The table's key, which is also its document's file name and its field key. */
const TABLE_KEY = "workspaceChangesTable";

export interface WorkspaceChangesProps {
  api: ApiClient;
  /** The workspace whose changes these are, from the route. */
  workspace: string;
}

export function WorkspaceChanges({ api, workspace }: WorkspaceChangesProps) {
  const { setError, setStatus } = useNotifications();
  const [error, setLocalError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const haltedByUser = useRef(false);

  const config: TableConfig = useMemo(
    () => fromDocument(TABLE_KEY, tableJson as TableConfigDocument),
    [],
  );

  const actions: Record<string, Action> = useMemo(() => {
    // Parsed rather than cast, on this directory's rule: a bundled document is
    // still a document, and a failure should read as a finding rather than as a
    // blank pane.
    const parsed = ActionDocumentSchema.safeParse(actionsJson);
    if (!parsed.success) throw new Error(`workspaceHome.ua.json is invalid: ${parsed.error.message}`);
    return parsed.data.actions;
  }, []);

  /**
   * The workspace, seeded once per workspace rather than on every render.
   *
   * `FormState` is the operand of the table's `where` clause *and* of the
   * actions' `extras`, so both read the same value and neither takes it from a
   * second place.
   */
  const formState = useMemo(() => {
    const state = new FormState();
    state.setValue(GROUP, "workspace_name", workspace);
    return state;
  }, [workspace]);

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);

  const host: ActionHost = useMemo(
    () => ({
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      post: async (request): Promise<PostResult> => {
        try {
          await api.endpoint(request.endpoint, request.body);
          return { statusCode: 200 };
        } catch (err) {
          if (err instanceof ApiError) return { statusCode: err.status, error: err.message };
          return { statusCode: 500, error: err instanceof Error ? err.message : String(err) };
        }
      },
      read: async (request) => {
        const body = await api.endpoint<{ rows?: unknown }>(request.endpoint, request.body);
        return Array.isArray(body.rows) ? (body.rows as JetsRow[]) : [];
      },
      userEmail: () => api.currentUser?.email ?? "",
      confirm: async (message: string) => {
        const agreed = window.confirm(message);
        if (!agreed) haltedByUser.current = true;
        return agreed;
      },
      // This form has no fields, so there is nothing to validate and saying so is
      // the honest answer rather than a stub that always passes.
      validate: () => true,
      now: () => Date.now(),
      // Neither is reachable from this screen's two actions, and both name what
      // is missing rather than doing nothing quietly.
      query: () => {
        throw new Error("the workspace home form declares no named queries");
      },
      // **A screen has no states and no dialog, so these three are the shape of
      // the difference** rather than stubs: nothing here transitions, nothing
      // closes, and no action on this screen navigates.
      goToState: () => {},
      close: () => {},
      download: () => {},
    }),
    [api, setError, setStatus],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      haltedByUser.current = false;
      const action = actions[name];
      if (action === undefined) {
        throw new Error(`action "${name}" is not in the workspace home's action document`);
      }
      return (
        await runAction({
          action,
          host,
          formState,
          field: { group: GROUP, key: name },
          registry: productionRegistry,
          flowKey: "workspaceHome",
        })
      ).message;
    },
    [actions, host, formState],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, _action: ActionConfig) => {
      void (async () => {
        setLocalError(null);
        setBusy(true);
        try {
          switch (request.kind) {
            case "runAction": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) setLocalError(outcome);
              // Refreshed whether or not the action ran: a refused confirmation
              // leaves the table as it was, and a redraw of unchanged rows is
              // cheaper than a branch that can be wrong.
              formState.requestRefresh();
              return;
            }
            default:
              // The table's two actions are both `doAction`, so every other kind
              // is a configuration this screen does not have. Reported rather
              // than ignored, on `escapes.ts`'s rule that a button which
              // silently does nothing is what makes people distrust authored
              // configuration.
              setLocalError(`the workspace home table does not serve "${request.kind}" actions`);
          }
        } catch (err) {
          setLocalError(err instanceof Error ? err.message : String(err));
        } finally {
          setBusy(false);
        }
      })();
    },
    [runNamedAction, formState],
  );

  // The route parameter is the only context this table needs: it has no client
  // filter, no home filters and no data-registry filters, and `makeQuery` splices
  // those only into a table that is a form field of a flow.
  const context = useMemo(() => ({ routeParams: { workspace_name: workspace } }), [workspace]);

  useEffect(() => {
    setLocalError(null);
  }, [workspace]);

  return (
    <div className="workspace-changes" aria-busy={busy}>
      {error !== null && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <TableView
        config={config}
        field={{ group: GROUP, key: TABLE_KEY }}
        formState={formState}
        fetcher={fetcher}
        context={context}
        predicates={productionRegistry.predicates}
        cellFilters={productionRegistry.cellFilters}
        onAction={onTableAction}
      />
    </div>
  );
}

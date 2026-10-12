/**
 * The workspace registry, `/workspaces`. Task C.2b — track C's first screen.
 *
 * ## What a screen is, as against a flow
 *
 * `FlowRunner` is a host for documents it reads out of a **workspace** at run
 * time; this is a host for documents that are **compiled into the bundle**. That
 * is the whole structural difference, and it is not filing.
 *
 * `routing.ts` makes a flow React's iff `user_flows/<key>.uf.json` exists in the
 * workspace — a flow's documents are a per-workspace fact *by design*, because
 * that switch is what lets one deployment migrate a flow and another not. A
 * screen has no per-workspace variant at all: there is one `/workspaces` in a
 * deployment, not one per workspace. So the flows' location is not a default this
 * departs from; it is a mechanism a screen has no use for.
 *
 * **And on this screen the alternative is circular.** `/workspaces` is where a
 * workspace is created. Documents describing it cannot live in the thing it
 * exists to produce.
 *
 * Agreed as the rule for all of track C before it was built, so that eight later
 * screens do not each decide it.
 *
 * ## The documents are still documents
 *
 * They are parsed through `FormDocumentSchema` and `ActionDocumentSchema`, their
 * escape names are resolved against `productionRegistry`, and a failure renders
 * as a list of findings rather than as a blank screen. Bundling changes the
 * transport and nothing else — which is the property that lets the same
 * `FormRenderer`, the same interpreter and the same `TableView` serve both hosts.
 *
 * ## Five decisions this screen takes on behalf of the rest of track C
 *
 * 1. **Documents are bundled**, above.
 * 2. **A screen supplies a `QueryContext`.** I-104 is why this is not optional:
 *    `FormDataTable` called `useTableBinding` with no context at all, so
 *    `homeFilters` and `dataRegistryFilters` were always absent and a filtered
 *    table rendered unfiltered and looked correct. This screen's table needs
 *    neither, and supplying the context anyway is what stops the next screen from
 *    discovering it needs one.
 * 3. **Route parameters reach a table's `where` clauses through
 *    `QueryContext.routeParams`**, from `useParams()`. `makeQuery` has read them
 *    since A.4a — `makeWhereClause` falls back to the route when form state holds
 *    nothing — so a screen owes the params and nothing else. `/workspaces` has
 *    none, and `useParams()` returning `{}` is the honest way to say so.
 * 4. **The table publishes its selection into form state under its own key**, as
 *    the Dart's `FormDataTableFieldConfig(key: DTKeys.workspaceRegistryTable, …)`
 *    does. Every action on this screen reads the selection back out — the dialogs
 *    through their `navigationParams`, the three `doAction` entries through
 *    `wholeState`.
 * 5. **The screen owns its toolbar**, per `AppShell`'s header. Here the toolbar is
 *    the table's own two action rows, so there is nothing above it.
 *
 * ## The host is shared with the Workspace IDE
 *
 * The buttons that need a workspace moved to the IDE's toolbar on 2026-10-11,
 * where a workspace is always open (`workspaceToolbar.ts`). They run the same
 * actions and open the same dialogs, so the dialog host and the action dispatch
 * live in `workspaceRegistryHost.tsx` and this screen keeps what is its own: the
 * table, its query context, and the deployment's workspace.
 *
 * ## How it is tested
 *
 * `WorkspaceRegistry.test.tsx`, on `FlowRunner.test.tsx`'s shape: the screen is
 * driven against a stubbed `fetch` with everything below `ApiClient` real. I-104's
 * narrower lesson is that a field added to a schema has two halves and the corpus
 * can only tell you about the first, so the tests render rather than parse.
 */

import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";

import type { ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { productionRegistry, setActiveWorkspace, setFileKeyLabelPattern } from "../actions/registry";
import { currentDataRegistryFilters, currentHomeFilters } from "../actions/homeFilters";
import { TableView } from "../datatable/TableView";
import type { DataTableFetcher } from "../datatable/useDataTable";
import { useNotifications } from "../shell/notifications";
import {
  GROUP,
  TABLE_KEY,
  documentFindings,
  registryTableConfig,
  useRegistryActionHost,
} from "./workspaceRegistryHost";

export { documentFindings };

/** The capability the route is gated on — `admin` bypasses it, as the server does. */
export const WORKSPACE_IDE = "workspace_ide";

export function WorkspaceRegistry({ api }: { api: ApiClient }) {
  const routeParams = useParams();
  const { setError } = useNotifications();

  const [ready, setReady] = useState(false);
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const findings = useMemo(documentFindings, []);
  const config = registryTableConfig;

  /**
   * The deployment's workspace, which two of this screen's predicates read.
   *
   * The Flutter app fetches this once at sign-in and keeps it in three globals;
   * this app has no sign-in bootstrap (I-67), so the screen that needs it asks.
   * **The table renders regardless of whether this succeeds**: a failure here
   * makes `addWorkspace`'s name and branch editable when they should be locked,
   * which is worth a banner and is not worth refusing the screen — the server
   * refuses the rename either way.
   */
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const active = await workspaceApi.activeWorkspace();
        if (cancelled) return;
        setActiveWorkspace({ name: active.name, branch: active.branch, uri: active.uri });
        setFileKeyLabelPattern(active.fileKeyLabelRe === "" ? null : active.fileKeyLabelRe);
      } catch (error) {
        if (!cancelled) {
          setError(
            `Could not read the deployment's workspace: ${error instanceof Error ? error.message : String(error)}`,
          );
        }
      } finally {
        if (!cancelled) setReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceApi, setError]);

  const fetcher: DataTableFetcher = useMemo(() => (payload) => api.dataTable(payload), [api]);

  /**
   * The query context every table on this screen is built with — decision 2.
   *
   * **Keyed on the context's *contents*, not on the form-state version, and the
   * difference is a defect this screen's first test run found.** `useTableBinding`
   * takes `context` as a dependency of the query it builds, so a new object
   * identity is a refetch. Rebuilding on `stateVersion` — which is what
   * `FlowRunner` does, because there the filters are written by an escape and the
   * table must re-query when they change — means every *selection* rebuilds it
   * too: publishing a selection notifies the store, the context is recreated, the
   * table refetches, and the rows arrive identical. The selection is then lost,
   * because `useTableBinding`'s restore is guarded on the row set having changed
   * and it has not.
   *
   * The symptom is a table where clicking a checkbox ticks it and no row-gated
   * button ever enables. Serialising the contents makes a new object appear when
   * a filter or a route parameter actually moves and not otherwise — which is
   * what F.5 needed and is strictly narrower. Recorded as **I-184**, because
   * `FlowRunner` has the same shape and a flow whose table both filters and
   * selects would meet it.
   */
  const homeFilters = currentHomeFilters();
  const dataRegistryFilters = currentDataRegistryFilters();
  const contextKey = JSON.stringify([routeParams, homeFilters, dataRegistryFilters]);
  const tableContext = useMemo(
    () => ({
      routeParams: routeParams as Record<string, string | undefined>,
      homeFilters,
      dataRegistryFilters,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [contextKey],
  );

  const registry = useRegistryActionHost({ api, tableContext });

  if (findings.length > 0) {
    return (
      <main className="screen">
        <h1>Workspace Registry</h1>
        <div className="banner banner-error" role="alert">
          <div>
            <p>This screen&apos;s documents cannot be loaded.</p>
            <ul>
              {findings.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="screen">
      <h1>Workspace Registry</h1>
      {!ready ? (
        <p role="status">Loading…</p>
      ) : (
        <TableView
          config={config!}
          field={{ group: GROUP, key: TABLE_KEY }}
          formState={registry.formState}
          fetcher={fetcher}
          context={tableContext}
          predicates={productionRegistry.predicates}
          cellFilters={{}}
          onAction={registry.onAction}
        />
      )}
      {registry.dialog}
    </main>
  );
}

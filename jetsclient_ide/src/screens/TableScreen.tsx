/**
 * A screen that is one table, filtered by its route.
 *
 * ## Two routes, one screen, and that is a measurement rather than a shortcut
 *
 * `/executionStatusDetails/:session_id` and `/executionStatsDetails/:session_id`
 * are both `ScreenOne` in `jetsclient/lib/routes/jets_routes_app.dart`
 * (`executionStatusDetailsPath` and `executionStatsDetailsPath`), each with an
 * inert `validatorDelegate` and an `actionsDelegate` that returns null; and their
 * two `ScreenConfig`s (`jetsclient/lib/modules/screen_config_impl.dart`,
 * `ScreenKeys.execStatusDetailsTable` and `ScreenKeys.execStatsDetailsTable`) are
 * identical apart from the key, down to a commented-out `title:` in both. The
 * tables differ, and nothing else does.
 *
 * **`ScreenOne` is four of the twenty-six routed screens**, and this component is
 * all four: the two above, `/domainTableViewer/:table_name/:session_id` and
 * `/filePreviewPath/:file_key`. It is not the shape of the migrated screens
 * generally — `/processErrors/:session_id` is `ScreenWithForm` with four tables and two
 * dialogs — and saying "four of twenty-six" rather than "the simple ones" is what
 * keeps that checkable.
 *
 * ## The route parameter is the whole mechanism
 *
 * A table configuration says `where: [{column: "session_id", formStateKey:
 * "session_id"}]`, and `formStateKey` names a *form state* key on a screen that
 * has no form. The Dart resolves it out of the router:
 * `JetsRouterDelegate().currentConfiguration?.params[wc.formStateKey]`, consulted
 * when there is no form state at all (`jetsclient/lib/components/data_table_source.dart`,
 * `_makeWhereClause`). `makeQuery`'s port of that is `firstRouteParam`
 * (`datatable/query.ts`), reached through `QueryContext.routeParams` — so this
 * screen's one job beyond rendering is to put `useParams()` there.
 *
 * **Without it both screens show every session's rows and look entirely correct**,
 * which is a contract tested from the end that cannot fail, with a different field. `TableScreen.test.tsx`
 * therefore asserts the request the table sent, not the rows it drew.
 *
 * ## What a screen does *not* take from `FlowRunner`'s context, measured
 *
 * - `homeFilters` and `dataRegistryFilters` are spliced by `makeQuery` only
 *   inside `if (field && …)`, and a table that is not a form field has no
 *   `formField`. A screen table can never be filtered by them.
 * - `selectedClient` only adds the implicit client filter when the table has a
 *   column named `client`, and neither of these two does. **When this screen was
 *   built there was no `selectedClient` store in this app at all**; the home
 *   screen is where it came to exist, because `inputRegistryTable` does have the
 *   column.
 *
 * ## The document comes from the bundle, not the workspace
 *
 * `FlowStore` reads a flow's `.tc.json` out of the workspace, because
 * `routing.ts` makes a flow React's *iff* `user_flows/<key>.uf.json` exists
 * there — a flow is a per-workspace fact by design. **A screen has no
 * per-workspace variant**: there is one `/executionStatusDetails` per deployment,
 * not one per workspace, and the Flutter equivalent is `getTableConfig(DTKeys.…)`
 * compiled into the app. So a screen imports its committed document and calls
 * `fromDocument`, and the workspace path is a mechanism screens have no use for
 * rather than a default they are departing from.
 *
 * The same rule covers a screen's **form** documents, at
 * `src/screens/documents/<screenKey>.form.json`; that half was reached
 * independently and stated here once rather than twice.
 */

import { useMemo } from "react";
import { useParams } from "react-router-dom";

import type { ApiClient } from "../api/client";
import { DataTable } from "../datatable/DataTable";
import type { TableConfigDocument } from "../datatable/table";
import { fromDocument } from "../datatable/tableTranslate";
import { useDataTable, type DataTableFetcher } from "../datatable/useDataTable";

export interface TableScreenProps {
  api: ApiClient;
  /** The table configuration key, which is also the document's file name. */
  tableKey: string;
  /** The committed `.tc.json`, imported by the route. */
  document: TableConfigDocument;
  /** The screen's heading. Both execution details screens have none — see below. */
  title?: string;
}

/**
 * One screen state per table. Every route that renders this component puts it at
 * the same place in the tree, so going straight from one such route to another
 * would otherwise reuse the first table's state (its columns and its sort) and
 * query the second table with them. The key makes each table its own instance.
 */
export function TableScreen(props: TableScreenProps) {
  return <TableScreenBody key={props.tableKey} {...props} />;
}

function TableScreenBody({ api, tableKey, document, title }: TableScreenProps) {
  const params = useParams();

  // `fromDocument` restores the constants the schema dropped — `apiPath`,
  // `apiAction` and the rest — so what the hooks below consume is the same
  // `TableConfig` shape the query builder has always taken. The document is a
  // module-level import, so this memo is about identity rather than cost:
  // `useDataTable` keys its effect on the serialised payload, and a config
  // rebuilt every render would be a new object inside it every time.
  const config = useMemo(() => fromDocument(tableKey, document), [tableKey, document]);

  const fetcher: DataTableFetcher = useMemo(
    () => (payload) => api.dataTable(payload),
    [api],
  );

  // `useDataTable` rather than `useTableBinding`, deliberately. The binding hook
  // wires a table to an enclosing form — it requires a `FormField` and a
  // `FormState` and publishes selections into them — and this table has neither.
  // `useDataTable`'s own header says it is "usable by a table that has no form at
  // all", and a screen is the case it was written for.
  const context = useMemo(() => ({ routeParams: params }), [params]);
  const state = useDataTable({ config, context, fetcher });

  return (
    <main className="screen">
      {/*
        **No heading unless a route asks for one, and both execution details
        screens do not.** Their `ScreenConfig`s carry `title:` commented out, so `ScreenOne`
        renders no title row and the table's own label is the only heading —
        "Pipeline Execution Details" and "CPIPES Execution Details", which come
        off the document and are drawn by `DataTable`. Adding one here would put
        the same words on the page twice.
      */}
      {title !== undefined && <h1>{title}</h1>}
      <DataTable config={config} state={state} />
    </main>
  );
}

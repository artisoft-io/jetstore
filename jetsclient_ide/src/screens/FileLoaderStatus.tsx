/**
 * The file loader status screen. Task D.10, from **I-260**.
 *
 * ## It is a tab that became a route
 *
 * *File Loader Status* was the second of the home screen's three tabs, in Flutter
 * and in this port. The report moves it: `fileMappingUF`'s first screen gains a
 * *Loader Status* button, which is a table action naming a `configScreenPath`,
 * and a `configScreenPath` needs a screen to name. So this file exists because a
 * button needs a destination, and the tab could not leave `Home` before it did —
 * which is why D.4 shipped the rest of I-260 and left the tab where it was.
 *
 * **The route has no Flutter predecessor**, which is the one thing worth knowing
 * before reading `screens/routes.ts`'s row for it: every other key in
 * `SERVED_SCREENS` is a template copied out of `jets_routes_app.dart`, and this
 * one is chosen here.
 *
 * ## Why not `TableScreen`
 *
 * `TableScreen` is the four `ScreenOne` routes and it renders a bare `DataTable`
 * — no action bar, no form state, no selection published anywhere. This table has
 * two configured actions, and one of them, *View Loaded Data*, is the only way in
 * the app to reach `/domainTableViewer` for a load; it reads `navigationParams`
 * off the selected row, which needs a selection, which needs the binding. So this
 * screen composes `TableView` the way `Home` does, and what is below is that
 * screen minus everything a single table does not have: no tab strip, no dialogs,
 * no forms, no action document.
 *
 * **The action handler is one case wide and says so for the rest.** The document
 * configures a `showScreen` and a `refreshTable`, and the second never reaches
 * here — `ActionBar` answers the widget's own two out of the binding (D.10,
 * `WidgetActions`). Anything else would be a document this screen does not have,
 * and it reports rather than failing silently, which is the convention
 * `routes.ts` states for an unserved path and `escapes.ts` for an unresolved
 * name.
 *
 * ## The client filter reaches this table, and it does so by default
 *
 * `inputLoaderStatusTable` has a `client` column, so the shell's picker must
 * narrow it — it did while the table was a tab on `Home`. Nothing here passes
 * `selectedClient`, and that is D.3's decision rather than an omission:
 * `useTableBinding` reads the store itself, because a forgotten context yields a
 * filter that is quietly *off* and returns **more** rows rather than an error.
 * `FileLoaderStatus.test.tsx` asserts the request rather than the rows, for the
 * same reason `TableScreen.test.tsx` does.
 */

import { useCallback, useMemo } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { cellFiltersOf } from "../actions/cellFilters";
import { productionRegistry } from "../actions/registry";
import type { ApiClient } from "../api/client";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import { TableConfigDocumentSchema, type TableConfigDocument } from "../datatable/table";
import { TableView } from "../datatable/TableView";
import { fromDocument } from "../datatable/tableTranslate";
import type { ActionConfig } from "../datatable/types";
import type { DataTableFetcher } from "../datatable/useDataTable";
import { useNotifications } from "../shell/notifications";
import { inAppPath, unservedScreenMessage, withReturnTo } from "./routes";

import inputLoaderStatusTable from "../datatable/tables/inputLoaderStatusTable.tc.json";

/** The table configuration key, which is also the document's file name. */
export const LOADER_STATUS_TABLE = "inputLoaderStatusTable";

/** One group, and the table's own key is what its selection is published under. */
const GROUP = 0;

/**
 * Parsed once at module load, for C.2b's stated reason: a document that does not
 * satisfy its schema fails in every test that imports this module rather than
 * when a user presses a button.
 */
const parsed = TableConfigDocumentSchema.safeParse(inputLoaderStatusTable);

/**
 * The document's two display filters, resolved once.
 *
 * **They are the reason this screen reads its filters at all**, and they are not
 * the same one: `file_key` names `fileKeyLabel` and `error_message` names
 * `errorMessageLabel`. The translation used to send both to `fileKeyLabel`, which
 * rendered the message as `.../` plus whatever followed its last slash — the
 * whole string, for a message with no slash in it. This travelled here with the
 * table; the assertion that kept it honest was on `Home.test.tsx` and is now on
 * this screen's test.
 */
const CELL_FILTERS = cellFiltersOf(
  parsed.success ? (parsed.data as TableConfigDocument) : undefined,
);

const CONFIG = parsed.success ? fromDocument(LOADER_STATUS_TABLE, parsed.data) : null;

export function FileLoaderStatus({ api }: { api: ApiClient }) {
  const navigate = useNavigate();
  /** Where this screen was opened, so a flow it opens can come back here (D.8). */
  const location = useLocation();
  const here = `${location.pathname}${location.search}`;
  const { setError } = useNotifications();

  const formState = useMemo(() => new FormState(), []);
  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      if (request.kind === "navigate") {
        const internal = inAppPath(action.configScreenPath, request.params);
        // `withReturnTo` is a no-op for a screen path and this table names only
        // one — it is here so that a flow added to this document later comes back
        // rather than landing on the front door (D.8).
        if (internal !== null) void navigate(withReturnTo(internal, here));
        else setError(unservedScreenMessage(action.label, request.path));
        return;
      }
      setError(`"${action.label}" is a kind of action this screen does not configure`);
    },
    [navigate, here, setError],
  );

  if (CONFIG === null) {
    return (
      <main className="screen">
        <h1>File Loader Status</h1>
        <div className="banner banner--error" role="alert">
          <p>
            {LOADER_STATUS_TABLE}.tc.json does not satisfy its schema, so this screen
            cannot draw its table.
          </p>
          <ul>
            {parsed.success
              ? null
              : parsed.error.issues.map((issue) => (
                  <li key={`${issue.path.join("/")}-${issue.message}`}>
                    {issue.path.join("/")} — {issue.message}
                  </li>
                ))}
          </ul>
        </div>
      </main>
    );
  }

  return (
    <main className="screen">
      {/*
        **No heading, and the table's own label is the page's.** The document says
        *File Loader Status* and `DataTable` draws it, so an `<h1>` here would put
        the same words on the page twice — the rule `TableScreen` states for C.7
        and C.8, and the same one that decides the home screen's pipelines caption
        the other way: there the tab strip already names it, so the caption goes.
      */}
      <TableView
        config={CONFIG}
        field={{ group: GROUP, key: LOADER_STATUS_TABLE }}
        formState={formState}
        fetcher={fetcher}
        predicates={productionRegistry.predicates}
        cellFilters={CELL_FILTERS}
        onAction={onTableAction}
      />
    </main>
  );
}

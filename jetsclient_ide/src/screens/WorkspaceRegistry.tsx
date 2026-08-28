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
 * ## How it is tested
 *
 * `WorkspaceRegistry.test.tsx`, on `FlowRunner.test.tsx`'s shape: the screen is
 * driven against a stubbed `fetch` with everything below `ApiClient` real. I-104's
 * narrower lesson is that a field added to a schema has two halves and the corpus
 * can only tell you about the first, so the tests render rather than parse.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry, setActiveWorkspace, setFileKeyLabelPattern } from "../actions/registry";
import { currentDataRegistryFilters, currentHomeFilters } from "../actions/homeFilters";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { describeUnresolved, resolveEscapes } from "../actions/escapes";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import { TableView } from "../datatable/TableView";
import { fromDocument } from "../datatable/tableTranslate";
import { TableConfigDocumentSchema, tableEscapeReferences } from "../datatable/table";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig, JetsRow, TableConfig } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { inAppPath, unservedScreenMessage, withReturnTo } from "./routes";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { resolveQuery } from "../userflow/formQueries";
import { useFormQueries } from "../userflow/useFormQueries";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";

import actionsJson from "./documents/workspaceRegistry.ua.json";
import formsJson from "./documents/workspaceRegistry.form.json";
import tableJson from "../datatable/tables/workspaceRegistryTable.tc.json";

/** The capability the route is gated on — `admin` bypasses it, as the server does. */
export const WORKSPACE_IDE = "workspace_ide";

/** The table's key, which is also the form-state key its selection is published under. */
const TABLE_KEY = "workspaceRegistryTable";

/**
 * One group, as every non-repeating form uses.
 *
 * The screen and its dialogs share it deliberately: a dialog's fields are seeded
 * from the row the table published, and a second group would mean copying the
 * selection across before every dialog and back after. The Dart shares one
 * `JetsFormState` between the screen and its dialogs for the same reason
 * (`components/data_table.dart`, `actionDispatcher`).
 */
const GROUP = 0;

/**
 * The three documents, parsed once at module load.
 *
 * **Parsed rather than cast**, which is the point of bundling them as JSON: a
 * document that does not satisfy its schema fails here, in every test that
 * imports this module, rather than at the moment a user presses a button. A cast
 * would make the bundling a way of skipping the check.
 */
const parsed = {
  forms: FormDocumentSchema.safeParse(formsJson),
  actions: ActionDocumentSchema.safeParse(actionsJson),
  table: TableConfigDocumentSchema.safeParse(tableJson),
};

/** Findings from parsing and from resolving escape names, for the banner. */
export function documentFindings(): string[] {
  const findings: string[] = [];
  for (const [name, result] of Object.entries(parsed)) {
    if (!result.success) {
      for (const issue of result.error.issues) {
        findings.push(`${name}: ${issue.path.join("/")} — ${issue.message}`);
      }
    }
  }
  if (findings.length > 0) return findings;
  const references = [
    ...formEscapeReferences(parsed.forms.data as FormDocument, "workspaceRegistry"),
    ...tableEscapeReferences(parsed.table.data!).map((r) => ({
      ...r,
      at: `workspaceRegistryTable.tc.json${r.at}`,
    })),
  ];
  for (const step of Object.entries((parsed.actions.data as ActionDocument).actions)) {
    step[1].steps.forEach((s, index) => {
      if (s.do === "escape") {
        references.push({ kind: "actions", name: s.name, at: `/actions/${step[0]}/steps/${index}` });
      }
      if (s.do === "query") {
        references.push({ kind: "queries", name: s.name, at: `/actions/${step[0]}/steps/${index}` });
      }
    });
  }
  // `describeUnresolved` takes the whole list and returns one message or null —
  // the same shape `FlowStore` shows for a flow, so a screen's failure and a
  // flow's read alike.
  const message = describeUnresolved(resolveEscapes(references, productionRegistry));
  return message === null ? [] : [message];
}

export function WorkspaceRegistry({ api }: { api: ApiClient }) {
  const routeParams = useParams();
  const navigate = useNavigate();
  /** Where a flow opened from this screen should return to (D.8). */
  const location = useLocation();
  const here = `${location.pathname}${location.search}`;
  const { setError, setStatus } = useNotifications();

  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  /**
   * Bumped on every form-state change so the screen re-renders with it.
   *
   * The same device `FlowRunner` uses and for the same reason: `FormState` is
   * mutable and keeps its identity, so nothing else tells React that a selection
   * was published or a field typed into. **It is deliberately not a dependency of
   * `tableContext` below** — see the note there.
   */
  const [, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  /**
   * Whether the last action stopped because the *user* stopped it. **I-186.**
   *
   * `runAction` returns `string | null`, and `null` means two different things:
   * the action ran to the end, or a `validate` step failed / a `confirm` was
   * refused — both of which return `{ done: true, outcome: null }`
   * (`actions/interpret.ts`, the `validate` and `confirm` arms). A flow runner
   * does not need to tell them apart, because the *engine* validates separately
   * and simply does not advance. **A dialog host does**: treating a failed
   * validation as success closes the dialog over the messages it just produced,
   * which is what the first version of this screen did — the required-field
   * error rendered for one frame into a dialog that was being unmounted.
   *
   * Rather than widen `runAction`'s return type — an exported contract another
   * project reads — the host records what it was asked. It is the host's own
   * answer being remembered, which is why a ref is honest here and not a
   * workaround.
   */
  const haltedByUser = useRef(false);
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);
  const dialog = useFormDialog();

  const findings = useMemo(documentFindings, []);
  const forms = parsed.forms.success ? parsed.forms.data.forms : {};
  const actions = parsed.actions.success ? parsed.actions.data.actions : {};
  const config: TableConfig | null = parsed.table.success
    ? fromDocument(TABLE_KEY, parsed.table.data)
    : null;

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);

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

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);
  const queryPost = useCallback(
    (payload: Record<string, unknown>) =>
      api.dataTable<{ result_map?: Record<string, unknown> }>(payload),
    [api],
  );

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

  const currentForm: Form | null =
    dialog.request === null ? null : (forms[dialog.request.form] ?? null);
  const queries = useFormQueries(currentForm, formState, queryPost);

  const host: ActionHost = useMemo(
    () => ({
      query: async (name: string) => {
        const registered = productionRegistry.queries[name];
        if (registered === undefined) throw new Error(`named query "${name}" is not registered`);
        const sql = resolveQuery(
          { sql: registered.sql, ...(registered.params ? { params: [...registered.params] } : {}) },
          formState,
        );
        if (sql === null) return null;
        const body = await queryPost({ action: "raw_query_map", query_map: { [name]: sql } });
        const rows = body.result_map?.[name];
        if (!Array.isArray(rows) || rows.length === 0) return null;
        const first = rows[0] as (string | null)[];
        return Object.fromEntries(
          registered.columns.map((column, index) => [column, first[index] ?? null]),
        );
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateAllGroups(currentForm, formState, GROUP);
        setErrors(found);
        if (found.length > 0) haltedByUser.current = true;
        return found.length === 0;
      },
      confirm: async (message: string) => {
        const agreed = window.confirm(message);
        if (!agreed) haltedByUser.current = true;
        return agreed;
      },
      post: async (request): Promise<PostResult> => {
        try {
          await api.endpoint(request.endpoint, request.body);
          return { statusCode: 200 };
        } catch (error) {
          if (error instanceof ApiError) return { statusCode: error.status, error: error.message };
          return { statusCode: 500, error: error instanceof Error ? error.message : String(error) };
        }
      },
      read: async (request) => {
        try {
          const body = await api.endpoint<{ rows?: unknown }>(request.endpoint, request.body);
          return Array.isArray(body.rows) ? (body.rows as JetsRow[]) : [];
        } catch {
          return null;
        }
      },
      download: (fileName, content) => {
        const url = URL.createObjectURL(new Blob([content], { type: "text/csv" }));
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = fileName;
        anchor.click();
        URL.revokeObjectURL(url);
      },
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      // **A screen has no states, so these two are the shape of the difference.**
      // `goToState` is a flow's transition and there is nothing here to go to;
      // `close` on a flow leaves it, and on a screen the nearest thing is
      // dismissing the dialog that is open. A dialog's action document says
      // nothing about either, so both are inert rather than wrong.
      goToState: () => {},
      close: () => dialog.close("ok"),
      // C.3's route, reached without a page reload. The cross-*app* link a
      // `showScreen` action makes is still `window.location.href` below; this is
      // for a destination inside this bundle, and `openWorkspace` is the one
      // escape that has one.
      navigate: (path: string) => navigate(path),
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [api, currentForm, formState, navigate, queryPost, setError, setStatus, dialog.close],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      haltedByUser.current = false;
      const action = actions[name];
      if (action === undefined) {
        throw new Error(`action "${name}" is not in this screen's action document`);
      }
      return (await runAction({
        action,
        host,
        formState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: "workspaceRegistry",
      })).message;
    },
    [actions, host, formState],
  );

  /**
   * A table action's button.
   *
   * The four request kinds a screen sees. `promptFilter` cannot arrive — this
   * table has no filter buttons — and reaching it would be a configuration this
   * screen does not have, so it reports rather than being silently absent.
   */
  /**
   * Opens a dialog and waits for it, **without holding `busy`**.
   *
   * `busy` means *an action is in flight*, and every button in the app reads it —
   * including the dialog's own. Holding it across the dialog's lifetime, which is
   * what awaiting inside the `try` did, disabled every button in the form the
   * dialog had just opened: the OK button could not be pressed, so the dialog
   * could not be closed, so `busy` was never released. **A modal is not work.**
   *
   * Found by the first rendering test of the dialog host and worth the paragraph
   * rather than the one-line fix, because the shape recurs: `setBusy(true)` in a
   * `try` and `setBusy(false)` in its `finally` is right for a request and wrong
   * for anything that waits on the user.
   */
  const openAndRefresh = useCallback(
    async (form: string, params: Record<string, string>) => {
      setBusy(false);
      const outcome = await dialog.open({ form, params });
      if (outcome === "ok") formState.requestRefresh();
    },
    [dialog, formState],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      void (async () => {
        setError(null);
        setBusy(true);
        try {
          switch (request.kind) {
            case "runAction": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) setError(outcome);
              formState.requestRefresh();
              return;
            }
            case "navigate": {
              // **Both of this table's `configScreenPath`s are flows** — *Load
              // Client Config* and *Pull Workspace* — and until X.1 this handed
              // the browser to the Flutter app to serve them. There is no other
              // app now, so the same two templates resolve in-app through the map
              // ported out of the Dart before it was deleted.
              const internal = inAppPath(action.configScreenPath, request.params);
              if (internal !== null) {
                // D.8: both of these are flows, so both come back here.
                void navigate(withReturnTo(internal, here));
                return;
              }
              setError(unservedScreenMessage(action.label, request.path));
              return;
            }
            case "openDialog": {
              // **The dialog host, I-68's consumer.** The params the action
              // resolved are seeded into the shared group before the form opens,
              // because the dialog's fields read them by key — that is what makes
              // `navigationParams` mean anything.
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              setErrors([]);
              await openAndRefresh(request.form, request.params);
              return;
            }
            case "runActionThenDialog": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) {
                setError(outcome);
                return;
              }
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              await openAndRefresh(request.form, request.params);
              return;
            }
            case "escape": {
              const escape = productionRegistry.actions[request.name];
              if (escape === undefined) {
                setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
                return;
              }
              await escape({ formState, group: GROUP, flowKey: "workspaceRegistry" }, host);
              formState.requestRefresh();
              return;
            }
            case "promptFilter":
              setError(`"${action.label}" asks for a filter, which this screen does not configure`);
              return;
          }
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
        } finally {
          setBusy(false);
        }
      })();
    },
    [dialog, formState, host, openAndRefresh, runNamedAction, setError],
  );

  /** A dialog's button. Cancel is the dialog's; everything else is the document's. */
  const onFormAction = useCallback(
    (action: FormAction) => {
      if (isDialogCancel(action)) {
        dialog.close("cancel");
        return;
      }
      void (async () => {
        setBusy(true);
        try {
          const outcome = await runNamedAction(action.action);
          if (outcome !== null) {
            setError(outcome);
            dialog.close("failed");
            return;
          }
          // The dialog stays open when the user stopped it — see `haltedByUser`.
          // The field errors `validate` produced are on screen and the form is
          // still there to correct.
          if (haltedByUser.current) return;
          dialog.close("ok");
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
          dialog.close("failed");
        } finally {
          setBusy(false);
        }
      })();
    },
    [dialog, runNamedAction, setError],
  );

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
          formState={formState}
          fetcher={fetcher}
          context={tableContext}
          predicates={productionRegistry.predicates}
          cellFilters={{}}
          onAction={onTableAction}
        />
      )}
      {currentForm !== null && (
        <FormDialog
          form={currentForm}
          errors={errors}
          onDismiss={() => dialog.close("cancel")}
          host={{
            formState,
            group: GROUP,
            queryRows: queries.rows,
            queriesLoading: queries.loading,
            tableConfig: () => config!,
            fetcher,
            tableContext,
            predicates: productionRegistry.predicates,
            cellFilters: () => ({}),
            onTableAction,
            onFormAction,
            formValid: true,
            groupCount: 1,
            busy,
          }}
        />
      )}
    </main>
  );
}

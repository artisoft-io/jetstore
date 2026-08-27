/**
 * The home screen — Flutter's `/`, this app's `/home`. Task C.6.
 *
 * ## Three tabs, not three tables
 *
 * `sizing_screen_migration.md` §7 records this row as *3 tables, 1 dialog*, and
 * that counts configuration rather than what is on screen at once. Flutter's
 * `FormKeys.home` is an **action-less form** whose `formTabsConfig` holds three
 * `FormTabConfig`s, one data-table field each
 * (`jetsclient/lib/modules/form_config_impl.dart`, `FormKeys.home`). Rendering
 * all three would put three `/dataTable` requests on the application's front
 * door where Flutter issues one, so the tabs are reproduced and only the selected
 * one is mounted.
 *
 * ## The tables are form fields, and the filters depend on it
 *
 * **C.7's handoff says a screen table can never be filtered by `homeFilters` or
 * `dataRegistryFilters`, and that is true of a `ScreenOne` and false here.**
 * `makeQuery` splices both only inside `if (field && field.key === …)`
 * (`datatable/query.ts`, `makeQuery`), and their reading — *a table that is not a
 * form field has no `formField`* — is exactly right. What does not carry is the
 * inference: this screen's tables **are** form fields, because the Dart's are,
 * and the field key it compares is `DTKeys.pipelineExecStatusTable` /
 * `DTKeys.inputRegistryTable` — the table's own key.
 *
 * So each `TableView` below is given `field={{group: 0, key: <the table key>}}`,
 * which is `FormDataTableFieldConfig(key: DTKeys.x, dataTableConfig: DTKeys.x)`.
 * **Without it the filters the whole of `homeFiltersUF` exists to set reach
 * nothing and the screen looks correct**, which is I-104 on a fifth surface.
 * `Home.test.tsx` mutation-tests the field key for that reason.
 *
 * ## Three sources of filter, and only one of them is this screen's
 *
 * | Filter | Set by | Reaches |
 * |---|---|---|
 * | `homeFilters` | `homeFiltersUF`, through `actions/homeFilters.ts` | `pipelineExecStatusTable` |
 * | `dataRegistryFilters` | the same flow | `inputRegistryTable` |
 * | `selectedClient` | the shell's picker (`shell/selectedClient.ts`) | any table with a `client` column — all three here |
 *
 * None of the three is stored on this screen, and that is I-102 decision 5 plus
 * its extension: *there is one build, so there is one set of filters*. The two
 * prompt buttons write `homeFilters` through `setIdFilter`, which is the same
 * module.
 *
 * ## The one table that is not translated from the screen corpus
 *
 * `inputRegistryTable` is one of the flows' 37. It is registered in
 * `jetsclient/lib/modules/user_flows/start_pipeline/data_table_config.dart` and
 * rendered here, which is **F18 read backwards** — that fact records a table
 * registered outside the flows and rendered by one. `pipelineExecStatusTable`
 * arrived with F.5. So this screen's three tables cost **one** translation.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry } from "../actions/registry";
import {
  currentDataRegistryFilters,
  currentHomeFilters,
  setIdFilter,
} from "../actions/homeFilters";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { describeUnresolved, resolveEscapes } from "../actions/escapes";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import { TableView } from "../datatable/TableView";
import { fromDocument } from "../datatable/tableTranslate";
import {
  TableConfigDocumentSchema,
  tableEscapeReferences,
  tablePath,
  type TableConfigDocument,
} from "../datatable/table";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig, JetsRow, TableConfig } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { selectedClient, subscribeToClient } from "../shell/selectedClient";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";
import { inAppPath, unservedScreenMessage } from "./routes";

import actionsJson from "./documents/homeScreen.ua.json";
import formsJson from "./documents/homeScreen.form.json";
import inputLoaderStatusTable from "../datatable/tables/inputLoaderStatusTable.tc.json";
import inputRegistryTable from "../datatable/tables/inputRegistryTable.tc.json";

/**
 * The three tabs, in the Dart's order, with the Dart's labels.
 *
 * The order is `formTabsConfig`'s and the labels are its `FormTabConfig.label`s —
 * not the tables' own `label`s, which differ: the second tab is *Pipeline
 * Execution Status* and its table's label is the same, while the third is *Data
 * Registry* and its table's is *File and Domain Table Registry*. Two names for
 * one thing, in the Dart, and this reproduces both because the tab strip and the
 * table caption are both visible.
 */
const TABS = [
  { key: "inputLoaderStatusTable", label: "File Loader Status", document: inputLoaderStatusTable },
  // **The middle tab's document is not bundled**, and it is the only table in
  // either corpus with two kinds of consumer: `homeFiltersUF` draws it from the
  // workspace and this screen draws it here. It was committed twice for a day —
  // once under `jets/workspace_assets/table_configs/` and once beside its
  // neighbours — with a test asserting the copies agreed, which is a guard rather
  // than a fix. One copy, read from where the flow reads it.
  { key: "pipelineExecStatusTable", label: "Pipeline Execution Status", document: null },
  { key: "inputRegistryTable", label: "Data Registry", document: inputRegistryTable },
] as const;

/** The tab whose document comes from the workspace. */
const WORKSPACE_TABLE = "pipelineExecStatusTable";

/** One group, shared with the dialog — C.2b's decision, and the Dart's. */
const GROUP = 0;

/** The screen key, which names its documents and its action-document scope. */
const SCREEN_KEY = "homeScreen";

/**
 * The five documents, parsed once at module load.
 *
 * Parsed rather than cast, for C.2b's stated reason: a document that does not
 * satisfy its schema fails in every test that imports this module rather than
 * when a user presses a button.
 */
const parsed = {
  forms: FormDocumentSchema.safeParse(formsJson),
  actions: ActionDocumentSchema.safeParse(actionsJson),
  tables: TABS.filter((tab) => tab.document !== null).map((tab) => ({
    key: tab.key,
    result: TableConfigDocumentSchema.safeParse(tab.document),
  })),
};

/** Findings from parsing and from resolving escape names, for the banner. */
/**
 * One parsed table, as `parsed.tables` holds them, so a workspace-loaded document
 * can join the bundled ones for every check below.
 */
type ParsedTable = { key: string; result: ReturnType<typeof TableConfigDocumentSchema.safeParse> };

/**
 * The bundled tables, plus the workspace one when it has arrived.
 *
 * **Every check in `documentFindings` needs all three and two of them are only
 * meaningful on the third.** `pipelineExecStatusTable` is the only table on this
 * screen whose actions include a `doAction` — `resubmitPipeline` — so a findings
 * pass that ran over the two bundled documents would resolve nothing and report
 * nothing, which reads exactly like a pass. That is why the parameter is
 * threaded through rather than the check being left at module scope.
 */
const tablesWith = (workspaceTable: TableConfigDocument | null): ParsedTable[] =>
  workspaceTable === null
    ? parsed.tables
    : [
        ...parsed.tables,
        { key: WORKSPACE_TABLE, result: TableConfigDocumentSchema.safeParse(workspaceTable) },
      ];

export function documentFindings(workspaceTable: TableConfigDocument | null = null): string[] {
  const tables = tablesWith(workspaceTable);
  const findings: string[] = [];
  for (const [name, result] of [["forms", parsed.forms], ["actions", parsed.actions]] as const) {
    if (!result.success) {
      for (const issue of result.error.issues) {
        findings.push(`${name}: ${issue.path.join("/")} — ${issue.message}`);
      }
    }
  }
  for (const table of tables) {
    if (!table.result.success) {
      for (const issue of table.result.error.issues) {
        findings.push(`${table.key}: ${issue.path.join("/")} — ${issue.message}`);
      }
    }
  }
  if (findings.length > 0) return findings;

  const references = [
    ...formEscapeReferences(parsed.forms.data as FormDocument, SCREEN_KEY),
    ...tables.flatMap((table) =>
      tableEscapeReferences(table.result.data as TableConfigDocument).map((r) => ({
        ...r,
        at: `${table.key}.tc.json${r.at}`,
      })),
    ),
  ];
  for (const [name, action] of Object.entries((parsed.actions.data as ActionDocument).actions)) {
    action.steps.forEach((step, index) => {
      if (step.do === "escape") {
        references.push({ kind: "actions", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
      if (step.do === "query") {
        references.push({ kind: "queries", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
    });
  }

  /**
   * **The check a document-level check cannot make**, and it is the one that
   * would have caught the cell-filter mistranslation had it existed sooner: every
   * `doAction` on any of the three tables must name an entry in *this screen's*
   * action document. `tableEscapeReferences` reports escape names; an
   * `actionName` is a reference into a different document and nothing resolved
   * it. `pipelineExecStatusTable`'s `resubmitPipeline` is the only one.
   */
  const declared = new Set(Object.keys((parsed.actions.data as ActionDocument).actions));
  for (const table of tables) {
    const document = table.result.data as TableConfigDocument;
    if (document.source !== "query") continue;
    for (const [row, actions] of [
      ["actions", document.actions ?? []],
      ["secondRowActions", document.secondRowActions ?? []],
    ] as const) {
      actions.forEach((action, index) => {
        if (action.action === "doAction" && !declared.has(action.actionName ?? "")) {
          findings.push(
            `${table.key}.tc.json/${row}/${index}: "${action.actionName}" is in no action of ${SCREEN_KEY}.ua.json`,
          );
        }
      });
    }
  }

  const message = describeUnresolved(resolveEscapes(references, productionRegistry));
  if (message !== null) findings.push(message);
  return findings;
}

export function Home({ api }: { api: ApiClient }) {
  const routeParams = useParams();
  const navigate = useNavigate();
  const { setError, setStatus } = useNotifications();

  const [tab, setTab] = useState(0);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [stateVersion, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  const dialog = useFormDialog();

  /**
   * The one table this screen reads from the workspace, and its failure.
   *
   * `null` while it is in flight, which is why the tab renders a placeholder
   * rather than an empty table: an empty `TableConfig` would draw a table with no
   * columns and say nothing, and this screen is the app's front door.
   */
  const [workspaceTable, setWorkspaceTable] = useState<TableConfigDocument | null>(null);
  const [workspaceFinding, setWorkspaceFinding] = useState<string | null>(null);

  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        // The deployment's workspace, not the IDE's picker — `FlowRunner` reads
        // the same table under the same rule, and the two must agree about which
        // workspace they mean or they are not sharing a document at all.
        const active = await workspaceApi.activeWorkspace();
        const file = await workspaceApi.readWorkspaceDocument(
          active.name,
          tablePath(WORKSPACE_TABLE),
        );
        const parsedTable = TableConfigDocumentSchema.safeParse(JSON.parse(file.content));
        if (cancelled) return;
        if (!parsedTable.success) {
          setWorkspaceFinding(
            `${WORKSPACE_TABLE}.tc.json: ${parsedTable.error.issues
              .map((i) => `${i.path.join("/")} — ${i.message}`)
              .join("; ")}`,
          );
          return;
        }
        setWorkspaceTable(parsedTable.data as TableConfigDocument);
      } catch (error) {
        if (cancelled) return;
        // Reported rather than thrown, and with the path in it: the likely causes
        // are a workspace built before these became assets and a deployment whose
        // install did not run, and both are answered by naming the file.
        setWorkspaceFinding(
          `Cannot read ${tablePath(WORKSPACE_TABLE)}: ${
            error instanceof ApiError ? error.message : (error as Error).message
          }`,
        );
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceApi]);

  const findings = useMemo(
    () => [...documentFindings(workspaceTable), ...(workspaceFinding === null ? [] : [workspaceFinding])],
    [workspaceTable, workspaceFinding],
  );
  const forms = parsed.forms.success ? parsed.forms.data.forms : {};
  const actions = parsed.actions.success ? parsed.actions.data.actions : {};
  const configs = useMemo(
    () =>
      tablesWith(workspaceTable).reduce<Record<string, TableConfig>>(
        (acc, table) =>
          table.result.success ? { ...acc, [table.key]: fromDocument(table.key, table.result.data) } : acc,
        {},
      ),
    [workspaceTable],
  );

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);

  /**
   * Re-render when the shell's client picker moves.
   *
   * The picker writes a module store the query builder reads, so nothing else
   * tells this screen that its tables' queries changed. Bumping the same counter
   * the form state bumps is deliberate: both feed `contextKey` below, and one
   * signal for "something the query depends on moved" is easier to reason about
   * than two.
   */
  useEffect(() => subscribeToClient(() => setStateVersion((n) => n + 1)), []);

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);
  const queryPost = useCallback(
    (payload: Record<string, unknown>) =>
      api.dataTable<{ result_map?: Record<string, unknown> }>(payload),
    [api],
  );

  /**
   * The query context all three tables are built with.
   *
   * **Keyed on the contents rather than on `stateVersion`, per I-184.** C.2b found
   * that rebuilding this object on every form-state change refetches on every
   * *selection* — the rows come back identical, the restore is guarded on the row
   * set having changed and does not run, and the selection is lost, so every
   * row-gated button stays dead. This screen has five such buttons on one table
   * and would have met it immediately.
   *
   * `stateVersion` is still read, because the values below are pulled out of
   * module stores that only notify through it; what is keyed is the serialised
   * result.
   */
  const client = selectedClient();
  const homeFilters = currentHomeFilters();
  const dataRegistryFilters = currentDataRegistryFilters();
  void stateVersion;
  const contextKey = JSON.stringify([routeParams, client, homeFilters, dataRegistryFilters]);
  const tableContext = useMemo(
    () => ({
      routeParams: routeParams as Record<string, string | undefined>,
      selectedClient: client,
      homeFilters,
      dataRegistryFilters,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [contextKey],
  );

  const currentForm: Form | null =
    dialog.request === null ? null : (forms[dialog.request.form] ?? null);

  const host: ActionHost = useMemo(
    () => ({
      // No step of this screen's one action queries, and `documentFindings`
      // refuses the screen if one appears without a registered statement — so
      // this throws rather than reporting an empty result, as `FlowRunner` does.
      query: async (name: string) => {
        throw new Error(`named query "${name}" is not registered in this build`);
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateAllGroups(currentForm, formState, GROUP);
        setErrors(found);
        return found.length === 0;
      },
      confirm: async (message: string) => window.confirm(message),
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
      // A screen has no states and nothing to close but its dialog — C.2b's two
      // inert seams, for the same reason.
      goToState: () => {},
      close: () => dialog.close("ok"),
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [api, currentForm, formState, queryPost, setError, setStatus, dialog.close],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
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
        flowKey: SCREEN_KEY,
      })).message;
    },
    [actions, host, formState],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      void (async () => {
        setError(null);
        try {
          switch (request.kind) {
            case "runAction": {
              setBusy(true);
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) setError(outcome);
              formState.requestRefresh();
              return;
            }
            case "navigate": {
              /**
               * **In-app when this app serves the screen, and into Flutter when
               * it does not.** `configScreenPath` is a Flutter route template;
               * five of them are React routes now, and `screens/routes.ts` is
               * the one place that says which. Sending them all to Flutter is
               * what every screen did until this one, and it is what would have
               * left C.7's and C.8's screens unreachable — they are entered from
               * this table and from nothing else.
               */
              const internal = inAppPath(action.configScreenPath, request.params);
              if (internal !== null) void navigate(internal);
              // The `else` was a hand-off to Flutter until X.1 retired it. This
              // screen's `startPipeline` and `setHomeFilters` buttons name flow
              // routes, which is what the second lookup above is for.
              else setError(unservedScreenMessage(action.label, request.path));
              return;
            }
            case "openDialog": {
              // The resolved `navigationParams` are seeded into the shared group
              // before the form opens, because the dialog's fields read them by
              // key — that is what makes a column index mean anything.
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              setErrors([]);
              setBusy(false);
              await dialog.open({ form: request.form, params: request.params });
              // **No refresh on close.** `showFailureDetailsDialog` is a viewer
              // with one Close button and no action document entry, so nothing
              // it does can have changed a row. C.2b refreshes on `ok` because
              // its seven dialogs write.
              return;
            }
            case "runActionThenDialog":
              // No configuration on this screen is of this kind; reaching it
              // would be a document this screen does not have.
              setError(`"${action.label}" runs an action and opens a dialog, which this screen does not configure`);
              return;
            case "escape": {
              const escape = productionRegistry.actions[request.name];
              if (escape === undefined) {
                setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
                return;
              }
              setBusy(true);
              await escape({ formState, group: GROUP, flowKey: SCREEN_KEY }, host);
              formState.requestRefresh();
              return;
            }
            case "promptFilter": {
              // **`window.prompt`, which is what the Dart's `showGetInputDialog`
              // is** — one `TextField` with CANCEL and OK
              // (`jetsclient/lib/components/dialogs.dart`, `showGetInputDialog`).
              // I-102 decision 4: these are a request kind rather than a dialog,
              // and they were never blocked on the dialog host.
              const answer = window.prompt(request.prompt);
              if (answer === null) return;
              setIdFilter(request.column, answer);
              formState.notifyListeners();
              formState.requestRefresh();
              return;
            }
          }
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
        } finally {
          setBusy(false);
        }
      })();
    },
    [dialog, formState, host, navigate, runNamedAction, setError],
  );

  const onFormAction = useCallback(
    (action: FormAction) => {
      // Every button of this screen's one dialog is the standard Close, whose
      // key is `dialogCancel` — the dialog's, not the action document's.
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
        <h1>JetStore Workspace</h1>
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

  const active = TABS[tab]!;

  return (
    <main className="screen">
      {/*
        `ScreenConfig(key: ScreenKeys.home)` sets `appBarLabel: 'JetStore
        Workspace'` and `title: ''`, so the Flutter screen shows no title row and
        the tab strip is the top of the page. The heading here is the app bar's
        label, which this shell's brand does not repeat per screen.
      */}
      <h1>JetStore Workspace</h1>

      <div className="tabstrip" role="tablist" aria-label="Home">
        {TABS.map((entry, index) => (
          <button
            key={entry.key}
            type="button"
            role="tab"
            id={`tab-${entry.key}`}
            aria-selected={index === tab}
            aria-controls={`panel-${entry.key}`}
            className={`tab${index === tab ? " is-active" : ""}`}
            onClick={() => setTab(index)}
          >
            {entry.label}
          </button>
        ))}
      </div>

      {/*
        **Only the selected tab is mounted**, which is what keeps the front door
        at one request. Keyed by the table so a tab change remounts rather than
        reconciling — two tabs' subtrees are structurally identical, which is the
        reconciliation trap `FlowRunner` documents at its `FormRenderer`.
      */}
      <div
        className="tabpanel"
        role="tabpanel"
        id={`panel-${active.key}`}
        aria-labelledby={`tab-${active.key}`}
      >
        {configs[active.key] === undefined ? (
          /*
            **Only reachable for the workspace-loaded tab, and only before it
            arrives.** The two bundled tabs have a config at module load; this one
            has one when the read returns. A failure is not here — it is a finding,
            and the banner above returns early.
          */
          <p className="muted" role="status">
            Loading {active.label}…
          </p>
        ) : (
          <TableView
            key={active.key}
            config={configs[active.key]!}
            field={{ group: GROUP, key: active.key }}
            formState={formState}
            fetcher={fetcher}
            context={tableContext}
            predicates={productionRegistry.predicates}
            cellFilters={cellFiltersFor(
              tablesWith(workspaceTable).find((t) => t.key === active.key)!.result.data!,
            )}
            onAction={onTableAction}
          />
        )}
      </div>

      {currentForm !== null && (
        <FormDialog
          form={currentForm}
          errors={errors}
          onDismiss={() => dialog.close("cancel")}
          host={{
            formState,
            group: GROUP,
            queryRows: () => undefined,
            queriesLoading: false,
            tableConfig: (key: string) => configs[key]!,
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

/**
 * The per-column display filters a table document names, resolved.
 *
 * `FlowRunner` has the same function against a loaded flow; this one reads a
 * bundled document. **Both matter more than they look on this screen**, because
 * two of its columns name filters and they are not the same one:
 * `inputLoaderStatusTable`'s `file_key` names `fileKeyLabel` and its
 * `error_message` names `errorMessageLabel`, which the translation used to
 * conflate.
 */
function cellFiltersFor(
  document: TableConfigDocument,
): Record<string, (value: string | null) => string | null> {
  const filters: Record<string, (value: string | null) => string | null> = {};
  for (const column of document.columns) {
    const filter = column.cellFilter ? productionRegistry.cellFilters[column.cellFilter] : undefined;
    if (filter !== undefined) filters[column.name] = filter;
  }
  return filters;
}

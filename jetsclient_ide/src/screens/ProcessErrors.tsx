/**
 * `/processErrors/:session_id` — the pipeline's execution errors. Task C.9.
 *
 * ## The largest screen in track C has the simplest layout, and that is a
 * correction rather than a boast
 *
 * `sizing_screen_migration.md` §7 ranks this row at **4 tables and 2 dialogs**,
 * the largest in the track. Both halves of the count are worth restating before
 * reading the file:
 *
 *  - **There are five configurations, not four.** `inputRecordsFromProcessErrorTable`
 *    is in no route inventory, because the reachability closure deliberately does
 *    not walk an `inputFieldRowBuilder` — the fields inside one do not exist until
 *    a form has been driven. That is the closure's stated limit meeting its
 *    consequence, not a defect in it.
 *  - **The screen renders one of them.** `viewProcessErrorsForm` is a single
 *    `FormDataTableFieldConfig` with `actions: []`
 *    (`jetsclient/lib/modules/form_config_impl.dart`, `FormKeys.viewProcessErrors`);
 *    the other four live inside the two dialogs.
 *
 * **So I-177's rule needs one more clause.** It says *a screen with one table
 * renders it; a screen with several declares them*, and it did not contemplate
 * that "several" might all be behind dialogs. The number that decides the screen's
 * layout is how many tables the *screen* has, which here is one — so this file
 * renders a `TableView` directly and declares no form document of its own, exactly
 * as C.2b's registry screen does.
 *
 * ## No `viewProcessErrorsForm` document, and the reason is not size — I-178
 *
 * The Dart form exists only to hold the table, and it is *action-less*:
 * `FormSchema.actions` is `min(1)` because every form in either corpus that a user
 * can act on has a button, and this one has none. A document for it would be a
 * form with no actions wrapping a field with no siblings — which the schema would
 * have to be weakened to admit, to describe a thing that carries no information.
 * This is the third reason a screen has declined a document, after C.4's (the
 * behaviour lives *between* two forms) and C.5's (the endpoint is outside the
 * allowlist).
 *
 * ## The route parameter is the whole filter
 *
 * `processErrorsTable`'s first where clause names `session_id` as a *form-state*
 * key and there is no form state to find it in, so `_makeWhereClause` falls
 * through to the route's parameters (F67). The React port is `routeParams` on the
 * `QueryContext`, which is the one thing this screen must not get wrong: without
 * it the screen renders perfectly and shows **every session's** errors.
 * `ProcessErrors.test.tsx` asserts the request rather than the rows for that
 * reason, and mutation-tests it.
 *
 * ## The two dialogs differ in kind, and both go through C.2b's host
 *
 * *View Input Records* is a `showDialog`: seed the parameters, open the form,
 * whose `repeat` draws one table per input source of the pipeline. *View Rule
 * Session* is a `doActionShowDialog`: run `reteSession.setupModelV2` first — it
 * loads a saved rete session into form state — and open a dialog whose three
 * tables are fed from that model and never touch the server.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry } from "../actions/registry";
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
import { inAppPath, unservedScreenMessage } from "./routes";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { resolveQuery } from "../userflow/formQueries";
import { useFormQueries } from "../userflow/useFormQueries";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";

import actionsJson from "./documents/processErrors.ua.json";
import formsJson from "./documents/processErrors.form.json";
import processErrorsTable from "../datatable/tables/processErrorsTable.tc.json";
import inputRecordsTable from "../datatable/tables/inputRecordsFromProcessErrorTable.tc.json";
import reteRdfTypeTable from "../datatable/tables/reteSessionRdfTypeTable.tc.json";
import reteEntityKeyTable from "../datatable/tables/reteSessionEntityKeyTable.tc.json";
import reteEntityDetailsTable from "../datatable/tables/reteSessionEntityDetailsTable.tc.json";

/** The screen's own table, and the form-state key its selection publishes under. */
const TABLE_KEY = "processErrorsTable";

/**
 * The base validation group. A dialog's repeat adds more; the screen has one.
 *
 * **A dialog gets its own `FormState` here, and C.2b's screen shares the
 * screen's.** That decision's stated ground is that *"the Dart shares one
 * `JetsFormState` between the screen and its dialogs"* — which is not what the
 * Dart does. `actionDispatcher` builds a **new** state,
 * `formConfig.makeFormState(parentFormState: formState)`
 * (`jetsclient/lib/components/data_table.dart`, the `showDialog` and
 * `doActionShowDialog` cases), and `parentFormState` is a back-pointer used only
 * to mark the parent's keys dirty (`components/jets_form_state.dart`,
 * `parentFormState`). What crosses the boundary is the `navigationParams` and
 * `stateFormNavigationParams` that are explicitly copied in, and nothing else.
 *
 * **On this screen the difference is not academic.** `viewInputRecordsDialog`
 * repeats, and its seed writes `session_id`, `table_name` and six more keys into
 * group 0 — which is the group the *screen's* table reads `session_id` from. With
 * one shared state the errors table silently re-queried for the dialog's first
 * input source: right shape, wrong session, no error anywhere. Measured, not
 * reasoned: the read went out with `session_id: "sess-main"`.
 *
 * `/workspaces` does not have the problem because none of its seven dialogs
 * writes a key its table filters on — so the sharing is latent there rather than
 * safe, which is worth saying to whoever adds the eighth.
 */
const GROUP = 0;

/**
 * The five table documents, by key.
 *
 * **A screen imports its committed documents rather than reading a workspace**
 * (I-170), and a dialog's tables are the screen's for the same reason its own is:
 * there is one `/processErrors` per deployment, not one per workspace.
 */
const TABLE_JSON: Record<string, unknown> = {
  processErrorsTable,
  inputRecordsFromProcessErrorTable: inputRecordsTable,
  reteSessionRdfTypeTable: reteRdfTypeTable,
  reteSessionEntityKeyTable: reteEntityKeyTable,
  reteSessionEntityDetailsTable: reteEntityDetailsTable,
};

const parsedForms = FormDocumentSchema.safeParse(formsJson);
const parsedActions = ActionDocumentSchema.safeParse(actionsJson);
const parsedTables = Object.fromEntries(
  Object.entries(TABLE_JSON).map(([key, json]) => [key, TableConfigDocumentSchema.safeParse(json)]),
);

/**
 * Findings from parsing and from resolving escape names, for the banner.
 *
 * The same shape C.2b's screen uses, and the same argument: a document that does
 * not satisfy its schema fails in every test that imports this module rather than
 * when a user presses a button.
 */
export function documentFindings(): string[] {
  const findings: string[] = [];
  const report = (name: string, result: { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } }): void => {
    if (result.success) return;
    for (const issue of result.error?.issues ?? []) {
      findings.push(`${name}: ${issue.path.join("/")} — ${issue.message}`);
    }
  };
  report("forms", parsedForms);
  report("actions", parsedActions);
  for (const [key, result] of Object.entries(parsedTables)) report(key, result);
  if (findings.length > 0) return findings;

  const references = [
    ...formEscapeReferences(parsedForms.data as FormDocument, "processErrors"),
    ...Object.entries(parsedTables).flatMap(([key, result]) =>
      tableEscapeReferences(result.data!).map((r) => ({ ...r, at: `${key}.tc.json${r.at}` })),
    ),
  ];
  for (const [name, action] of Object.entries((parsedActions.data as ActionDocument).actions)) {
    action.steps.forEach((step, index) => {
      if (step.do === "escape") {
        references.push({ kind: "actions", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
      if (step.do === "query") {
        references.push({ kind: "queries", name: step.name, at: `/actions/${name}/steps/${index}` });
      }
    });
  }
  const message = describeUnresolved(resolveEscapes(references, productionRegistry));
  return message === null ? [] : [message];
}

export function ProcessErrors({ api }: { api: ApiClient }) {
  // X.1: a `navigate` request resolves in this app now.
  const navigate = useNavigate();
  const routeParams = useParams();
  const { setError, setStatus } = useNotifications();

  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  const dialog = useFormDialog();
  /**
   * The open dialog's own state, or null.
   *
   * Created per open rather than cleared on close, which is the same thing with
   * one fewer way to be wrong: a state that is discarded cannot leak a key into
   * the next dialog, and `FormState` is cheap.
   */
  const [dialogState, setDialogState] = useState<FormState | null>(null);
  /** Whichever state the thing being acted on belongs to. */
  const activeState = dialogState ?? formState;

  const findings = useMemo(documentFindings, []);
  const forms = parsedForms.success ? parsedForms.data.forms : {};
  const actions = parsedActions.success ? parsedActions.data.actions : {};

  /** Every table this screen or one of its dialogs renders, translated once. */
  const configs = useMemo<Record<string, TableConfig>>(
    () =>
      Object.fromEntries(
        Object.entries(parsedTables)
          .filter(([, result]) => result.success)
          .map(([key, result]) => [key, fromDocument(key, result.data!)]),
      ),
    [],
  );

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);
  useEffect(
    () => (dialogState === null ? undefined : dialogState.subscribe(() => setStateVersion((n) => n + 1))),
    [dialogState],
  );

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);
  const queryPost = useCallback(
    (payload: Record<string, unknown>) =>
      api.dataTable<{ result_map?: Record<string, unknown> }>(payload),
    [api],
  );

  /**
   * The query context, keyed on its contents rather than on the form version.
   *
   * **I-184, inherited rather than rediscovered.** Rebuilding this on every
   * form-state change makes publishing a selection refetch the table, and the
   * refetch clears the selection that was just published — so every row-gated
   * button stays dead. Both of this screen's buttons are row-gated.
   *
   * `routeParams` is the only member with a value here: this screen has no client
   * selector and reads no filter state, and `makeQuery` splices `homeFilters` and
   * `dataRegistryFilters` only for a table with a form field naming them (F67).
   */
  const contextKey = JSON.stringify(routeParams);
  const tableContext = useMemo(
    () => ({ routeParams: routeParams as Record<string, string | undefined> }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [contextKey],
  );

  const currentForm: Form | null =
    dialog.request === null ? null : (forms[dialog.request.form] ?? null);
  const queries = useFormQueries(currentForm, activeState, queryPost);

  /**
   * Sizing and seeding the repeating dialog. Task F.1's mechanism, second consumer.
   *
   * `viewInputRecordsDialog` draws one group per row of `inputFields`, and
   * `seedInputRecordsRow` writes the eight values each group's table filters on.
   * `resizeFormState` before the seeds, because a seed writes into a group that
   * has to exist.
   */
  const repeatRows = currentForm?.repeat === undefined ? undefined : queries.rows(currentForm.repeat.from);
  /**
   * The rows this form has actually been seeded for.
   *
   * **Not a tidy-up: without it the dialog throws on its first render.** The seed
   * runs in an effect and the groups are drawn in the render before it, so a group
   * exists for one frame holding none of the eight values its table filters on —
   * and one of those eight is a *column name*, resolved through
   * `lookupColumnInFormState`, which `makeWhereClause` refuses rather than sending
   * (`datatable/query.ts`, `makeWhereClause`). Drawing nothing until the seeds have
   * run is the Dart's own ordering: `inputFieldRowBuilder` seeds and returns the
   * field in one call, so a group and its values arrive together.
   *
   * **`FlowRunner` has the same shape and does not have this bug**, which is worth
   * a line rather than a fix in someone else's file: `fmMappingFormUF`'s repeated
   * rows hold text and typeahead fields, and a field that renders empty for one
   * frame and then fills is invisible. The ordering was always wrong and only a
   * repeated *table* could show it.
   */
  const [seededFor, setSeededFor] = useState<JetsRow[] | null>(null);
  useEffect(() => {
    const repeat = currentForm?.repeat;
    if (repeat === undefined || repeatRows === undefined) return;
    const seed = productionRegistry.rowInitializers[repeat.seed];
    if (seed === undefined || dialogState === null) return;
    dialogState.resizeFormState(repeatRows.length);
    repeatRows.forEach((row, index) => {
      seed({ formState: dialogState, group: GROUP, flowKey: "processErrors" }, row, index);
    });
    setSeededFor(repeatRows);
    dialogState.notifyListeners();
  }, [currentForm, repeatRows, dialogState]);

  const host: ActionHost = useMemo(
    () => ({
      query: async (name: string) => {
        const registered = productionRegistry.queries[name];
        if (registered === undefined) throw new Error(`named query "${name}" is not registered`);
        const sql = resolveQuery(
          { sql: registered.sql, ...(registered.params ? { params: [...registered.params] } : {}) },
          activeState,
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
        const found = validateAllGroups(currentForm, activeState, GROUP);
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
      download: () => {},
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      goToState: () => {},
      close: () => dialog.close("ok"),
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [api, currentForm, activeState, queryPost, setError, setStatus, dialog.close],
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
        formState: activeState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: "processErrors",
      })).message;
    },
    [actions, host, activeState],
  );

  /** Opens a dialog and waits. **Not while `busy`** — C.2b's finding: a modal is not work. */
  const openDialog = useCallback(
    async (form: string, params: Record<string, string>) => {
      setBusy(false);
      const outcome = await dialog.open({ form, params });
      // The state is discarded here rather than in `close`, so an action that
      // reads it after the promise settles still can.
      setDialogState(null);
      setSeededFor(null);
      // `parentFormState`'s whole purpose in the Dart: a dialog that changed
      // something marks the screen's tables dirty. A viewer never reaches this.
      if (outcome === "ok") formState.requestRefresh();
    },
    [dialog, formState],
  );

  /**
   * Builds the dialog's state and copies the resolved parameters into it.
   *
   * This is `actionDispatcher`'s three lines — make the state, apply
   * `stateFormNavigationParams`, apply `navigationParams` — with the two maps
   * already merged by `resolveParams` (`datatable/actionDispatch.ts`), which is
   * where the precedence between them is decided and recorded.
   */
  const seed = useCallback((params: Record<string, string>): FormState => {
    const next = new FormState();
    for (const [key, value] of Object.entries(params)) next.setValue(GROUP, key, value);
    setDialogState(next);
    setErrors([]);
    return next;
  }, []);

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      void (async () => {
        setError(null);
        setBusy(true);
        try {
          switch (request.kind) {
            case "openDialog":
              seed(request.params);
              await openDialog(request.form, request.params);
              return;
            case "runActionThenDialog": {
              // *View Rule Session*. **Seed first, then run**, which is the order
              // the Dart uses and not the order it reads in: `actionDispatcher`
              // copies `stateFormNavigationParams` into the dialog's form state
              // and *then* calls the delegate with it
              // (`jetsclient/lib/components/data_table.dart`, the
              // `doActionShowDialog` case, `:777`–`:796`). It has to — this
              // action's only parameter is `key`, taken from the table's published
              // selection, and the escape reads exactly that key to know which
              // row's session to load. Running first hands the escape an empty
              // form state and it reports *no row is selected* over a row that is.
              //
              // **`WorkspaceRegistry.tsx` has the other order**, and it is latent
              // rather than wrong there: none of its `doActionShowDialog` buttons
              // has an action that reads a parameter. Reported rather than edited.
              const opened = seed(request.params);
              const { message: outcome } = await runAction({
                action: actions[request.name]!,
                host,
                // The state `seed` just built, not `activeState` — this render has
                // not seen `setDialogState` yet, and the escape reads `key` from
                // it. Passing the value rather than waiting a tick is what makes
                // the ordering the Dart's rather than React's.
                formState: opened,
                field: { group: GROUP, key: request.name },
                registry: productionRegistry,
                flowKey: "processErrors",
              });
              if (outcome !== null) {
                // The Dart shows the error *and opens the dialog anyway*. This
                // does not: three tables with nothing behind them is a dialog that
                // says the load worked. The banner names what went wrong and the
                // selection is still on the row to try again.
                setError(outcome);
                return;
              }
              await openDialog(request.form, request.params);
              return;
            }
            case "runAction": {
              // *Visit Object Entity*, inside the explorer dialog. It writes form
              // state and the two dependent tables recompute from it — there is
              // nothing to refresh, and asking for one would put a request behind
              // a button whose whole point is that it makes none.
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) setError(outcome);
              return;
            }
            case "navigate": {
              // **This arm used to hand the browser to Flutter unconditionally**,
              // which was right when the other app served every screen this one
              // does not. X.1 removed the destination, so it resolves in-app and
              // reports when it cannot — the convention `escapes.ts` set for a name
              // with no body.
              const to = inAppPath(action.configScreenPath, request.params);
              if (to !== null) void navigate(to);
              else setError(unservedScreenMessage(action.label, request.path));
              return;
            }
            case "escape": {
              const escape = productionRegistry.actions[request.name];
              if (escape === undefined) {
                setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
                return;
              }
              await escape({ formState: activeState, group: GROUP, flowKey: "processErrors" }, host);
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
    [actions, activeState, host, openDialog, runNamedAction, seed, setError],
  );

  /**
   * A dialog's button.
   *
   * **Both dialogs declare exactly one and it is Cancel**, so nothing here reaches
   * `runNamedAction` today. The branch is kept rather than replaced by
   * `dialog.close("cancel")`, because a viewer that grew a second button would
   * otherwise close the dialog instead of running it — which is the failure mode
   * that reads as working.
   */
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
        <h1>Pipeline Execution Errors</h1>
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
      <h1>Pipeline Execution Errors</h1>
      <TableView
        config={configs[TABLE_KEY]!}
        field={{ group: GROUP, key: TABLE_KEY }}
        formState={formState}
        fetcher={fetcher}
        context={tableContext}
        predicates={productionRegistry.predicates}
        cellFilters={{}}
        onAction={onTableAction}
      />
      {currentForm !== null && (
        <FormDialog
          form={currentForm}
          errors={errors}
          onDismiss={() => dialog.close("cancel")}
          host={{
            formState: dialogState ?? formState,
            group: GROUP,
            queryRows: queries.rows,
            queriesLoading: queries.loading,
            tableConfig: (key: string) => configs[key]!,
            fetcher,
            tableContext,
            predicates: productionRegistry.predicates,
            cellFilters: () => ({}),
            onTableAction,
            onFormAction,
            formValid: true,
            groupCount:
              currentForm.repeat === undefined
                ? 1
                : seededFor === repeatRows
                  ? Math.min(repeatRows?.length ?? 0, (dialogState ?? formState).groupCount)
                  : 0,
            busy,
          }}
        />
      )}
    </main>
  );
}

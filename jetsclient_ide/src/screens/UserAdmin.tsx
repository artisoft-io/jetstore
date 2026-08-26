/**
 * `/userAdmin` — the user administration screen. Task C.13.
 *
 * ## The only route in the corpus gated on `admin` alone
 *
 * Six of the 27 routes name `admin` in their access set and five of those also
 * name a capability; this one names `admin` and nothing else
 * (`screens/fixtures/screen_reachability.json`, `accessSummary` and the
 * `/userAdmin` entry). The Flutter reason is that its menu entry lives only in
 * `adminMenuEntries` (`jetsclient/lib/modules/screen_config_impl.dart`,
 * `ScreenKeys.userAdmin`).
 *
 * ## What gates the two writes this screen performs, server-side
 *
 * **`IsAdmin()` is `u.Email == AdminEmail`** (`jets/user/user.go`, `IsAdmin`) — a
 * single account, not a role — and everything else follows from that sentence.
 *
 * `update/users` and `delete/users` are the **only** two entries in
 * `jets/datatable/sql_stmts.go` carrying `AdminOnly: true`, and both pair it with
 * `Capability: "none"`. `VerifyUserPermission` requires *both* arms
 * (`jets/datatable/data_table_action.go`, `VerifyUserPermission`), and
 * `HasCapability` returns true for the admin account whatever it is asked
 * (`jets/user/user.go`, `HasCapability`) — so `"none"` is a sentinel whose only
 * job is to clear the *empty capability is a configuration error* check above it.
 * **The gate holds**: a caller who is not that account is refused on the first
 * arm.
 *
 * The Flutter buttons declare `capability: "user_profile"` on *Delete User* and on
 * this dialog's *Submit*. That claim is **reproduced unchanged and is inert on
 * this route**: only the admin account can reach the screen, and `ApiClient.can`
 * short-circuits on `isAdmin` exactly as `HasCapability` does
 * (`jetsclient_ide/src/api/client.ts`, `can`), so it can never withhold a control
 * from anyone who is here. Nothing is weakened and nothing is added.
 *
 * **A single-account administrator is an operational property of JetStore, not
 * something this port introduces** — no second administrator, and no revocation
 * short of changing the address. Recorded, not changed.
 *
 * ## The screen renders one table and declares no form document of its own
 *
 * `userAdmin` is action-less and holds a single data-table field
 * (`jetsclient/lib/modules/form_config_impl.dart`, `FormKeys.userAdmin`), exactly
 * as C.9's screen form is — so this is I-177's clause again, and the second
 * screen to decline a document for that reason rather than for size. The
 * *dialog's* form is a document; the screen's is markup.
 *
 * ## This is C.9's file with different documents, and that is worth saying rather
 * than hiding
 *
 * Everything from `documentFindings` to `onFormAction` is the same shape as
 * `ProcessErrors.tsx`, which is in turn `WorkspaceRegistry.tsx`'s. **That is three
 * copies of a host and C.2b's own argument against copying `FormDataTable`
 * applies one level up**: what each copy has to get right independently is not the
 * markup but that a dialog gets its own form state, that the parameters are seeded
 * before a `doActionShowDialog` action runs, and that a modal is not `busy`. Every
 * one of those was a defect in one copy before it was a comment in another.
 * Filed as debt with the extraction named rather than done here, because
 * `WorkspaceRegistry.tsx` belongs to an open pull request and a fourth screen is
 * the point at which the shape stops being a coincidence.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";

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
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { resolveQuery } from "../userflow/formQueries";
import { useFormQueries } from "../userflow/useFormQueries";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";

import actionsJson from "./documents/userAdmin.ua.json";
import formsJson from "./documents/userAdmin.form.json";
import userTableJson from "../datatable/tables/userTable.tc.json";
import userRolesTableJson from "../datatable/tables/userRolesTable.tc.json";

/** The screen's own table, and the form-state key its selection publishes under. */
const TABLE_KEY = "userTable";

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
 * The two table documents, by key. Bundled rather than read from a workspace
 * (I-170): there is one `/userAdmin` per deployment.
 */
const TABLE_JSON: Record<string, unknown> = {
  userTable: userTableJson,
  userRolesTable: userRolesTableJson,
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
    ...formEscapeReferences(parsedForms.data as FormDocument, "userAdmin"),
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

export function UserAdmin({ api }: { api: ApiClient }) {
  const routeParams = useParams();
  const { setError, setStatus } = useNotifications();

  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  const dialog = useFormDialog();
  /**
   * Whether the last action stopped because the *user* stopped it. **I-186**, and
   * this is the first screen of mine that needs it: C.9's two dialogs are viewers
   * with nothing to validate, and this one submits.
   *
   * `runAction` returns null both when it ran to the end and when a `validate`
   * step failed or a `confirm` was refused. Treating the second as success closes
   * the dialog over the messages it just produced.
   */
  const haltedByUser = useRef(false);
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
   * **Empty in practice on this screen**, and kept rather than dropped: the route
   * carries no parameters, neither table has a `client` column and neither reads a
   * filter list, so every member `makeQuery` would splice is absent. What it buys
   * is that a table added here later gets the same context every other screen's
   * does, which is decision 2's whole point.
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
   * Sizing and seeding a repeating dialog. **Unused here**, because neither of
   * this screen's forms repeats — kept because it is part of the host shape the
   * three screens share and removing it per screen is how the copies drift.
   *
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
      seed({ formState: dialogState, group: GROUP, flowKey: "userAdmin" }, row, index);
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
      haltedByUser.current = false;
      const action = actions[name];
      if (action === undefined) {
        throw new Error(`action "${name}" is not in this screen's action document`);
      }
      return runAction({
        action,
        host,
        formState: activeState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: "userAdmin",
      });
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
              // **Seed first, then run** — no button on this screen takes this
              // path today, and the order is the Dart's rather than the one it
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
              const outcome = await runAction({
                action: actions[request.name]!,
                host,
                // The state `seed` just built, not `activeState` — this render has
                // not seen `setDialogState` yet, and the escape reads `key` from
                // it. Passing the value rather than waiting a tick is what makes
                // the ordering the Dart's rather than React's.
                formState: opened,
                field: { group: GROUP, key: request.name },
                registry: productionRegistry,
                flowKey: "userAdmin",
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
              // *Delete User*. It confirms, posts one row per selected account,
              // and the table must re-read — a deletion that leaves the rows on
              // screen is the failure that reads as working. **A refused
              // confirmation is not a deletion**, so it does not refresh either:
              // re-reading after a cancel is harmless and says the opposite of
              // what happened.
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) {
                setError(outcome);
                return;
              }
              if (haltedByUser.current) return;
              setStatus("Delete User(s) Successful");
              formState.requestRefresh();
              return;
            }
            case "navigate":
              window.location.href = `/#${request.path}`;
              return;
            case "escape": {
              const escape = productionRegistry.actions[request.name];
              if (escape === undefined) {
                setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
                return;
              }
              await escape({ formState: activeState, group: GROUP, flowKey: "userAdmin" }, host);
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
    [actions, activeState, formState, host, openDialog, runNamedAction, seed, setError, setStatus],
  );

  /**
   * A dialog's button. **The first on any track C screen of mine that writes** —
   * `editUserProfile.ok` validates and posts, and Cancel is the dialog's.
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
          // The dialog stays open when the user stopped it — the field errors
          // `validate` produced are on screen and the form is still there to
          // correct.
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
        <h1>User Administration</h1>
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
      <h1>User Administration</h1>
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

/**
 * The flow runner. Task F.0a, and the thing I-50 found nobody had built.
 *
 * Phase 2 built a schema, a validator, an interpreter, an engine, a store and
 * two proof flows' documents, and `proofFlows.test.ts` drove them end to end in
 * memory. **No browser ran a flow**, because there was no `/flow/:key` route, no
 * screen that read a `.form.json`, and no `EscapeRegistry` value outside tests.
 * The Flutter app was meanwhile handing users to `/ide/flow/<key>` for two flows,
 * which landed on `App.tsx`'s `path="*"` and redirected to the Workspace IDE.
 *
 * ## This screen is a host, and that is the whole design
 *
 * Everything with a decision in it was already written and tested without a DOM:
 * `engine.step` decides where the flow goes, `runAction` decides what a button
 * does, `validateForm` decides whether the form passes, `FlowStore.load` decides
 * whether the documents are runnable at all. What was missing is the
 * *implementation of the seams* — `ActionHost`'s ten methods and `StepRequest`'s
 * four — against a real API, a real router and a real user.
 *
 * So this file is deliberately close to `proofFlows.test.ts`'s `harness`. Where
 * the test supplies `post: async (r) => { posts.push(r); return {statusCode: 200} }`,
 * this supplies the api client. If the two ever disagree about *ordering* the
 * test is right, because the ordering is `engine.ts`'s and is asserted there.
 *
 * ## Which workspace
 *
 * The deployment's active workspace, from `get_workspace_uri` — not the IDE's
 * picker. A user editing four workspaces still runs the flows of the one this
 * apiserver is configured for; `WorkspaceApi.activeWorkspace` explains why the
 * registry is the wrong question here.
 *
 * ## What this does not do yet, stated rather than hidden
 *
 * A table action of kind `showDialog` or `doActionShowDialog` opens a *dialog*,
 * and this app has no dialog host — five of the 25 configured table actions are
 * one of those two kinds and none is on a proof flow's tables. They report
 * plainly rather than doing nothing, which is the same choice `escapes.ts` made
 * about an unresolved name (I-68).
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry, setFileKeyLabelPattern } from "../actions/registry";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { FormRenderer } from "../userflow/FormRenderer";
import type { FormAction } from "../userflow/form";
import {
  advance,
  isStandardAction,
  startAt,
  step,
  type FlowPosition,
  type StandardAction,
} from "../userflow/engine";
import { FlowLoadError, FlowStore, tableConfigOf, type LoadedFlow } from "../userflow/store";
import { isFormValid, validateForm, type FieldError } from "../userflow/validateForm";

/** The capability the server requires for every workspace read. */
export const FLOW_RUNNER = "workspace_ide";

/**
 * The form-state group a flow runs in.
 *
 * Zero, and every flow in the corpus is zero: the group dimension is for a form
 * with repeating rows, which is `file_mapping`'s mechanism (I-31, I.1) and not a
 * flow-level one. A flow that grows one will size it from a query at load, which
 * is what `resizeFormState` is for.
 */
const GROUP = 0;

interface Loaded {
  workspaceName: string;
  flow: LoadedFlow;
}

export function FlowRunner({ api }: { api: ApiClient }) {
  const { key } = useParams<{ key: string }>();
  const navigate = useNavigate();
  const { setError, setStatus } = useNotifications();

  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [loadFindings, setLoadFindings] = useState<string[] | null>(null);
  const [position, setPosition] = useState<FlowPosition | null>(null);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [busy, setBusy] = useState(false);

  // One form state for the whole flow, deliberately: a flow's states share their
  // values — `loadFilesUF`'s second table filters on the first's selection — and
  // a per-state store would lose them at every transition.
  const formState = useMemo(() => new FormState(), [key]); // eslint-disable-line react-hooks/exhaustive-deps
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);

  useEffect(() => {
    if (key === undefined) return;
    let cancelled = false;
    setLoaded(null);
    setLoadFindings(null);
    setBusy(true);
    void (async () => {
      try {
        const active = await workspaceApi.activeWorkspace();
        setFileKeyLabelPattern(active.fileKeyLabelRe === "" ? null : active.fileKeyLabelRe);
        const store = new FlowStore(workspaceApi, {
          workspaceName: active.name,
          registry: productionRegistry,
        });
        const flow = await store.load(key);
        if (cancelled) return;
        setLoaded({ workspaceName: active.name, flow });
        setPosition(startAt(flow.flow));
      } catch (error) {
        if (cancelled) return;
        // **A load failure renders here rather than in the banner.** The findings
        // are a list with a JSON Pointer each, and they are the answer to "why
        // will this flow not run" — a one-line banner would show the first and
        // lose the rest, which is what `resolveEscapes` returning a list exists
        // to prevent.
        setLoadFindings(
          error instanceof FlowLoadError
            ? error.findings.map((f) => (f.path === "" ? f.message : `${f.path}: ${f.message}`))
            : [error instanceof Error ? error.message : String(error)],
        );
      } finally {
        if (!cancelled) setBusy(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [key, workspaceApi]);

  const fetcher: DataTableFetcher = useCallback(
    (payload) => api.dataTable(payload),
    [api],
  );

  /** Leaves the flow: the document's `exitScreenPath`, or the IDE. */
  const exit = useCallback(() => {
    const path = loaded?.flow.flow.exitScreenPath;
    // A Flutter route rather than one of ours — two flows set it, both to
    // `/workspaces`, and that screen is track C's. Sending the browser there is
    // correct while both apps exist and is the same full page load the handoff
    // in the other direction is (`userflow/routing.ts`).
    if (path !== undefined && path !== "") window.location.href = `/#${path}`;
    else void navigate("/workspace");
  }, [loaded, navigate]);

  const currentForm = useMemo(() => {
    if (loaded === null || position === null) return null;
    const state = loaded.flow.flow.states[position.stateKey];
    if (state === undefined) return null;
    return loaded.flow.forms.forms[state.formConfig] ?? null;
  }, [loaded, position]);

  // `goToState` writes through a ref because the interpreter may call it in the
  // middle of an action whose result also moves the flow; the last write wins,
  // and `step` applies it after the action returns.
  const jumpTo = useRef<string | null>(null);

  const host: ActionHost = useMemo(
    () => ({
      query: async (name: string) => {
        // The one Dart site is `getProcessInputRdfTypes`, in a flow track F has
        // not reached. Refusing loudly beats returning null, which the
        // interpreter reads as "no rows" and reports as data rather than as a
        // build gap.
        throw new Error(`named query "${name}" is not registered in this build`);
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateForm(currentForm, formState, GROUP);
        setErrors(found);
        return found.length === 0;
      },
      confirm: async (message: string) => window.confirm(message),
      post: async (request): Promise<PostResult> => {
        try {
          // The endpoint is the document's, from the two `EndpointSchema` allows;
          // the body is the interpreter's, already built from the step.
          await api.endpoint(request.endpoint, request.body);
          return { statusCode: 200 };
        } catch (error) {
          if (error instanceof ApiError) {
            return { statusCode: error.status, error: error.message };
          }
          return { statusCode: 500, error: error instanceof Error ? error.message : String(error) };
        }
      },
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      goToState: (state: string) => {
        jumpTo.current = state;
      },
      close: exit,
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    [api, currentForm, exit, formState, setError, setStatus],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      const action = loaded?.flow.actions.actions[name];
      if (action === undefined) {
        // `validateDocumentSet` refuses this at load, so reaching it means a
        // document was run without being checked — say that rather than
        // reporting a missing button to the user.
        throw new Error(`action "${name}" is not in this flow's action document`);
      }
      return runAction({
        action,
        host,
        formState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: loaded!.flow.key,
      });
    },
    [loaded, host, formState],
  );

  const press = useCallback(
    async (name: string) => {
      if (loaded === null || position === null) return;
      setBusy(true);
      setError(null);
      try {
        if (!isStandardAction(name)) {
          // A form button naming an entry in the action document. It does not go
          // through `step`, which is the Dart's shape too: the six standard keys
          // are the flow's and everything else is the action grammar's.
          const outcome = await runNamedAction(name);
          if (outcome !== null) setError(outcome);
        } else {
          const result = await step(name as StandardAction, {
            flow: loaded.flow.flow,
            position,
            formState,
            group: GROUP,
            runStateAction: runNamedAction,
            validate: () => host.validate(),
            exit,
          });
          if (result.outcome !== null) setError(result.outcome);
          setPosition(result.position);
        }
        // An action's `goToState` is applied last, after the engine has had its
        // say — the Dart's `setCurrentUserFlowState` does the same `visitedPages`
        // bookkeeping `ufNext` would have (`schema.ts`, `goToStates`).
        const jump = jumpTo.current;
        jumpTo.current = null;
        if (jump !== null) setPosition((current) => (current === null ? current : advance(current, jump)));
      } catch (error) {
        setError(error instanceof Error ? error.message : String(error));
      } finally {
        setBusy(false);
      }
    },
    [loaded, position, formState, host, exit, runNamedAction, setError],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      switch (request.kind) {
        case "runAction":
          void press(request.name);
          return;
        case "navigate":
          // A `configScreenPath` is a Flutter route; see `exit`.
          window.location.href = `/#${request.path}`;
          return;
        case "escape": {
          const escape = productionRegistry.actions[request.name];
          if (escape === undefined) {
            setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
            return;
          }
          void escape({ formState, group: GROUP, flowKey: loaded?.flow.key ?? "" }).then(() =>
            formState.requestRefresh(),
          );
          return;
        }
        case "openDialog":
        case "runActionThenDialog":
          setError(
            `"${action.label}" opens a dialog, and this app has no dialog host yet — see I-68`,
          );
          return;
      }
    },
    [press, formState, loaded, setError],
  );

  const onFormAction = useCallback((action: FormAction) => void press(action.action), [press]);

  if (key === undefined) return null;

  if (loadFindings !== null) {
    return (
      <main className="screen uf-runner">
        <h1>{key}</h1>
        <div className="banner banner-error" role="alert">
          <div>
            <p>This user flow cannot be loaded.</p>
            <ul>
              {loadFindings.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        </div>
      </main>
    );
  }

  if (loaded === null || position === null || currentForm === null) {
    return (
      <main className="screen uf-runner">
        <p role="status">{busy ? `Loading ${key}…` : `${key} has no form to show.`}</p>
      </main>
    );
  }

  const state = loaded.flow.flow.states[position.stateKey]!;

  return (
    <main className="screen uf-runner">
      <header className="uf-runner__header">
        {/* A flow document carries no title — S.1 dropped it, because the key is
            the name and two names for one thing is one too many (I-14). The form
            may carry one, and `FormRenderer` shows it. */}
        <h1>{key}</h1>
        <p className="uf-runner__state">{state.description}</p>
      </header>

      <FormRenderer
        // **Remount on every transition, deliberately.** Two states' forms are
        // structurally alike — `loadFilesUF`'s are both one `dataTable` field in
        // one row — so React reconciles them as the same element and the widgets
        // keep their internal state: the table went on showing the *previous*
        // table's label and rows after Next. The values that must survive live in
        // `FormState`, outside React, and `useTableBinding` restores a selection
        // from it on mount — so a fresh subtree per state is both correct and
        // what the Flutter app does, which pushes a new page per state.
        key={position.stateKey}
        form={currentForm}
        errors={errors}
        host={{
          formState,
          group: GROUP,
          tableConfig: (tableKey) => tableConfigOf(loaded.flow, tableKey),
          fetcher,
          predicates: productionRegistry.predicates,
          cellFilters: (tableKey) => cellFiltersFor(loaded.flow, tableKey),
          onTableAction,
          onFormAction,
          formValid: isFormValid(currentForm, formState, GROUP),
          busy,
        }}
      />
    </main>
  );
}

/**
 * The per-column display filters a table document names, resolved.
 *
 * The document carries a *name* per column and `DataTable` takes a map from
 * column name to function — so this is where the registry is consulted. A name
 * that does not resolve cannot reach here: `FlowStore.load` refuses the whole set
 * first, which is the point of resolving at load rather than at render.
 */
function cellFiltersFor(
  loaded: LoadedFlow,
  tableKey: string,
): Record<string, (value: string | null) => string | null> {
  const document = loaded.tables[tableKey];
  if (document === undefined) return {};
  const filters: Record<string, (value: string | null) => string | null> = {};
  for (const column of document.columns) {
    const filter = column.cellFilter ? productionRegistry.cellFilters[column.cellFilter] : undefined;
    if (filter !== undefined) filters[column.name] = filter;
  }
  return filters;
}

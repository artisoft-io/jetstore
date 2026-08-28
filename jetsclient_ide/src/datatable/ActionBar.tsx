/**
 * The table's action bar. Task S.2b.
 *
 * A.4b left `DataTable`'s `actions` prop with the comment *"where S.2's action
 * buttons go"*. This is what goes there — the rendering is small, because the
 * two parts that can be wrong live in `actionBarModel.ts` and
 * `actionDispatch.ts` and are tested without a DOM.
 *
 * Buttons obey A.2's capability model through `ActionButton`, which disables
 * rather than hides and explains itself. Nine of the 25 actions name a
 * capability — `client_config` on six, `run_pipelines` on three.
 */

import type { ReactNode } from "react";

import { ActionButton } from "../shell/capabilities";
import { availability, type ActionContext } from "./actionBarModel";
import { requestFor, type ActionRequest } from "./actionDispatch";
import type { ActionConfig } from "./types";

/**
 * The two actions the widget owns, and what pressing one does. Task D.10.
 *
 * **A.5 established that `refreshTable` and `toggleCheckboxVisible` act on the
 * table rather than on the flow, and `useTableBinding`'s header says the bar
 * "only has to find it here". It never did.** `requestFor` throws
 * `UnsupportedActionType` for both — deliberately, and correctly, since a
 * *request* is the wrong shape for them — and nothing caught the throw, so a
 * press on one of the seven documents that configure one raised an uncaught
 * error out of an event handler and the button appeared inert. `refreshTable`
 * sits on `pipelineExecStatusTable`, which is this app's front door.
 *
 * Found at D.10 while moving `inputLoaderStatusTable` to a screen of its own,
 * which is a screen whose *Refresh* button would otherwise have shipped broken.
 * Fixed here rather than filed, on I-270's argument: a three-line handler beats
 * an entry saying somebody should look at it.
 */
export interface WidgetActions {
  /** `_refreshTable`, as a button rather than as a reaction to the form. */
  refresh(): void;
  toggleCheckboxVisible(): void;
}

export interface ActionBarProps {
  actions: ActionConfig[];
  context: ActionContext;
  /**
   * The widget's own two actions. Optional so that a bar can be rendered without
   * a binding — which is what the corpus tests do — and a document configuring
   * one without it is the same reported failure it was before.
   */
  widget?: WidgetActions;
  /**
   * The row a `navigationParams` column index reads from — and, since C.2, the
   * row `enableWhen` tests.
   */
  selectedRow?: (string | null)[];
  onAction(request: ActionRequest, action: ActionConfig): void;
}

export function ActionBar({
  actions,
  context,
  widget,
  selectedRow,
  onAction,
}: ActionBarProps): ReactNode {
  // **Folded in here rather than asked of every caller.** The bar already holds
  // the selected row for `requestFor`, and C.2 gave `availability` a second use
  // for the same value; making each caller put it in the context as well would
  // be two sources for one fact, and the one a screen forgot would be the one
  // that silently enabled a gated button. Every existing caller is correct
  // without changing.
  const decideWith: ActionContext =
    selectedRow === undefined ? context : { ...context, selectedRow };
  const shown = actions
    .map((action) => ({ action, state: availability(action, decideWith) }))
    .filter(({ state }) => state.visible);
  if (shown.length === 0) return null;

  return (
    <div className="jets-datatable__actions" role="group" aria-label="Table actions">
      {shown.map(({ action, state }) => (
        <ActionButton
          key={action.key}
          // `btn-` and not `btn--`: the stylesheet defines `.btn-primary` and
          // the rest of the app writes `btn btn-primary` (`Login.tsx`,
          // `GitProfile.tsx`, `WorkspaceIde.tsx`). This emitted `btn--primary`,
          // which matched no rule, so **every configured action rendered as a
          // plain `.btn` and each document's declared `style` was inert** — 49
          // buttons across 24 table documents, of which 4 are `danger` Deletes.
          // D.4, from **I-264**, which reported the visible half: *Start
          // Pipeline* is declared `primary` and did not look it.
          className={`btn btn-${action.style}`}
          disabled={!state.enabled}
          {...(action.capability !== undefined ? { capability: action.capability } : {})}
          {...(state.reason !== undefined ? { title: state.reason } : {})}
          onClick={() => {
            // The widget's two, before `requestFor` — which throws for both, and
            // says why. See `WidgetActions`.
            if (widget !== undefined) {
              if (action.actionType === "refreshTable") return widget.refresh();
              if (action.actionType === "toggleCheckboxVisible") {
                return widget.toggleCheckboxVisible();
              }
            }
            onAction(requestFor(action, context.formState, selectedRow), action);
          }}
        >
          {action.label}
        </ActionButton>
      ))}
    </div>
  );
}

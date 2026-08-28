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

export interface ActionBarProps {
  actions: ActionConfig[];
  context: ActionContext;
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
          onClick={() => onAction(requestFor(action, context.formState, selectedRow), action)}
        >
          {action.label}
        </ActionButton>
      ))}
    </div>
  );
}

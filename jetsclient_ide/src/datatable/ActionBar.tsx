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
  /** The row a `navigationParams` column index reads from. */
  selectedRow?: (string | null)[];
  onAction(request: ActionRequest, action: ActionConfig): void;
}

export function ActionBar({
  actions,
  context,
  selectedRow,
  onAction,
}: ActionBarProps): ReactNode {
  const shown = actions
    .map((action) => ({ action, state: availability(action, context) }))
    .filter(({ state }) => state.visible);
  if (shown.length === 0) return null;

  return (
    <div className="jets-datatable__actions" role="group" aria-label="Table actions">
      {shown.map(({ action, state }) => (
        <ActionButton
          key={action.key}
          className={`btn btn--${action.style}`}
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

/**
 * The deployment's custom buttons, on every screen that draws Pipeline Status.
 * jetstore_maintenance_02, defect `D04`, `I-26` (2026-10-01).
 *
 * **Decided: the buttons appear wherever the table is drawn**, not on Home alone.
 * `AE.8` built them into Home, and `homeFiltersUF`'s status step draws the same
 * table through `FlowRunner` without them — narrower than Flutter, which drew the
 * custom row wherever the table appeared, and against the rule `AD.3` and `AE.4`
 * had already found for this table's authored buttons: *a button on this table
 * belongs to every screen that draws it*. Plan `AE.8`'s "Pipeline Status only"
 * means *this table only*.
 *
 * **So the two halves a screen needs are here, once**, rather than copied into
 * each screen: which table gets the row, and how a press runs. A third screen
 * drawing this table calls the same two functions and cannot drift.
 */

import type { ActionConfig, TableConfig } from "../datatable/types";
import { customButtonAction, customButtonFor, customButtonSpec, type CustomButton } from "./customButtons";
import type { EscapeContext, EscapeHost } from "./escapes";
import { runStageClipboard } from "./stageClipboard";

/** The one table the custom buttons belong to — Flutter's choice, and the report's. */
export const CUSTOM_BUTTON_TABLE = "pipelineExecStatusTable";

/**
 * The table's configuration with the custom buttons filled in, when it is the
 * Pipeline Status table and there are any; any other table, unchanged.
 * `TableView` appends `fromConfigRowActions` to the last row.
 */
export function withCustomButtons(config: TableConfig, buttons: readonly CustomButton[]): TableConfig {
  if (config.key !== CUSTOM_BUTTON_TABLE || buttons.length === 0) return config;
  return { ...config, fromConfigRowActions: buttons.map(customButtonAction) };
}

/**
 * Runs a press if it is a custom button's, and says so.
 *
 * Undefined when the action is not a custom button, so the screen's own dispatch
 * carries on; otherwise the outcome — an error to show, or null. A custom button
 * names no action-document entry, because it is authored nowhere, so it cannot go
 * through `runAction`.
 */
export function runCustomButton(
  buttons: readonly CustomButton[],
  action: ActionConfig,
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> | undefined {
  const button = customButtonFor(buttons, action);
  if (button === undefined) return undefined;
  return runStageClipboard(customButtonSpec(button), context, host);
}

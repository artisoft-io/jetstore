/**
 * A deployment's own Pipeline Status buttons. jetstore_maintenance_02, defect `D04`,
 * task `AE.8` (2026-10-01).
 *
 * ## Where they come from, and why not from a document
 *
 * `JETS_CUSTOM_BUTTONS_CONFIG_JSON` is set per deployment, put into the apiserver's
 * environment by the CDK, and served at sign-in as `custom_buttons`
 * (`jets/apiserver/custom_buttons.go`). The Flutter app compiled the same variable
 * in (`jetsclient/lib/button_config.dart`, at `c38483b` in
 * `repo_infra_jetstore_platform`) and drew its entries as a third row on the
 * Pipeline Status table only; they disappeared with that app. **They stay out of
 * every `.tc.json`** — both table schemas refuse `fromConfigRowActions`, which is
 * `ui_refresh` I-102's objection kept: a workspace file must not be a second way
 * to configure a deployment. What changes is that the field is filled at run time.
 *
 * ## The shape is Flutter's, and so is the all-or-nothing
 *
 * `ButtonConfig.fromJson` required a string `type` and `key` and a string-list
 * `fsk_params`, and `parseButtonConfig` answered `[]` for the whole value on any
 * failure. Kept, because a half-applied configuration is harder to diagnose than an
 * absent one, and the apiserver already logged anything structurally wrong. Two
 * things are refused that Flutter accepted, both because the button could not work:
 * a `type` other than `fetch_stage_to_clipboard` (Flutter drew it and the press did
 * nothing), and a missing `file_path` (Flutter drew it and the press reported an
 * error).
 *
 * **`fsk_params` is read and not needed.** Flutter substituted only the names it
 * listed, so a placeholder outside the list was sent with its braces. The escape
 * substitutes every `{{key}}` and refuses one it cannot fill
 * (`stageClipboard.ts`, `substituteStagePath`), which is the same result for every
 * configuration that worked and a message for every one that did not.
 *
 * ## Capability
 *
 * `jetstore_read`, which is what `fetch_file_from_stage` requires
 * (`jets/apiserver/api_tables.go`). Flutter declared `run_pipelines`. The plan's
 * rule for `D01` (`AD.3`, `I-2`) is *the endpoint's capability, not the retired
 * client's*, and it applies here unchanged (`Q-14`, answered so).
 */

import type { ActionConfig } from "../datatable/types";
import type { StageClipboardSpec } from "./stageClipboard";

export const FETCH_STAGE_TO_CLIPBOARD = "fetch_stage_to_clipboard";

/** Flutter's default label, used when an entry names none. */
export const DEFAULT_CUSTOM_LABEL = "Fetch from Stage to Clipboard";

/** Prefixes a custom button's action key, so it cannot collide with a built-in one. */
const KEY_PREFIX = "customButton.";

export interface CustomButton {
  type: typeof FETCH_STAGE_TO_CLIPBOARD;
  key: string;
  label: string;
  description?: string;
  filePath: string;
  replaceText?: string;
  replaceWith?: string;
  fskParams?: string[];
}

const optionalString = (value: unknown): value is string | undefined =>
  value === undefined || value === null || typeof value === "string";

/** One entry, or null when it is not a button this app can draw and run. */
function parseOne(entry: unknown): CustomButton | null {
  if (entry === null || typeof entry !== "object" || Array.isArray(entry)) return null;
  const e = entry as Record<string, unknown>;
  if (e["type"] !== FETCH_STAGE_TO_CLIPBOARD) return null;
  if (typeof e["key"] !== "string" || e["key"] === "") return null;
  if (typeof e["file_path"] !== "string" || e["file_path"] === "") return null;
  for (const field of ["label", "description", "replace_text", "replace_with"]) {
    if (!optionalString(e[field])) return null;
  }
  const fsk = e["fsk_params"];
  if (fsk !== undefined && !(Array.isArray(fsk) && fsk.every((p) => typeof p === "string"))) return null;
  const text = (field: string): string | undefined => (typeof e[field] === "string" ? (e[field] as string) : undefined);
  return {
    type: FETCH_STAGE_TO_CLIPBOARD,
    key: e["key"],
    label: text("label") || DEFAULT_CUSTOM_LABEL,
    ...(text("description") !== undefined ? { description: text("description")! } : {}),
    filePath: e["file_path"],
    ...(text("replace_text") !== undefined ? { replaceText: text("replace_text")! } : {}),
    ...(text("replace_with") !== undefined ? { replaceWith: text("replace_with")! } : {}),
    ...(fsk !== undefined ? { fskParams: fsk as string[] } : {}),
  };
}

/**
 * The login response's `custom_buttons`, as buttons — all of them or none.
 *
 * Anything but an array of valid entries is none, and says why in the console;
 * the screen draws its built-in buttons either way. Duplicate keys are refused
 * too, because a key is how a press finds its button.
 */
export function parseCustomButtons(raw: unknown): CustomButton[] {
  if (raw === undefined || raw === null) return [];
  if (!Array.isArray(raw)) {
    console.warn("custom_buttons is not a list; no custom buttons are drawn");
    return [];
  }
  const buttons: CustomButton[] = [];
  const keys = new Set<string>();
  for (const [index, entry] of raw.entries()) {
    const button = parseOne(entry);
    if (button === null || keys.has(button.key)) {
      console.warn(`custom_buttons entry ${index} cannot be used; no custom buttons are drawn`);
      return [];
    }
    keys.add(button.key);
    buttons.push(button);
  }
  return buttons;
}

/** The action-bar entry for one custom button. */
export function customButtonAction(button: CustomButton): ActionConfig {
  return {
    actionType: "doAction",
    key: `${KEY_PREFIX}${button.key}`,
    label: button.label,
    style: "secondary",
    actionName: button.key,
    capability: "jetstore_read",
    isVisibleWhenCheckboxVisible: true,
    isEnabledWhenHavingSelectedRows: true,
    stateGroup: 0,
    hasIsEnabledFnc: false,
    hasActionDelegate: false,
  };
}

/** The custom button an action-bar entry stands for, or undefined for any other. */
export function customButtonFor(buttons: readonly CustomButton[], action: ActionConfig): CustomButton | undefined {
  if (!action.key.startsWith(KEY_PREFIX)) return undefined;
  const key = action.key.slice(KEY_PREFIX.length);
  return buttons.find((b) => b.key === key);
}

/** What the escape needs to run one. */
export function customButtonSpec(button: CustomButton): StageClipboardSpec {
  return {
    filePath: button.filePath,
    label: button.label,
    replaceText: button.replaceText ?? null,
    replaceWith: button.replaceWith ?? null,
  };
}

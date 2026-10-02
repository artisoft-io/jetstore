/**
 * Parsing a deployment's custom buttons — jetstore_maintenance_02, `D04`, `AE.8`.
 * The rendering half is `screens/Home.test.tsx`, *the third row (D04)*.
 */

import { describe, expect, it, vi } from "vitest";

import {
  DEFAULT_CUSTOM_LABEL,
  customButtonAction,
  customButtonFor,
  customButtonSpec,
  parseCustomButtons,
} from "./customButtons";

/**
 * Cedargate's value, verbatim from `build_jetstore_scripts`
 * (`internal/workspaces/cedargate_ws.sh`, read 2026-10-01). A string, parsed here,
 * so the test is of the value the operator actually writes.
 */
const CEDARGATE =
  '[{"type":"fetch_stage_to_clipboard","key":"analysis_report_to_clipboard","description":"Get Analysis Report from JetStore stage s3 location to clipboard","label":"Analysis Report","replace_text":"|","replace_with":",","fsk_params":["process_name","session_id"],"file_path":"process_name={{process_name}}/session_id={{session_id}}/step_id=analysis_lookup/jets_partition=analysis_data/part0000-0000001.csv"}]';

describe("parseCustomButtons", () => {
  it("parses cedargate's value into one Analysis Report button", () => {
    const buttons = parseCustomButtons(JSON.parse(CEDARGATE));
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toMatchObject({
      key: "analysis_report_to_clipboard",
      label: "Analysis Report",
      replaceText: "|",
      replaceWith: ",",
      fskParams: ["process_name", "session_id"],
    });
  });

  it("draws none for anything it cannot use, all or nothing, as Flutter did", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const good = JSON.parse(CEDARGATE)[0] as Record<string, unknown>;
    for (const bad of [
      { not: "a list" },
      "a string",
      [{ ...good, type: "open_url" }], // a type this app cannot run
      [{ ...good, key: 3 }],
      [{ ...good, file_path: undefined }],
      [{ ...good, fsk_params: "process_name" }],
      [{ ...good, label: 7 }],
      [good, { ...good, label: "Twice" }], // the same key twice
      [good, null],
    ]) {
      expect(parseCustomButtons(bad)).toEqual([]);
    }
    warn.mockRestore();
  });

  it("is empty, silently, when the server sends none", () => {
    expect(parseCustomButtons(undefined)).toEqual([]);
    expect(parseCustomButtons([])).toEqual([]);
  });

  it("uses Flutter's label when an entry names none", () => {
    const { label: _label, ...unlabelled } = JSON.parse(CEDARGATE)[0] as Record<string, unknown>;
    expect(parseCustomButtons([unlabelled])[0]!.label).toBe(DEFAULT_CUSTOM_LABEL);
  });
});

describe("a custom button on the action bar", () => {
  const [button] = parseCustomButtons(JSON.parse(CEDARGATE));

  it("needs a selected row and jetstore_read, the capability fetch_file_from_stage checks (Q-14)", () => {
    const action = customButtonAction(button!);
    expect(action).toMatchObject({
      actionType: "doAction",
      label: "Analysis Report",
      capability: "jetstore_read",
      isEnabledWhenHavingSelectedRows: true,
    });
  });

  it("is found again from its action, and no built-in action is mistaken for one", () => {
    const action = customButtonAction(button!);
    expect(customButtonFor([button!], action)).toBe(button);
    expect(
      customButtonFor([button!], { ...action, key: "analysis_report_to_clipboard" }),
    ).toBeUndefined();
  });

  it("carries its path and replacement to the escape", () => {
    expect(customButtonSpec(button!)).toEqual({
      filePath: button!.filePath,
      label: "Analysis Report",
      replaceText: "|",
      replaceWith: ",",
    });
  });
});

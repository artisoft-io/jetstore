/**
 * `copySchemaEvent` — jetstore_maintenance_02, `D04`, `AE.5`. The screen-level
 * case is `screens/Home.test.tsx`; these are the edges a screen test would not
 * reach without a fixture per row.
 */

import { describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import type { JetsRow } from "../datatable/types";
import type { EscapeHost } from "./escapes";
import { copySchemaEvent } from "./schemaEvent";

function hostReading(rows: JetsRow[] | null) {
  const notes: string[] = [];
  const copied: string[] = [];
  const queries: string[] = [];
  const host: EscapeHost = {
    post: async () => ({ statusCode: 200 }),
    read: async (request) => {
      queries.push(String(request.body["query"]));
      return rows;
    },
    notify: (_level, message) => notes.push(message),
    setBusy: () => {},
    close: () => {},
    userEmail: () => "",
    download: () => {},
    writeClipboard: async (text) => {
      copied.push(text);
    },
  };
  return { host, notes, copied, queries };
}

const run = (value: string | string[] | null, host: EscapeHost) => {
  const formState = new FormState();
  formState.setValue(0, "main_input_registry_key", value);
  return copySchemaEvent({ formState, group: 0, flowKey: "homeScreen" }, host);
};

describe("copySchemaEvent", () => {
  it("reads by the registry key, coerced to an integer, and copies the event", async () => {
    const { host, copied, queries } = hostReading([['{"e":1}']]);
    expect(await run(["42"], host)).toBeNull();
    expect(queries).toEqual(["SELECT schema_provider_json FROM jetsapi.input_registry WHERE key = 42"]);
    expect(copied).toEqual(['{"e":1}']);
  });

  it("refuses a key that is not an integer before reading anything", async () => {
    const { host, queries } = hostReading([]);
    expect(await run(["42; DROP TABLE x"], host)).toMatch(/Select a run first/);
    expect(await run(null, host)).toMatch(/Select a run first/);
    expect(queries).toEqual([]);
  });

  it("refuses two selected runs rather than taking the first", async () => {
    const { host } = hostReading([]);
    expect(await run(["1", "2"], host)).toMatch(/more than one/);
  });

  it("reports a registration with no schema event as a fact, not an error", async () => {
    const { host, notes, copied } = hostReading([[null]]);
    expect(await run(["42"], host)).toBeNull();
    expect(notes).toEqual(["This run's input registration has no schema event."]);
    expect(copied).toEqual([]);
  });

  it("reports a missing registration row and a failed read as errors", async () => {
    expect(await run(["42"], hostReading([]).host)).toMatch(/no input registration 42/);
    expect(await run(["42"], hostReading(null).host)).toMatch(/Could not read/);
  });
});

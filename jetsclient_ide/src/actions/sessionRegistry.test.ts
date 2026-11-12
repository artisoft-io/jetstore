/**
 * The Session Registry tab's filters (`homeFilters.ts`) and its *Delete*'s
 * preview and gate (`sessionDeletion.ts`), without a screen. `session_registry`
 * Phase 2, JM-081, 2026-11-08. `screens/Home.test.tsx` drives both through the
 * screen; this file holds the cases a screen would make slow to enumerate.
 */

import { afterEach, describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import type { JetsRow } from "../datatable/types";
import sessionRegistryTable from "../datatable/tables/sessionRegistryTable.tc.json";
import {
  clearSessionRegistryFilters,
  currentSessionRegistryFilters,
  hasSessionRegistryFilters,
  resetHomeFilters,
  savedSessionRegistryFilterState,
  sessionRegistryFiltersValidator,
  setSessionRegistryIdFilter,
  updateSessionRegistryFilters,
} from "./homeFilters";
import type { EndpointRequest, EscapeHost } from "./escapes";
import {
  EXPECTED_KEY,
  PREVIEW_KEY,
  SELECTED_DELETION_STATUS,
  noSessionDeletionRequested,
  previewLine,
  previewRowOf,
  previewSessionDeletion,
} from "./sessionDeletion";

afterEach(resetHomeFilters);

const context = (formState = new FormState()) => ({ formState, group: 0, flowKey: "homeScreen" });
const clauses = () =>
  (currentSessionRegistryFilters() ?? []).map(({ column, ge, le, like, defaultValue }) => ({
    column,
    ...(like !== undefined ? { like } : {}),
    ...(ge !== undefined ? { ge } : {}),
    ...(le !== undefined ? { le } : {}),
    ...(defaultValue.length > 0 ? { values: defaultValue } : {}),
  }));

describe("Set Session Id on Session Registry (Q-6)", () => {
  it.each([
    ["100..200", [{ column: "session_num", ge: "100" }, { column: "session_num", le: "200" }]],
    ["100..", [{ column: "session_num", ge: "100" }]],
    ["..200", [{ column: "session_num", le: "200" }]],
    [" 100 .. 100 ", [{ column: "session_num", ge: "100" }, { column: "session_num", le: "100" }]],
    ["123456789012345678..", [{ column: "session_num", ge: "123456789012345678" }]],
    ["a-1, 2002 ,", [{ column: "session_id", values: ["a-1", "2002"] }]],
  ])("%j becomes %j", (input, expected) => {
    expect(setSessionRegistryIdFilter(input)).toBeNull();
    expect(clauses()).toEqual(expected);
    expect(hasSessionRegistryFilters(new FormState(), 0)).toBe(true);
  });

  it.each([
    ["..", /at least one end/],
    ["1e3..", /digits only/],
    ["-1..5", /digits only/],
    ["100..2' OR '1'='1", /digits only/],
    ["1234567890123456789..", /digits only, at most 18/],
    ["300..200", /is empty/],
    ["", /Enter session ids/],
    [" , ", /Enter session ids/],
    ["2002', '1", /is not a session id/],
  ])("refuses %j and leaves the filters as they were", (input, message) => {
    expect(setSessionRegistryIdFilter("7")).toBeNull();
    expect(setSessionRegistryIdFilter(input)).toMatch(message);
    expect(clauses()).toEqual([{ column: "session_id", values: ["7"] }]);
  });

  it("names the view on every clause, which the server qualifies the column with", () => {
    setSessionRegistryIdFilter("1..2");
    expect(currentSessionRegistryFilters()!.map((w) => w.table)).toEqual([
      "session_registry_view",
      "session_registry_view",
    ]);
  });
});

describe("Set Filters on Session Registry", () => {
  it("compiles every answered field, in the dialog's order, and saves the answers", async () => {
    const formState = new FormState();
    formState.setValue(0, "srProcessName", ["Map_MedicalClaim", "Map_Eligibility"]);
    formState.setValue(0, "srSourceType", "domain_table");
    formState.setValue(0, "srFileKeyFragment", "client=CI");
    formState.setValue(0, "srFileKeyDateFrom", "2026-09-01");
    formState.setValue(0, "srFileKeyDateTo", "2026-10-31");
    formState.setValue(0, "srUpdatedFrom", "2026-11-01T08:30:00Z");
    formState.setValue(0, "srUpdatedTo", "12 hours");
    expect(await updateSessionRegistryFilters(context(formState))).toBeNull();
    expect(clauses()).toEqual([
      { column: "process_name", values: ["Map_MedicalClaim", "Map_Eligibility"] },
      { column: "source_type", values: ["domain_table"] },
      { column: "file_key", like: "%client=CI%" },
      { column: "file_key_date", ge: "timestamp '2026-09-01'" },
      { column: "file_key_date", le: "timestamp '2026-10-31'" },
      { column: "last_update", ge: "timestamp '2026-11-01 08:30:00'" },
      { column: "last_update", le: "now()-interval '12 hours'" },
    ]);
    expect(savedSessionRegistryFilterState()).toEqual({
      srProcessName: ["Map_MedicalClaim", "Map_Eligibility"],
      srSourceType: "domain_table",
      srFileKeyFragment: "client=CI",
      srFileKeyDateFrom: "2026-09-01",
      srFileKeyDateTo: "2026-10-31",
      srUpdatedFrom: "2026-11-01T08:30:00Z",
      srUpdatedTo: "12 hours",
    });
  });

  it("matches a file key fragment as typed: LIKE's wildcards and a quote are data", async () => {
    const formState = new FormState();
    formState.setValue(0, "srFileKeyFragment", " CI_A%x\\y'z ");
    expect(await updateSessionRegistryFilters(context(formState))).toBeNull();
    expect(clauses()).toEqual([{ column: "file_key", like: "%CI\\_A\\%x\\\\y'z%" }]);
  });

  it("filters no process when none is selected", async () => {
    const formState = new FormState();
    formState.setValue(0, "srProcessName", []);
    expect(await updateSessionRegistryFilters(context(formState))).toBeNull();
    expect(currentSessionRegistryFilters()).toEqual([]);
  });

  it("replaces the list, the session-id filter included, and blank means no filter", async () => {
    setSessionRegistryIdFilter("1..2");
    expect(await updateSessionRegistryFilters(context())).toBeNull();
    expect(currentSessionRegistryFilters()).toEqual([]);
    expect(hasSessionRegistryFilters(new FormState(), 0)).toBe(false);
  });

  it("refuses what it would otherwise splice, even past the validator", async () => {
    // `validate` runs first on the screen; this is the second check, because a
    // filter that reached the store unchecked would reach the server unchecked.
    const formState = new FormState();
    formState.setValue(0, "srUpdatedFrom", "now()); DROP TABLE x; --");
    expect(await updateSessionRegistryFilters(context(formState))).toMatch(/Enter a date/);
    expect(currentSessionRegistryFilters()).toBeNull();
  });

  it.each([
    ["srSourceType", "file", null],
    ["srSourceType", "file'", /one word/],
    ["srFileKeyFragment", "year=2026/month=10", null],
    ["srFileKeyFragment", "x".repeat(201), /at most 200/],
    ["srFileKeyDateFrom", "2026-10-31", null],
    ["srFileKeyDateTo", "Oct 31", /Enter a date/],
    ["srUpdatedTo", "2026-11-08", null],
    ["srUpdatedTo", "2026-11-08 15:00", null],
    ["srUpdatedTo", "3 days", null],
    ["srUpdatedTo", "3 fortnights", /Enter a date/],
    ["srUpdatedTo", "", null],
    ["somethingElse", "anything'", null],
  ])("the validator answers %s = %j with %s", (key, value, expected) => {
    const answer = sessionRegistryFiltersValidator(context(), key, value);
    if (expected === null) expect(answer).toBeNull();
    else expect(answer).toMatch(expected);
  });

  it("is cleared by its own Clear Filters, which leaves the home filters alone", async () => {
    setSessionRegistryIdFilter("5");
    await clearSessionRegistryFilters(context());
    expect(currentSessionRegistryFilters()).toEqual([]);
    expect(hasSessionRegistryFilters(new FormState(), 0)).toBe(false);
  });
});

/** An escape host that answers `read` with the given rows, or null. */
function readingHost(rows: unknown[] | null) {
  const reads: EndpointRequest[] = [];
  const host = {
    read: async (request: EndpointRequest) => {
      reads.push(request);
      return rows as JetsRow[] | null;
    },
    setBusy: () => {},
  } as unknown as EscapeHost;
  return { host, reads };
}

describe("previewSessionDeletion", () => {
  const selected = (ids: string[]) => {
    const formState = new FormState();
    formState.setValue(0, "sessionRegistryTable", ids);
    return formState;
  };

  it("asks for the selected sessions and writes the ids and the text the confirmation shows", async () => {
    const formState = selected(["1001"]);
    const { host, reads } = readingHost([
      { session_id: "1001", client: "(unknown)", source_type: "file", object_type: "claim", reads_session: "", active: false },
      { session_id: "2002", client: "acme", process_name: "map_records", reads_session: "1001", active: true, month_period: 313 },
    ]);
    expect(await previewSessionDeletion(context(formState), host)).toBeNull();
    expect(reads).toEqual([
      { endpoint: "/dataTable", body: { action: "preview_session_deletion", data: [{ session_id: "1001" }] } },
    ]);
    expect(formState.getValue(0, EXPECTED_KEY)).toEqual(["1001", "2002"]);
    expect(formState.getValue(0, PREVIEW_KEY)).toBe(
      "2 sessions, 1 of them reading a selected one:\n" +
        "1001: claim, client (unknown)\n" +
        "2002: map_records, client acme, month period 313, reads 1001, waits for its run to end",
    );
  });

  it("stops with a message, and leaves no earlier preview behind, when the server refuses", async () => {
    const formState = selected(["1001"]);
    formState.setValue(0, PREVIEW_KEY, "an earlier list");
    formState.setValue(0, EXPECTED_KEY, ["9"]);
    const { host } = readingHost(null);
    expect(await previewSessionDeletion(context(formState), host)).toMatch(/refused the preview/);
    // Cleared, which form state reads back as absent.
    expect(formState.getValue(0, PREVIEW_KEY) ?? null).toBeNull();
    expect(formState.getValue(0, EXPECTED_KEY) ?? null).toBeNull();
  });

  it("stops before asking when nothing is selected", async () => {
    const { host, reads } = readingHost([]);
    expect(await previewSessionDeletion(context(), host)).toMatch(/Select the sessions/);
    expect(reads).toHaveLength(0);
  });

  it("stops when the server lists nothing", async () => {
    const { host } = readingHost([{ client: "acme" }, ["positional"]]);
    expect(await previewSessionDeletion(context(selected(["1"])), host)).toMatch(/listed no session/);
  });

  it("reads a row serialised with Go's untagged field names as well", () => {
    expect(
      previewRowOf({ SessionID: "1", Client: "c", ReadsSession: "0", ProcessName: "p", Active: true }),
    ).toMatchObject({ sessionId: "1", client: "c", readsSession: "0", processName: "p", active: true });
  });

  it("names a session by its source type when it has neither process nor object type", () => {
    expect(previewLine(previewRowOf({ session_id: "5", source_type: "file" })!)).toBe("5: file");
  });
});

describe("noSessionDeletionRequested", () => {
  it("is true when no selected row has a deletion status, and false when any has", () => {
    const formState = new FormState();
    expect(noSessionDeletionRequested(formState, 0)).toBe(true);
    formState.setValue(0, SELECTED_DELETION_STATUS, []);
    expect(noSessionDeletionRequested(formState, 0)).toBe(true);
    formState.setValue(0, SELECTED_DELETION_STATUS, ["pending"]);
    expect(noSessionDeletionRequested(formState, 0)).toBe(false);
  });
});

/**
 * The binding names its column by position, so a column added before
 * `deletion_status` moves it: Year, Month and Day replacing Domain Key did, and
 * the gate then read `last_update`. Pin the index to the name.
 */
describe("sessionRegistryTable's state binding", () => {
  it("binds sr_deletion_status to the deletion_status column", () => {
    const names = sessionRegistryTable.columns.map((c: { name: string }) => c.name);
    const bound = sessionRegistryTable.formStateBinding.otherColumns.find(
      (o: { stateKey: string }) => o.stateKey === SELECTED_DELETION_STATUS,
    );
    expect(bound).toBeDefined();
    expect(names[bound!.columnIdx]).toBe("deletion_status");
  });
});

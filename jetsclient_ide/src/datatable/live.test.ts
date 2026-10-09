/**
 * The data table against a real apiserver.
 *
 * **Skipped unless pointed at one.** Set both:
 *
 *     JETS_LIVE_API=http://localhost:8080 \
 *     JETS_LIVE_TOKEN="$(cat /path/to/token)" \
 *       npx vitest run src/datatable/live.test.ts
 *
 * Everything else in this package drives a stub, which is the right default —
 * these tests need a database with a schema in it and cannot run in CI as it
 * stands. But a stub can only prove that the client agrees with itself. Issues
 * I-4 and I-7 both say the same thing from different ends: the payloads A.4a
 * builds were read out of the Dart rather than produced by it, and the responses
 * A.4b unpacks were read out of the Go. This file is where those two readings
 * meet something that can contradict them.
 *
 * **Read-only.** Every request here is `action: "read"`, which is what
 * `makeQuery` emits and all it can emit. Nothing in this file writes.
 */

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { makeQuery } from "./query";
import type { DataTableAction, TableConfig } from "./types";

const origin = process.env["JETS_LIVE_API"];
const token = process.env["JETS_LIVE_TOKEN"];
const live = Boolean(origin && token);

const tables = corpus.tables as unknown as Record<string, TableConfig>;
const querying = Object.keys(tables).filter((k) => tables[k]!.apiPath === "/dataTable");

interface Result {
  status: number;
  body: Record<string, unknown>;
}

async function post(payload: unknown): Promise<Result> {
  const res = await fetch(`${origin}/dataTable`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(payload),
  });
  const text = await res.text();
  let body: Record<string, unknown> = {};
  try {
    body = text === "" ? {} : (JSON.parse(text) as Record<string, unknown>);
  } catch {
    body = { error: text };
  }
  return { status: res.status, body };
}

function payloadFor(key: string, over: Partial<Record<string, unknown>> = {}): DataTableAction {
  const config = tables[key]!;
  return {
    ...makeQuery({
      config,
      formField: { group: 0, key },
      indexOffset: 0,
      rowsPerPage: 5,
      sortColumnName: config.sortColumnName,
      sortColumnTableName: config.sortColumnTableName,
      sortAscending: config.sortAscending,
    }),
    ...over,
  };
}

describe.skipIf(!live)("against a live apiserver", () => {
  it("accepts a token and answers a minimal read", async () => {
    // Fails fast and clearly if the token is stale, rather than reporting 28
    // authorisation failures as 28 schema problems.
    const { status, body } = await post(payloadFor(querying[0]!));
    expect(status, `server said: ${JSON.stringify(body)}`).toBe(200);
  });

  it.each(querying)("accepts the payload A.4a builds for %s", async (key) => {
    const { status, body } = await post(payloadFor(key));
    expect(status, `server said: ${JSON.stringify(body)}`).toBe(200);

    // The response shape A.4b unpacks, asserted against the real thing rather
    // than against a reading of `DoReadAction`.
    expect(Array.isArray(body["rows"])).toBe(true);
    for (const row of (body["rows"] as unknown[]).slice(0, 3)) {
      // Positional arrays, not objects. Everything downstream — the form-state
      // bindings especially — depends on this and nothing else states it.
      expect(Array.isArray(row)).toBe(true);
    }
    if (body["totalRowCount"] !== undefined) {
      expect(typeof body["totalRowCount"]).toBe("number");
    }
  });

  it("returns rows no wider than the columns the config selects", async () => {
    // A row wider than the select list would mean the server is not honouring
    // `columns`, and every `columnIdx` binding would be off.
    const key = "lfSourceConfigTable";
    const payload = payloadFor(key);
    const { status, body } = await post(payload);
    expect(status).toBe(200);
    const rows = body["rows"] as unknown[][];
    if (rows.length === 0) return; // an empty table proves nothing, and is fine
    expect(rows[0]!.length).toBe((payload.columns as unknown[]).length);
  });

  it("honours the limit", async () => {
    const { status, body } = await post(payloadFor("lfSourceConfigTable", { limit: 2 }));
    expect(status).toBe(200);
    expect((body["rows"] as unknown[]).length).toBeLessThanOrEqual(2);
  });

  it("returns a columnDef when the request names no columns", async () => {
    // The least-certain path in A.4b: no user flow uses it, and its field names
    // — `isnumeric`, lowercase — were read from the Go rather than observed.
    const { status, body } = await post({
      ...payloadFor("lfSourceConfigTable"),
      columns: [],
      sortColumn: "",
      sortColumnTable: "",
    });
    expect(status, `server said: ${JSON.stringify(body)}`).toBe(200);

    const columnDef = body["columnDef"] as Record<string, unknown>[] | undefined;
    expect(columnDef, "expected a columnDef for a request with no columns").toBeTruthy();
    const first = columnDef![0]!;
    for (const field of ["index", "name", "label"]) {
      expect(first, `columnDef is missing ${field}`).toHaveProperty(field);
    }
    // Asserted separately because this is the exact spelling A.4b reads.
    expect(Object.keys(first)).toContain("isnumeric");
  });
});

describe.skipIf(live)("live tests are skipped", () => {
  it("says so, rather than passing silently", () => {
    expect(live).toBe(false);
  });
});

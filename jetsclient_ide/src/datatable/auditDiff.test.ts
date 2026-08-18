/**
 * Diffs the Flutter app's `/dataTable` requests against the ones A.4a builds.
 *
 * **This closes the second half of I-4.** The live suite established that our
 * payloads are *valid* — the server accepts all 28 and answers coherently. It
 * could not establish that they are *identical* to the Dart's, and that gap is
 * not cosmetic: a dropped filter returns more rows rather than an error, so a
 * misreading of `_makeQuery` survives every test that only talks to a server.
 *
 * The Flutter app's payloads need no instrumentation to capture.
 * `DoDataTableAction` audit-logs each request body verbatim
 * (`jets/apiserver/api_tables.go`), and the audit logger writes JSON lines to
 * **stdout** (`jets/apiserver/server.go:405`). So driving a flow in the Flutter
 * app with the apiserver's stdout captured is the whole apparatus.
 *
 * **A capture is bundled, so this runs by default.**
 * `fixtures/load_files_flutter_audit.log` is the apiserver's stdout from driving
 * `load_files` in the Flutter app on 2026-08-18, against synthetic data. It makes
 * the comparison a standing regression guard on A.4a rather than an exercise
 * someone has to remember to repeat: change `makeQuery` in a way the Dart would
 * not have, and this fails.
 *
 * To check a fresh capture instead — a different flow, or one that pages and
 * re-sorts — point `JETS_AUDIT_LOG` at it:
 *
 *     # apiserver started with:  ./apiserver … 2>&1 | tee apiserver.log
 *     # then drive http://localhost:8080/#/loadFilesUF and pick a source config
 *     JETS_AUDIT_LOG=/path/to/apiserver.log npx vitest run src/datatable/auditDiff.test.ts
 *
 * **What the bundled capture does not cover**, and a fresh one should: the other
 * 26 querying configurations, and any paging or sorting interaction — both of its
 * requests carry `offset: 0` and the configured sort.
 *
 * ## What is taken from the observation, and what is not
 *
 * This is the one design decision that decides whether the test proves anything.
 * Rebuilding the expected payload *from* the observed payload would make it
 * vacuous — it would compare a value with itself. So the boundary is drawn at
 * the difference between **state** and **logic**:
 *
 *  - **Taken from the observation**, because it encodes what the user and the
 *    widget were doing and is unknowable from the configuration alone: `offset`,
 *    `limit`, `sortColumn`, `sortColumnTable`, `sortAscending`, which client is
 *    selected, and which row was clicked (the `table_name` the selection
 *    published).
 *  - **Produced independently by `makeQuery`** from the table configuration, and
 *    compared: `fromClauses`, `columns`, `withClauses`, `distinctOnClauses`, the
 *    entire `whereClauses` array including its ordering, and `action`.
 *
 * A field in the second group differing is a real finding. A field in the first
 * group is an input, and the test says so rather than pretending otherwise.
 */

import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/table_configs.json";
import { makeQuery } from "./query";
import type { DataTableAction, TableConfig, WhereClausePayload } from "./types";

/** An override for a fresh capture; otherwise the bundled one. */
const logPath =
  process.env["JETS_AUDIT_LOG"] ??
  new URL("./fixtures/load_files_flutter_audit.log", import.meta.url).pathname;
const tables = corpus.tables as unknown as Record<string, TableConfig>;

/** One zap line: `{"level":…,"message":"<the raw body>","logger_type":"audit_log"}`. */
export function extractDataTableRequests(log: string): DataTableAction[] {
  const out: DataTableAction[] = [];
  for (const line of log.split("\n")) {
    const trimmed = line.trim();
    if (trimmed === "" || !trimmed.startsWith("{")) continue;
    let entry: Record<string, unknown>;
    try {
      entry = JSON.parse(trimmed) as Record<string, unknown>;
    } catch {
      continue;
    }
    if (entry["logger_type"] !== "audit_log") continue;
    const message = entry["message"];
    if (typeof message !== "string" || !message.trimStart().startsWith("{")) continue;
    try {
      const body = JSON.parse(message) as DataTableAction;
      if (typeof body.action === "string" && Array.isArray(body.fromClauses)) {
        out.push(body);
      }
    } catch {
      // A non-JSON audit message — a login, for instance. Not ours.
    }
  }
  return out;
}

/**
 * Which corpus tables could have produced this request.
 *
 * **A captured payload does not say which widget sent it, and the obvious
 * identifier is not unique.** Five FROM lists are shared across twenty of the 28
 * querying configurations — ten tables alone select from `process_input`. Adding
 * the column list separates most of them, and still leaves four groups
 * indistinguishable, among them `inputRegistryTable` and
 * `main_input_registry_key`.
 *
 * So this returns *candidates*, and the comparison below accepts a request if
 * **any** candidate reproduces it exactly. That is the honest question — does
 * some configuration in our corpus generate this payload byte for byte — rather
 * than pretending to know which screen the user was on.
 *
 * The `load_files` pair this test exists for is unambiguous once columns are
 * included, so for the flow that matters the candidate list is a single entry.
 */
function candidateTables(observed: DataTableAction): string[] {
  const from = observed.fromClauses.map((f) => f.table).join(",");
  // `columns` is absent on the write actions — `insert_rows` and friends share
  // the endpoint and so share the audit log. They match no read configuration
  // and fall out below; this guard is what stops them throwing on the way.
  const columns = ((observed.columns ?? []) as { table?: string; column: string }[])
    .map((c) => `${c.table ?? ""}.${c.column}`)
    .join(",");
  const byFromAndColumns: string[] = [];
  const byFromOnly: string[] = [];

  for (const [key, config] of Object.entries(tables)) {
    if (config.apiPath !== "/dataTable") continue;
    if (config.fromClauses.map((f) => f.tableName).join(",") !== from) continue;
    byFromOnly.push(key);
    const configColumns = config.columns.map((c) => `${c.table ?? ""}.${c.name}`).join(",");
    if (configColumns === columns) byFromAndColumns.push(key);
  }
  // Falling back to FROM alone matters: if no column list matches, the select
  // list itself differs, and reporting that as "unrecognised" would hide it.
  return byFromAndColumns.length > 0 ? byFromAndColumns : byFromOnly;
}

/** The client filter `_makeQuery` appends last, if the observation carries one. */
function observedClient(observed: DataTableAction, config: TableConfig): string | undefined {
  const first = config.fromClauses[0]?.tableName ?? "";
  const clause = (observed.whereClauses ?? []).find(
    (wc) => wc.column === "client" && wc.table === first && Array.isArray(wc.values),
  );
  return clause?.values?.[0] ?? undefined;
}

/** The form-state values the observation implies — the row the user clicked. */
function observedFormState(observed: DataTableAction): Record<string, string[]> {
  const values: Record<string, string[]> = {};
  for (const wc of observed.whereClauses ?? []) {
    // Only clauses that a form-state key could have produced: a plain values
    // list on a column the config filters by form state.
    if (wc.values && wc.column !== "client") {
      values[wc.column] = wc.values.filter((v): v is string => v != null);
    }
  }
  return values;
}

function rebuild(key: string, observed: DataTableAction): DataTableAction {
  const config = tables[key]!;
  const state = observedFormState(observed);
  const formStateKeys = new Set(
    config.whereClauses.map((wc) => wc.formStateKey).filter((k): k is string => k != null),
  );

  return makeQuery({
    config,
    formField: { group: 0, key },
    formState: {
      isDialog: false,
      getValue: (_group, k) => {
        // Only keys the configuration actually filters by are answerable; the
        // rest must come back undefined so the fallbacks are exercised rather
        // than bypassed.
        if (!formStateKeys.has(k)) return undefined;
        // A form-state key filters some column; find which, then read the value
        // the observation shows for it.
        const clause = config.whereClauses.find((wc) => wc.formStateKey === k);
        return clause ? state[clause.column] : undefined;
      },
    },
    selectedClient: observedClient(observed, config) ?? null,
    indexOffset: observed.offset,
    rowsPerPage: observed.limit,
    sortColumnName: observed.sortColumn,
    sortColumnTableName: observed.sortColumnTable,
    sortAscending: observed.sortAscending,
  });
}

/** Compared without regard to ordering only where the Dart's own order is not fixed. */
function normalise(payload: DataTableAction): Record<string, unknown> {
  return {
    action: payload.action,
    fromClauses: payload.fromClauses,
    columns: payload.columns,
    withClauses: payload.withClauses,
    distinctOnClauses: payload.distinctOnClauses ?? [],
    whereClauses: (payload.whereClauses ?? []) as WhereClausePayload[],
    workspaceName: payload.workspaceName ?? null,
  };
}

describe("the audit-log parser", () => {
  // Runs always: the parser is the part that can be wrong without a server.
  it("pulls a request body out of a zap audit line", () => {
    const line = JSON.stringify({
      level: "info",
      message: JSON.stringify({ action: "read", fromClauses: [{ schema: "jetsapi", table: "source_config" }] }),
      logger_type: "audit_log",
      user: "michel@artisoft.io",
    });
    const found = extractDataTableRequests(`noise\n${line}\n`);
    expect(found).toHaveLength(1);
    expect(found[0]!.action).toBe("read");
  });

  it("ignores audit lines that are not request bodies", () => {
    const login = JSON.stringify({ message: "user login", logger_type: "audit_log" });
    const other = JSON.stringify({ message: "{}", logger_type: "not_audit" });
    expect(extractDataTableRequests(`${login}\n${other}\n`)).toHaveLength(0);
  });
});

describe("captured Flutter requests match what A.4a builds", () => {
  const log = readFileSync(logPath, "utf8");
  const observed = extractDataTableRequests(log);

  it("found requests in the log at all", () => {
    // Fails loudly rather than passing on an empty capture, which is the way
    // this test would otherwise lie.
    expect(observed.length, `no /dataTable request bodies in ${logPath}`).toBeGreaterThan(0);
  });

  it("recognises which tables were driven", () => {
    const keys = [...new Set(observed.flatMap(candidateTables))];
    expect(keys.length, `none of the captured requests matched a corpus table`).toBeGreaterThan(0);
    // eslint-disable-next-line no-console
    console.log(`captured ${observed.length} requests; candidate tables: ${keys.join(", ")}`);
  });

  it("produces byte-identical payloads for every captured request", () => {
    const mismatches: string[] = [];
    const matched: string[] = [];
    let compared = 0;

    for (const request of observed) {
      const candidates = candidateTables(request);
      if (candidates.length === 0) continue;
      compared++;

      const theirs = JSON.stringify(normalise(request));
      const attempts = candidates.map((key) => ({
        key,
        ours: JSON.stringify(normalise(rebuild(key, request))),
      }));
      const hit = attempts.find((a) => a.ours === theirs);
      if (hit) {
        matched.push(hit.key);
        continue;
      }

      mismatches.push(
        [
          `no candidate reproduces this request (tried ${candidates.join(", ")})`,
          `  flutter: ${theirs}`,
          ...attempts.map((a) => `  react/${a.key}: ${a.ours}`),
        ].join("\n"),
      );
    }

    // Logged rather than merely asserted: "all matched" and "one was silently
    // skipped" produce the same green tick otherwise, and the count is the only
    // thing that separates them.
    // eslint-disable-next-line no-console
    console.log(`compared ${compared} of ${observed.length} captured requests: ${matched.join(", ")}`);

    expect(compared, "no captured request matched a corpus table").toBeGreaterThan(0);
    expect(mismatches.join("\n\n"), `${mismatches.length} of ${compared} differ`).toBe("");
  });
});

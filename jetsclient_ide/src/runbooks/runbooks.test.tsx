/**
 * @vitest-environment jsdom
 *
 * The runbook screens (`runbooks` project, Phase 1, track AC).
 *
 * One test per control and state, driven through a stubbed `fetch` as
 * `proposals.test.tsx` drives its screens, so these exercise
 * `ApiClient.endpoint` and the wire shapes as well as the components.
 *
 * **The test that matters most is the `ddl` confirmation**: the project's safety
 * rule is that every `ddl` step pauses for a person who sees the exact
 * statement and every bound value before *Accept*. A later edit that moved the
 * statement behind a toggle would pass every other test here.
 */

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NavEntryProvider } from "../shell/navEntry";
import { NotificationsProvider } from "../shell/notifications";
import { PromptsProvider } from "../shell/prompts";
import { answerConfirm } from "../shell/promptsTesting";
import {
  RunbooksApi,
  currentStep,
  parseTyped,
  runControls,
  shouldPoll,
  type Run,
  type StepRun,
} from "./api";
import { RunScreen } from "./RunScreen";
import { RunbooksScreen } from "./RunbooksScreen";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

type Reply = { status?: number; body: unknown };
type Sent = { path: string; body: Record<string, unknown> };

/**
 * An ApiClient with a live session whose calls are answered from a per-action
 * script. Every request is captured with its path, so a test can assert both
 * what the screen sent and where.
 */
async function clientWith(
  script: Record<string, Reply | ((req: Record<string, unknown>) => Reply)>,
  capabilities: string[] = ["runbook_execute"],
) {
  const sent: Sent[] = [];
  const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Engineer",
          user_email: "engineer@example.com",
          is_admin: false,
          capabilities,
        }),
        { status: 200 },
      );
    }
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    sent.push({ path: new URL(String(url), "http://x").pathname, body });
    const entry = script[String(body["action"])];
    const reply = typeof entry === "function" ? entry(body) : entry;
    if (!reply) return new Response(JSON.stringify({ error: "no script" }), { status: 500 });
    return new Response(JSON.stringify({ token: "t1", ...(reply.body as object) }), {
      status: reply.status ?? 200,
    });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("engineer@example.com", "pw");
  const actions = (name: string) => sent.filter((s) => s.body["action"] === name);
  return { api, sent, actions };
}

function Where() {
  const location = useLocation();
  return <div data-testid="where">{location.pathname}</div>;
}

/** The top-bar entries the screens claimed, in order (`navEntry.tsx`). */
let navClaims: string[] = [];
const navSlot = {
  claim(to: string) {
    navClaims.push(to);
    return () => undefined;
  },
};

function renderAt(api: ApiClient, path: string, pollMs = 60_000) {
  navClaims = [];
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <PromptsProvider>
          <NavEntryProvider slot={navSlot}>
            <MemoryRouter initialEntries={[path]}>
              <Routes>
                <Route path="/runbooks" element={<RunbooksScreen api={api} />} />
                <Route path="/workspaces/:workspace_name/runbooks" element={<RunbooksScreen api={api} />} />
                <Route path="/runbooks/run/:run_key" element={<RunScreen api={api} pollMs={pollMs} />} />
                <Route path="/workspaces/:workspace_name/home" element={<p>IDE</p>} />
              </Routes>
              <Where />
            </MemoryRouter>
          </NavEntryProvider>
        </PromptsProvider>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

const workspaceRows = {
  raw_query: { body: { rows: [["jets_ws", "uri", "jets_ai"]] } },
  get_workspace_uri: { body: { workspace_name: "jets_ws" } },
};

const healthDoc = {
  format: 1,
  runbook: "example_health_check",
  description: "Checks the deployment",
  parameters: {
    since_utc: { type: "timestamp", description: "from when", default: "2026-11-06T17:00:00Z" },
    max_sessions: { type: "integer" },
    dry_run: { type: "boolean", default: true },
    session_ids: { type: "list<session_id>" },
  },
  steps: [
    { id: "count_sessions", type: "query", description: "Counts sessions", returns: { n: "integer" } },
    { id: "hold", type: "pause", message: "Check the counts before going on." },
    { id: "make_scratch", type: "ddl", preview: true },
    { id: "start_it", type: "start_pipeline", row: { process_name: "p1" } },
    { id: "rebuild", type: "compile", deadline: "15m" },
    {
      id: "loop",
      type: "repeat",
      steps: [{ id: "confirm_box", type: "manual", instruction: "Look at the box and record what it says.", records: { reading: "integer", note: "text" } }],
    },
  ],
};

function step(over: Partial<StepRun> & Pick<StepRun, "step_key" | "sequence" | "step_id" | "type" | "status">): StepRun {
  return { iteration: 0, revision: 1, ...over };
}

function runWith(status: Run["status"], steps: StepRun[], over: Partial<Run> = {}): Run {
  return {
    run_key: 12,
    workspace_name: "jets_ws",
    runbook: "example_health_check",
    status,
    parameters: { since_utc: "2026-11-06T17:00:00Z" },
    revisions: [{ revision: 1, files: { "example_health_check.rb.json": "sha256:abc" }, from_step: "count_sessions" }],
    environment: { jetstore_release: "4.1.3", workspace_version: "v42" },
    started_by: "engineer@example.com",
    started_at: "2026-11-06T18:00:00Z",
    steps,
    ...over,
  };
}

const queryDone = step({
  step_key: 101,
  sequence: 1,
  step_id: "count_sessions",
  type: "query",
  status: "completed",
  sql: "SELECT count(*) AS n FROM jetsapi.session_registry WHERE created_at >= :since_utc",
  sql_hash: "sha256:q1",
  bound: { since_utc: "2026-11-06T17:00:00Z" },
  outputs: { n: 3 },
  rows: [{ n: 3 }, { n: 4 }],
  row_count: 1500,
  truncated: true,
  actor: "engineer@example.com",
  started_at: "2026-11-06T18:00:00Z",
  finished_at: "2026-11-06T18:00:01Z",
});

const ddlAwaiting = step({
  step_key: 103,
  sequence: 2,
  step_id: "make_scratch",
  type: "ddl",
  status: "awaiting_confirmation",
  sql: "CREATE TABLE scratch.counts AS SELECT :n::int AS n;\nDELETE FROM scratch.counts WHERE n < :floor;",
  sql_hash: "sha256:d1",
  bound: { n: 3, floor: 1 },
  preview: [{ statement: 1, command_tag: "SELECT 1" }, "DELETE 0"],
});

const sessionsScript = () => ({
  ...workspaceRows,
  get_runbook: { body: { runbook: healthDoc, findings: [], sql_files: {} } },
});

describe("the api client", () => {
  it("posts every action to /runbook in snake_case and unwraps the run", async () => {
    const run = runWith("running", []);
    const { api, sent } = await clientWith({
      accept: { body: { run } },
      deny: { body: { run } },
      continue: { body: { run } },
      skip: { body: { run } },
      retry: { body: { run } },
      abort: { body: { run } },
      record: { body: { run } },
      start_run: { body: { run } },
      get_run: { body: { run } },
      list_runs: { body: { runs: [run] } },
      release: { body: { result: { format: 1, runbook: "r", workspace: "jets_ws", run_key: 12, status: "completed" } } },
      export: { body: { result: { format: 1, runbook: "r", workspace: "jets_ws", run_key: 12, status: "completed" } } },
    });
    const rb = new RunbooksApi(api);
    expect((await rb.startRun("jets_ws", "r", { a: 1 })).run_key).toBe(12);
    await rb.decide("accept", 12, 7, "looks right");
    await rb.decide("deny", 12, 7, "  ");
    await rb.control("skip", 12, 7);
    await rb.control("abort", 12);
    await rb.record(12, 7, { reading: 4 });
    expect((await rb.listRuns("jets_ws", "r", 5)).length).toBe(1);
    expect((await rb.release(12)).run_key).toBe(12);
    expect((await rb.exportResult(12)).status).toBe("completed");

    expect(sent.every((s) => s.path === "/runbook")).toBe(true);
    const by = (a: string) => sent.filter((s) => s.body["action"] === a).map((s) => s.body);
    expect(by("start_run")[0]).toEqual({ action: "start_run", workspace_name: "jets_ws", runbook: "r", parameters: { a: 1 } });
    expect(by("accept")[0]).toEqual({ action: "accept", run_key: 12, step_key: 7, comment: "looks right" });
    // A blank comment is not sent, and no decision carries an actor.
    expect(by("deny")[0]).toEqual({ action: "deny", run_key: 12, step_key: 7 });
    expect(by("skip")[0]).toEqual({ action: "skip", run_key: 12, step_key: 7 });
    expect(by("abort")[0]).toEqual({ action: "abort", run_key: 12 });
    expect(by("record")[0]).toEqual({ action: "record", run_key: 12, step_key: 7, values: { reading: 4 } });
    expect(by("list_runs")[0]).toEqual({ action: "list_runs", workspace_name: "jets_ws", runbook: "r", limit: 5 });
  });

  it("refuses an answer with no run in it rather than rendering nothing", async () => {
    const { api } = await clientWith({ get_run: { body: {} } });
    await expect(new RunbooksApi(api).getRun(1)).rejects.toThrow(/without a run/);
  });
});

describe("parseTyped", () => {
  it("parses each declared type, and refuses what does not fit", () => {
    expect(parseTyped("integer", "42")).toEqual({ value: 42 });
    expect(parseTyped("integer", "4.2")).toHaveProperty("error");
    expect(parseTyped("numeric", "4.2")).toEqual({ value: 4.2 });
    expect(parseTyped("boolean", true)).toEqual({ value: true });
    expect(parseTyped("session_id", "1724000")).toEqual({ value: "1724000" });
    expect(parseTyped("session_id", "17a")).toHaveProperty("error");
    expect(parseTyped("timestamp", "yesterday")).toHaveProperty("error");
    expect(parseTyped("text", " kept as typed ")).toEqual({ value: " kept as typed " });
    expect(parseTyped("list<integer>", "1\n2\n\n3")).toEqual({ value: [1, 2, 3] });
    expect(parseTyped("list<integer>", "1\nx")).toHaveProperty("error");
  });
});

describe("polling and the frontier", () => {
  it("polls while the executor works, and not while a person is asked or the run is paused", () => {
    expect(shouldPoll(runWith("running", [step({ step_key: 1, sequence: 1, step_id: "w", type: "wait", status: "waiting" })]))).toBe(true);
    expect(shouldPoll(runWith("running", [ddlAwaiting]))).toBe(false);
    expect(shouldPoll(runWith("paused", []))).toBe(false);
    expect(shouldPoll(runWith("completed", []))).toBe(false);
  });

  // The server's frontierStep: the last step that is not final, else the last
  // one. Every control names this step, and the server refuses any other.
  it("takes the server's frontier as the current step", () => {
    const hold = step({ step_key: 102, sequence: 2, step_id: "hold", type: "pause", status: "awaiting_input" });
    expect(currentStep(runWith("paused", [queryDone, hold]))?.step_id).toBe("hold");
    // A step recorded pending is the frontier, not the step before it.
    const next = step({ step_key: 103, sequence: 3, step_id: "make_scratch", type: "ddl", status: "pending" });
    expect(currentStep(runWith("running", [queryDone, { ...hold, status: "completed" }, next]))?.step_id).toBe("make_scratch");
    // A repeat waiting between iterations is the frontier, though its body's
    // rows come after it.
    const loop = step({ step_key: 104, sequence: 1, step_id: "loop", type: "repeat", status: "waiting" });
    const body = step({ step_key: 105, sequence: 2, step_id: "confirm_box", type: "manual", status: "completed" });
    expect(currentStep(runWith("running", [loop, body]))?.step_id).toBe("loop");
    // A repeat running its body is not: the body's step is.
    expect(currentStep(runWith("paused", [{ ...loop, status: "running" }, { ...body, status: "awaiting_input" }]))?.step_id).toBe("confirm_box");
    // Every step final: the last one.
    expect(currentStep(runWith("completed", [queryDone, { ...hold, status: "completed" }]))?.step_id).toBe("hold");
    expect(currentStep(runWith("running", []))).toBeNull();
  });
});

/**
 * The controls the server accepts in each state, from `resume`, `Accept`,
 * `Deny`, `Record` and `Abort` in `jets/datatable/runbook_actions.go`. A row
 * here that the screen offered and the server refused is the defect this
 * table exists for: a pause step was offered Record (409) and not Continue.
 */
describe("runControls", () => {
  type Want = Partial<Record<"continue" | "skip" | "retry" | "decide" | "record", boolean>>;
  const at = (status: StepRun["status"], type: StepRun["type"] = "query") =>
    step({ step_key: 200, sequence: 2, step_id: "s", type, status });
  const cases: Array<[string, Run, Want]> = [
    ["a pause step awaiting input", runWith("paused", [queryDone, at("awaiting_input", "pause")]), { continue: true, skip: true }],
    ["a manual step awaiting input", runWith("paused", [queryDone, at("awaiting_input", "manual")]), { skip: true, record: true }],
    ["a ddl step awaiting confirmation", runWith("paused", [queryDone, at("awaiting_confirmation", "ddl")]), { skip: true, decide: true }],
    ["a compile step awaiting confirmation", runWith("paused", [at("awaiting_confirmation", "compile")]), { skip: true, decide: true }],
    ["a failed step", runWith("paused", [queryDone, at("failed")]), { continue: true, skip: true, retry: true }],
    ["a denied step", runWith("paused", [queryDone, at("denied", "ddl")]), { continue: true, skip: true, retry: true }],
    ["an interrupted step", runWith("paused", [queryDone, at("interrupted", "ddl")]), { skip: true, retry: true }],
    ["a completed step on a paused run", runWith("paused", [queryDone]), { continue: true, skip: true }],
    ["a waiting step on a paused run", runWith("paused", [at("waiting", "wait")]), { skip: true }],
    ["a waiting step on a running run", runWith("running", [queryDone, at("waiting", "wait")]), { skip: true }],
    ["a running step", runWith("running", [queryDone, at("running")]), {}],
    ["a pending step", runWith("running", [queryDone, at("pending")]), {}],
    ["a run with no step yet", runWith("running", []), {}],
  ];
  for (const [name, run, want] of cases) {
    it(`at ${name}`, () => {
      const got = runControls(run);
      expect({
        continue: got.continue,
        skip: got.skip,
        retry: got.retry,
        decide: got.decide,
        record: got.record,
      }).toEqual({
        continue: want.continue ?? false,
        skip: want.skip ?? false,
        retry: want.retry ?? false,
        decide: want.decide ?? false,
        record: want.record ?? false,
      });
      expect(got.abort).toBe(true);
    });
  }
  for (const status of ["completed", "failed", "aborted"] as const) {
    it(`offers nothing on a run that is ${status}`, () => {
      const { step: _frontier, ...flags } = runControls(runWith(status, [queryDone, at("failed")]));
      expect(Object.values(flags).some(Boolean)).toBe(false);
    });
  }
});

describe("the runbooks screen", () => {
  it("says why, and sends nothing, without the capability", async () => {
    const { api, sent } = await clientWith({}, []);
    renderAt(api, "/runbooks");
    await screen.findByText(/do not hold the runbook_execute capability/);
    expect(sent.length).toBe(0);
  });

  it("lists runbooks with validity, findings and last run, and the recent runs", async () => {
    const { api, actions } = await clientWith({
      ...workspaceRows,
      list_runbooks: {
        body: {
          runbooks: [
            { name: "example_health_check", description: "Checks the deployment", valid: true, findings: [], last_run: { run_key: 12, status: "paused", started_at: "2026-11-06T18:00:00Z" } },
            { name: "broken_one", valid: false, findings: [{ severity: "error", code: "ddl_confirm", message: "confirm: false is not allowed", path: "/steps/2/confirm" }] },
          ],
        },
      },
      list_runs: { body: { runs: [runWith("completed", [], { run_key: 11, finished_at: "2026-11-06T17:59:00Z" })] } },
    });
    renderAt(api, "/workspaces/jets_ws/runbooks");
    expect((await screen.findByRole("link", { name: "Open run 12" })).getAttribute("href")).toBe("/runbooks/run/12");
    await screen.findByText(/confirm: false is not allowed/);
    expect(screen.getByText("invalid")).toBeTruthy();
    expect((screen.getByRole("link", { name: "Open run 11" })).getAttribute("href")).toBe("/runbooks/run/11");
    expect(actions("list_runbooks")[0]!.body["workspace_name"]).toBe("jets_ws");
    // An invalid runbook cannot be started.
    expect((screen.getByRole("button", { name: "Start broken_one" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Start example_health_check" }) as HTMLButtonElement).disabled).toBe(false);
  });

  // The top bar would otherwise mark Workspaces, whose address the workspace
  // route is under.
  it("is reached from the Runbooks entry of the top bar, on each of its routes", async () => {
    const { api } = await clientWith({
      ...workspaceRows,
      list_runbooks: { body: { runbooks: [] } },
      list_runs: { body: { runs: [] } },
      ...sessionsScript(),
      get_run: { body: { run: runWith("completed", [queryDone]) } },
    });
    for (const at of ["/workspaces/jets_ws/runbooks", "/runbooks", "/runbooks/run/12"]) {
      renderAt(api, at);
      await waitFor(() => expect(navClaims).toContain("/runbooks"));
      expect(new Set(navClaims)).toEqual(new Set(["/runbooks"]));
      cleanup();
    }
  });

  it("chooses the deployment's workspace on the bare route, and the picker navigates", async () => {
    const { api, actions } = await clientWith({
      raw_query: { body: { rows: [["another_ws", "u", "b"], ["jets_ws", "u", "b"]] } },
      get_workspace_uri: { body: { workspace_name: "jets_ws" } },
      list_runbooks: { body: { runbooks: [] } },
      list_runs: { body: { runs: [] } },
    });
    renderAt(api, "/runbooks");
    await screen.findByText(/No runbooks in jets_ws/);
    expect(actions("list_runbooks")[0]!.body["workspace_name"]).toBe("jets_ws");
    fireEvent.change(screen.getByRole("combobox", { name: "Workspace" }), { target: { value: "another_ws" } });
    await waitFor(() => expect(screen.getByTestId("where").textContent).toBe("/workspaces/another_ws/runbooks"));
  });

  it("starts a run with the declared parameters, typed, and opens it", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      list_runbooks: { body: { runbooks: [{ name: "example_health_check", valid: true, findings: [] }] } },
      list_runs: { body: { runs: [] } },
      start_run: { body: { run: runWith("running", []) } },
      get_run: { body: { run: runWith("running", []) } },
    });
    renderAt(api, "/workspaces/jets_ws/runbooks");
    fireEvent.click(await screen.findByRole("button", { name: "Start example_health_check" }));
    const form = await screen.findByRole("region", { name: "Start example_health_check" });
    // The default is filled in, and each field says its type.
    expect((within(form).getByLabelText(/since_utc/) as HTMLInputElement).value).toBe("2026-11-06T17:00:00Z");
    expect(within(form).getByText("list<session_id>")).toBeTruthy();
    fireEvent.change(within(form).getByLabelText(/max_sessions/), { target: { value: "25" } });
    fireEvent.change(within(form).getByLabelText(/session_ids/), { target: { value: "1724\n1725" } });
    fireEvent.click(within(form).getByRole("button", { name: "Start run" }));
    await waitFor(() => expect(screen.getByTestId("where").textContent).toBe("/runbooks/run/12"));
    expect(actions("start_run")[0]!.body).toEqual({
      action: "start_run",
      workspace_name: "jets_ws",
      runbook: "example_health_check",
      parameters: { since_utc: "2026-11-06T17:00:00Z", max_sessions: 25, dry_run: true, session_ids: ["1724", "1725"] },
    });
  });

  it("refuses a parameter that does not parse, and starts nothing", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      list_runbooks: { body: { runbooks: [{ name: "example_health_check", valid: true, findings: [] }] } },
      list_runs: { body: { runs: [] } },
    });
    renderAt(api, "/workspaces/jets_ws/runbooks");
    fireEvent.click(await screen.findByRole("button", { name: "Start example_health_check" }));
    const form = await screen.findByRole("region", { name: "Start example_health_check" });
    fireEvent.change(within(form).getByLabelText(/max_sessions/), { target: { value: "lots" } });
    fireEvent.click(within(form).getByRole("button", { name: "Start run" }));
    await screen.findByText("expected an integer");
    expect(actions("start_run").length).toBe(0);
  });
});

describe("the run screen", () => {
  it("shows the header and each step's outputs, rows and timings, and the SQL on demand", async () => {
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("completed", [queryDone], { finished_at: "2026-11-06T18:01:00Z" }) } } });
    renderAt(api, "/runbooks/run/12");
    await screen.findByText(/JetStore 4.1.3/);
    expect(screen.getByText(/workspace version v42/)).toBeTruthy();
    const card = screen.getByRole("listitem", { name: "Step count_sessions" });
    expect(within(card).getByText("count_sessions.n")).toBeTruthy();
    expect(within(card).getByText("Showing 2 of 1500 rows (capped).")).toBeTruthy();
    expect(within(card).getByText(/by engineer@example.com/)).toBeTruthy();
    // The declaration's description annotates the step once it has been read.
    await within(card).findByText("Counts sessions");
    expect(within(card).queryByText(/FROM jetsapi.session_registry/)).toBeNull();
    fireEvent.click(within(card).getByRole("button", { name: "Show SQL" }));
    expect(within(card).getByText(/FROM jetsapi.session_registry/)).toBeTruthy();
    expect(within(card).getByText(":since_utc")).toBeTruthy();
  });

  it("shows a ddl step's exact statement, every bound value and its preview beside Accept", async () => {
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [queryDone, ddlAwaiting]) } } });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm make_scratch" });
    // The statement is on screen, whole, without a click.
    expect(within(panel).getByLabelText("Statement").textContent).toBe(ddlAwaiting.sql);
    expect(within(panel).getByText(":n")).toBeTruthy();
    expect(within(panel).getByText(":floor")).toBeTruthy();
    expect(within(panel).getByText("3")).toBeTruthy();
    expect(within(panel).getByText("SELECT 1")).toBeTruthy();
    expect(within(panel).getByText("DELETE 0")).toBeTruthy();
    expect(within(panel).getByText(/sha256:d1/)).toBeTruthy();
    expect(within(panel).getByRole("button", { name: "Accept" })).toBeTruthy();
    expect(within(panel).getByRole("button", { name: "Deny" })).toBeTruthy();
    // Accept is offered nowhere else.
    expect(screen.getAllByRole("button", { name: "Accept" }).length).toBe(1);
  });

  it("accepts with the comment, and sends no actor", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, ddlAwaiting]) } },
      accept: {
        body: {
          run: runWith("running", [
            queryDone,
            { ...ddlAwaiting, status: "completed", decision: { by: "engineer@example.com", at: "2026-11-06T18:02:00Z", accepted: true, comment: "ok" } },
          ]),
        },
      },
    });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm make_scratch" });
    fireEvent.change(within(panel).getByRole("textbox"), { target: { value: "ok" } });
    fireEvent.click(within(panel).getByRole("button", { name: "Accept" }));
    await screen.findByText(/Accepted by engineer@example.com/);
    expect(actions("accept")[0]!.body).toEqual({ action: "accept", run_key: 12, step_key: 103, comment: "ok" });
  });

  it("denies, and shows the denial", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [ddlAwaiting]) } },
      deny: {
        body: {
          run: runWith("paused", [
            { ...ddlAwaiting, status: "denied", decision: { by: "engineer@example.com", at: "2026-11-06T18:02:00Z", accepted: false, comment: "wrong floor" } },
          ]),
        },
      },
    });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm make_scratch" });
    fireEvent.change(within(panel).getByRole("textbox"), { target: { value: "wrong floor" } });
    fireEvent.click(within(panel).getByRole("button", { name: "Deny" }));
    await screen.findByText(/Denied by engineer@example.com/);
    expect(actions("deny")[0]!.body).toEqual({ action: "deny", run_key: 12, step_key: 103, comment: "wrong floor" });
    // A denial leaves the run paused, where the step can be skipped.
    expect(screen.getByRole("button", { name: "Skip" })).toBeTruthy();
  });

  it("says what a start_pipeline step will insert before Accept", async () => {
    const start = step({ step_key: 104, sequence: 1, step_id: "start_it", type: "start_pipeline", status: "awaiting_confirmation", bound: { process_name: "p1", main_input_session_id: "1724" } });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [start]) } } });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm start_it" });
    expect(within(panel).getByText(/Accept starts a pipeline/)).toBeTruthy();
    expect(within(panel).getByText("main_input_session_id")).toBeTruthy();
    expect(within(panel).getByText("1724")).toBeTruthy();
  });

  it("says what a compile step will do before Accept", async () => {
    const compile = step({ step_key: 105, sequence: 1, step_id: "rebuild", type: "compile", status: "awaiting_confirmation" });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [compile]) } } });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm rebuild" });
    expect(within(panel).getByText(/Accept compiles workspace jets_ws/)).toBeTruthy();
    await within(panel).findByText(/for up to 15m/);
  });

  it("says what a put_schema_event step will put before Accept", async () => {
    const put = step({ step_key: 106, sequence: 1, step_id: "put_it", type: "put_schema_event", status: "awaiting_confirmation", bound: { event: '{"event":"x"}', key: "k1" } });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [put]) } } });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Confirm put_it" });
    expect(within(panel).getByLabelText("Schema event").textContent).toBe('{"event":"x"}');
    expect(within(panel).getAllByText("k1").length).toBeGreaterThan(0);
  });

  it("asks a manual step's instruction, and records its declared values typed", async () => {
    const manual = step({ step_key: 107, sequence: 1, step_id: "confirm_box", type: "manual", status: "awaiting_input" });
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [manual]) } },
      record: { body: { run: runWith("running", [{ ...manual, status: "completed", outputs: { reading: 7, note: "fine" } }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Record confirm_box" });
    await within(panel).findByText("Look at the box and record what it says.");
    fireEvent.change(await within(panel).findByLabelText(/reading/), { target: { value: "7" } });
    fireEvent.change(within(panel).getByLabelText(/note/), { target: { value: "fine" } });
    fireEvent.click(within(panel).getByRole("button", { name: "Record" }));
    await screen.findByText("confirm_box.reading");
    expect(actions("record")[0]!.body).toEqual({ action: "record", run_key: 12, step_key: 107, values: { reading: 7, note: "fine" } });
  });

  // The server records a pause step as awaiting input on a paused run, and
  // takes Continue or Skip on it; Record is refused (409), for a manual step only.
  it("at a pause step offers Continue and Skip, not Record, and links to the IDE", async () => {
    const hold = step({ step_key: 102, sequence: 2, step_id: "hold", type: "pause", status: "awaiting_input" });
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, hold]) } },
      continue: { body: { run: runWith("running", [queryDone, { ...hold, status: "completed" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    const panel = await screen.findByRole("region", { name: "Pause hold" });
    await within(panel).findByText("Check the counts before going on.");
    expect(screen.queryByRole("region", { name: "Record hold" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Record" })).toBeNull();
    expect(screen.getByRole("button", { name: "Skip" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.getByRole("button", { name: "Abort" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Edit future steps" }).getAttribute("href")).toBe("/workspaces/jets_ws/home");
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    await waitFor(() => expect(actions("continue").length).toBe(1));
    expect(actions("continue")[0]!.body).toEqual({ action: "continue", run_key: 12, step_key: 102 });
    await waitFor(() => expect(screen.queryByRole("button", { name: "Continue" })).toBeNull());
    expect(screen.queryByRole("region", { name: "Pause hold" })).toBeNull();
  });

  it("skips a pause step", async () => {
    const hold = step({ step_key: 102, sequence: 2, step_id: "hold", type: "pause", status: "awaiting_input" });
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, hold]) } },
      skip: { body: { run: runWith("running", [queryDone, { ...hold, status: "skipped" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    fireEvent.click(await screen.findByRole("button", { name: "Skip" }));
    await waitFor(() => expect(actions("skip").length).toBe(1));
    expect(actions("skip")[0]!.body).toEqual({ action: "skip", run_key: 12, step_key: 102 });
  });

  it("at a manual step offers Record and Skip, and not Continue", async () => {
    const manual = step({ step_key: 107, sequence: 2, step_id: "confirm_box", type: "manual", status: "awaiting_input" });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [queryDone, manual]) } } });
    renderAt(api, "/runbooks/run/12");
    await screen.findByRole("region", { name: "Record confirm_box" });
    expect(screen.getByRole("button", { name: "Skip" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.queryByRole("region", { name: "Pause confirm_box" })).toBeNull();
  });

  it("at a step awaiting confirmation offers Accept, Deny and Skip, and not Continue", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, ddlAwaiting]) } },
      skip: { body: { run: runWith("running", [queryDone, { ...ddlAwaiting, status: "skipped" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    await screen.findByRole("region", { name: "Confirm make_scratch" });
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Skip" }));
    await waitFor(() => expect(actions("skip").length).toBe(1));
    expect(actions("skip")[0]!.body).toEqual({ action: "skip", run_key: 12, step_key: 103 });
  });

  it("after a denial offers Continue, Skip and Retry, on the denied step", async () => {
    const denied = {
      ...ddlAwaiting,
      status: "denied" as const,
      decision: { by: "engineer@example.com", at: "2026-11-06T18:02:00Z", accepted: false },
    };
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, denied]) } },
      retry: { body: { run: runWith("running", [queryDone, denied]) } },
    });
    renderAt(api, "/runbooks/run/12");
    expect(await screen.findByRole("button", { name: "Continue" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Skip" })).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Confirm make_scratch" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(actions("retry").length).toBe(1));
    expect(actions("retry")[0]!.body).toEqual({ action: "retry", run_key: 12, step_key: 103 });
  });

  it("takes a wait out of a running run with Skip, and offers nothing else but Abort", async () => {
    const waiting = step({ step_key: 110, sequence: 2, step_id: "wait_load", type: "wait", status: "waiting", next_check_at: "2026-11-06T18:03:00Z" });
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("running", [queryDone, waiting]) } },
      skip: { body: { run: runWith("running", [queryDone, { ...waiting, status: "skipped" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    const skip = await screen.findByRole("button", { name: "Skip" });
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.getByRole("button", { name: "Abort" })).toBeTruthy();
    fireEvent.click(skip);
    await waitFor(() => expect(actions("skip").length).toBe(1));
    expect(actions("skip")[0]!.body).toEqual({ action: "skip", run_key: 12, step_key: 110 });
  });

  it("offers only Abort while a step runs", async () => {
    const running = step({ step_key: 111, sequence: 2, step_id: "count_again", type: "query", status: "running" });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("running", [queryDone, running]) } } });
    renderAt(api, "/runbooks/run/12");
    expect(await screen.findByRole("button", { name: "Abort" })).toBeTruthy();
    for (const name of ["Continue", "Skip", "Retry", "Accept", "Record"]) {
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
  });

  it("at a failed step offers Retry and Skip, each on that step", async () => {
    const failed = step({ step_key: 108, sequence: 2, step_id: "count_again", type: "query", status: "failed", error: "ERROR: canceling statement due to statement timeout" });
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [queryDone, failed]) } },
      retry: { body: { run: runWith("running", [queryDone, { ...failed, status: "running", error: null }]) } },
      skip: { body: { run: runWith("running", [queryDone, { ...failed, status: "skipped" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    await screen.findByText(/statement timeout/);
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(actions("retry").length).toBe(1));
    expect(actions("retry")[0]!.body).toEqual({ action: "retry", run_key: 12, step_key: 108 });

    cleanup();
    renderAt(api, "/runbooks/run/12");
    fireEvent.click(await screen.findByRole("button", { name: "Skip" }));
    await waitFor(() => expect(actions("skip").length).toBe(1));
    expect(actions("skip")[0]!.body).toEqual({ action: "skip", run_key: 12, step_key: 108 });
  });

  it("offers Retry on a step the restart sweep interrupted", async () => {
    const interrupted = step({ step_key: 109, sequence: 1, step_id: "make_scratch", type: "ddl", status: "interrupted" });
    const { api } = await clientWith({ ...sessionsScript(), get_run: { body: { run: runWith("paused", [interrupted]) } } });
    renderAt(api, "/runbooks/run/12");
    expect(await screen.findByRole("button", { name: "Retry" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Skip" })).toBeTruthy();
    // It may have taken effect: the server refuses Continue on it.
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
  });

  it("aborts a running run", async () => {
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: { body: { run: runWith("paused", [ddlAwaiting]) } },
      abort: { body: { run: runWith("aborted", [{ ...ddlAwaiting, status: "skipped" }]) } },
    });
    renderAt(api, "/runbooks/run/12");
    fireEvent.click(await screen.findByRole("button", { name: "Abort" }));
    await screen.findByRole("button", { name: "Release" });
    expect(actions("abort")[0]!.body).toEqual({ action: "abort", run_key: 12 });
    expect(screen.queryByRole("button", { name: "Abort" })).toBeNull();
  });

  it("releases only after the app's own confirmation, then offers the download", async () => {
    const done = runWith("completed", [queryDone], { finished_at: "2026-11-06T18:01:00Z" });
    let released = false;
    const { api, actions } = await clientWith({
      ...sessionsScript(),
      get_run: () => ({ body: { run: released ? { ...done, released_at: "2026-11-06T18:05:00Z", released_by: "engineer@example.com" } : done } }),
      release: () => {
        released = true;
        return { body: { result: { format: 1, runbook: "example_health_check", workspace: "jets_ws", run_key: 12, status: "completed" } } };
      },
    });
    renderAt(api, "/runbooks/run/12");
    expect(screen.queryByRole("button", { name: "Download .rbr.json" })).toBeNull();

    fireEvent.click(await screen.findByRole("button", { name: "Release" }));
    expect(await answerConfirm("Cancel")).toMatch(/Release run 12/);
    expect(actions("release").length).toBe(0);

    fireEvent.click(screen.getByRole("button", { name: "Release" }));
    await answerConfirm("OK");
    await screen.findByRole("button", { name: "Download .rbr.json" });
    expect(actions("release")[0]!.body).toEqual({ action: "release", run_key: 12 });
    expect(screen.getByText(/2026-11-06T18:05:00Z by engineer@example.com/)).toBeTruthy();
  });

  it("downloads a released run's result through export", async () => {
    const done = runWith("completed", [queryDone], { released_at: "2026-11-06T18:05:00Z", released_by: "engineer@example.com" });
    const result = { format: 1, runbook: "example_health_check", workspace: "jets_ws", run_key: 12, status: "completed" };
    const { api, actions } = await clientWith({ ...sessionsScript(), get_run: { body: { run: done } }, export: { body: { result } } });
    const created: Blob[] = [];
    URL.createObjectURL = vi.fn((b: Blob) => {
      created.push(b);
      return "blob:x";
    }) as unknown as typeof URL.createObjectURL;
    URL.revokeObjectURL = vi.fn();
    const clicks: string[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clicks.push(this.download);
    });
    renderAt(api, "/runbooks/run/12");
    expect(screen.queryByRole("button", { name: "Release" })).toBeNull();
    fireEvent.click(await screen.findByRole("button", { name: "Download .rbr.json" }));
    await waitFor(() => expect(clicks).toEqual(["example_health_check-12.rbr.json"]));
    expect(actions("export")[0]!.body).toEqual({ action: "export", run_key: 12 });
    // jsdom's Blob has no `text()`; FileReader is what it does implement.
    const text = await new Promise<string>((resolve) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.readAsText(created[0]!);
    });
    expect(JSON.parse(text)).toEqual(result);
  });

  it("re-reads a running run until it pauses, then stops", async () => {
    let reads = 0;
    const waiting = step({ step_key: 110, sequence: 1, step_id: "wait_load", type: "wait", status: "waiting", next_check_at: "2026-11-06T18:03:00Z" });
    const hold = step({ step_key: 111, sequence: 2, step_id: "hold", type: "pause", status: "awaiting_input" });
    const { api } = await clientWith({
      ...sessionsScript(),
      get_run: () => {
        reads += 1;
        return { body: { run: reads < 3 ? runWith("running", [waiting]) : runWith("paused", [{ ...waiting, status: "completed" }, hold]) } };
      },
    });
    renderAt(api, "/runbooks/run/12", 10);
    await screen.findByText(/next check 2026-11-06T18:03:00Z/);
    await screen.findByRole("button", { name: "Continue" });
    const settled = reads;
    expect(settled).toBe(3);
    await new Promise((r) => setTimeout(r, 60));
    expect(reads).toBe(settled);
  });

  it("says why, and sends nothing, without the capability", async () => {
    const { api, sent } = await clientWith({}, []);
    renderAt(api, "/runbooks/run/12");
    await screen.findByText(/do not hold the runbook_execute capability/);
    expect(sent.length).toBe(0);
  });
});

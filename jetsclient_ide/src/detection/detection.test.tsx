/**
 * @vitest-environment jsdom
 *
 * The Detection screen against a scripted `/agentic` (agentic_ai I-861), in the
 * shape of `incidents/incidents.test.tsx`: a real `ApiClient` over a fetch that
 * answers by action name, so what is asserted is what the screen sends and how
 * it renders the answer.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { DetectionScreen } from "./DetectionScreen";
import { groupByTask, inFlight, type DetectionRunRow } from "./api";

afterEach(cleanup);

type Reply = { status?: number; body: unknown };

async function clientWith(
  script: Record<string, Reply | ((req: Record<string, unknown>) => Reply)>,
  capabilities: string[] = ["agent_supervision"],
) {
  const sent: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Supervisor",
          user_email: "supervisor@example.com",
          is_admin: false,
          capabilities,
        }),
        { status: 200 },
      );
    }
    const req = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    sent.push(req);
    const entry = script[String(req["action"])];
    const reply = typeof entry === "function" ? entry(req) : entry;
    if (!reply)
      return new Response(JSON.stringify({ error: "no script" }), {
        status: 500,
      });
    return new Response(JSON.stringify({ token: "t1", ...(reply.body as object) }), {
      status: reply.status ?? 200,
    });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("supervisor@example.com", "pw");
  return { api, sent };
}

function NotificationProbe() {
  const { error, status } = useNotifications();
  return <div data-testid="banner">{error ?? status ?? ""}</div>;
}

function renderScreen(api: ApiClient) {
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <NotificationProbe />
        <MemoryRouter initialEntries={["/detection"]}>
          <Routes>
            <Route path="/detection" element={<DetectionScreen api={api} />} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

function row(over: Partial<DetectionRunRow>): DetectionRunRow {
  return {
    key: 1,
    taskRef: "arn:aws:ecs:us-east-1:1:task/c/aaa",
    command: "anomaly_detect",
    trigger: "schedule",
    requestedBy: "",
    status: "ok",
    since: "26h",
    requestedAt: "",
    startedAt: "2026-10-13T08:00:05Z",
    finishedAt: "2026-10-13T08:00:09Z",
    summary: null,
    errorMessage: "",
    ...over,
  };
}

const scheduled = [
  row({
    key: 1,
    summary: {
      workerRows: 40,
      anomaliesFound: 1,
      anomaliesWritten: 1,
      anomaliesAlreadyRecorded: 0,
      baselineRead: true,
    },
  }),
  row({
    key: 2,
    command: "shadow_triage",
    startedAt: "2026-10-13T08:00:12Z",
    finishedAt: "2026-10-13T08:00:15Z",
    summary: {
      runsClassified: 3,
      incidentsWritten: 1,
      hypothesesWritten: 4,
      alreadyRaised: 0,
      attestationRead: true,
      attestationHeld: true,
      census: {
        rows_lost_silently: { present: 1, absent: 2, notEvaluable: 0 },
      },
      attestation: "shadow-mode attestation: holds",
    },
  }),
];

const deployedList = (runs: DetectionRunRow[], taskStates = {}) => ({
  body: {
    deployed: true,
    scheduleHourUtc: "8",
    defaultSince: "26h",
    runs,
    taskStates,
  },
});

describe("groupByTask", () => {
  it("puts a task's two commands on one row, newest task first", () => {
    const older = row({
      key: 3,
      taskRef: "arn:old",
      startedAt: "2026-10-12T08:00:00Z",
    });
    const tasks = groupByTask([older, ...scheduled]);
    expect(tasks.map((t) => t.taskRef)).toEqual(["arn:aws:ecs:us-east-1:1:task/c/aaa", "arn:old"]);
    expect(tasks[0]?.detect?.key).toBe(1);
    expect(tasks[0]?.triage?.key).toBe(2);
    expect(tasks[0]?.at).toBe("2026-10-13T08:00:05Z");
  });

  it("names the requester and the manual trigger even from a row the command wrote", () => {
    const tasks = groupByTask([
      row({ trigger: "command", requestedBy: "" }),
      row({
        command: "shadow_triage",
        trigger: "manual",
        requestedBy: "kim@example.com",
        status: "requested",
      }),
    ]);
    expect(tasks[0]?.trigger).toBe("manual");
    expect(tasks[0]?.requestedBy).toBe("kim@example.com");
    expect(inFlight(tasks[0]!)).toBe(true);
  });
});

describe("DetectionScreen", () => {
  it("shows the schedule and a task's two results side by side", async () => {
    const { api } = await clientWith({
      list_detection_runs: deployedList(scheduled),
    });
    renderScreen(api);
    expect((await screen.findByTestId("detection-schedule")).textContent).toContain(
      "Scheduled daily at 08:00 UTC",
    );
    expect(await screen.findByText(/1 anomaly over 40 worker rows/)).toBeTruthy();
    expect(screen.getByText(/3 runs classified: 1 incident and 4 hypotheses written/)).toBeTruthy();
    expect(screen.getByTestId("attestation").textContent).toContain(
      "Shadow-mode attestation holds",
    );
    expect(screen.getByRole("button", { name: "Run detection now" }).hasAttribute("disabled")).toBe(
      false,
    );
  });

  it("opens the census and the attestation in a row of their own", async () => {
    const { api } = await clientWith({
      list_detection_runs: deployedList(scheduled),
    });
    renderScreen(api);
    const toggle = await screen.findByRole("button", {
      name: "Show census and attestation",
    });
    expect(screen.queryByText("rows_lost_silently")).toBeNull();
    fireEvent.click(toggle);
    expect(screen.getByText("rows_lost_silently")).toBeTruthy();
    expect(screen.getByText("shadow-mode attestation: holds")).toBeTruthy();
    expect(
      screen
        .getByRole("button", { name: "Hide census and attestation" })
        .getAttribute("aria-expanded"),
    ).toBe("true");
  });

  it("starts the task over the chosen window and reloads", async () => {
    let listed = 0;
    const { api, sent } = await clientWith({
      list_detection_runs: () => {
        listed++;
        return deployedList(scheduled);
      },
      run_detection: {
        body: { taskArn: "arn:aws:ecs:us-east-1:1:task/c/bbb", since: "720h" },
      },
    });
    renderScreen(api);
    await screen.findByText(/1 anomaly over 40 worker rows/);
    fireEvent.change(screen.getByLabelText("Window to judge"), {
      target: { value: "720h" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Run detection now" }));
    await waitFor(() => expect(sent.some((r) => r["action"] === "run_detection")).toBe(true));
    expect(sent.find((r) => r["action"] === "run_detection")?.["since"]).toBe("720h");
    expect((await screen.findByTestId("banner")).textContent).toContain(
      "Detection task started over the last 720h",
    );
    await waitFor(() => expect(listed).toBe(2));
  });

  it("shows what ECS says about a task that has not recorded its end, and does not offer a second run", async () => {
    const requested = [
      row({
        key: 5,
        taskRef: "arn:t9",
        trigger: "manual",
        requestedBy: "kim@example.com",
        status: "requested",
        startedAt: "",
        finishedAt: "",
        requestedAt: "2026-10-13T09:00:00Z",
      }),
      row({
        key: 6,
        taskRef: "arn:t9",
        command: "shadow_triage",
        trigger: "manual",
        requestedBy: "kim@example.com",
        status: "requested",
        startedAt: "",
        finishedAt: "",
        requestedAt: "2026-10-13T09:00:00Z",
      }),
    ];
    const { api } = await clientWith({
      list_detection_runs: deployedList(requested, {
        "arn:t9": {
          lastStatus: "STOPPED",
          stoppedReason: "CannotPullContainerError",
          exitCodes: {},
        },
      }),
    });
    renderScreen(api);
    expect((await screen.findByTestId("ecs-state")).textContent).toContain(
      "ECS: STOPPED — CannotPullContainerError",
    );
    expect(screen.getByText("kim@example.com")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Run detection now" }).hasAttribute("disabled")).toBe(
      true,
    );
  });

  it("says the task is not deployed and offers no run", async () => {
    const { api, sent } = await clientWith({
      list_detection_runs: {
        body: {
          deployed: false,
          scheduleHourUtc: "",
          defaultSince: "26h",
          runs: [],
          taskStates: {},
        },
      },
    });
    renderScreen(api);
    expect((await screen.findByTestId("detection-schedule")).textContent).toContain(
      "Not part of this deployment",
    );
    expect(screen.getByRole("button", { name: "Run detection now" }).hasAttribute("disabled")).toBe(
      true,
    );
    expect(screen.getByText("No detection run is recorded.")).toBeTruthy();
    expect(sent.some((r) => r["action"] === "run_detection")).toBe(false);
  });

  it("renders a failed command with its error", async () => {
    const { api } = await clientWith({
      list_detection_runs: deployedList([
        row({
          status: "failed",
          summary: null,
          errorMessage: "jetsapi.anomaly does not exist in this database",
        }),
      ]),
    });
    renderScreen(api);
    expect(await screen.findByText("jetsapi.anomaly does not exist in this database")).toBeTruthy();
    expect(screen.getByText("failed")).toBeTruthy();
  });

  it("sends nothing without the capability", async () => {
    const { api, sent } = await clientWith({}, []);
    renderScreen(api);
    expect(
      await screen.findByText(/You do not hold the agent_supervision capability/),
    ).toBeTruthy();
    expect(sent).toHaveLength(0);
  });
});

/**
 * @vitest-environment jsdom
 *
 * **C.5's exit condition: the screen renders and reaches the server, not that
 * the document parses.**
 *
 * I-104 is why this is a rendering test rather than a check over the form
 * document: a document-level check passes on a field nothing draws. So this
 * drives the screen the way `FlowRunner.test.tsx` drives a flow — everything
 * below `ApiClient` is real, and the only stub is `fetch`, answering as
 * `jets/apiserver/api_infer_server.go` does.
 *
 * Six cases, each of which would have caught a different real mistake:
 *
 *  - the buttons and fields are on screen at all, and Submit is disabled on an
 *    empty request (`enableOnlyWhenFormValid` against the `required` rule);
 *  - the status line is fetched on mount rather than left at its default, which
 *    is the `onLoadActionKey` the schema cannot express;
 *  - **Start and Stop are gated on the reported state** — the `enabledWhen` this
 *    task added to `FormActionSchema`, and the assertion that says the new member
 *    reaches a rendered button rather than merely parsing;
 *  - a macro click fills the request box, which is the whole point of the macro
 *    buttons and needs `syncWithFormState` — the second member this task added,
 *    and the one without which the screen silently does nothing;
 *  - a request that is not json is reported without a call, so a typo in the box
 *    does not reach the proxy;
 *  - every button is inert without the capability. Presentation only — the
 *    endpoint is the enforcement point — but eight buttons naming a capability
 *    is eight chances to omit one.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider } from "../shell/notifications";
import { InferServerAdmin } from "./InferServerAdmin";

afterEach(cleanup);

interface Call {
  action: string;
}

/**
 * The apiserver's `/inferServer`, as far as this screen can tell.
 *
 * `status` is only reported by the three lifecycle actions, which is the
 * distinction `carriesStatus` draws; the proxy actions answer with whatever
 * Ollama said.
 */
function server(state = "stopped") {
  const calls: Call[] = [];
  let current = state;
  const fetchImpl = vi.fn(async (_url: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
    const action = String(body["action"]);
    calls.push({ action });
    const status = {
      state: current,
      runningTasks: current === "running" ? 1 : 0,
      desiredTasks: current === "running" ? 1 : 0,
      instanceCount: current === "running" ? 1 : 0,
      desiredCapacity: current === "running" ? 1 : 0,
    };
    let answer: Record<string, unknown>;
    switch (action) {
      case "server_status":
        answer = { status };
        break;
      case "start_server":
        current = "running";
        answer = { changed: true, status: { ...status, state: "running" } };
        break;
      case "stop_server":
        current = "stopped";
        answer = { changed: true, status: { ...status, state: "stopped" } };
        break;
      default:
        answer = { models: [] };
    }
    // Every authenticated response carries a refreshed token; the client
    // consumes and strips it, and this screen must never render it.
    return new Response(JSON.stringify({ ...answer, token: "refreshed" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });
  return { calls, fetchImpl };
}

async function signedIn(fetchImpl: typeof fetch, capabilities = ["infer_server_admin"]) {
  const login = vi.fn(async () =>
    Response.json({ token: "t", user_email: "a@b.c", name: "A", capabilities }),
  );
  const api = new ApiClient("", (async (url: RequestInfo | URL, init?: RequestInit) =>
    String(url).endsWith("/login") ? login() : fetchImpl(url, init)) as typeof fetch);
  await api.login("a@b.c", "pw");
  return api;
}

function draw(api: ApiClient) {
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <InferServerAdmin api={api} />
      </NotificationsProvider>
    </ApiProvider>,
  );
}

const button = (name: RegExp | string) => screen.getByRole("button", { name });

describe("the Infer Server Admin screen", () => {
  it("draws the document's buttons and fields", async () => {
    const { fetchImpl } = server();
    draw(await signedIn(fetchImpl));

    for (const label of ["Start", "Stop", "Models", "Pull Model", "Model Details", "Delete", "Refresh", "Submit"]) {
      expect(button(label)).toBeTruthy();
    }
    expect(screen.getByLabelText("Request")).toBeTruthy();
    expect(screen.getByLabelText("Response")).toBeTruthy();
    // `enableOnlyWhenFormValid` against the `required` rule on the request.
    expect(button("Submit").hasAttribute("disabled")).toBe(true);
  });

  it("fetches the status on mount, which is the onLoadActionKey the schema cannot say", async () => {
    const { calls, fetchImpl } = server("running");
    draw(await signedIn(fetchImpl));

    await waitFor(() => {
      expect(calls.map((c) => c.action)).toEqual(["server_status"]);
    });
    const status = screen.getByLabelText("Infer Server") as HTMLInputElement;
    await waitFor(() => {
      expect(status.value).toBe("Status: running  (tasks 1/1, instances 1/1)");
    });
  });

  it("gates Start and Stop on the reported state — FormActionSchema.enabledWhen", async () => {
    const { fetchImpl } = server("running");
    draw(await signedIn(fetchImpl));

    // Running: Stop is live and Start is not.
    await waitFor(() => expect(button("Start").hasAttribute("disabled")).toBe(true));
    expect(button("Stop").hasAttribute("disabled")).toBe(false);

    fireEvent.click(button("Stop"));
    fireEvent.click(button("Stop it"));

    // Stopped: the two swap, which is the predicate being re-read on a render
    // the form-state write caused.
    await waitFor(() => expect(button("Start").hasAttribute("disabled")).toBe(false));
    expect(button("Stop").hasAttribute("disabled")).toBe(true);
  });

  it("a macro button fills the request box with a submittable envelope", async () => {
    const { calls, fetchImpl } = server();
    draw(await signedIn(fetchImpl));

    fireEvent.click(button("Models"));
    const request = screen.getByLabelText("Request") as HTMLTextAreaElement;
    await waitFor(() => {
      expect(JSON.parse(request.value)).toEqual({ action: "list_models", body: {} });
    });

    // Submittable with no editing, which is what the macro exists for.
    await waitFor(() => expect(button("Submit").hasAttribute("disabled")).toBe(false));
    fireEvent.click(button("Submit"));
    await waitFor(() => {
      expect(calls.map((c) => c.action)).toEqual(["server_status", "list_models"]);
    });
    const response = screen.getByLabelText("Response") as HTMLTextAreaElement;
    await waitFor(() => expect(JSON.parse(response.value)).toEqual({ models: [] }));
    // The refreshed token is consumed by the client and must not reach a box the
    // user can read and copy.
    expect(response.value).not.toContain("refreshed");
  });

  it("reports a request that is not json without calling the server", async () => {
    const { calls, fetchImpl } = server();
    draw(await signedIn(fetchImpl));
    await waitFor(() => expect(calls).toHaveLength(1));

    fireEvent.change(screen.getByLabelText("Request"), { target: { value: "not json" } });
    fireEvent.click(button("Submit"));

    const response = screen.getByLabelText("Response") as HTMLTextAreaElement;
    await waitFor(() => expect(response.value).toMatch(/not valid json/));
    expect(calls).toHaveLength(1);
  });

  it("disables every button for a user without the capability", async () => {
    const { fetchImpl } = server();
    draw(await signedIn(fetchImpl, ["workspace_ide"]));

    // Presentation only — `api_infer_server.go` is the enforcement point — but
    // all eight carry the capability in the document, so all eight are inert.
    for (const label of ["Start", "Stop", "Models", "Pull Model", "Model Details", "Delete", "Refresh", "Submit"]) {
      expect(button(label).hasAttribute("disabled")).toBe(true);
    }
  });
});

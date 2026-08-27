/**
 * A stopped action does not move the flow. I-29.
 *
 * **The entry was open for eight days as a deliberate reproduction.** `ufNext`
 * ran the state's action and advanced whatever came back, because the Dart logs
 * the error and carries on (`user_flow_actions.dart:62`) and the port's job was
 * to be the Flutter app. On `loadFilesUF` that meant a failed save showed the
 * user the next page and the flow ended looking successful with no loader
 * started.
 *
 * The reasoning expired with the Flutter app (track X). These tests are the
 * behaviour that replaced it, driven through `step` directly rather than through
 * a flow, because the interesting cases are the ones the corpus cannot reach:
 * only one of its 31 state actions can fail on data rather than on a server, and
 * `proofFlows.test.ts` covers that one end to end.
 */

import { describe, expect, it, vi } from "vitest";

import type { ActionResult } from "../actions/interpret";
import { FormState } from "../datatable/formState";
import { startAt, step, type StepRequest } from "./engine";
import type { UserFlow } from "./schema";

/** Two states and a transition, which is the least that can advance. */
const FLOW = {
  schemaVersion: 1,
  startAtKey: "first",
  states: {
    first: {
      formConfig: "firstForm",
      stateAction: "save",
      defaultNextState: "second",
    },
    second: { formConfig: "secondForm", isEnd: true },
  },
} as unknown as UserFlow;

function harness(result: ActionResult) {
  const events: string[] = [];
  const request: StepRequest = {
    flow: FLOW,
    position: startAt(FLOW),
    formState: new FormState(),
    group: 0,
    runStateAction: vi.fn(async () => {
      events.push("ran");
      return result;
    }),
    validate: () => true,
    exit: () => events.push("exit"),
  };
  return { request, events };
}

const failed: ActionResult = { completed: false, message: "the server said no" };
const silent: ActionResult = { completed: false, message: null };
const ok: ActionResult = { completed: true, message: null };

describe("ufNext", () => {
  it("advances when the action completes", async () => {
    const { request, events } = harness(ok);
    const result = await step("ufNext", request);
    expect(events).toEqual(["ran"]);
    expect(result.position.stateKey).toBe("second");
  });

  it("stays where it is when the action fails, and says why", async () => {
    const { request } = harness(failed);
    const result = await step("ufNext", request);
    expect(result.position.stateKey).toBe("first");
    expect(result.outcome).toBe("the server said no");
  });

  /**
   * **The half that a message could not express.** A `validate` step that does
   * not pass and a refused `confirm` both stop the action with `null`, so before
   * `ActionResult` the engine saw the same value it sees on success. Eleven of
   * the corpus's 31 state actions open with a `validate` step.
   *
   * The flow stays and says nothing: a user who declines a confirmation has not
   * made a mistake, and telling them off for it would be a worse answer than the
   * one this replaces.
   */
  it("stays, silently, when the action stops without a message", async () => {
    const { request } = harness(silent);
    const result = await step("ufNext", request);
    expect(result.position.stateKey).toBe("first");
    expect(result.outcome).toBeNull();
  });

  it("does not run the action at all when the form does not validate", async () => {
    const { request, events } = harness(ok);
    const result = await step("ufNext", { ...request, validate: () => false });
    expect(events).toEqual([]);
    expect(result.position.stateKey).toBe("first");
  });
});

describe("ufCompleted", () => {
  it("exits when the action completes", async () => {
    const { request, events } = harness(ok);
    const result = await step("ufCompleted", request);
    expect(events).toEqual(["ran", "exit"]);
    expect(result.finished).toBe(true);
  });

  /**
   * **Leaving is a move too**, which is the part of this fix that is easy to miss:
   * `ufCompleted` did not advance a state, it closed the flow, so "does not
   * advance" would not have covered it. A save that failed is not a reason to
   * shut the flow on the user and discard what they typed.
   */
  it("does not exit when the action fails", async () => {
    const { request, events } = harness(failed);
    const result = await step("ufCompleted", request);
    expect(events).toEqual(["ran"]);
    expect(result.finished).toBe(false);
    expect(result.outcome).toBe("the server said no");
  });
});

describe("ufContinueLater", () => {
  it("does not exit when the action fails", async () => {
    const { request, events } = harness(failed);
    const result = await step("ufContinueLater", request);
    expect(events).toEqual(["ran"]);
    expect(result.finished).toBe(false);
  });
});

describe("ufCancel", () => {
  /**
   * Unchanged, and asserted because it is the one exit that must *not* depend on
   * an action: cancelling runs nothing, so there is nothing to fail and nothing
   * to keep the user in a flow they asked to leave.
   */
  it("still leaves without running the action", async () => {
    const { request, events } = harness(failed);
    const result = await step("ufCancel", request);
    expect(events).toEqual(["exit"]);
    expect(result.finished).toBe(true);
  });
});

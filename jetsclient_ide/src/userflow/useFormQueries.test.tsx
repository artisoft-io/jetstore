/**
 * @vitest-environment jsdom
 *
 * Scheduling a form's named queries. Task I.2b.
 *
 * `formQueries.test.ts` covers the rules; what is asserted here is the part that
 * needs React: **how often a request is made.** The Dart spends two guards on
 * that question — `predicatePreviousValue` and `isKeyUpdated` — and this hook
 * spends one signature, so the cases that matter are the ones where a naive
 * effect would re-query and this one must not.
 */

import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import type { Form } from "./form";
import { useFormQueries } from "./useFormQueries";

const form = {
  queries: {
    orgs: {
      sql: "SELECT org FROM jetsapi.client_org_registry WHERE client = '{client}'",
      params: ["client"],
    },
    clients: { sql: "SELECT client FROM jetsapi.client_registry" },
  },
  rows: [[{ field: "spacer" }]],
  actions: [{ action: "ufNext", label: "Next" }],
} as unknown as Form;

function poster() {
  const calls: Record<string, string>[] = [];
  const post = async (payload: Record<string, unknown>) => {
    const map = payload["query_map"] as Record<string, string>;
    calls.push(map);
    const result_map: Record<string, unknown> = {};
    for (const name of Object.keys(map)) result_map[name] = [[`${name}-row`]];
    return { result_map };
  };
  return { calls, post };
}

describe("useFormQueries", () => {
  it("runs the queries that are ready and puts the rows in form state", async () => {
    const formState = new FormState();
    const { calls, post } = poster();
    const { result } = renderHook(() => useFormQueries(form, formState, post));

    await waitFor(() => expect(result.current.rows("clients")).toBeDefined());
    expect(calls).toEqual([{ clients: form.queries!["clients"]!.sql }]);
    // In form state, not in React state, so an escape handed only a `FormState`
    // can read it (`actions/escapes.ts`, `EscapeContext`).
    expect(formState.queryRows("clients")).toEqual([["clients-row"]]);
    expect(result.current.rows("orgs")).toBeUndefined();
  });

  it("runs the waiting query once its parameter arrives", async () => {
    const formState = new FormState();
    const { calls, post } = poster();
    const { result } = renderHook(() => useFormQueries(form, formState, post));
    await waitFor(() => expect(result.current.rows("clients")).toBeDefined());

    await act(async () => {
      formState.setValue(0, "client", "acme");
      formState.notifyListeners();
    });
    await waitFor(() => expect(result.current.rows("orgs")).toBeDefined());
    expect(calls).toHaveLength(2);
    expect(calls[1]!["orgs"]).toContain("client = 'acme'");
  });

  it("does not re-query when a write leaves every statement unchanged", async () => {
    const formState = new FormState();
    const { calls, post } = poster();
    const { result } = renderHook(() => useFormQueries(form, formState, post));
    await waitFor(() => expect(result.current.rows("clients")).toBeDefined());

    // This is the case `isKeyUpdated` exists for in the Dart: the widget writes
    // its own value back and the listener fires. Nothing here reads `org`.
    await act(async () => {
      formState.setValue(0, "org", "eastern");
      formState.notifyListeners();
    });
    expect(calls).toHaveLength(1);
  });

  it("does not re-query when a parameter is set to the value it already had", async () => {
    // `predicatePreviousValue`'s job. Here it falls out of the signature rather
    // than needing a field of its own.
    const formState = new FormState();
    formState.setValue(0, "client", "acme");
    const { calls, post } = poster();
    const { result } = renderHook(() => useFormQueries(form, formState, post));
    await waitFor(() => expect(result.current.rows("orgs")).toBeDefined());
    expect(calls).toHaveLength(1);

    await act(async () => {
      formState.setValue(0, "client", "acme");
      formState.notifyListeners();
    });
    expect(calls).toHaveLength(1);
  });

  it("reports a failure and keeps the rows it already had", async () => {
    const formState = new FormState();
    let fail = false;
    const post = async (payload: Record<string, unknown>) => {
      if (fail) throw new Error("SQLSTATE 42P01");
      const map = payload["query_map"] as Record<string, string>;
      const result_map: Record<string, unknown> = {};
      for (const name of Object.keys(map)) result_map[name] = [["first"]];
      return { result_map };
    };
    const { result } = renderHook(() => useFormQueries(form, formState, post));
    await waitFor(() => expect(result.current.rows("clients")).toBeDefined());

    fail = true;
    await act(async () => {
      formState.setValue(0, "client", "acme");
      formState.notifyListeners();
    });
    await waitFor(() => expect(result.current.error).toContain("42P01"));
    // A failed batch is a banner, not a reset: the dropdown that was working goes
    // on working.
    expect(result.current.rows("clients")).toEqual([["first"]]);
  });

  it("does nothing at all for a form with no queries", async () => {
    const formState = new FormState();
    const { calls, post } = poster();
    const plain = { rows: [[{ field: "spacer" }]], actions: [] } as unknown as Form;
    const { result } = renderHook(() => useFormQueries(plain, formState, post));
    await act(async () => {});
    expect(calls).toEqual([]);
    expect(result.current.loading).toBe(false);
  });

  it("does nothing before a flow has loaded a form", async () => {
    const formState = new FormState();
    const { calls, post } = poster();
    renderHook(() => useFormQueries(null, formState, post));
    await act(async () => {});
    expect(calls).toEqual([]);
  });
});

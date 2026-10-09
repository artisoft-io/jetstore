/**
 * The negative suite, run where the schemas are authored. Task S.6.
 *
 * **The same file the Go tests drive** (`jets/userflow/negative_suite_test.go`),
 * for the reason both sides exist: the schema is authored here and enforced
 * there, so a hole opened by a TypeScript edit should fail before it reaches Go
 * rather than after. One suite, two runners, no second list of cases to drift.
 *
 * The rule, from the cpipes side: **a negative that validates is a hole, not a
 * pass.** The three `valid` cases are the other half — a suite whose bases do not
 * pass proves nothing, because everything would fail for the same uninteresting
 * reason.
 */

import { describe, expect, it } from "vitest";

import { ActionDocumentSchema } from "../actions/schema";
import { TableConfigDocumentSchema } from "../datatable/table";
import { FormDocumentSchema } from "./form";
import suite from "./negative_suite.json";
import { UserFlowSchema, type UserFlow } from "./schema";
import { validateFlow } from "./validate";

interface Case {
  name: string;
  class: string;
  document: "flow" | "action" | "form" | "table";
  expect: "valid" | "invalid";
  by?: "schema" | "reference";
  content: unknown;
}

const cases = (suite as { cases: Case[] }).cases;

const schemas = {
  flow: UserFlowSchema,
  action: ActionDocumentSchema,
  form: FormDocumentSchema,
  table: TableConfigDocumentSchema,
} as const;

/** Schema, then — for a flow — the reference checks the schema cannot express. */
function rejects(tc: Case): { rejected: boolean; by: "schema" | "reference" | null } {
  const parsed = schemas[tc.document].safeParse(tc.content);
  if (!parsed.success) return { rejected: true, by: "schema" };
  if (tc.document !== "flow") return { rejected: false, by: null };
  const errors = validateFlow(parsed.data as UserFlow).filter((f) => f.severity === "error");
  return errors.length > 0
    ? { rejected: true, by: "reference" }
    : { rejected: false, by: null };
}

describe("the negative suite", () => {
  it("has not quietly shrunk", () => {
    // A suite that gets smaller is a deletion, not a pass.
    expect(cases.length).toBeGreaterThanOrEqual(20);
    // **Six valid cases now, and the count alone stopped saying what it meant.**
    // It was five and it meant "the five bases are all here"; C.4 added a
    // *positive* case — a query table with no columns, which the server describes
    // — because a schema relaxation is a hole unless something asserts the thing
    // it now admits. So the bases are named rather than counted, and the count is
    // kept beside them to catch a deletion.
    expect(cases.filter((c) => c.expect === "valid").map((c) => c.name).sort()).toEqual([
      "a query table with no columns, which the server describes",
      "base actions",
      "base flow",
      "base form",
      "base query table",
      "base static table",
    ]);
  });

  it.each(cases.map((c) => [`${c.expect}: ${c.name}`, c] as const))("%s", (_label, tc) => {
    const result = rejects(tc);
    if (tc.expect === "valid") {
      expect({ [tc.name]: result.rejected }).toEqual({ [tc.name]: false });
      return;
    }
    expect({ [tc.name]: result.rejected }).toEqual({ [tc.name]: true });
    // A case the suite says the *schema* should catch must be caught by the
    // schema, not merely by something downstream of it.
    if (tc.by === "schema") {
      expect({ [tc.name]: result.by }).toEqual({ [tc.name]: "schema" });
    }
  });

  it("probes every class and every document type", () => {
    const classes = new Set(cases.map((c) => c.class));
    expect([...classes].sort()).toEqual([
      "inapplicable-field",
      "invented-field",
      "missing-required",
      "not-allowed",
      "reference",
      "sanity",
      "unknown-type",
      "value-range",
      "wrong-discriminator",
    ]);
    for (const doc of ["flow", "action", "form"] as const) {
      expect(cases.filter((c) => c.document === doc).length).toBeGreaterThanOrEqual(3);
    }
  });
});

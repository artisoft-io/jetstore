/**
 * The form-query rules. Task I.2b.
 *
 * Every case here is a claim about `jetsclient`'s behaviour, so each names the
 * Dart function it is a port of. The three that are *not* ports — the quoting,
 * the per-query readiness and the discarding of empty priority parts — are
 * marked as divergences, because a test that silently encodes a change reads
 * exactly like one that encodes a port.
 */

import { describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import type { Form } from "./form";
import {
  itemsFromRows,
  planQueries,
  quoteLiteral,
  resolveQuery,
  runQueries,
  suggestionsFor,
} from "./formQueries";

/** `fmMappingFormUF`'s three queries, shortened but with the real parameters. */
const mappingForm = {
  queries: {
    inputFields: {
      sql: "SELECT md.data_property FROM jetsapi.object_type_mapping_details md LEFT JOIN (SELECT * FROM jetsapi.process_mapping WHERE table_name = '{table_name}') pm ON true WHERE md.object_type = '{object_type}'",
      params: ["table_name", "object_type"],
    },
    inputColumns: {
      sql: "SELECT column_name FROM information_schema.columns WHERE table_name = '{table_name}'",
      params: ["table_name"],
    },
    mappingFunctions: { sql: "SELECT function_name FROM jetsapi.mapping_function_registry" },
  },
  rows: [[{ field: "spacer" }]],
  actions: [{ action: "ufCompleted", label: "Done" }],
} as unknown as Form;

describe("quoteLiteral", () => {
  it("doubles an embedded quote", () => {
    expect(quoteLiteral("O'Brien Health")).toBe("O''Brien Health");
  });

  it("leaves a backslash alone", () => {
    // Under `standard_conforming_strings`, on since Postgres 9.1 and assumed by
    // every other statement in this repository, a backslash in a literal is an
    // ordinary character. Escaping it would corrupt the value.
    expect(quoteLiteral("a\\b")).toBe("a\\b");
  });

  it("is a divergence from the Dart, which substitutes raw", () => {
    // `form.dart`'s `queryInputFieldItems` and `dropdown_form_field.dart`'s
    // `queryDropdownItems` both do `replaceAll(RegExp('{$key}'), value)`, so a
    // quote in a form-state value ends the literal and the rest is SQL. All four
    // substitution sites in the corpus land inside a single-quoted literal, which
    // is what makes doubling both correct and sufficient (I-72).
    const query = { sql: "SELECT org FROM r WHERE client = '{client}'", params: ["client"] };
    const formState = new FormState();
    formState.setValue(0, "client", "x' OR '1'='1");
    expect(resolveQuery(query, formState)).toBe(
      "SELECT org FROM r WHERE client = 'x'' OR ''1''=''1'",
    );
  });
});

describe("resolveQuery", () => {
  it("substitutes every occurrence of a parameter", () => {
    const formState = new FormState();
    formState.setValue(0, "table_name", "acme_org_claim");
    const sql = resolveQuery(mappingForm.queries!["inputFields"]!, formState);
    expect(sql).toBeNull(); // object_type is still missing
    formState.setValue(0, "object_type", "Claim");
    const resolved = resolveQuery(mappingForm.queries!["inputFields"]!, formState);
    expect(resolved).toContain("table_name = 'acme_org_claim'");
    expect(resolved).toContain("object_type = 'Claim'");
    expect(resolved).not.toContain("{");
  });

  it("treats an empty value as missing", () => {
    // `form.dart` tests `value == null`; a cleared widget stores nothing at all
    // (A.3, `useFormField`), so undefined and "" are the same state here.
    const formState = new FormState();
    formState.setValue(0, "table_name", "");
    expect(resolveQuery(mappingForm.queries!["inputColumns"]!, formState)).toBeNull();
  });

  it("takes the first element of a list-valued parameter", () => {
    // `dropdown_form_field.dart` does this (`value[0]`); `form.dart` means to and
    // substitutes into an empty local instead. Following the intent.
    const formState = new FormState();
    formState.setValue(0, "table_name", ["acme_org_claim", "other"]);
    expect(resolveQuery(mappingForm.queries!["inputColumns"]!, formState)).toContain(
      "'acme_org_claim'",
    );
  });

  it("runs a query with no parameters at once", () => {
    expect(resolveQuery(mappingForm.queries!["mappingFunctions"]!, new FormState())).toBe(
      mappingForm.queries!["mappingFunctions"]!.sql,
    );
  });
});

describe("planQueries", () => {
  it("runs what it can and waits for the rest — a divergence, per query", () => {
    // `form.dart` abandons the whole batch on the first missing predicate. Here
    // the parameterless query still runs. `fmMappingFormUF` is the only form in
    // the corpus with more than one query, so nothing observable changes.
    const formState = new FormState();
    const plan = planQueries(mappingForm, formState);
    expect(Object.keys(plan.ready)).toEqual(["mappingFunctions"]);
    expect(plan.waiting).toEqual(["inputColumns", "inputFields"]);
  });

  it("changes its signature when a parameter changes, and not otherwise", () => {
    const formState = new FormState();
    formState.setValue(0, "table_name", "acme_org_claim");
    formState.setValue(0, "object_type", "Claim");
    const first = planQueries(mappingForm, formState).signature;

    // A write to a key no query reads is what `isKeyUpdated` guards against in
    // the Dart; here it simply does not move the signature.
    formState.setValue(0, "input_column", "member_id");
    expect(planQueries(mappingForm, formState).signature).toBe(first);

    formState.setValue(0, "object_type", "Member");
    expect(planQueries(mappingForm, formState).signature).not.toBe(first);
  });

  it("is empty for a form with no queries", () => {
    const plan = planQueries({ rows: [], actions: [] } as unknown as Form, new FormState());
    expect(plan).toEqual({ ready: {}, waiting: [], signature: "[]" });
  });
});

describe("runQueries", () => {
  it("posts one raw_query_map for the whole batch", async () => {
    const posted: Record<string, unknown>[] = [];
    const rows = await runQueries(
      { ready: { a: "SELECT 1", b: "SELECT 2" }, waiting: [], signature: "s" },
      async (payload) => {
        posted.push(payload);
        return { result_map: { a: [["one"]], b: [] } };
      },
    );
    expect(posted).toEqual([
      { action: "raw_query_map", query_map: { a: "SELECT 1", b: "SELECT 2" } },
    ]);
    expect(rows).toEqual({ a: [["one"]], b: [] });
  });

  it("does not post when nothing is ready", async () => {
    let called = 0;
    const rows = await runQueries({ ready: {}, waiting: ["a"], signature: "" }, async () => {
      called += 1;
      return {};
    });
    expect(called).toBe(0);
    expect(rows).toEqual({});
  });

  it("reads a name the server omitted as empty rather than as missing", async () => {
    // `ExecRawQueryMap` fills every key it was given or fails the whole request,
    // so a gap is a shape this client did not expect — and an empty list is the
    // reading that keeps the form usable.
    const rows = await runQueries(
      { ready: { a: "SELECT 1" }, waiting: [], signature: "s" },
      async () => ({ result_map: {} }),
    );
    expect(rows).toEqual({ a: [] });
  });
});

describe("itemsFromRows", () => {
  it("takes column 0 as both value and label, as both Dart paths do", () => {
    expect(itemsFromRows([["trim"], ["to_upper"]])).toEqual([
      { value: "trim", label: "trim" },
      { value: "to_upper", label: "to_upper" },
    ]);
  });

  it("ignores a second column rather than offering it", () => {
    // `SELECT process_name, key FROM jetsapi.process_config` is read for its
    // second column by an action, through the rows, not through the dropdown.
    expect(itemsFromRows([["load", "17"]])).toEqual([{ value: "load", label: "load" }]);
  });

  it("drops a null or empty first column instead of offering a blank choice", () => {
    expect(itemsFromRows([[null], [""], ["ok"]])).toEqual([{ value: "ok", label: "ok" }]);
  });

  it("distinguishes a query that has not run from one that returned nothing", () => {
    expect(itemsFromRows(undefined)).toEqual([]);
    expect(itemsFromRows([])).toEqual([]);
  });
});

describe("suggestionsFor", () => {
  const columns = ["member_id", "MEMBER DOB", "claim_id", "paid_amount"];

  it("matches a substring, ignoring case and spaces", () => {
    // `doesMatch`: both sides lowercased and stripped of spaces.
    expect(suggestionsFor(columns, "memberdob", null)).toEqual(["MEMBER DOB"]);
    // `paid_amount` is here on purpose: "id" is a substring of "paid", and a
    // substring match is what the Dart does. The suggestion list is a help, not
    // a filter of what is legal — which is why membership is the validator's job.
    expect(suggestionsFor(columns, "ID", null)).toEqual(["member_id", "claim_id", "paid_amount"]);
  });

  it("does not apply the priority ordering while the user is typing", () => {
    // The Dart's non-empty branch has no priority half; the typed text is the
    // better signal once there is any.
    expect(suggestionsFor(columns, "id", "claim")).toEqual([
      "member_id",
      "claim_id",
      "paid_amount",
    ]);
  });

  it("floats the suggestions resembling the priority target, keeping the rest", () => {
    // `member:dob` splits to `member` and `dob`, so both member columns lead and
    // the other two follow in query order. Nothing is hidden.
    expect(suggestionsFor(columns, "", "member:dob")).toEqual([
      "member_id",
      "MEMBER DOB",
      "claim_id",
      "paid_amount",
    ]);
  });

  it("returns the items in query order when there is no target", () => {
    expect(suggestionsFor(columns, "", null)).toEqual(columns);
    expect(suggestionsFor(columns, "", "")).toEqual(columns);
  });

  it("discards empty parts of the target — the one divergence", () => {
    // The Dart splits without filtering, so a target ending in a separator yields
    // a part every suggestion contains and the whole list becomes priority, which
    // is indistinguishable from no priority at all. No corpus data property ends
    // in one, so this changes the rule and not the outcome.
    expect(suggestionsFor(columns, "", "claim_")).toEqual([
      "claim_id",
      "member_id",
      "MEMBER DOB",
      "paid_amount",
    ]);
  });
});

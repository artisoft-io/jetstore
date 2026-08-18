/**
 * Tests for the `UserFlowConfig` schema (task S.1).
 *
 * Three things are being checked, and they are not the same thing.
 *
 * 1. **The emitted JSON Schema matches the committed artifact.** That file is
 *    what Go reads at S.4; if it can drift from this source, the two enforcement
 *    points disagree and nothing says so. Regenerate with `UPDATE_SCHEMA=1`.
 * 2. **All eleven shipping flows validate.** The plan's rule: a real config that
 *    fails means the schema is wrong, not the config.
 * 3. **The schema rejects what it claims to reject.** A schema with only
 *    positive tests is one nobody has probed — the negative suite proper is
 *    S.6's, but the rules this task *added* over the Dart are asserted here,
 *    because a rule that cannot be shown to fire is not a rule.
 */

import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import corpus from "./fixtures/user_flows.json";
import { ConditionSchema, StateSchema, UserFlowSchema, emitJsonSchema } from "./schema";
import { toUserFlow, toUserFlows, type Corpus } from "./translate";
import { errorsOnly, validateFlow } from "./validate";

const artifactPath = fileURLToPath(new URL("./userflow.schema.json", import.meta.url));
const flowsDir = fileURLToPath(new URL("./flows/", import.meta.url));
const flows = toUserFlows(corpus as unknown as Corpus);

const documentOf = (key: string) => `${JSON.stringify(flows[key], null, 2)}\n`;

describe("the emitted JSON Schema", () => {
  it("matches the committed artifact", () => {
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") {
      writeFileSync(artifactPath, emitted);
    }
    expect(readFileSync(artifactPath, "utf8")).toBe(emitted);
  });

  it("has the eleven converted flows committed beside it, for the Go check", () => {
    // `jets/userflow/schema_test.go` reads both this directory and the emitted
    // schema, and asserts the same documents pass the Go validator that will
    // enforce them at save time. Committing the documents is what lets the two
    // languages be checked against one artifact rather than two readings of it.
    if (process.env.UPDATE_SCHEMA === "1") {
      mkdirSync(flowsDir, { recursive: true });
      for (const key of Object.keys(flows)) {
        writeFileSync(`${flowsDir}${key}.uf.json`, documentOf(key));
      }
    }
    const onDisk = readdirSync(flowsDir).filter((f) => f.endsWith(".uf.json")).sort();
    expect(onDisk).toEqual(Object.keys(flows).sort().map((k) => `${k}.uf.json`));
    for (const key of Object.keys(flows)) {
      expect(readFileSync(`${flowsDir}${key}.uf.json`, "utf8")).toBe(documentOf(key));
    }
  });

  it("is draft 2020-12, which is what santhosh-tekuri/jsonschema/v6 reads", () => {
    const emitted = emitJsonSchema() as Record<string, unknown>;
    expect(emitted["$schema"]).toBe("https://json-schema.org/draft/2020-12/schema");
  });

  it("closes every object except the states map, which is open by design", () => {
    // The failure this prevents is the quiet one: `defaultNexState` accepted,
    // ignored, and the flow dead-ending at run time. `states` is the single
    // exception and cannot be closed — its property *names* are the state keys —
    // so it constrains them with `propertyNames` instead, and its values with a
    // `$ref`. Asserting the exception by name is the point: a second open object
    // appearing later is a hole, not a design.
    const open: string[] = [];
    const walk = (node: unknown, path: string): void => {
      if (node === null || typeof node !== "object") return;
      const obj = node as Record<string, unknown>;
      if (obj["type"] === "object" && obj["additionalProperties"] !== false) open.push(path);
      for (const [key, value] of Object.entries(obj)) walk(value, `${path}.${key}`);
    };
    walk(emitJsonSchema(), "$");
    expect(open).toEqual(["$.properties.states"]);

    const states = (emitJsonSchema() as any).properties.states;
    expect(states.propertyNames.pattern).toBe("^[A-Za-z0-9_.-]+$");
    expect(states.additionalProperties.$ref).toBe("#/$defs/State");
  });
});

describe("the shipping corpus", () => {
  it("is the eleven flows and 46 states the app registers", () => {
    // Eleven, not the nine the documents said: `workspace_pull/` defines two
    // flows and `file_mapping/` defines two.
    expect(Object.keys(flows)).toHaveLength(11);
    expect(Object.values(flows).reduce((n, f) => n + Object.keys(f.states).length, 0)).toBe(46);
  });

  it.each(Object.keys(flows))("%s validates against the schema", (key) => {
    const result = UserFlowSchema.safeParse(flows[key]);
    expect(result.success ? [] : result.error.issues).toEqual([]);
  });

  it.each(Object.keys(flows))("%s has no reference errors", (key) => {
    expect(errorsOnly(validateFlow(flows[key]!))).toEqual([]);
  });

  it("uses four of the seven condition forms, and no operator outside two", () => {
    // The other three are carried for the reason A.3 carried its unused text
    // options: two lines each, and their absence is a silent behaviour change
    // the first time a flow wants one.
    const ops = new Set<string>();
    const walk = (c: unknown): void => {
      if (c === null || typeof c !== "object") return;
      const node = c as Record<string, unknown>;
      if (typeof node["op"] === "string") ops.add(node["op"]);
      for (const value of Object.values(node)) {
        if (Array.isArray(value)) value.forEach(walk);
        else walk(value);
      }
    };
    for (const flow of Object.values(flows)) {
      for (const state of Object.values(flow.states)) {
        if ("choices" in state && state.choices) state.choices.forEach((c) => walk(c.when));
      }
    }
    expect([...ops].sort()).toEqual(["contains", "equals", "isNullOrEmpty", "not"]);
  });

  it("carries every literal comparison as `value`, never as a state key", () => {
    // `isRhsStateKey` is false at all 17 sites, so `valueFromKey` is unexercised
    // by the corpus and is asserted separately below.
    const s = JSON.stringify(flows);
    expect(s).not.toContain("valueFromKey");
  });
});

describe("the rules the schema adds over the Dart validator", () => {
  const endState = { description: "d", formConfig: "f", isEnd: true } as const;

  it("rejects an end state that also transitions", () => {
    // `UserFlowState`'s doc comment says this "must" hold and
    // `validateConfiguration()` never checks it.
    expect(StateSchema.safeParse({ ...endState, defaultNextState: "x" }).success).toBe(false);
    expect(
      StateSchema.safeParse({
        ...endState,
        choices: [{ when: { op: "isNull", key: "k" }, nextState: "x" }],
      }).success,
    ).toBe(false);
  });

  it("rejects a non-end state with nowhere to go", () => {
    expect(StateSchema.safeParse({ description: "d", formConfig: "f" }).success).toBe(false);
    expect(StateSchema.safeParse({ description: "d", formConfig: "f", choices: [] }).success).toBe(
      false,
    );
  });

  it("rejects a comparison that gives both a literal and a key", () => {
    // The Dart's `isRhsStateKey` flag makes this state unrepresentable; two
    // named fields make it representable, so it has to be rejected.
    expect(
      ConditionSchema.safeParse({ op: "equals", key: "k", value: "v", valueFromKey: "o" }).success,
    ).toBe(false);
    expect(ConditionSchema.safeParse({ op: "equals", key: "k", valueFromKey: "o" }).success).toBe(
      true,
    );
  });

  it("rejects a nested condition that carries a nextState", () => {
    expect(
      ConditionSchema.safeParse({
        op: "not",
        condition: { op: "isNull", key: "k", nextState: "x" },
      }).success,
    ).toBe(false);
  });

  it("rejects an identifier with a space in it", () => {
    expect(UserFlowSchema.safeParse({ ...flows["loadFilesUF"], startAtKey: "a b" }).success).toBe(
      false,
    );
  });
});

describe("the reference checks the schema cannot express", () => {
  const flow = () => structuredClone(flows["loadFilesUF"]!);

  it("catches a start state that is not a state", () => {
    const f = flow();
    f.startAtKey = "nope";
    expect(validateFlow(f).map((x) => x.code)).toContain("unknownStartState");
  });

  it("catches a transition to a state that does not exist — the Dart's blind spot", () => {
    // `user_flow_actions.dart:82` does `states[nextStateKey]!.formConfig`, so
    // the Flutter failure mode is a crash on the Next button, not a message.
    const f = flow();
    const first = f.states[f.startAtKey]!;
    expect("defaultNextState" in first).toBe(true);
    (first as { defaultNextState: string }).defaultNextState = "typo";
    const findings = validateFlow(f);
    expect(findings.map((x) => x.code)).toContain("unknownTarget");
    expect(errorsOnly(findings)).toHaveLength(1);
  });

  it("finds nothing unreachable in any of the eleven flows", () => {
    // **This assertion is the reverse of the one S.1 shipped, and the reversal
    // is the point.** S.1 reported two states in `pipelineConfigUF` as dead on
    // the strength of a nearby comment. They are reached by a button — an action
    // that jumps the flow — which the walk could not see because the document
    // did not declare the edge. It does now, via `goToStates` (I-18).
    for (const key of Object.keys(flows)) {
      expect({ [key]: validateFlow(flows[key]!) }).toEqual({ [key]: [] });
    }
  });

  it("still reports a state that genuinely nothing reaches", () => {
    // The check has to keep working, or the correction above would have been a
    // way of making a failing test pass.
    const flow = structuredClone(flows["loadFilesUF"]!);
    flow.states["orphan"] = { description: "d", formConfig: "f", isEnd: true };
    expect(validateFlow(flow).map((f) => f.message)).toEqual([
      'state "orphan" is not reachable from "select_source_config"',
    ]);
  });

  it("counts a goToStates edge as reaching, and only the states it names", () => {
    const flow = structuredClone(flows["loadFilesUF"]!);
    flow.states["reached"] = { description: "d", formConfig: "f", isEnd: true };
    flow.states["orphan"] = { description: "d", formConfig: "f", isEnd: true };
    flow.states[flow.startAtKey]!.goToStates = ["reached"];
    expect(validateFlow(flow).map((f) => f.message)).toEqual([
      'state "orphan" is not reachable from "select_source_config"',
    ]);
  });

  it("treats a goToStates target that does not exist as an error", () => {
    // A declared edge is still a reference, and gets the same check as the rest.
    const flow = structuredClone(flows["loadFilesUF"]!);
    flow.states[flow.startAtKey]!.goToStates = ["typo"];
    expect(errorsOnly(validateFlow(flow)).map((f) => f.code)).toEqual(["unknownTarget"]);
  });

  it("does not bury an unknown start state under a flood of unreachables", () => {
    const f = flow();
    f.startAtKey = "nope";
    expect(validateFlow(f)).toHaveLength(1);
  });
});

describe("the conversion from the Dart corpus", () => {
  it("refuses a nested condition whose dead nextState was actually used", () => {
    // The schema's premise is that nested `nextState` is never read. If a flow
    // ever sets one, converting it would silently drop a value.
    expect(() =>
      toUserFlow("x", {
        startAtKey: "s",
        stateCount: 1,
        validationErrors: [],
        states: {
          s: {
            key: "s",
            description: "d",
            formConfig: "f",
            defaultNextState: "s",
            choices: [
              {
                type: "IsNotExpression",
                nextState: "s",
                expression: { type: "IsNullExpression", nextState: "used", lhsStateKey: "k" },
              },
            ],
          },
        },
      }),
    ).toThrow(/carries nextState "used"/);
  });

  it("refuses a flow whose initializer closure has no name", () => {
    expect(() =>
      toUserFlow("unnamedUF", {
        startAtKey: "s",
        hasFormStateInitializer: true,
        stateCount: 1,
        validationErrors: [],
        states: { s: { key: "s", description: "d", formConfig: "f", isEnd: true } },
      }),
    ).toThrow(/no name in the table/);
  });

  it("names homeFiltersUF's initializer rather than dropping it", () => {
    expect(flows["homeFiltersUF"]!.formStateInitializer).toBe("seedFromHomeFilters");
  });
});

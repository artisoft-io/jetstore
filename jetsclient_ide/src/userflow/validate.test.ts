/**
 * Tests for the reference checks and the reachability policy (task S.1).
 *
 * The reference checks themselves are exercised in `schema.test.ts`, against the
 * real flows. What is here is the **policy**, and the reason it has its own file
 * is that it has a counterpart: `jets/userflow/validate_test.go` runs the same
 * value table, and the two must agree. A deployment-time switch that means one
 * thing to the server and another to the client would be worse than no switch.
 */

import { describe, expect, it } from "vitest";

import corpus from "./fixtures/user_flows.json";
import { toUserFlows, type Corpus } from "./translate";
import {
  defaultPolicy,
  errorsOnly,
  isTruthy,
  policyFromEnv,
  strictPolicy,
  validateFlow,
} from "./validate";

const flows = toUserFlows(corpus as unknown as Corpus);
const VAR = "JETS_USERFLOW_STRICT_REACHABILITY";

describe("isTruthy", () => {
  // This table is duplicated in TestPolicyFromEnv in
  // jets/userflow/validate_test.go and must stay identical.
  it.each(["1", "true", "TRUE", "True", "yes", "YES", "on", "ON", " true ", "\ttrue\n"])(
    "%j enables strict reachability",
    (value) => expect(isTruthy(value)).toBe(true),
  );

  it.each(["", "0", "false", "no", "off", "2", "y", "t", "tru", "enabled", " "])(
    "%j leaves the warning a warning",
    (value) => expect(isTruthy(value)).toBe(false),
  );

  it("treats an unset variable as off, so a typo fails safe", () => {
    expect(policyFromEnv({})).toEqual(defaultPolicy);
    expect(policyFromEnv({ [VAR]: "enabled" })).toEqual(defaultPolicy);
    expect(policyFromEnv({ [VAR]: "1" })).toEqual(strictPolicy);
  });
});

describe("the reachability policy", () => {
  /** A flow with one state nothing reaches, since the corpus no longer has one. */
  const withOrphan = () => {
    const flow = structuredClone(flows["loadFilesUF"]!);
    flow.states["orphan"] = { description: "d", formConfig: "f", isEnd: true };
    return flow;
  };

  it("reports an unreached state as a warning by default", () => {
    const findings = validateFlow(withOrphan());
    expect(errorsOnly(findings)).toEqual([]);
    expect(findings).toHaveLength(1);
  });

  it("reports the same one as an error under the strict policy", () => {
    const lenient = validateFlow(withOrphan());
    const strict = validateFlow(withOrphan(), strictPolicy);
    expect(errorsOnly(strict)).toHaveLength(1);
    // Same findings, one field apart. A switch that changed *which* states were
    // reported would be a different check wearing the same name.
    expect(strict.map((f) => ({ ...f, severity: "warning" }))).toEqual(lenient);
  });

  it("refuses nothing in the shipping corpus, which is what changed", () => {
    // **S.1 asserted the opposite — that a strict deployment could not save
    // `pipelineConfigUF`.** That was true of the check, not of the flow: two of
    // its states are reached by a button and the document did not say so (I-18).
    // The switch is now free to turn on, which is the trade it should always
    // have been.
    const policy = policyFromEnv({ [VAR]: "true" });
    const refused = Object.keys(flows).filter(
      (key) => errorsOnly(validateFlow(flows[key]!, policy)).length > 0,
    );
    expect(refused).toEqual([]);
  });

  it("does not change the severity of a reference error", () => {
    const broken = structuredClone(flows["loadFilesUF"]!);
    broken.startAtKey = "nope";
    for (const policy of [defaultPolicy, strictPolicy]) {
      const findings = validateFlow(broken, policy);
      expect(findings).toHaveLength(1);
      expect(findings[0]!.code).toBe("unknownStartState");
      expect(findings[0]!.severity).toBe("error");
    }
  });
});

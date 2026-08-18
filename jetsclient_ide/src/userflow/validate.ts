/**
 * The rules a JSON Schema cannot state. Task S.1.
 *
 * `schema.ts` absorbs everything expressible in the document's *shape*,
 * including two rules the Dart validator either checks or only documents. What
 * is left here is reference integrity, which JSON Schema has no vocabulary for:
 * a `$ref` points at a schema, never at a sibling key's value.
 *
 * **S.4 ports this file to Go**, beside the schema check in
 * `SaveWorkspaceFileContent`. It is written to make that port mechanical — no
 * dependency on zod, plain data in and a list of findings out, and the same
 * shape of return the Dart's `validateConfiguration()` has
 * (`user_flow_config.dart:61`).
 *
 * ## What the Dart checks, and the one it misses
 *
 * `validateConfiguration()` checks three things: the start state exists, each
 * map key matches its state's `key` field, and a non-end state has somewhere to
 * go. The second disappears in this representation — a state has no `key` field
 * to disagree with, because `states` is keyed by it. The third is structural in
 * `schema.ts`.
 *
 * **It never checks that a transition names a state that exists**, and the
 * failure it lets through is not a diagnostic: `user_flow_actions.dart:82` does
 * `states[nextStateKey]!.formConfig` and the `!` turns a typo into a crash on
 * the button press. That check is `unknownTarget` below, and it is the one rule
 * here the Flutter app does not have.
 */

import type { UserFlow } from "./schema";

export type Severity = "error" | "warning";

export interface Finding {
  severity: Severity;
  /** Machine-readable so the Go port and the IDE can agree on a message. */
  code: "unknownStartState" | "unknownTarget" | "unreachableState";
  message: string;
}

/** Every state a state can transition to, choices first then the default. */
export function targetsOf(flow: UserFlow, stateKey: string): string[] {
  const state = flow.states[stateKey];
  if (state === undefined) return [];
  const targets = "choices" in state && state.choices ? state.choices.map((c) => c.nextState) : [];
  if ("defaultNextState" in state && state.defaultNextState !== undefined) {
    targets.push(state.defaultNextState);
  }
  return targets;
}

/**
 * Reference checks over a document that has already passed the schema.
 *
 * Order is deliberate: an unknown start state suppresses the reachability walk,
 * because a walk from nowhere reports every state as unreachable and buries the
 * one finding that matters.
 */
export function validateFlow(flow: UserFlow): Finding[] {
  const findings: Finding[] = [];
  const keys = Object.keys(flow.states);

  if (!(flow.startAtKey in flow.states)) {
    findings.push({
      severity: "error",
      code: "unknownStartState",
      message: `startAtKey "${flow.startAtKey}" is not a state`,
    });
  }

  for (const key of keys) {
    for (const target of targetsOf(flow, key)) {
      if (!(target in flow.states)) {
        findings.push({
          severity: "error",
          code: "unknownTarget",
          message: `state "${key}" transitions to "${target}", which is not a state`,
        });
      }
    }
  }

  if (flow.startAtKey in flow.states) {
    const reached = new Set<string>([flow.startAtKey]);
    const frontier = [flow.startAtKey];
    while (frontier.length > 0) {
      for (const target of targetsOf(flow, frontier.pop()!)) {
        if (!reached.has(target) && target in flow.states) {
          reached.add(target);
          frontier.push(target);
        }
      }
    }
    for (const key of keys) {
      if (!reached.has(key)) {
        findings.push({
          severity: "warning",
          code: "unreachableState",
          message: `state "${key}" is not reachable from "${flow.startAtKey}"`,
        });
      }
    }
  }

  return findings;
}

/**
 * **Unreachability is a warning, and the corpus is the reason.**
 *
 * `pipelineConfigUF` ships with two states nothing transitions to —
 * `add_merge_process_inputs` and `add_injected_process_inputs`, sitting under a
 * comment reading "new_process_input is implemented as a dialog". They are the
 * remains of an approach that was replaced, and the two dialog forms that
 * replaced them (`pcNewProcessInputDialog`, `pcNewProcessInputDialog4MI`) are
 * among the four registered forms no flow state references.
 *
 * So a reachability *error* would reject a configuration that has shipped for
 * years and works — which is precisely the case the plan's rule covers: a real
 * config that fails means the check is wrong, not the config. It stays as a
 * warning, where it is still worth having: a state nothing reaches is either
 * dead or a typo in the transition that should have reached it, and both are
 * worth a line in the IDE.
 *
 * A second entry point would change this analysis, and one exists — the
 * `startAtKey` navigation parameter (`screens/user_flow_screen.dart:105`) lets a
 * screen enter a flow anywhere. It does not rescue these two: the three screens
 * that use it name `select_client`, `select_source_config` and
 * `select_pipeline_config` (`modules/screen_config_impl.dart`). If entry points
 * ever become part of the document, this walk should start from all of them.
 */
export const errorsOnly = (findings: Finding[]): Finding[] =>
  findings.filter((f) => f.severity === "error");

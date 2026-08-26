/**
 * The one escape `/ruleConfig` needs. Task C.10.
 *
 * ## What the grammar cannot say, stated precisely
 *
 * `ruleConfigv2FormActions` writes `process_config_key` by looking the chosen
 * process name up in the rows the *process* dropdown's own query returned:
 * `processConfigCache.firstWhere((e) => e[0] == processName)` and then `row[1]`
 * (`jetsclient/lib/modules/actions/config_delegates.dart`,
 * `ruleConfigv2FormActions`). That is a lookup **into a query result**, and
 * `ValueSchema` has no member for one — `literal`, `fromKey`, `fromKeyList`,
 * `pgArrayFromKey` and `userEmail` all read form state or a constant
 * (`actions/schema.ts`, `ValueSchema`).
 *
 * **The alternative was a sixth `ValueSchema` member and it is worth saying why
 * not.** A `fromQueryRow` value would need a query name, a match column, a match
 * value and a result column — four fields for one site, and a vocabulary for
 * joining a query to form state that nothing else in either corpus asks for. This
 * project's answer to computation in a document is a named escape, and I-74's
 * rule says the escape count is an upper bound on what the grammar cannot say
 * rather than a target.
 *
 * ## `returnedModelCacheKey` is why this is possible at all
 *
 * The Dart caches the dropdown's rows under `FSK.processConfigCache` and the
 * delegate reads that cache. The form document deliberately dropped
 * `returnedModelCacheKey` — "the query's own name addresses them"
 * (`userflow/form.ts`, `FormQuerySchema`) — so this reads
 * `FormState.queryRows("processes")` instead. **Same rows, one name rather than
 * two**, and the cut is what makes the escape three lines instead of a second
 * lookup table.
 */

import type { EscapeContext } from "./escapes";

/** The `queries` entry whose second column is `process_config.key`. */
export const PROCESSES_QUERY = "processes";

export async function resolveProcessConfigKey(context: EscapeContext): Promise<string | null> {
  const { formState, group } = context;
  const held = formState.getValue(group, "process_name");
  const name = Array.isArray(held) ? held[0] : held;
  if (typeof name !== "string" || name === "") return null;
  const rows = formState.queryRows(PROCESSES_QUERY);
  const row = rows?.find((r) => r[0] === name);
  // **Silent when there is no match, and that is the deliberate half.** The Dart
  // reports *"can't find process_name in ruleConfigv2Cache"* and refuses; here the
  // only way to choose a process name is to pick one of that query's rows, so a
  // miss means the query has not run — which the `required` rule on the field
  // already refuses one step earlier. Reporting a second time would put a message
  // about a cache in front of a user who has not chosen a process.
  if (row === undefined) return null;
  const key = row[1];
  if (typeof key === "string" && key !== "") formState.setValue(group, "process_config_key", key);
  return null;
}

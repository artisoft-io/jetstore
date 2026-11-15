/**
 * The statements a `query` step may run.
 *
 * **One entry, and the corpus has exactly one site.** An earlier sizing reported the
 * `query` primitive at zero sites and was wrong: `pcAddPipelineConfigUF`
 * reaches it through `getProcessInputRdfTypes`
 * (`modules/actions/utils/get_process_info.dart`, `getProcessInputRdfTypes`), a
 * directory the grep did not cover. `pipelineConfigUF` is therefore the first and
 * so far only flow whose action document needs this, which is why `host.query` in
 * `FlowRunner.tsx` threw *"not registered in this build"* until now — it named
 * the gap and left it for the flow that would close it.
 *
 * **The document names the query and this file holds it**; `escapes.ts`
 * `NamedQuery` carries the reasoning, and `FormQuerySchema.sql` the opposite
 * view, which puts the statement in the document instead.
 */

import type { NamedQuery } from "./escapes";

/**
 * The process configuration behind a process name.
 *
 * **Two columns and the second is the one the flow is really after.**
 * `input_rdf_types::text` becomes `entity_rdf_type`, which is the where value of
 * three of this flow's data tables — `pcMainProcessInputKey`,
 * `pcMergedProcessInputKeys` and `pcInjectedProcessInputKeys` all filter
 * `process_input` by it (`pipeline_config/data_table_config.dart`). A flow that
 * skipped this step would show every data source of every entity type.
 *
 * **`process_name` is a parameter rather than an interpolation and the
 * difference matters.** The Dart builds the statement with `'$processName'`
 * inline; here the substitution goes through the same `{key}` mechanism a form's
 * queries use, so the value is quoted as a SQL literal
 * (`userflow/formQueries.ts`, `quoteLiteral`). The site lands inside a
 * single-quoted literal, which is where doubling the quote is both
 * correct and sufficient.
 */
export const processInputRdfTypes: NamedQuery = {
  sql:
    "SELECT key, input_rdf_types::text FROM jetsapi.process_config " +
    "WHERE process_name = '{process_name}'",
  params: ["process_name"],
  columns: ["key", "input_rdf_types"],
};

/** Every query this build registers, by the name a `query` step may use. */
export const productionQueries: Readonly<Record<string, NamedQuery>> = {
  processInputRdfTypes,
};

/**
 * The table configuration document. Task I.3, and the last piece of R2 that
 * Phase 2 left referenced rather than authored.
 *
 * ## What was missing
 *
 * S.5 gave a form a `dataTable` field carrying `table: Identifier` — "the table
 * configuration key; A.4's widget resolves it" (`userflow/form.ts`). Nothing
 * resolved it. The 37 configurations the flows use existed only as
 * `fixtures/table_configs.json`, generated out of the running Flutter app for
 * the query builder's tests, and `types.ts` mirrors the Dart class rather than
 * describing a document anyone could write. So a flow was authorable, its forms
 * were authorable, and the tables those forms render were still Dart.
 *
 * This file is the document that closes that. `table_configs/<key>.tc.json`,
 * validated in Go at save time like the other three.
 *
 * ## One table per document, and no self-key
 *
 * `.uf.json` is one flow; `.ua.json` and `.form.json` are records of many. This
 * follows the flow rather than the other two, and the corpus is why: **a table
 * configuration is shared and a form is not.** The form document could be a
 * record keyed inside its flow's file because 46 states have 46 distinct forms,
 * none shared (I-15). Tables are not like that — `wpClientList` and
 * `wpClientListRO` are each named by *two* flows, `loadConfigUF` and
 * `workspacePullUF`, and `pipelineExecStatusTable` is registered on the non-flow
 * side and rendered by `homeFiltersUF` (F18). A per-flow container would force
 * either a duplicated configuration or an ownership decision the configuration
 * does not support.
 *
 * So the file name is the key, which is S.3's rule for the flow document applied
 * to the same problem: two names for one thing is one name too many (I-14).
 *
 * ## Measured against the 37, not transcribed from the Dart
 *
 * Every field here is set by at least one of the 37 flow configurations, and
 * every field the 37 set is here. That cuts the mirror in `types.ts` down
 * considerably, and each cut is a count rather than a judgement:
 *
 * | Dropped | Why |
 * |---|---|
 * | `apiPath`, `apiAction` | `/dataTable` and `read` on all 37. See the note below — this is the security decision, not a tidy-up. |
 * | `index` on a column | equals the array position on all 275. A field that can only ever be wrong. |
 * | ~~`maxLines`, `columnWidth`~~ | `0` on all 275 columns — **and set by four non-flow ones; back as of C.7.** |
 * | `calculatedAs` | never set in the flows. |
 * | `defaultToAllRows`, ~~`requestColumnDef`~~ | `false` on all 37 — `requestColumnDef` back as of C.4. |
 * | ~~`sortColumnTableName`~~ | `""` on all 37 — **and set by eight non-flow ones, all the Workspace IDE's; back as of C.3.** |
 * | `lookupColumnInFormState` | `false` on all 49 where clauses. |
 * | `stateGroup` on an action | `0` on all 25. |
 * | `withClauses`, `distinctOnClauses`, `secondRowActions`, `fromConfigRowActions` | empty on all 37. |
 * | `sqlQuery`, `modelStateFormKey`, ~~`actionEnableCriterias`~~, `dataRowMinHeight`, `dataRowMaxHeight`, `predicate`/`ge`/`le` | never set in the flows. |
 * | `hasActionDelegate`, `hasModelStateHandler` | `false` on all 25 actions and all 37 tables. A table action reaches a delegate through `actionType: doAction` and `actionName`, not through this field. |
 *
 * **Four of those are set by the non-flow configurations and will come back**,
 * and knowing which is the point of having measured both corpora at once —
 * `withClauses` (1 table), `secondRowActions` (2), `modelStateFormKey` (2),
 * `hasModelStateHandler` (2), `actionEnableCriterias` (8 actions), `like` (2
 * clauses), `calculatedAs` (2 columns), `dataRowMin/MaxHeight` (1), and three
 * more `apiAction` values. Track C adds them as a screen needs them, which is
 * the discipline S.5 set for the form document and I.2a followed. ~~**The shape
 * here is chosen to admit them**: every one is an added optional property or an
 * added enum member, none needs a different union.~~
 *
 * **That last sentence is false and C.9 is where it broke.** `modelStateFormKey`
 * and `hasModelStateHandler` are named above as two of the fields that would come
 * back, and they need **a third arm on the union**: the three tables of
 * `viewReteTriplesDialogV2` carry `fromClauses: []` and no rows, and are fed from
 * form state rather than from a server or from the document
 * (`modules/data_table_config_impl.dart`, `reteSessionRdfTypeTable`,
 * `reteSessionEntityKeyTable`, `reteSessionEntityDetailsTable`). Forcing them into
 * `source: "query"` would mean relaxing `from`'s `min(1)`, which makes an existing
 * kind vaguer so a different kind fits inside it. `source: "formState"` is below.
 *
 * **This header has now been wrong twice in one day about the same kind of claim**
 * — C.7 found its list of nine returning fields short by four and replaced the
 * list with a derivation (**F66**, **I-173**); this is the sentence underneath that
 * list, and it fails the same way. Both are **predictions about the shape of future
 * work, written by the party who had not done it**, and neither was careless: the
 * fields were read off a corpus and the shape was reasoned from the fields. What a
 * field name cannot tell you is whether the thing it names is a *property* of a
 * table or a *kind* of table, and `modelStateFormKey` reads like the first.
 *
 * **~~none needs a different union~~ — that clause has now failed three times and
 * should be read as a hope rather than a property.** C.2's `enableWhen` needed a
 * new object schema; C.3's `rowHeight` needed a second one; and C.3's
 * `SchemaNameSchema` needed exactly the thing the clause says none of them would,
 * a union, because `$SCHEMA` is not a bare identifier. **The prediction was about
 * the shape of work its author had not done**, which is the same failure the
 * table below records about the *list* — and both halves were wrong for the same
 * reason, that they were derived from what the flow corpus does not set rather
 * than measured against the corpus that does. Left standing and corrected here
 * rather than edited away, per the register's convention.
 *
 * **F.5 brought back three and C.2 brings back two more, and that prediction is
 * worth scoring rather than quietly consuming.** `secondRowActions`,
 * `calculatedAs` and two enum members arrived with `pipelineExecStatusTable`;
 * `apiAction` and `actionEnableCriterias` — spelled `enableWhen` — arrive with
 * `workspaceRegistryTable`. The counts held exactly: the 8 criteria-bearing
 * actions are all on this one table, and the 2 tables with a second action row
 * are these two. **The "added optional property or added enum member" claim held
 * for four of the five**; `enableWhen` needed a new object schema as well, which
 * is not a different union and is more than the sentence promised.
 *
 * **C.4 brings back two the prediction did not name at all**, and that is the more
 * interesting score. `requestColumnDef` and an *absent* `columns` list are both on
 * `queryToolResultSetTable`, and neither is in the list above — the list was built
 * by asking which dropped fields the non-flow corpus *sets*, and a field it sets to
 * `false` on 24 of 25 reads as settled by that question. **The one it does set is
 * the whole of a screen**, and the empty column list was invisible to the question
 * entirely, because "the corpus sets this field" cannot notice a field the corpus
 * leaves empty on purpose. So: a prediction built from what the corpus *declares*
 * misses what it *omits*, which is I-20's limit arriving on the schema rather than
 * on a count.
 *
 * ~~What is left outstanding for the remaining 25 tables: `withClauses` (1 table),
 * `modelStateFormKey` and `hasModelStateHandler` (2), `like` (2 clauses),
 * `dataRowMin/MaxHeight` (1).~~ **C.9 takes the first three**, leaving `like`
 * (2 clauses), `dataRowMin/MaxHeight` (1) and `sortColumnTableName` (8 tables).
 * The derivation in `table.test.ts` is the list to trust; this one is prose.
 *
 * **That list is short by four, and C.7 found out by being refused.** Translating
 * `pipelineExecDetailsTable` threw on `maxLines` and `columnWidth`, neither of
 * which is named above. Re-measuring every dropped field against
 * `screens/fixtures/screen_configs.json` rather than against the nine remembered
 * ones gives the whole set:
 *
 * | Also set outside the flows | Sites | Whose screen |
 * |---|---|---|
 * | `maxLines`, `columnWidth` | 4 columns in 4 tables | C.6, C.7, C.9, C.3 |
 * | `sortColumnTableName` | 8 tables | C.3 (done), all Workspace IDE |
 * | `lookupColumnInFormState` | 1 clause | C.9 |
 * | `requestColumnDef` | 1 table | C.4 |
 *
 * The prediction was not careless — it was derived from what the *flow* corpus
 * leaves unset, and a field the flows set to a falsy default reads the same as a
 * field nothing uses. The lesson is that **a list of what will come back has to
 * be generated from the other corpus, not recalled from the reasons for the
 * cuts**, and the generated version is **F66**.
 *
 * ## `apiAction` is dropped rather than enumerated, and that is the security call
 *
 * A table configuration's `apiAction` goes straight into `DataTableAction.Action`
 * and dispatches over the whole switch in `jets/apiserver/api_tables.go:42` —
 * thirty-odd names including `exec_ddl`, `drop_table`, `insert_rows` and
 * `save_workspace_file_content`. An authored table naming one of those is the
 * confused-deputy problem S.7 described for the action grammar, arriving through
 * a second door: the person writing the file needs only `workspace_ide`.
 *
 * S.7 answered its version with an enum (`actions/schema.ts`, `ServerActionSchema`)
 * because flows genuinely use six actions. **All 37 tables use one**, so the
 * stronger answer is available here: the field does not exist, and a query table
 * issues `read`. Track C needs `workspace_read`, `preview_file` and
 * `raw_query_tool` for nine, one and one of its 28 — at which point this becomes
 * ~~a four-member enum~~ **a three-member enum whose absence means `read`**, with
 * the same reasoning S.7 wrote down, and never a bare identifier.
 *
 * **C.2 is that point, and the enum has three members rather than the four
 * predicted above.** The prediction is left standing and corrected here rather
 * than edited away. The two corpora hold **four** values between them —
 * `read`, `workspace_read`, `preview_file` and `raw_query_tool` — and the first
 * is the default. Admitting it as a member would give one meaning two spellings,
 * which is I-14's rule, and would rewrite all 37 committed documents to say the
 * thing they already say by saying nothing. So `ApiActionSchema` carries the
 * three exceptions and `fromDocument` restores `read` for a document that names
 * none.
 *
 * **The counts are asserted in `table.test.ts` rather than written here**, and
 * that is deliberate: an earlier draft of this paragraph carried them, and they
 * were four hours from being wrong — C.0a's fixture regeneration removes three
 * dead configurations and takes `read` from 17 to 14 without touching either
 * argument this enum rests on. A count in a comment is a claim nobody re-derives;
 * the same count in an assertion fails the moment the corpus moves. This is the
 * project's own "generate the measurement" rule applied to a sentence rather than
 * to a document.
 *
 * **The absence is a sentinel and that is a cost worth naming**, because this
 * file argues the other way about `source`: the Dart tells a static table from a
 * query table by `apiPath` being empty, and `source` replaced that sentinel with
 * a statement.
 *
 * **A sentinel is a problem when absence means something a reader must
 * distinguish, and here nothing distinguishes them.** `source` had to stop being
 * one because a document's kind is what *every* reader branches on, so absence
 * was load-bearing for control flow. `apiAction` is read by one thing at one
 * moment — `makeQuery` setting the request's `action` (`query.ts`) — and the
 * absent case and the `read` case take the same arm. That is the reason the cost
 * is acceptable, and it is written down because the paragraph above states the
 * cost without it and would otherwise read as a known defect.
 *
 * **The direction of the failure is the other half.** `read` is the *least*
 * privileged of the four, so a forgotten field cannot buy an authority: the worst
 * a mistake here produces is a table more restricted than its author meant, which
 * is a bug report rather than an incident. A sentinel standing for the permissive
 * value would not be defensible on any of these grounds.
 *
 * So: the three exceptions are on the page and the rule is off it, which makes a
 * security review of this field a reading rather than a count.
 *
 * ## Two kinds, because the corpus has exactly two
 *
 * Nine of the 37 carry a `staticTableModel` and issue nothing: no `fromClauses`,
 * no `whereClauses`, no actions, three columns, and rows compiled in. The other
 * 28 query and all set `fromClauses`. Splitting them structurally means a static
 * table cannot name a schema and a query table cannot carry rows, which is a
 * property neither the Dart class nor `types.ts` can state.
 *
 * The discriminant `source` is this file's invention, like `field` in the form
 * document. The Dart distinguishes the two by `apiPath` being empty, which is a
 * sentinel rather than a statement.
 *
 * **Only use constructs that emit.** Same rule as `userflow/schema.ts`:
 * `.refine()` is invisible to `z.toJSONSchema`, so it would be enforced in the
 * browser and silently absent in Go. Everything expressible structurally is.
 */

import { z } from "zod";

import type { EscapeReferences } from "../actions/escapes";
import { Identifier } from "../userflow/schema";

/**
 * A column of the table.
 *
 * **No `index`.** All 275 columns in the corpus have `index` equal to their
 * position in the list, which makes it a field that can only ever disagree with
 * the array holding it. This is S.1's reasoning about `nextState` on a nested
 * choice, on a second surface: a value that is always derivable is a value that
 * can be wrong.
 *
 * `table` qualifies the column when the query joins — 75 of 275 set it, and they
 * are the tables with more than one `fromClause` or a `joinWith`.
 *
 * ## Two arms, because a visible column must be labelled and a hidden one need not
 *
 * **22 of 275 columns carry an empty label, and all 22 are hidden.** The converse
 * does not hold — 41 of the 63 hidden columns *are* labelled — so this is not
 * "hidden implies unlabelled", it is "unlabelled implies hidden", holding on
 * 275 of 275. A union states it; a `label: z.string()` allowing the empty string
 * everywhere would let an authored table render a blank column header, which is
 * the failure the corpus happens never to contain.
 *
 * The first arm is any labelled column; the second is a hidden one that may omit
 * the label. A visible column with no label matches neither.
 */
const columnCommon = {
  name: Identifier,
  /** Qualifies `name` when more than one table is in scope. */
  table: Identifier.optional(),
  /** 226 of 275 set one; the empty string is how the Dart says "none". */
  tooltip: z.string().min(1).optional(),
  isNumeric: z.boolean().optional(),
  /**
   * A named entry in the escape registry, rendering the cell.
   *
   * Three columns in the corpus have one and all three are `file_key`
   * (`types.ts`), on `inputRegistryTable`, `main_input_registry_key` and
   * `merged_input_registry_keys`. A closure cannot be data; a name for one
   * can, which is what S.2a built the registry for.
   */
  cellFilter: Identifier.optional(),
  /**
   * A SQL expression the server selects in place of the column. Task F.5.
   *
   * **Never set by a flow's 37 tables and set by exactly one column of the first
   * non-flow table this project authored** — `run_duration` on
   * `pipelineExecStatusTable`, which is `AGE(last_update, start_time)` and has no
   * column behind it (`modules/data_table_config_impl.dart`, `run_duration`). The
   * query builder has always emitted it (`datatable/query.ts`, `makeQuery` sends
   * `calculatedAs` per selected column); what was missing was a way to author it,
   * which is the same one-sided gap I-62 described for `isReadOnly`.
   *
   * **It is a string the server splices into the select list, and that is worth
   * naming rather than leaving to the reader.** `apiAction` was dropped from this
   * document because an authored value reaches a switch in the apiserver (S.7's
   * confused deputy); this one reaches SQL. The two differ in that a table
   * configuration is workspace content edited by a user who already holds
   * `workspace_ide`, and that `calculatedAs` cannot name an action — but it is
   * the widest field in the document and the next person to widen this schema
   * should know it is here. Recorded as **I-105** — this comment said I-104 until
   * C.7, which is the entry about a rendering seam rather than about SQL.
   *
   * **C.7 settled whether it stays a fragment, and the answer is yes.** I-105
   * framed the second site as an argument for *naming* it, on I-54's analogy.
   * The analogy does not hold: I-54 collapsed six closures to two names because a
   * closure cannot be data at all, and `AGE(last_update, start_time)` already is.
   * What settles it is the server. `makeSelectColumns`
   * (`jets/datatable/data_table_action.go`, `makeSelectColumns`) writes
   * `CalculatedAs` into the select list verbatim while sanitising the plain
   * column beside it through `pgx.Identifier{…}.Sanitize()`, and `DoReadAction`
   * gates `read` on `CapabilityReadData` and nothing else — so any holder of
   * `read_data` can already post any expression here. A registry would move the
   * string into the bundle and leave the trust boundary exactly where it is,
   * at the cost of a third escape namespace. The gate that would matter is
   * server-side and is filed as **I-172**.
   * should know it is here. Recorded as **I-105**.
   */
  calculatedAs: z.string().min(1).optional(),
  /**
   * Truncate the cell to this many lines. Task C.7.
   *
   * **Zero on all 275 flow columns and set on four of the 151 non-flow ones** —
   * `error_message` on `inputLoaderStatusTable`, `pipelineExecDetailsTable` and
   * `processErrorsTable`, and `authored_label` on `wsJetRulesTable`. A stack
   * trace in a table cell is the case it exists for.
   *
   * **Not on the list of fields this file predicted would come back**, which is
   * the finding rather than the field: the header's table of nine was derived
   * from what the *flow* corpus does not set, and four more are set by the
   * non-flow one — this, `columnWidth`, `sortColumnTableName` (8 tables) and
   * `lookupColumnInFormState` (1 clause). **F66** carries the re-measurement.
   */
  maxLines: z.number().int().positive().optional(),
  /**
   * The cell's width in pixels. Task C.7.
   *
   * The same four columns, and in the Dart the two are not independent: the
   * width is a `SizedBox` *inside* the `maxLines > 0` branch
   * (`jetsclient/lib/components/data_table_source.dart`, the `cells:` map), so a
   * width without a line limit draws nothing there. All four set both, so the
   * schema states them separately and no configuration distinguishes them.
   */
  columnWidth: z.number().int().positive().optional(),
} as const;

export const ColumnSchema = z
  .union([
    z.strictObject({ ...columnCommon, label: z.string().min(1), isHidden: z.boolean().optional() }),
    z.strictObject({ ...columnCommon, label: z.string().min(1).optional(), isHidden: z.literal(true) }),
  ])
  .meta({ id: "Column", description: "One column of a table configuration" });

/**
 * A `FROM` entry.
 *
 * `tableName` may be empty, and that is not sloppiness: `types.ts` records that
 * an empty name means "resolve from form state or route params at query time",
 * which the query builder implements. It is spelled as an explicit absence here
 * so a document cannot say it by accident.
 *
 * **`schemaName` may be empty too, and it is the same kind of absence with a
 * different meaning — found at C.9 by being refused.** One clause in either
 * corpus has one: `inputRecordsFromProcessErrorTable`'s second from clause is
 * `FromClause(schemaName: '', tableName: 'sessions')`, and `sessions` is the
 * table's *own* `WITH` — a common table expression is not in a schema and naming
 * one would not resolve. The server has an explicit branch for it:
 * `makeFromClauses` writes `pgx.Identifier{Table}.Sanitize()` rather than
 * `{Schema, Table}` when the schema is empty
 * (`jets/datatable/data_table_action.go`, `makeFromClauses`).
 *
 * **So this arrived with `with` and not by coincidence.** A table that declares a
 * CTE is a table that has something unqualified to select from, which is why one
 * field's return brought the other's — and why "the corpus sets this field" would
 * never have predicted it: the corpus records the schema as the empty string,
 * which reads as unset.
 */
/**
 * The schema a `FROM` entry names, or the sentinel for the compiled workspace.
 *
 * Task C.3. **`$SCHEMA` is not a variable the client substitutes and not a
 * schema name; it is a statement that this query reads `workspace.db` rather
 * than Postgres.** `DoWorkspaceReadAction`
 * (`jets/datatable/workspace_data_table_action.go`, `DoWorkspaceReadAction`)
 * rewrites it to the **empty string** before building the query, because the
 * compiled artifact is a SQLite file opened directly and SQLite has no schemas.
 *
 * The comment above that loop says it replaces `$SCHEMA` "with the
 * workspace_name" and the line below it assigns `""`; the *note* beside it is
 * the accurate one. Read the assignment.
 *
 * **The other 37 configurations name `jetsapi` and would not notice this
 * existed**, which is why the sentinel is a member of this union rather than a
 * widening of `Identifier`: `$` is not a character a bare identifier may hold,
 * and admitting it everywhere to serve one placeholder would let a `.uf.json`
 * name a state `$foo`. Eight configurations use it and all eight are the
 * Workspace IDE's compiled views.
 */
export const SchemaNameSchema = z
  .union([Identifier, z.literal("$SCHEMA")])
  .meta({
    id: "SchemaName",
    description: "A database schema, or $SCHEMA for the compiled workspace",
  });

export const FromClauseSchema = z
  .strictObject({
    /**
     * Absent means "unqualified" — the Dart's empty `schemaName`.
     *
     * **`SchemaNameSchema` rather than `Identifier` as of C.3**, because eight
     * configurations name `$SCHEMA` and it is not a bare identifier. Still
     * optional: the two facts are independent, and collapsing them would make an
     * unqualified `FROM` unexpressible to serve a placeholder.
     */
    schema: SchemaNameSchema.optional(),
    /** Absent means "resolve at query time" — the Dart's empty `tableName`. */
    table: Identifier.optional(),
    as: Identifier.optional(),
  })
  .meta({ id: "FromClause" });

/**
 * A `WITH` entry. Task C.9.
 *
 * **One table in either corpus declares one**, and it is the reason
 * `inputRecordsFromProcessErrorTable` can show a domain key's input records at
 * all: the statement builds the set of session ids in the lookback window, and
 * the table's first where clause joins against it
 * (`modules/data_table_config_impl.dart`, the
 * `inputRecordsFromProcessErrorTable` entry).
 *
 * ## This is authored SQL in a document, and the question was asked before
 *
 * The same question `FormQuerySchema.sql` answered (I-71) and the same answer:
 * saving a workspace file requires `workspace_ide`
 * (`jets/datatable/workspace_data_table_action.go`, `SaveWorkspaceFileContent`)
 * and `workspace_ide` **is** `CapabilityQueryTool`
 * (`jets/datatable/data_table_action.go`, `CapabilityQueryTool`), so an author who
 * can write this file can already run the statement through `raw_query_tool`. The
 * document grants nobody authority they did not have.
 *
 * **It is a narrower grant than `sql` on a form query, and worth saying so.** A
 * `WITH` runs inside a `read`, which `DoReadAction` gates on `CapabilityReadData`
 * and which returns rows through the same paging the table already uses; a form
 * query names its own statement outright. This is the wider of the two fields on
 * this document — wider than `calculatedAs`, which is an expression — and the next
 * person to widen this schema should know both are here.
 *
 * `params` is the Dart's `stateVariables`: keys substituted as `{key}` into
 * `sql`, from the data table field's group, exactly as `makeQuery` already does
 * (`datatable/query.ts`).
 */
export const WithClauseSchema = z
  .strictObject({
    name: Identifier,
    sql: z.string().min(1),
    params: z.array(Identifier).min(1).optional(),
  })
  .meta({ id: "WithClause" });

/**
 * Where a form-state-backed table's rows come from. Task C.9.
 *
 * **Two shapes, and the second is two of the Dart's closures rather than a
 * language invented for them.** `modelStateFormKey` is the first: the rows *are*
 * the value under a form-state key. `modelStateHandler` is a Dart function
 * pointer, and both of the two that exist do the same thing —
 * `reteSessionEntityKeyStateHandler` and `reteSessionEntityDetailsStateHandler`
 * (`jetsclient/lib/modules/rete_session/model_handlers.dart`) each read a **map**
 * held under one key and index it by the current value of another.
 *
 * So the second shape is that lookup, stated as data. **The alternative was a
 * sixth escape namespace holding two functions**, and what decides against it is
 * I-54's own rule read the right way round: a closure has to become a name
 * *because a closure cannot be data*. These two already are — a key and an index
 * key — the way `AGE(last_update, start_time)` already was (**I-171**).
 *
 * **The reversal condition, because two sites is a thin base for a construct.** A
 * third handler that is not this lookup should be a named escape rather than a
 * third member here; that is the point at which the closures stop being data and
 * the namespace earns itself.
 */
export const ModelSourceSchema = z
  .discriminatedUnion("from", [
    z.strictObject({
      from: z.literal("key"),
      /** The form-state key holding the rows, read from group 0 as the Dart does. */
      key: Identifier,
    }),
    z.strictObject({
      from: z.literal("map"),
      /** The form-state key holding the map. */
      key: Identifier,
      /** The form-state key whose current value indexes it. */
      indexBy: Identifier,
    }),
  ])
  .meta({ id: "ModelSource", description: "Where a form-state table's rows come from" });

/**
 * A `WHERE` entry.
 *
 * Recursive through `orWith`, which two clauses in the corpus use. Zod needs the
 * explicit type annotation for a recursive object, and `z.toJSONSchema` emits it
 * as a `$ref` to the same definition.
 *
 * ~~**`lookupColumnInFormState` is not here**: `false` on all 49.~~ It is here as
 * of C.9 — one clause in either corpus sets it. **`predicate`, `ge` and `le` are
 * not here**: never set by a flow. ~~`like` is not here either, and is the last of
 * the four that track C brings back — two of its clauses use it.~~ **It is here as of
 * C.3b**, and the two clauses are that task's own — `wsDataModelFilesTable` and
 * `wsJetRulesFilesTable` are the tabs whose rows are the section's files, and a
 * prefix match is how each one selects its section out of one `workspace_control`
 * table.
 */
export interface WhereClauseDocument {
  column?: string;
  table?: string;
  formStateKey?: string;
  defaultValue?: string[];
  joinWith?: string;
  like?: string;
  lookupColumnInFormState?: true;
  orWith?: WhereClauseDocument;
}

export const WhereClauseSchema: z.ZodType<WhereClauseDocument> = z
  .strictObject({
    /**
     * The column the clause filters on. Task C.4.
     *
     * **Optional, because one clause of the corpus's fifty names none**, and it
     * is not an omission: `queryToolResultSetTable`'s only clause is
     * `WhereClause(column: '', formStateKey: FSK.queryReady)`
     * (`jetsclient/lib/modules/data_table_config_impl.dart`, the
     * `queryToolResultSetTable` entry). Nothing puts it in a request — a
     * raw-statement table's body carries no `whereClauses` at all — and its whole
     * effect is `hasBlockingFilter`, which reads `formStateKey` and never looks at
     * the column (`datatable/binding.ts`, `hasBlockingFilter`). So a column-less
     * clause is a **gate**: it says *do not query until this key is set*.
     *
     * Absent rather than an empty string, which is the same move `label` and
     * `from.table` make in this file: the Dart says "none" with a sentinel and the
     * document says it by omission. `makeWhereClause` drops such a clause rather
     * than emitting `column: ""` (`datatable/query.ts`), so a structured table
     * that grew one would filter on nothing rather than on a nameless column.
     */
    column: Identifier.optional(),
    table: Identifier.optional(),
    /** 35 of 49 read their value from form state. */
    formStateKey: Identifier.optional(),
    /** Used when the form state holds nothing; 4 of 49 declare one. */
    defaultValue: z.array(z.string()).min(1).optional(),
    /** `source_period.key` and seven others — a qualified column, not a key. */
    joinWith: z.string().min(1).optional(),
    /**
     * A `LIKE` pattern, matched against `column`. Task C.3b.
     *
     * **Two clauses in either corpus and both are one task's**, which is why this
     * was the last of the four the header above predicted: `wsDataModelFilesTable`
     * and `wsJetRulesFilesTable` are the file lists of the Data Model and Jets
     * Rules sections, and both read the *same* `workspace_control` table. The
     * prefix — `data_model/%`, `jet_rules/%` — is the whole of what separates
     * them, because a workspace file's section is its directory.
     *
     * **The pattern is the document's, not the user's**, and that is the reason
     * it can be a plain string. `makeWhereClause` (`datatable/query.ts`, the
     * `wc.like` branch) puts it on the request envelope, and `buildWhereClause`
     * (`jets/datatable/data_table_action.go`, the `len(wc.Like) > 0` case) binds
     * it as a parameter rather than interpolating it — so a `%` in a document is
     * a wildcard and nothing in it reaches SQL as text. A clause whose pattern
     * came from form state would be a different field and is not one the corpus
     * has.
     */
    like: z.string().min(1).optional(),
    /**
     * `column` is not a column name: it is a form-state key holding one. Task C.9.
     *
     * **One clause in either corpus, and it is the only way its table can be
     * written at all.** `inputRecordsFromProcessErrorTable` reads a *domain* table
     * whose key column is named after the object type — `'${objectType}:domain_key'`
     * — so the column differs per row of the dialog that opens it
     * (`modules/form_config_impl.dart`, `viewInputRecords`'s `inputFieldRowBuilder`,
     * which writes `FSK.domainKeyColumn`). A configuration cannot name it and a
     * document cannot either.
     *
     * **The indirection is the client's and the server never sees it**: nothing
     * under `jets/` reads the field. `_makeWhereClause`
     * (`jetsclient/lib/components/data_table_source.dart`, `_makeWhereClause`)
     * substitutes before the request leaves, and it reads the key **in the data
     * table field's own group** rather than in group 0 — which is the whole
     * mechanism, because the dialog draws one table per input source and each row's
     * column name is its own.
     *
     * A `true` literal rather than a boolean: `false` would be a second spelling of
     * absence, and the corpus's 48 other clauses say it by omission.
     */
    lookupColumnInFormState: z.literal(true).optional(),
    // A getter rather than `z.lazy`: both express the recursion, and `z.lazy`
    // emits an extra anonymous `$defs` entry that indirects to the real one.
    // This artifact is read by people as well as by v6.
    get orWith() {
      return WhereClauseSchema.optional();
    },
  })
  .meta({ id: "WhereClause" });

/**
 * What an action does when pressed.
 *
 * Seven kinds across the 37, and this is the whole list rather than the Dart's:
 * `refreshTable` (1), `toggleCheckboxVisible` (3), `clearHomeFilters` (3),
 * `showScreen` (3), `showDialog` (2), `doActionShowDialog` (3), `doAction` (10).
 *
 * **Nine as of F.5, and the two extra arrived from track F rather than track C.**
 * This comment said *track C's 28 add `setSessionIdFilter` and `setRequestIdFilter`,
 * one each* — true about where they are counted and wrong about who meets them
 * first. Both are on `pipelineExecStatusTable`, the one configuration registered
 * on the non-flow side and rendered by a flow (**F18**), so `homeFiltersUF` could
 * not be authored without them. The prediction was right about the number and
 * wrong about the schedule, which is what F18 is for.
 */
export const ActionTypeSchema = z
  .enum([
    "doAction",
    "doActionShowDialog",
    "showDialog",
    "showScreen",
    "refreshTable",
    "toggleCheckboxVisible",
    "clearHomeFilters",
    "setSessionIdFilter",
    "setRequestIdFilter",
  ])
  .meta({ id: "TableActionType" });

/**
 * The server action a query table issues. Task C.2.
 *
 * **Three members, and the absent fourth is `read`** — see the `apiAction` note
 * in this file's header for why the field exists at all, why it is an enum, and
 * why `read` is not in it.
 *
 * Each member is a `case` of the `/dataTable` dispatch
 * (`jets/apiserver/api_tables.go`, `DoDataTableAction`) and each gates itself:
 * `workspace_read` requires `workspace_ide`
 * (`jets/datatable/workspace_data_table_action.go`, `DoWorkspaceReadAction`),
 * `raw_query_tool` requires `CapabilityQueryTool` and `preview_file` requires
 * `CapabilityReadData`. **The enum is not the authorisation** — it decides what
 * an authored document may *ask for*, and the server decides who may have it.
 * That is S.7's division and this field does not move it.
 *
 * **`raw_query_tool` is the member that shows what the enum is for**, and this
 * paragraph is C.4's, contributed by the task that owns the one table naming it.
 * The other three name a handler that composes its own SQL from the request's
 * structured parts; this one names the handler that executes the request's
 * `query` string verbatim — `ExecRawQuery` passes `dataTableAction.RawQuery`
 * straight to `dbpool.Query` (`jets/datatable/data_table_action.go`, `execQuery`).
 * It is gated accordingly: `raw_query` takes `datatable.CapabilityReadData` and
 * `raw_query_tool` takes `datatable.CapabilityQueryTool`, which is
 * `"workspace_ide"` (`jets/apiserver/api_tables.go`, the `raw_query_tool` case;
 * `jets/datatable/data_table_action.go`, the `CapabilityQueryTool` const), and
 * `TestRawQueryToolIsGatedMoreTightlyThanRawQuery`
 * (`jets/apiserver/read_dispatch_test.go`) refuses to let the two share a case
 * again. **So this is not a tidy-up of a free string; it is the list of
 * authorities an authored table may reach**, and `raw_query_tool` is the one
 * where the authority is "run this SQL". One table has it —
 * `queryToolResultSetTable` — and C.4 is the only screen that will ever author
 * it.
 */
export const ApiActionSchema = z
  .enum(["workspace_read", "preview_file", "raw_query_tool"])
  .meta({ id: "TableApiAction", description: "The server action a query table issues" });

/**
 * One conjunct of a button's row gate. Task C.2.
 *
 * `ActionEnableCriteria` (`jetsclient/lib/models/data_table_config.dart`,
 * `ActionEnableCriteria`), with one change: **the Dart names a column by
 * position and this names it by name.** `columnPos` is an index into the same
 * `columns` array whose `index` field this document already dropped for being a
 * value that can only ever disagree with the array holding it; a criterion
 * pointing at column 6 is that same hazard with a worse failure, because
 * inserting a column above it silently regates the button instead of rendering a
 * blank header. `fromDocument` restores the position by looking the name up, so
 * the round trip is exact.
 *
 * **Four comparison kinds, and the corpus uses two.** All 8 criteria in either
 * corpus are `contains` or `doesNotContain`. `equals` and `notEquals` are
 * admitted anyway, which is `textRestriction`'s argument in `userflow/form.ts`:
 * `isCriteriaMet` implements four, so `availability` ports four, and a
 * two-member enum would have to be widened by whoever meets the third — a change
 * to an exported union rather than to a document.
 *
 * **`value` is required here and nullable in the Dart.** `isCriteriaMet` returns
 * `false` for a null value on `contains`/`doesNotContain` and compares it as null
 * on `equals`/`notEquals`, so a null is either a gate that never opens or a test
 * for an empty cell — neither of which any of the 8 sites wants, and both of
 * which a document should have to say some other way if it ever does.
 */
export const EnableCriterionSchema = z
  .strictObject({
    /** The column whose value is tested, by name. */
    column: Identifier,
    is: z.enum(["equals", "notEquals", "contains", "doesNotContain"]),
    value: z.string().min(1),
  })
  .meta({ id: "EnableCriterion", description: "One test on the selected row" });

/**
 * One button on the table's action bar.
 *
 * **No `stateGroup`**: `0` on all 25. **No `actionDelegate`**: `false` on all 25,
 * because a table action reaches its delegate through `actionName` and the
 * central switch in `config_delegates.dart`, not through a field on the config.
 * That is worth stating rather than leaving as an omission — `types.ts` declares
 * `hasActionDelegate` and a reader would reasonably expect it to be the way out.
 *
 * `capability` is presentation only, as it is everywhere in this app
 * (assessment §3.5): 9 of 25 declare one, `client_config` or `run_pipelines`.
 * The server gates the write it performs, not the button that starts it.
 */
export const TableActionSchema = z
  .strictObject({
    key: Identifier,
    label: z.string().min(1),
    action: ActionTypeSchema,
    style: z.enum(["primary", "secondary", "danger"]),
    /** The action-document entry `doAction` runs. 11 distinct names in the corpus. */
    actionName: Identifier.optional(),
    /** The form a dialog or screen opens. */
    configForm: Identifier.optional(),
    /** The route `showScreen` navigates to; a path template, not an identifier. */
    configScreenPath: z.string().min(1).optional(),
    capability: Identifier.optional(),
    isVisibleWhenCheckboxVisible: z.boolean().optional(),
    isEnabledWhenHavingSelectedRows: z.boolean().optional(),
    isEnabledWhenWhereClauseSatisfied: z.boolean().optional(),
    isEnabledWhenStateHasKeys: z.array(Identifier).min(1).optional(),
    /**
     * Tests on the selected row that must pass. Task C.2.
     *
     * **A disjunction of conjunctions, which is the Dart's own description of
     * it** — *"the outer list is 'or' and the inner list is 'and'"*
     * (`jetsclient/lib/models/data_table_config.dart`, `ActionConfig.isEnabled`).
     * The shape is kept rather than flattened because two of the eight sites use
     * the inner list: *Export Client Config* wants a workspace that is neither
     * `removed` nor `in progress`.
     *
     * **Empty on all 37 flow tables and set on 8 actions of one non-flow one**,
     * which is `table.ts`'s own prediction and is now exact: every criteria-
     * bearing action in either corpus is on `workspaceRegistryTable`, and every
     * one of them tests the `status` column.
     *
     * It is not decoration. On that table it is what refuses *Delete* on an
     * active workspace and *Open* on one mid-compile — see **I-181** for what it
     * cost that nothing read it.
     */
    enableWhen: z.array(z.array(EnableCriterionSchema).min(1)).min(1).optional(),
    /** Route parameters, taken literally. */
    navigationParams: z.record(Identifier, z.union([z.string(), z.number()])).optional(),
    /** Route parameters read out of form state, by key. */
    stateFormNavigationParams: z.record(Identifier, Identifier).optional(),
    /**
     * A named registry entry deciding whether the button is enabled.
     *
     * Three actions have one, all of them the `clearFilters` button of a
     * `clearHomeFilters` action — the predicate is over router state rather than
     * over the table, which is exactly why it is not expressible here.
     */
    isEnabled: Identifier.optional(),
  })
  .meta({ id: "TableAction", description: "One button on a table's action bar" });

/**
 * How a selected row lands in form state.
 *
 * `keyColumnIdx` is an index into `columns`; 36 of 37 tables declare one, 29 at
 * column 0 and 7 at column 1. `otherColumns` copies further columns out under
 * their own keys — 22 tables copy none, and one copies fifteen.
 *
 * **Bounded below and not above**, for the reason I.2a's `defaultItemPos` is: an
 * index into a sibling array is a cross-field rule, this file may only use
 * constructs that emit, and a `.refine()` would be enforced in the browser and
 * silently absent in Go.
 */
export const FormStateBindingSchema = z
  .strictObject({
    keyColumnIdx: z.number().int().nonnegative(),
    otherColumns: z
      .array(
        z.strictObject({
          stateKey: Identifier,
          columnIdx: z.number().int().nonnegative(),
        }),
      )
      .optional(),
  })
  .meta({ id: "FormStateBinding" });

/** Fields both kinds carry. Split out so the union states only what differs. */
const commonFields = {
  schemaVersion: z.literal(1),
  /**
   * The heading the table renders above itself.
   *
   * **Optional, because two of the 37 have none** — `pcProcessInputRegistry` and
   * `pcProcessInputRegistry4MI`, both reached only from a `doActionShowDialog`
   * button, where the dialog carries the title and a second one would be
   * duplicated. Absent means "the container titles this", which is a statement;
   * the Dart's empty string is a sentinel for the same thing.
   */
  label: z.string().min(1).optional(),
  /**
   * Which `FROM` entry `sortColumn` belongs to. Task C.3.
   *
   * **Empty on all 37 flow tables and set on eight non-flow ones, every one of
   * them a Workspace IDE compiled view** — `wsDomainClassTable` sorts on
   * `domain_classes.name` while `domain_classes` is one of two tables in its
   * `FROM`, and `wsMainSupportFilesTable` sorts on `main_file.source_file_name`
   * where `main_file` is an *alias* of `workspace_control`, joined to itself.
   *
   * **The flows can leave it empty because none of them sorts on a joined
   * table**, not because the field is optional in spirit: `makeQuery` writes it
   * to `sortColumnTable` on every request, and the server composes
   * `ORDER BY <table>.<column>` from the pair when it is non-empty and
   * `ORDER BY <column>` when it is not (`jets/datatable/data_table_action.go`,
   * `buildQuery`, the `SortColumnTable` branch). A multi-table `FROM` whose sort
   * column name appears in more than one of them is ambiguous without it — and
   * `name` is a column of `domain_classes`, `data_properties`, `domain_tables`
   * and `jet_rules` alike, which is why six of the eight need it.
   *
   * Absent means the empty string, which is what a single-table `FROM` sends
   * today — so no existing document changes and none has to.
   */
  sortColumnTable: Identifier.optional(),
  /**
   * The band a data row is drawn in, in pixels. Task C.3.
   *
   * **One configuration in either corpus sets either half, and it sets both** —
   * `wsJetRulesTable` at 64 and 90 (`screens/fixtures/screen_configs.json`), the
   * Jet Rules tab of the `jet_rules` compiled view. It is the table whose
   * `authored_label` column carries `maxLines: 5` and `columnWidth: 900`: a rule
   * as written is several lines of text, and without a band every row in the page
   * is as tall as the longest rule on it.
   *
   * **The two are one object rather than two fields, which is the opposite of the
   * call C.7 made on `maxLines` and `columnWidth`, and the difference is real.**
   * Those two are independent properties of a column that happen to co-occur;
   * these two are the ends of one range, and a document declaring a minimum with
   * no maximum would be describing a state no configuration has and nothing
   * draws. Both are therefore required when the object is present.
   */
  rowHeight: z
    .strictObject({
      min: z.number().int().positive(),
      max: z.number().int().positive(),
    })
    .meta({ id: "RowHeight", description: "The pixel band a data row is drawn in" })
    .optional(),
  sortAscending: z.boolean().optional(),
  rowsPerPage: z.number().int().positive(),
  isCheckboxVisible: z.boolean().optional(),
  isCheckboxSingleSelect: z.boolean().optional(),
  isReadOnly: z.boolean().optional(),
  showSelectedOnly: z.boolean().optional(),
  noFooter: z.boolean().optional(),
  noCopy2Clipboard: z.boolean().optional(),
  formStateBinding: FormStateBindingSchema.optional(),
} as const;

/**
 * The document. One table configuration.
 *
 * **`source` is the discriminant and the Dart has no equivalent** — it tells the
 * two apart by `apiPath` being the empty string, which is a sentinel standing in
 * for a statement. Nine static, 28 query, and nothing in the corpus is both or
 * neither.
 *
 * `refreshOnKeyUpdateEvent` is on the query arm only: the two tables that
 * declare one are both query tables, and a static table has nothing to refresh.
 */
export const TableConfigDocumentSchema = z
  .discriminatedUnion("source", [
    z.strictObject({
      ...commonFields,
      source: z.literal("query"),
      /**
       * What this table asks the server to do. Absent means `read`. Task C.2.
       *
       * On the query arm only: a static table issues nothing, so a static
       * document naming one would be asking for something it never sends.
       */
      apiAction: ApiActionSchema.optional(),
      /**
       * The columns, when the table knows them. Task C.4.
       *
       * **A query table may declare none, and four of the non-flow 25 do.** The
       * server supplies them in that case, and it does so by two mechanisms that
       * agree on the outcome: `DoReadAction` builds a `columnDef` whenever the
       * request carries no columns (`jets/datatable/data_table_action.go`,
       * `DoReadAction`), `DoPreviewFileAction` always sends one, and `execQuery`
       * sends one when the request sets `requestColumnDef` below. The client
       * replaces its columns with what comes back — `columnsFromResponse`
       * (`datatable/useDataTable.ts`), which has done this since A.4b.
       *
       * So `min(1)` moved off `commonFields` and on to the static arm rather than
       * being deleted: a table whose rows are compiled into it and whose columns
       * are unknown is a document with nothing to render. **This is where the
       * union earns its keep for a second reason** — the first was that a static
       * table cannot name a schema.
       *
       * The four are `queryToolResultSetTable` (C.4), `inputFileViewerTable`
       * (C.12), `inputRecordsFromProcessErrorTable` (C.9) and `inputTable` (C.11),
       * and they are the same four with no `sortColumn`.
       */
      columns: z.array(ColumnSchema),
      /**
       * The column the first page is sorted on. Absent means "the server's first".
       *
       * Required on the static arm and optional here, for the reason above: the
       * four tables that declare no columns have no name to put in it, and the
       * Dart says so with an empty string. `resolveSortColumn` sorts on column 0
       * of whatever the response defines (`datatable/model.ts`), which is what
       * `state.setSortingColumn(columnIndex: 0)` does in the Dart after a
       * `columnDef` arrives.
       */
      sortColumn: Identifier.optional(),
      /**
       * Ask the server to describe the result's columns. Task C.4.
       *
       * **One configuration in either corpus sets it, and it is only read on the
       * raw-statement path**: `execQuery` builds a `columnDef` out of the pgx
       * field descriptions when `DataTableAction.RequestColumnDef` is true
       * (`jets/datatable/data_table_action.go`, `execQuery`), and nothing else in
       * the apiserver reads the field. The structured read path needs no flag,
       * because sending no columns is already the request for them.
       *
       * It is a separate field rather than implied by `apiAction: "raw_query_tool"`
       * because implying it would be a rule inferred from one configuration, and
       * because the Go struct's own comment scopes it to `raw_query` **and**
       * `raw_query_tool` — two actions, of which a table may name one.
       */
      requestColumnDef: z.literal(true).optional(),
      /** Common table expressions, prepended to the read. Task C.9 — one table. */
      with: z.array(WithClauseSchema).min(1).optional(),
      from: z.array(FromClauseSchema).min(1),
      where: z.array(WhereClauseSchema).optional(),
      actions: z.array(TableActionSchema).optional(),
      /**
       * A second row of buttons, enabled by a selected row. Task F.5.
       *
       * **Empty on all 37 flow tables and five entries long on the first non-flow
       * one**, which is why it was cut and why it is back: `pipelineExecStatusTable`
       * puts *View Execution Details*, *View Process Errors*, *View Failure
       * Details*, *View Execution Stats* and *Resubmit* here
       * (`modules/data_table_config_impl.dart`, `secondRowActions`).
       *
       * **Same element type as `actions`, deliberately.** The Dart's two lists hold
       * the same class and differ only in where the widget draws them
       * (`components/data_table.dart`, `_actionsRow`), so a second schema would be
       * a distinction the configuration does not make. What differs is convention:
       * four of the five set `isEnabledWhenHavingSelectedRows`, because a row
       * action without a row has nothing to act on.
       *
       * **`validateTableActions` reads both** (`userflow/documentSet.ts`) — I-88's
       * check would otherwise have been blind to the two entries here that name a
       * form and an action, which is every cross-document reference this table has.
       */
      secondRowActions: z.array(TableActionSchema).min(1).optional(),
      /** Re-query when one of these form-state keys changes. Two tables use it. */
      refreshOnKeyUpdateEvent: z.array(Identifier).min(1).optional(),
    }),
    z.strictObject({
      ...commonFields,
      source: z.literal("static"),
      /** All nine carry three; a static table with none has nothing to render. */
      columns: z.array(ColumnSchema).min(1),
      sortColumn: Identifier,
      /**
       * The rows, positionally, one entry per column.
       *
       * All nine static tables in the corpus are option lists: three columns —
       * a description, a value and an `option_order` — and two to nine rows.
       * They are the same shape as I.2a's dropdown items rendered as a table,
       * which is an observation about the Flutter UI rather than a licence to
       * change one into the other here.
       */
      rows: z.array(z.array(z.string().nullable())).min(1),
    }),
    /**
     * A table whose rows are in form state. Task C.9, and the third kind.
     *
     * Three tables, all of `viewReteTriplesDialogV2`. The Rete Session Explorer
     * fetches one JSON blob out of `process_errors.rete_session_triples` and then
     * navigates it entirely in the browser — class names, the entity keys of the
     * selected class, the properties of the selected key — so none of the three
     * ever issues a request (`modules/actions/process_errors_delegates.dart`, the
     * `setupShowReteTriplesV2` case).
     *
     * **`where` is here and it is a gate rather than a predicate**, which is F82's
     * observation arriving on a whole arm rather than on one clause. Two of the
     * three carry a clause naming a form-state key and no server ever sees it: its
     * only effect is `hasBlockingFilter` (`datatable/binding.ts`,
     * `hasBlockingFilter`), which is what makes *pick a class first* different from
     * *this class has no entities*. The Dart runs the same check before it decides
     * where the rows come from (`components/data_table_source.dart`, the
     * `hasBlockingFilter` block, which precedes the `staticTableModel` /
     * `modelStateFormKey` / `modelStateHandler` / `fetchData` chain).
     *
     * **No `from`, no `apiAction`, no `secondRowActions`, no
     * `refreshOnKeyUpdateEvent`** — a table that never queries has nothing to
     * address, nothing to ask for and nothing to re-ask. `actions` *is* here: the
     * third table's *Visit Object Entity* button is a `doAction`, and a button that
     * writes form state is as available to this kind as to any other.
     */
    z.strictObject({
      ...commonFields,
      source: z.literal("formState"),
      columns: z.array(ColumnSchema).min(1),
      sortColumn: Identifier,
      model: ModelSourceSchema,
      where: z.array(WhereClauseSchema).optional(),
      actions: z.array(TableActionSchema).optional(),
    }),
  ])
  .meta({
    id: "TableConfigDocument",
    title: "JetStore table configuration",
    description: "One data-table configuration, authored as data",
  });

export type Column = z.infer<typeof ColumnSchema>;
export type FromClause = z.infer<typeof FromClauseSchema>;
export type ApiAction = z.infer<typeof ApiActionSchema>;
export type EnableCriterion = z.infer<typeof EnableCriterionSchema>;
export type TableAction = z.infer<typeof TableActionSchema>;
export type FormStateBinding = z.infer<typeof FormStateBindingSchema>;
export type WithClause = z.infer<typeof WithClauseSchema>;
export type ModelSource = z.infer<typeof ModelSourceSchema>;
export type TableConfigDocument = z.infer<typeof TableConfigDocumentSchema>;

export function emitJsonSchema(): unknown {
  return z.toJSONSchema(TableConfigDocumentSchema, { io: "input" });
}

/** The directory a workspace keeps its table configurations in. */
export const TABLE_DIR = "table_configs";

export const tablePath = (key: string): string => `${TABLE_DIR}/${key}.tc.json`;

/**
 * The escape names a table configuration references.
 *
 * Same shape as `escapeReferences` for a flow (`userflow/store.ts`), and for the
 * same reason: the registry is compiled into the bundle, so whether a name
 * resolves is a question only the client can answer. The server validates the
 * document's shape and cannot do this one.
 */
/**
 * Every action a table declares, on whichever rows its kind has. Task C.9.
 *
 * **Written because a third arm is a breaking change for anything that reads
 * one.** The three walks below each opened with `if (table.source === "query")`,
 * which is exactly right for two arms and silently wrong for three: a
 * `formState` table declares `actions` — `reteSessionEntityDetailsTable`'s
 * *Visit Object Entity* is a `doAction` — and every one of them would have
 * reported it as declaring none. Nothing would have failed; the button would have
 * rendered and its action would have been absent from the reference list that
 * proves the registry can resolve it.
 *
 * A helper that asks *what does this table have* rather than *is this table a
 * query* makes the fourth arm a non-event, which is the fix the seam rule asks
 * for rather than the fix that makes today's tests pass.
 */
function actionsOf(table: TableConfigDocument): TableAction[] {
  if (table.source === "static") return [];
  if (table.source === "formState") return table.actions ?? [];
  return [...(table.actions ?? []), ...(table.secondRowActions ?? [])];
}

export function escapeNamesOf(table: TableConfigDocument): string[] {
  const names = new Set<string>();
  for (const column of table.columns) if (column.cellFilter) names.add(column.cellFilter);
  // Both rows, since F.5 — an `isEnabled` on a second-row button resolves out of
  // the same registry and an unresolved one must fail the load the same way.
  for (const action of actionsOf(table)) if (action.isEnabled) names.add(action.isEnabled);
  return [...names].sort();
}

/**
 * The same references, carrying which namespace each belongs to. Task I.3b.
 *
 * `escapeNamesOf` flattens both kinds into one sorted list, which is what a
 * *count* wants and not what resolution wants: a `cellFilter` and an `isEnabled`
 * are looked up in different namespaces, and a name registered as one does not
 * satisfy the other. Both functions are kept — the flat one is what the round-trip
 * and corpus tests assert against, and rewriting them to unwrap this would make
 * them less legible for no gain.
 *
 * `at` is a JSON Pointer into the table document, so a load failure names the
 * offending line rather than the file — the same contract `escapeReferences` has
 * for a flow (`userflow/store.ts`).
 */
export function tableEscapeReferences(table: TableConfigDocument): EscapeReferences[] {
  const references: EscapeReferences[] = [];
  table.columns.forEach((column, index) => {
    if (column.cellFilter) {
      references.push({ kind: "cellFilters", name: column.cellFilter, at: `/columns/${index}/cellFilter` });
    }
  });
  // **Both rows and every arm, as of C.9 — this walked `actions` only.** The two
  // functions above and below have covered `secondRowActions` since F.5 and this
  // one did not, so an `isEnabled` on a second-row button was counted by
  // `escapeNamesOf` and never *resolved*. No configuration exercises it today —
  // `workspaceRegistryTable` puts eight of thirteen buttons on the second row and
  // gates all eight on criteria rather than on a predicate — which is why it went
  // unnoticed rather than why it was harmless.
  if (table.source !== "static") {
    (table.actions ?? []).forEach((action, index) => {
      if (action.isEnabled) {
        references.push({ kind: "predicates", name: action.isEnabled, at: `/actions/${index}/isEnabled` });
      }
    });
  }
  if (table.source === "query") {
    (table.secondRowActions ?? []).forEach((action, index) => {
      if (action.isEnabled) {
        references.push({
          kind: "predicates",
          name: action.isEnabled,
          at: `/secondRowActions/${index}/isEnabled`,
        });
      }
    });
  }
  return references;
}

/**
 * The action-document entries a table configuration names.
 *
 * A table's `doAction` button runs an entry in the *flow's* action document, so
 * this is a cross-document reference like a state's `stateAction` — and, like
 * that one, it cannot be checked at save time. A `.tc.json` is legitimately
 * saved before the flow that uses it exists, and a table is shared between
 * flows, so there is no single document to check it against.
 */
export function actionNamesOf(table: TableConfigDocument): string[] {
  const names = new Set<string>();
  // Both rows since F.5: `resubmitPipeline` is `pipelineExecStatusTable`'s only
  // `doAction` and it is a second-row button, so a first-row-only walk would have
  // reported this table as naming no action at all. Every arm since C.9: see
  // `actionsOf`.
  for (const action of actionsOf(table)) if (action.actionName) names.add(action.actionName);
  return [...names].sort();
}

// JetRule data model in TypeScript.
//
// This module mirrors the serialized JetRule model produced by the JetStore
// compiler (see jets/jetrules/rete/rete_meta_store_model.go) and reflects the
// constructs defined by the JetRule ANTLR grammar
// (jets/compilerv2/compiler/JetRule.g4).
//
// Field names use the same JSON keys emitted by the Go model so that JSON
// produced by the compiler can be consumed directly, e.g.:
//
//   const model: JetruleModel = JSON.parse(jsonText);

// --------------------------------------------------------------------------------------
// Primitive / enumerated grammar types
// --------------------------------------------------------------------------------------

/**
 * Data property types supported by the JetRule grammar (dataPropertyType rule).
 * Matches the JetRule keywords: int, uint, long, ulong, double, text, date,
 * datetime, bool and resource.
 */
export type DataPropertyType =
  | 'int'
  | 'uint'
  | 'long'
  | 'ulong'
  | 'double'
  | 'text'
  | 'date'
  | 'datetime'
  | 'bool'
  | 'resource';

/**
 * RDF value type carried by a {@link ResourceNode}. One of JetStore's rdf
 * types: resource, text, volatile_resource, var, int, keyword, double, etc.
 */
export type ResourceType =
  | 'resource'
  | 'volatile_resource'
  | 'text'
  | 'var'
  | 'int'
  | 'uint'
  | 'long'
  | 'ulong'
  | 'double'
  | 'date'
  | 'datetime'
  | 'bool'
  | 'keyword'
  | 'null';

/** Kind of a rule term: an antecedent (if) or a consequent (then). */
export type RuleTermType = 'antecedent' | 'consequent';

/** Kind of an expression node in a filter or object expression. */
export type ExpressionType = 'identifier' | 'unary' | 'binary';

// --------------------------------------------------------------------------------------
// Root model
// --------------------------------------------------------------------------------------

/**
 * Root of the JetRule model, the output of compiling a set of JetRule files.
 */
export interface JetruleModel {
  main_rule_file_name: string;
  compiler_directives?: Record<string, string>;
  resources?: ResourceNode[];
  lookup_tables?: LookupTableNode[];
  jet_rules?: JetruleNode[];
  rete_nodes?: RuleTerm[];
  jetstore_config?: Record<string, string>;
  rule_sequences?: RuleSequence[];
  classes?: ClassNode[];
  tables?: TableNode[];
  triples?: TripleNode[];
  head_rule_term?: RuleTerm;
  antecedents?: RuleTerm[];
  consequents?: RuleTerm[];
}

// --------------------------------------------------------------------------------------
// Resources
// --------------------------------------------------------------------------------------

/**
 * ResourceNode represents a resource, literal or variable in the model.
 * `type` can be one of JetStore's rdf types (see {@link ResourceType}).
 */
export interface ResourceNode {
  id?: string;
  inline?: boolean;
  is_antecedent?: boolean;
  is_binded?: boolean;
  key?: number;
  source_file_name?: string;
  type?: ResourceType;
  value?: string;
  var_pos?: number;
  vertex?: number;
}

// --------------------------------------------------------------------------------------
// Lookup tables
// --------------------------------------------------------------------------------------

export interface LookupTableNode {
  columns?: LookupTableColumn[];
  data_file_info?: LookupTableDataInfo;
  csv_file?: string;
  key?: string[];
  name?: string;
  resources?: string[];
  source_file_name?: string;
  type?: string;
}

export interface LookupTableColumn {
  name?: string;
  type?: DataPropertyType | string;
  as_array?: boolean;
}

/**
 * Information about the lookup table data. Historically the `lookup.db`
 * sqlite3 file (the default when `data_file_info` is not specified).
 */
export interface LookupTableDataInfo {
  db_file_name?: string;
  format?: string;
  index_file_name?: string;
}

// --------------------------------------------------------------------------------------
// Rules
// --------------------------------------------------------------------------------------

/**
 * JetruleNode provides a rule view of the rete network.
 * - `authoredLabel` is generated pre-optimization using the original variable names.
 * - `normalizedLabel` is a normalized version using the variable IDs.
 * - `label` is the text version using the original variable names.
 * - `properties` is a map of properties defined in the rule header.
 * - `optimization` indicates if the rule should be optimized (default true).
 * - `salience` indicates the salience of the rule (default 100).
 * - `is_valid` indicates if the rule passed validation.
 */
export interface JetruleNode {
  name?: string;
  properties?: Record<string, string>;
  optimization?: boolean;
  salience?: number;
  antecedents?: RuleTerm[];
  consequents?: RuleTerm[];
  authoredLabel?: string;
  source_file_name?: string;
  normalizedLabel?: string;
  label?: string;
  is_valid?: boolean;
}

/**
 * RuleTerm is a single antecedent or consequent within the rete network.
 * - `beta_relation_vars` is the full list of variable IDs used in beta relations.
 * - `pruned_var` is the list of variable IDs not needed by current/descendent nodes.
 * - `beta_var_nodes` is the net list (beta_relation_vars minus pruned_var).
 * - `children_vertexes` is a list of vertexes of the child nodes.
 * - `rules`/`salience` associate rule names/saliences (antecedents only).
 * - `consequent_seq`/`consequent_for_rule`/`consequent_salience` (consequents only).
 * - `subject_key`/`predicate_key`/`object_key` are resource keys in the model.
 * - `obj_expr` is the object when it is an expression.
 * - `filter` is the expression applied to the antecedent.
 */
export interface RuleTerm {
  type?: RuleTermType;
  isNot?: boolean;
  normalizedLabel?: string;
  vertex?: number;
  parent_vertex?: number;
  beta_relation_vars?: string[];
  pruned_var?: string[];
  beta_var_nodes?: BetaVarNode[];
  children_vertexes?: number[];
  rules?: string[];
  salience?: number[];
  consequent_seq?: number;
  consequent_for_rule?: string;
  consequent_salience?: number;
  subject_key?: number;
  predicate_key?: number;
  object_key?: number;
  obj_expr?: ExpressionNode;
  filter?: ExpressionNode;
}

/**
 * ExpressionNode represents an expression in a filter or object position.
 * - `type` is "identifier", "unary" or "binary".
 * - `op` is the operator for unary and binary expressions.
 * - `arg` is the argument for unary expressions.
 * - `lhs`/`rhs` are the operands for binary expressions.
 * - `value` is the resource key for an identifier.
 */
export interface ExpressionNode {
  type?: ExpressionType;
  op?: string;
  arg?: ExpressionNode;
  lhs?: ExpressionNode;
  rhs?: ExpressionNode;
  value?: number;
}

/**
 * BetaVarNode provides information about a variable in a beta relation.
 * - `type` is always "var".
 * - `id` is the variable ID (e.g. ?x1).
 * - `is_binded` indicates if the variable is binded in the parent nodes.
 * - `var_pos` is the 1-based position of the variable in the rule.
 * - `vertex` is the vertex of the node where the variable is used.
 */
export interface BetaVarNode {
  type?: string;
  id?: string;
  is_binded?: boolean;
  var_pos?: number;
  vertex?: number;
  source_file_name?: string;
}

// --------------------------------------------------------------------------------------
// Classes and tables (data model)
// --------------------------------------------------------------------------------------

export interface ClassNode {
  type?: string;
  name?: string;
  base_classes?: string[];
  data_properties?: PropertyNode[];
  object_properties?: PropertyNode[];
  source_file_name?: string;
  as_table?: boolean;
  sub_classes?: string[];
}

export interface PropertyNode {
  type?: DataPropertyType | string;
  name?: string;
  class_name?: string;
  as_array?: boolean;
  is_object?: boolean;
}

export interface TableNode {
  domain_class_key?: number;
  table_name?: string;
  class_name?: string;
  columns?: TableColumnNode[];
  source_file_name?: string;
}

export interface TableColumnNode {
  type?: DataPropertyType | string;
  as_array?: boolean;
  is_object?: boolean;
  column_name?: string;
}

// --------------------------------------------------------------------------------------
// Triples
// --------------------------------------------------------------------------------------

export interface TripleNode {
  type?: string;
  subject_key?: number;
  predicate_key?: number;
  object_key?: number;
  source_file_name?: string;
}

// --------------------------------------------------------------------------------------
// Rule sequences
// --------------------------------------------------------------------------------------

/**
 * RuleSequence defines a rule process made of a cascading sequence of rule sets.
 * (see jets/jetrules/rete/workspace_control.go)
 */
export interface RuleSequence {
  name: string;
  rule_sets: string[];
}

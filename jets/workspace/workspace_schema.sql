-- Schema for workspace.db
CREATE TABLE base_classes (
  domain_class_key INTEGER NOT NULL,
  base_class_key INTEGER NOT NULL,
  UNIQUE (domain_class_key, base_class_key)
);
CREATE TABLE beta_row_config (
  key INTEGER PRIMARY KEY,
  vertex INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  source_file_key INTEGER NOT NULL,
  row_pos INTEGER NOT NULL,
  is_binded INTEGER,
  id TEXT,
  UNIQUE (vertex, seq, source_file_key)
);
CREATE TABLE data_properties (
  key INTEGER PRIMARY KEY,
  domain_class_key INTEGER NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  as_array BOOL DEFAULT FALSE,
  is_grouping BOOL DEFAULT FALSE,
  -- domain property name must be unique in workspace
  UNIQUE (name)
);
CREATE TABLE domain_classes (
  key INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  as_table BOOL DEFAULT FALSE,
  source_file_key INTEGER NOT NULL,
  -- domain class name must be unique in workspace
  UNIQUE (name)
);
CREATE TABLE domain_columns (
  domain_table_key INTEGER NOT NULL,
  data_property_key INTEGER NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  as_array BOOL DEFAULT FALSE,
  is_grouping BOOL DEFAULT FALSE,
  -- a column must appear only once in a table
  UNIQUE (domain_table_key, data_property_key),
  -- a column name must be unique for a table
  UNIQUE (domain_table_key, name)
);
CREATE TABLE domain_tables (
  key INTEGER PRIMARY KEY,
  domain_class_key INTEGER NOT NULL,
  name TEXT NOT NULL,
  -- domain table name must be unique since domain_class are unique
  UNIQUE (name)
);
CREATE TABLE expressions (
  key INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  arg0_key INTEGER,
  arg1_key INTEGER,
  arg2_key INTEGER,
  arg3_key INTEGER,
  arg4_key INTEGER,
  arg5_key INTEGER,
  op TEXT,
  source_file_key INTEGER NOT NULL
);
CREATE TABLE jet_rules (
  key INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  optimization BOOL,
  salience INTEGER,
  authored_label TEXT,
  normalized_label TEXT,
  label TEXT,
  source_file_key INTEGER NOT NULL
);
CREATE TABLE jetstore_config (
  config_key TEXT NOT NULL,
  config_value TEXT NOT NULL,
  source_file_key INTEGER NOT NULL
);
CREATE TABLE lookup_columns (
  lookup_table_key INTEGER NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  as_array BOOL DEFAULT FALSE,
  -- a column name must be unique for a table
  UNIQUE (lookup_table_key, name)
);
CREATE TABLE lookup_tables (
  key INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  table_name TEXT,
  csv_file TEXT,
  lookup_key TEXT,
  lookup_resources TEXT,
  source_file_key INTEGER NOT NULL,
  UNIQUE (name, source_file_key)
);
CREATE TABLE main_rule_sets (
  rule_sequence_key INTEGER NOT NULL,
  main_ruleset_name TEXT NOT NULL,
  ruleset_file_key INTEGER NOT NULL,
  seq INTEGER NOT NULL,
  UNIQUE (rule_sequence_key, ruleset_file_key)
);
CREATE TABLE main_support_files (
  main_file_key INTEGER NOT NULL,
  support_file_key INTEGER NOT NULL,
  UNIQUE (main_file_key, support_file_key)
);
CREATE TABLE resources (
  key INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  id TEXT,
  value TEXT,
  symbol TEXT,
  is_binded BOOL,
  -- for var type only
  inline BOOL,
  source_file_key INTEGER NOT NULL,
  vertex INTEGER,
  -- for var type only, var for vertex
  row_pos INTEGER -- for var type only, pos in beta row
);
CREATE TABLE rete_nodes (
  key INTEGER PRIMARY KEY,
  vertex INTEGER NOT NULL,
  type TEXT NOT NULL,
  subject_key INTEGER,
  predicate_key INTEGER,
  object_key INTEGER,
  obj_expr_key INTEGER,
  filter_expr_key INTEGER,
  normalizedLabel TEXT,
  parent_vertex INTEGER,
  source_file_key INTEGER NOT NULL,
  is_negation INTEGER,
  salience INTEGER,
  consequent_seq INTEGER NOT NULL,
  UNIQUE (vertex, type, consequent_seq, source_file_key)
);
CREATE TABLE rule_properties (
  rule_key INTEGER NOT NULL,
  name TEXT NOT NULL,
  value TEXT
);
CREATE TABLE rule_sequences (
  key INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  source_file_key INTEGER NOT NULL,
  -- rule seq name must be unique in workspace
  UNIQUE (name)
);
CREATE TABLE rule_terms (
  rule_key INTEGER NOT NULL,
  rete_node_key INTEGER NOT NULL,
  is_antecedent BOOL,
  PRIMARY KEY (rule_key, rete_node_key)
);
CREATE TABLE schema_info (
  version_major INTEGER NOT NULL,
  version_minor INTEGER NOT NULL,
  UNIQUE (version_major, version_minor)
);
CREATE TABLE triples (
  subject_key INTEGER NOT NULL,
  predicate_key INTEGER NOT NULL,
  object_key INTEGER NOT NULL,
  source_file_key INTEGER NOT NULL
);
CREATE TABLE workspace_control (
  key INTEGER PRIMARY KEY,
  source_file_name TEXT,
  is_main BOOL
);
CREATE INDEX rule_properties_idx ON rule_properties (rule_key);
CREATE INDEX triples_source_file_key_idx ON triples (source_file_key);
# Compiler v2 Notes

## Rule syntax improvement

Have a 2 rule statement like:

```

  # Calculate the adherence and assign it to the wrs:SDMCAnalysis of the associated claim
  resource adherenceRatio = "adherenceRatio";
  [Adherence41]:
    (iState uniqueGpi ?gpi).
    (iState ?gpi ?state).
    (?state firstDOS ?first_dos).
    (?state sumDaysOfSupply ?all_days_of_supply).
    (?state lastRenewalDate ?last_renewal_date)
  -> 
    (?state adherenceRatio ((1.0 * ?all_days_of_supply) / (?last_renewal_date - ?first_dos)))
  ;

  # Assign it to the wrs:SDMCAnalysis
  [SetAdherence01]: 
    (?opportunity rdf:type wrs:SDMCAnalysis).
    (?opportunity sdmc:gpi ?gpi).
    (iState ?gpi ?state).
    (?state adherenceRatio ?ratio)
  -> 
    (?opportunity sdmc:adherence_ratio ?ratio)
  ;
```

Combined into one:

```bash
  # Calculate the adherence and assign it to the wrs:SDMCAnalysis of the associated claim
  [SetAdherence01]: 
    (?opportunity rdf:type wrs:SDMCAnalysis).
    (?opportunity sdmc:gpi ?gpi).
    (iState ?gpi ?state).
    (?state firstDOS ?first_dos).
    (?state sumDaysOfSupply ?all_days_of_supply).
    (?state lastRenewalDate ?last_renewal_date)
  -> 
    (?opportunity sdmc:adherence_ratio ((1.0 * ?all_days_of_supply) / (?last_renewal_date - ?first_dos)))
  ;

```

## Existing Workspace DB Tables (SQLite)

```sql
      -- --------------------
      -- workspace_control table
      -- --------------------
      CREATE TABLE IF NOT  EXISTS workspace_control (
        key                INTEGER PRIMARY KEY,
        source_file_name   STRING,
        is_main            BOOL
      );
      CREATE TABLE IF NOT EXISTS main_support_files (
        main_file_key      INTEGER NOT NULL,
        support_file_key   INTEGER NOT NULL,
        UNIQUE (main_file_key, support_file_key)
      );


      -- --------------------
      -- jetstore_config table
      -- --------------------
      CREATE TABLE IF NOT  EXISTS jetstore_config (
        config_key         STRING NOT NULL,
        config_value       STRING NOT NULL,
        source_file_key    INTEGER NOT NULL
      );


      -- --------------------
      -- rule_sequences tables
      -- --------------------
      CREATE TABLE IF NOT EXISTS rule_sequences (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        source_file_key    INTEGER NOT NULL,
        -- rule seq name must be unique in workspace
        UNIQUE (name)
      );
      CREATE TABLE IF NOT EXISTS main_rule_sets (
        rule_sequence_key  INTEGER NOT NULL,
        main_ruleset_name  TEXT NOT NULL,
        ruleset_file_key   INTEGER NOT NULL,
        seq                INTEGER NOT NULL,
        UNIQUE (rule_sequence_key, ruleset_file_key)
      );


      -- --------------------
      -- domain_classes tables
      -- --------------------
      CREATE TABLE IF NOT EXISTS domain_classes (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        as_table           BOOL DEFAULT FALSE,
        source_file_key    INTEGER NOT NULL,
        -- domain class name must be unique in workspace
        UNIQUE (name)
      );
      CREATE TABLE IF NOT EXISTS base_classes (
        domain_class_key   INTEGER NOT NULL,
        base_class_key     INTEGER NOT NULL,
        UNIQUE (domain_class_key, base_class_key)
      );
      CREATE TABLE IF NOT EXISTS data_properties (
        key                INTEGER PRIMARY KEY,
        domain_class_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        is_grouping        BOOL DEFAULT FALSE,
        -- domain property name must be unique in workspace
        UNIQUE (name)
      );
      INSERT INTO domain_classes (key, name, source_file_key) VALUES (0, 'owl:Thing', -1)
        ON CONFLICT (key) DO NOTHING;

      -- --------------------
      -- domain_tables tables
      -- --------------------
      CREATE TABLE IF NOT EXISTS domain_tables (
        key                INTEGER PRIMARY KEY,
        domain_class_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        -- domain table name must be unique since domain_class are unique
        UNIQUE (name)
      );
      CREATE TABLE IF NOT EXISTS domain_columns (
        domain_table_key   INTEGER NOT NULL,
        data_property_key  INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        is_grouping        BOOL DEFAULT FALSE,
        -- a column must appear only once in a table
        UNIQUE (domain_table_key, data_property_key),
        -- a column name must be unique for a table
        UNIQUE (domain_table_key, name)
      );

      -- --------------------
      -- resources table
      -- --------------------
      CREATE TABLE IF NOT EXISTS resources (
        key                INTEGER PRIMARY KEY,
        type               STRING NOT NULL,
        id                 STRING,
        value              STRING,
        symbol             STRING,
        is_binded          BOOL,     -- for var type only
        inline             BOOL,
        source_file_key    INTEGER NOT NULL,
        vertex             INTEGER,  -- for var type only, var for vertex
        row_pos            INTEGER   -- for var type only, pos in beta row
      );

      -- --------------------
      -- lookup_tables table
      -- --------------------
      CREATE TABLE IF NOT EXISTS lookup_tables (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        table_name         STRING,
        csv_file           STRING,
        lookup_key         STRING,
        lookup_resources   STRING,
        source_file_key    INTEGER NOT NULL,
        UNIQUE (name, source_file_key)
      );
      CREATE TABLE IF NOT EXISTS lookup_columns (
        lookup_table_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        -- a column name must be unique for a table
        UNIQUE (lookup_table_key, name)
      );

      -- --------------------
      -- jet_rules table
      -- --------------------
      CREATE TABLE IF NOT EXISTS jet_rules (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        optimization       BOOL,
        salience           INTEGER,
        authored_label     STRING,
        normalized_label   STRING,
        label              STRING,
        source_file_key    INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS rule_terms (
        rule_key           INTEGER NOT NULL,
        rete_node_key      INTEGER NOT NULL,
        is_antecedent      BOOL,
        PRIMARY KEY (rule_key, rete_node_key)
      );
      CREATE TABLE IF NOT EXISTS rule_properties (
        rule_key           INTEGER NOT NULL,
        name               STRING NOT NULL,
        value              STRING
      );
      CREATE INDEX IF NOT EXISTS rule_properties_idx ON rule_properties (rule_key);

      -- --------------------
      -- expressions table
      -- --------------------
      -- type = {'binary', 'unary', 'resource', 'function'}
      -- when type == 'resource', arg0_key is resources.key
      CREATE TABLE IF NOT EXISTS expressions (
        key                INTEGER PRIMARY KEY,
        type               STRING NOT NULL,
        arg0_key           INTEGER,
        arg1_key           INTEGER,
        arg2_key           INTEGER,
        arg3_key           INTEGER,
        arg4_key           INTEGER,
        arg5_key           INTEGER,
        op                 STRING,
        source_file_key    INTEGER NOT NULL
      );

      -- --------------------
      -- rete_nodes table
      -- --------------------
      CREATE TABLE IF NOT EXISTS rete_nodes (
        key                INTEGER PRIMARY KEY,
        vertex             INTEGER NOT NULL,
        type               STRING NOT NULL,
        subject_key        INTEGER,
        predicate_key      INTEGER,
        object_key         INTEGER,
        obj_expr_key       INTEGER,
        filter_expr_key    INTEGER,
        normalizedLabel    STRING,
        parent_vertex      INTEGER,
        source_file_key    INTEGER NOT NULL,
        is_negation        INTEGER,
        salience           INTEGER,
        consequent_seq     INTEGER NOT NULL,
        UNIQUE (vertex, type, consequent_seq, source_file_key)
      );

      -- --------------------
      -- beta_row_config table
      -- --------------------
      CREATE TABLE IF NOT EXISTS beta_row_config (
        key                INTEGER PRIMARY KEY,
        vertex             INTEGER NOT NULL,
        seq                INTEGER NOT NULL,
        source_file_key    INTEGER NOT NULL,
        row_pos            INTEGER NOT NULL,
        is_binded          INTEGER,
        id                 STRING,
        UNIQUE (vertex, seq, source_file_key)
      );

      -- --------------------
      -- triples table
      -- --------------------
      CREATE TABLE IF NOT EXISTS triples (
        subject_key        INTEGER NOT NULL,
        predicate_key      INTEGER NOT NULL,
        object_key         INTEGER NOT NULL,
        source_file_key    INTEGER NOT NULL
      );
      CREATE INDEX IF NOT EXISTS triples_source_file_key_idx ON triples (source_file_key);

      -- --------------------
      -- schema_info table
      -- --------------------
      CREATE TABLE IF NOT EXISTS schema_info (
        version_major      INTEGER NOT NULL,
        version_minor      INTEGER NOT NULL,
        UNIQUE (version_major, version_minor)
      );
      INSERT INTO schema_info (version_major, version_minor) VALUES (1, 0)
        ON CONFLICT (version_major, version_minor) DO NOTHING;

```

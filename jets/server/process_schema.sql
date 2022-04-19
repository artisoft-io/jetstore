-- initial schema for server process

DROP TABLE IF EXISTS process_merge;
DROP TABLE IF EXISTS rule_config;
DROP TABLE IF EXISTS process_mapping;
DROP TABLE IF EXISTS process_input;

DROP TABLE IF EXISTS process_config;
CREATE TABLE IF NOT EXISTS process_config (
    key SERIAL PRIMARY KEY  ,
    client text  ,
    description text  ,
    main_entity_rdf_type text NOT NULL,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);

DROP TABLE IF EXISTS process_run;
CREATE TABLE IF NOT EXISTS process_run (
    key SERIAL PRIMARY KEY  ,
    process_config_key int NOT NULL ,
    workspace_db string NOT NULL ,
    lookup_db string ,
    note text  ,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);

CREATE TABLE IF NOT EXISTS process_input (
    key SERIAL PRIMARY KEY  ,
    process_key integer REFERENCES process_config ON DELETE CASCADE NOT NULL,
    input_table text  NOT NULL,
    entity_rdf_type text NOT NULL,
    grouping_column text NOT NULL,
    UNIQUE (process_key, input_table)
);
CREATE INDEX IF NOT EXISTS process_input_process_key_idx ON process_input (process_key);

CREATE TABLE IF NOT EXISTS process_mapping (
    process_input_key integer REFERENCES process_input ON DELETE CASCADE NOT NULL,
    input_column text  NOT NULL,
    data_property text  NOT NULL,
    function_name text  ,
    argument text  ,
    default_value text ,
    PRIMARY KEY (process_input_key, input_column, data_property)
);
CREATE INDEX IF NOT EXISTS process_mapping_process_input_key_idx ON process_mapping (process_input_key);

CREATE TABLE IF NOT EXISTS rule_config (
    process_key integer REFERENCES process_config ON DELETE CASCADE NOT NULL,
    subject text  NOT NULL,
    predicate text  NOT NULL,
    object text  NOT NULL,
    rdf_type text NOT NULL
);
CREATE INDEX IF NOT EXISTS rule_config_process_key_idx ON rule_config (process_key);

-- not implemented yet
CREATE TABLE IF NOT EXISTS process_merge (
    process_key integer REFERENCES process_config ON DELETE CASCADE ,
    entity_rdf_type text  NOT NULL,
    query_rdf_property_list text NOT NULL,
    grouping_rdf_property text NOT NULL
);
CREATE INDEX IF NOT EXISTS process_merge_process_key_idx ON process_merge (process_key);

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
    main_entity_rdf_type text ,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);

CREATE TABLE IF NOT EXISTS process_input (
    key SERIAL PRIMARY KEY  ,
    process_key integer REFERENCES process_config ON DELETE CASCADE ,
    input_table text  ,
    entity_rdf_type text ,
    UNIQUE (process_key, input_table)
);
CREATE INDEX IF NOT EXISTS process_input_process_key_idx ON process_input (process_key);

CREATE TABLE IF NOT EXISTS process_mapping (
    process_input_key integer REFERENCES process_input ON DELETE CASCADE ,
    input_column text  ,
    data_property text  ,
    function text  ,
    argument text  ,
    default_value text ,
    PRIMARY KEY (process_input_key, input_column, data_property)
);
CREATE INDEX IF NOT EXISTS process_mapping_process_input_key_idx ON process_mapping (process_input_key);

CREATE TABLE IF NOT EXISTS rule_config (
    process_key integer REFERENCES process_config ON DELETE CASCADE ,
    subject text  ,
    predicate text  ,
    object text  ,
    rdf_type text  ,
    default_value text 
);
CREATE INDEX IF NOT EXISTS rule_config_process_key_idx ON rule_config (process_key);

CREATE TABLE IF NOT EXISTS process_merge (
    process_key integer REFERENCES process_config ON DELETE CASCADE ,
    entity_rdf_type text  ,
    query_rdf_property_list text ,
    grouping_rdf_property text
);
CREATE INDEX IF NOT EXISTS process_merge_process_key_idx ON process_merge (process_key);

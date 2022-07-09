-- initial schema for database registry tables
CREATE SCHEMA IF NOT EXISTS jetsapi;

DROP TABLE IF EXISTS jetsapi.input_registry;

CREATE TABLE IF NOT EXISTS jetsapi.input_registry (
  file_name TEXT NOT NULL, 
  table_name TEXT NOT NULL, 
  session_id TEXT NOT NULL, 
  load_count INTEGER, 
  bad_row_count INTEGER, 
  node_id INTEGER DEFAULT 0 NOT NULL, 
  last_update timestamp without time zone DEFAULT now() NOT NULL, 
  UNIQUE (file_name, table_name, session_id)
);

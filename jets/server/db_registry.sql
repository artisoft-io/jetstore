-- initial schema for database registry tables

DROP TABLE IF EXISTS input_registry;

CREATE TABLE IF NOT EXISTS input_registry (
  file_name TEXT NOT NULL, 
  table_name TEXT NOT NULL, 
  session_id TEXT NOT NULL, 
  load_count INTEGER, 
  bad_row_count INTEGER, 
  node_id INTEGER DEFAULT 0 NOT NULL, 
  last_update timestamp without time zone DEFAULT now() NOT NULL, 
  UNIQUE (table_name, session_id)
);

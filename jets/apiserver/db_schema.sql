-- initial schema for database apiserver tables

DROP TABLE IF EXISTS users;

CREATE TABLE IF NOT EXISTS users (
  user_id SERIAL PRIMARY KEY, 
  name TEXT NOT NULL, 
  email TEXT NOT NULL, 
  password TEXT NOT NULL, 
  last_update timestamp without time zone DEFAULT now() NOT NULL,
  UNIQUE (email)
);

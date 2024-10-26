-- Jets Database Init Script

CREATE EXTENSION IF NOT EXISTS aws_s3 CASCADE;

-- Define the system user, needed to start pipeline automatically
INSERT INTO jetsapi.users (user_email, name, password, encrypted_roles) VALUES
  ('system', 'system', 'invalid', '{system_role}')
ON CONFLICT DO NOTHING 
;

-- Initialize mapping function here
TRUNCATE jetsapi.mapping_function_registry;
INSERT INTO jetsapi.mapping_function_registry (function_name, is_argument_required) VALUES
  ('apply_regex',            '1'),
  ('concat',                 '1'),
  ('concat_with',            '1'),
  ('format_phone',           '0'),
  ('overpunch_number',       '1'),
  ('parse_amount',           '1'),
  ('reformat0',              '1'),
  ('scale_units',            '1'),
  ('split_on',               '1'),
  ('to_upper',               '0'),
  ('to_zip5',                '0'),
  ('to_zipext4_from_zip9',   '0'),
  ('to_zipext4',             '0'),
  ('trim',                   '0'),
  ('unique_split_on',        '1'),
  ('validate_date',          '0'),
  ('substring',              '1'),
  ('find_and_replace',       '1')
;

-- Initialize roles table
TRUNCATE jetsapi.roles;
INSERT INTO jetsapi.roles (role, details) VALUES
  ('ops_user', 'Role to load files and execute pipelines'),
  ('client_advocate', 'Role to administer client configuration'),
  ('knowledge_engineer', 'Super user role to administer the JetStore workspace and client configuration'),
  ('system_role', 'System role needed to start pipeline automatically')
;

-- Initialize role_capability table
-- JetStore Capabilities:
-- 	- jetstore_read: read data in JetStore
-- 	- client_config: Add, modify client configuration
--	- workspace_ide: Access workspace IDE screens and functions, including query tool and git functions
--	- run_pipelines: Load files & execute pipelines
TRUNCATE jetsapi.role_capability;
INSERT INTO jetsapi.role_capability (role, capability) VALUES
  ('ops_user', 'jetstore_read'),
  ('ops_user', 'run_pipelines'),
  ('ops_user', 'user_profile'),
  ('client_advocate', 'jetstore_read'),
  ('client_advocate', 'client_config'),
  ('client_advocate', 'run_pipelines'),
  ('client_advocate', 'user_profile'),
  ('knowledge_engineer', 'jetstore_read'),
  ('knowledge_engineer', 'workspace_ide'),
  ('knowledge_engineer', 'client_config'),
  ('knowledge_engineer', 'run_pipelines'),
  ('knowledge_engineer', 'user_profile'),
  ('system_role', 'run_pipelines')
;

-- Creating the process_input_registry as a view:
CREATE OR REPLACE VIEW jetsapi.process_input_registry AS
SELECT 
  process_name || object_type || table_name || source_type AS key,
  process_name,
  (CASE WHEN client IS NULL THEN '' ELSE client END) AS client,
  (CASE WHEN org IS NULL THEN '' ELSE org END) AS org,
  object_type, 
  table_name, 
  source_type, 
  entity_rdf_type
FROM 
(
	SELECT process_name, unnest(input_rdf_types) as input_rdf_type FROM jetsapi.process_config
) AS pconfig, 
(
  SELECT sc.client, sc.org, UNNEST(sc.domain_keys) AS object_type, sc.table_name, text 'file' AS source_type, otr.entity_rdf_type
  FROM jetsapi.object_type_registry AS otr, jetsapi.source_config AS sc
  WHERE otr.object_type = sc.object_type
  UNION
  SELECT NULL as client, NULL as org, UNNEST(object_types) AS object_type, entity_rdf_type AS table_name, text 'domain_table' AS source_type, entity_rdf_type 
  FROM jetsapi.domain_keys_registry
  UNION
  SELECT NULL as client, NULL as org, UNNEST(object_types) AS object_type, entity_rdf_type AS table_name, text 'alias_domain_table' AS source_type, alias_rdf_type 
  FROM jetsapi.alias_rdf_type_registry
) AS registry
WHERE input_rdf_type = entity_rdf_type
;

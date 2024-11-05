-- Process Specific Jets Database Init Script

-- Define the ObjectType of the workspace with corresponding Domain Class
DELETE FROM jetsapi.object_type_registry WHERE object_type IN 
('TestLookup', 'TestLooping', 'HF_Person', 'FW_Thing');
INSERT INTO jetsapi.object_type_registry 
  (object_type,       entity_rdf_type,    domain_key_object_types,    details) VALUES
  ('TestLookup',        'tl:Patient',       '{TestLookup}',          'Test Lookup UC'),
  ('TestLooping',       'lp:Person',        '{TestLooping}',         'Test Looping UC'),
  ('HF_Person',         'hf:Person',        '{HF_Person}',           'Test CPIPES UC'),
  ('FW_Thing',          'fw:Thing',         '{FW_Thing}',            'Test CPIPES UC'),
  ('CSV_Thing',         'fw:Thing',         '{CSV_Thing}',           'Test CPIPES UC')
ON CONFLICT DO NOTHING 
;

-- Define the Domain Key associated with the persisted class
-- domain_keys_json provides the column names for computing the composite key associated with the object_type
-- domain_keys_json is needed for Domain Class created from the rules (e.g. scorecards)
-- Note: All output tables should have domain key associated with the process main object type
-- NOTE jetsapi.domain_keys_registry.entity_rdf_type SHOULD BE  CALLED:
--      jetsapi.domain_keys_registry.table_name (which is the same for domain_table but not for alias_table_name)
DELETE FROM jetsapi.domain_keys_registry WHERE entity_rdf_type IN 
('tl:Patient', 'lp:Person', 'hf:Person', 'fw:Thing');
INSERT INTO jetsapi.domain_keys_registry 
  (entity_rdf_type,   object_types,        domain_keys_json) VALUES
  ('tl:Patient',     '{"TestLookup"}',             '{"TestLookup":"jets:key","jets:hashing_override":"none"}'),
  ('lp:Person',      '{"TestLooping"}',            '{"TestLooping":"jets:key","jets:hashing_override":"none"}'),
  ('hf:Person',      '{"HF_Person"}',              '{"HF_Person":"jets:key","jets:hashing_override":"none"}'),
  ('fw:Thing',       '{"FW_Thing","CSV_Thing"}',   '{"FW_Thing":"jets:key","CSV_Thing":"jets:key","jets:hashing_override":"none"}')
ON CONFLICT DO NOTHING
;

-- Define the workspace processes
DELETE FROM jetsapi.process_config WHERE process_name IN 
('TestLookup', 'TestLooping', 'Test_HF_Analysis', 'Test_FW_Schema');
INSERT INTO jetsapi.process_config 
  (process_name,        main_rules,                       is_rule_set, input_rdf_types,  devmode_code,     state_machine_name,  output_tables, user_email) VALUES
  ('TestLookup',       'jet_rules/test_lookup_main.jr',          1,    '{tl:Patient}',  'run_server_only', 'serverv2SM',        '{tl:Patient}', 'admin'),
  ('TestLooping',      'jet_rules/test_looping_main.jr',         1,    '{lp:Person}',   'run_server_only', 'serverv2SM',        '{lp:Person}',  'admin'),
  ('Test_HF_Analysis', 'pipes_config/hf_token_analysis.pc.json', 1,    '{hf:Person}',   'run_cpipes_only', 'cpipesSM',          '{}',           'admin'),
  ('Test_FW_Schema',   'pipes_config/fixed_width_test.pc.json',  1,    '{fw:Thing}',    'run_cpipes_only', 'cpipesSM',          '{}',           'admin'),
  ('Test_CSV_Schema',  'pipes_config/csv_test.pc.json',          1,    '{fw:Thing}',    'run_cpipes_only', 'cpipesSM',          '{}',           'admin')
ON CONFLICT DO NOTHING
;

-- mapping_details and process_mapping not defined for hf:Person since this is not yet used by cpipes (comming soon!)
-- Define the minimal mapping spec
DELETE FROM jetsapi.object_type_mapping_details WHERE object_type = 'TestLookup';
INSERT INTO jetsapi.object_type_mapping_details (object_type, data_property, is_required) VALUES
  ('TestLookup', 'jets:key', '1'),
  ('TestLookup', 'tl:condition', '0'),
  ('TestLookup', 'tl:diagnosis', '0')
;
DELETE FROM jetsapi.object_type_mapping_details WHERE object_type = 'TestLooping';
INSERT INTO jetsapi.object_type_mapping_details (object_type, data_property, is_required) VALUES
  ('TestLooping', 'jets:key', '1'),
  ('TestLooping', 'lp:name', '0')
;

-- This is not really needed since we are not having process using Domain Table as input
DELETE FROM jetsapi.process_mapping WHERE table_name = 'tl:Patient';
INSERT INTO jetsapi.process_mapping (table_name, data_property, input_column, function_name, argument, default_value, error_message, user_email) VALUES
  ('tl:Patient', 'rdf:type',    'rdf:type',     NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:client', 'jets:client',  NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:org',    'jets:org',     NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:key',    'jets:key',     NULL, NULL, NULL, NULL, 'admin')
;

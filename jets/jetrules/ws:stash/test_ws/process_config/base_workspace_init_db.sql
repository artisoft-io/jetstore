-- Process Specific Jets Database Init Script

-- Define the ObjectType of the workspace with corresponding Domain Class
DELETE FROM jetsapi.object_type_registry WHERE object_type IN ('TestLookup');
INSERT INTO jetsapi.object_type_registry (object_type, entity_rdf_type, domain_key_object_types, details) VALUES
  ('TestLookup',                'tl:Patient',                  '{TestLookup}',  'Test Lookup UC')
ON CONFLICT DO NOTHING 
;

-- Define the workspace processes
DELETE FROM jetsapi.process_config WHERE process_name IN ('TestLookup');
INSERT INTO jetsapi.process_config 
  (process_name,   main_rules,                      is_rule_set, input_rdf_types,  output_tables, user_email) VALUES
  ('TestLookup',  'jet_rules/test_lookup_main.jr',  1,           '{tl:Patient}',  '{tl:Patient}', 'admin')
ON CONFLICT DO NOTHING
;

-- Define the minimal mapping spec
DELETE FROM jetsapi.process_mapping WHERE table_name = 'tl:Patient';
INSERT INTO jetsapi.process_mapping (table_name, data_property, input_column, function_name, argument, default_value, error_message, user_email) VALUES
  ('tl:Patient', 'rdf:type',    'rdf:type',     NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:client', 'jets:client',  NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:org',    'jets:org',     NULL, NULL, NULL, NULL, 'admin'),
  ('tl:Patient', 'jets:key',    'jets:key',     NULL, NULL, NULL, NULL, 'admin')
;

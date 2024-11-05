-- =============================================================================================================
-- UTest
-- generated on 2024-11-04 16:07:59.495309267 -0500 EST m=+4797.561954749
-- =============================================================================================================
-- Jets Database Init Script

-- Table client_registry
DELETE FROM jetsapi."client_registry" WHERE "client" = 'UTest';
INSERT INTO jetsapi."client_registry" (client,details) VALUES
  ('UTest', 'Unit test client for test_ws')
ON CONFLICT DO NOTHING;


-- Table client_org_registry
DELETE FROM jetsapi."client_org_registry" WHERE "client" = 'UTest';
INSERT INTO jetsapi."client_org_registry" (client,org,details) VALUES
  ('UTest', 'Test', NULL)
ON CONFLICT DO NOTHING;


-- Table source_config
DELETE FROM jetsapi."source_config" WHERE "client" = 'UTest';
INSERT INTO jetsapi."source_config" (object_type,client,org,automated,table_name,domain_keys_json,domain_keys,code_values_mapping_json,input_columns_json,input_columns_positions_csv,input_format,compression,input_format_data_json,is_part_files,compute_pipes_json,user_email) VALUES
  ('TestLookup',
   'UTest',
   'Test',
   0,
   'UTest_Test_TestLookup',
   NULL,
   '{TestLookup}',
   NULL,
   NULL,
   NULL,
   'csv',
   'none',
   '',
   0,
   NULL,
   'michel@artisoft.io'),
  ('FW_Thing',
   'UTest',
   'Test',
   0,
   'UTest_Test_FW_Thing',
   NULL,
   '{FW_Thing}',
   NULL,
   NULL,
   NULL,
   'fixed_width',
   'none',
   '',
   0,
   NULL,
   'michel@artisoft.io'),
  ('TestLooping',
   'UTest',
   'Test',
   0,
   'UTest_Test_TestLooping',
   NULL,
   '{TestLooping}',
   NULL,
   NULL,
   NULL,
   'csv',
   'none',
   '',
   0,
   NULL,
   'michel@artisoft.io'),
  ('HF_Person',
   'UTest',
   'Test',
   0,
   'UTest_Test_HF_Person',
   NULL,
   '{HF_Person}',
   NULL,
   NULL,
   NULL,
   'csv',
   'none',
   '',
   0,
   NULL,
   'michel@artisoft.io')
ON CONFLICT DO NOTHING;


-- Table rule_config
DELETE FROM jetsapi."rule_config" WHERE "client" = 'UTest';
-- Table rule_config has no row for client = UTest

-- Table rule_configv2
DELETE FROM jetsapi."rule_configv2" WHERE "client" = 'UTest';
-- Table rule_configv2 has no row for client = UTest

-- Table process_input
DELETE FROM jetsapi."process_input" WHERE "client" = 'UTest';
INSERT INTO jetsapi."process_input" (key,client,org,object_type,table_name,source_type,lookback_periods,entity_rdf_type,key_column,status,user_email) VALUES
  (320667, 'UTest', 'Test', 'TestLookup', 'UTest_Test_TestLookup', 'file', 0, 'tl:Patient', NULL, 'created', 'michel@artisoft.io'),
  (320668, 'UTest', 'Test', 'TestLooping', 'UTest_Test_TestLooping', 'file', 0, 'lp:Person', NULL, 'created', 'michel@artisoft.io'),
  (320669, 'UTest', 'Test', 'HF_Person', 'UTest_Test_HF_Person', 'file', 0, 'hf:Person', NULL, 'created', 'michel@artisoft.io'),
  (320700, 'UTest', 'Test', 'FW_Thing', 'UTest_Test_FW_Thing', 'file', 0, 'fw:Thing', NULL, 'created', 'michel@artisoft.io')
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('jetsapi.process_input', 'key'), max(key)) FROM jetsapi.process_input;

-- Table pipeline_config
DELETE FROM jetsapi."pipeline_config" WHERE "client" = 'UTest';
INSERT INTO jetsapi."pipeline_config" (process_name,client,process_config_key,main_process_input_key,merged_process_input_keys,injected_process_input_keys,main_object_type,main_source_type,source_period_type,automated,max_rete_sessions_saved,rule_config_json,description,user_email) VALUES
  ('TestLookup',
   'UTest',
   70,
   320667,
   '{}',
   '{}',
   'TestLookup',
   'file',
   'month_period',
   0,
   0,
   '[]',
   'Unit testing lookups',
   'michel@artisoft.io'),
  ('Test_FW_Schema',
   'UTest',
   106,
   320700,
   '{}',
   '{}',
   'FW_Thing',
   'file',
   'month_period',
   0,
   0,
   '[]',
   'Test cpipes with fixed_width file',
   'michel@artisoft.io'),
  ('TestLooping',
   'UTest',
   74,
   320668,
   '{}',
   '{}',
   'TestLooping',
   'file',
   'month_period',
   0,
   0,
   '[]',
   'Unit testing looping',
   'michel@artisoft.io'),
  ('Test_HF_Analysis',
   'UTest',
   69,
   320669,
   '{}',
   '{}',
   'HF_Person',
   'file',
   'month_period',
   0,
   0,
   '[]',
   'Unit test cpipes with schema provider',
   'michel@artisoft.io')
ON CONFLICT DO NOTHING;


-- Table process_mapping
DELETE FROM jetsapi."process_mapping" WHERE "table_name" = 'UTest_Test_TestLookup';
INSERT INTO jetsapi."process_mapping" (table_name,data_property,input_column,function_name,argument,default_value,error_message,user_email) VALUES
  ('UTest_Test_TestLookup', 'jets:key', 'jets:key', NULL, NULL, NULL, NULL, 'michel@artisoft.io'),
  ('UTest_Test_TestLookup', 'tl:condition', NULL, NULL, NULL, NULL, NULL, 'michel@artisoft.io'),
  ('UTest_Test_TestLookup', 'tl:diagnosis', NULL, NULL, NULL, NULL, NULL, 'michel@artisoft.io')
ON CONFLICT DO NOTHING;


-- Table process_mapping
DELETE FROM jetsapi."process_mapping" WHERE "table_name" = 'UTest_Test_FW_Thing';
-- Table process_mapping has no row for table_name = UTest_Test_FW_Thing

-- Table process_mapping
DELETE FROM jetsapi."process_mapping" WHERE "table_name" = 'UTest_Test_TestLooping';
INSERT INTO jetsapi."process_mapping" (table_name,data_property,input_column,function_name,argument,default_value,error_message,user_email) VALUES
  ('UTest_Test_TestLooping', 'jets:key', 'jets:key', NULL, NULL, NULL, NULL, 'michel@artisoft.io'),
  ('UTest_Test_TestLooping', 'lp:name', NULL, NULL, NULL, NULL, NULL, 'michel@artisoft.io')
ON CONFLICT DO NOTHING;


-- Table process_mapping
DELETE FROM jetsapi."process_mapping" WHERE "table_name" = 'UTest_Test_HF_Person';
-- Table process_mapping has no row for table_name = UTest_Test_HF_Person

-- End of Export Client Script

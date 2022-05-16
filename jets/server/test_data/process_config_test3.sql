-- TRUNCATE TABLE process_config, process_input, process_mapping, rule_config, process_merge;
DELETE FROM process_config WHERE key in (4);

INSERT INTO process_config (key, client, description, main_entity_rdf_type) VALUES
  (4, 'TEST3', 'TEST for exist_not', 'hc:Test_3_Claim')
RETURNING key;

INSERT INTO process_input (key, process_key, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (4, 4, 'test3', 'hc:Test_3_Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER')
;

INSERT INTO process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (4, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL),
  (4, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (4, 'MEMBER_ZIP', 'hc:member_zip', 'to_zip5', NULL, NULL, NULL),
  (4, 'PROVIDER_ZIP', 'hc:provider_zip', 'to_zip5', NULL, NULL, NULL)
;



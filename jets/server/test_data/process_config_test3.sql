-- TRUNCATE TABLE jetsapi.pipeline_config, jetsapi.process_input, jetsapi.process_mapping, jetsapi.rule_config, jetsapi.process_merge;
DELETE FROM jetsapi.pipeline_config WHERE key in (301);

INSERT INTO jetsapi.pipeline_config (key, client, description, main_entity_rdf_type) VALUES
  (301, 'TEST3', 'TEST for not operator', 'hc:TestClaim')
RETURNING key;

INSERT INTO jetsapi.process_input (key, process_key, input_type, table_name, entity_rdf_type, grouping_column, key_column) VALUES
  (321, 301, 0, 'test3', 'hc:TestClaim', 'MEMBER_NUMBER', 'CLAIM_NUMBER')
;

INSERT INTO jetsapi.process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (321, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL),
  (321, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (321, 'CODE', 'hc:code', NULL, NULL, NULL, NULL)
;

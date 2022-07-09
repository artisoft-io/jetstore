-- TRUNCATE TABLE jetsapi.process_config, jetsapi.process_input, jetsapi.process_mapping, jetsapi.rule_config, jetsapi.process_merge;
DELETE FROM jetsapi.process_config WHERE key in (101, 102);

INSERT INTO jetsapi.process_config (key, client, description, main_entity_rdf_type) VALUES
  (101, 'ACME', 'Lookup ACME Service Code with Modifier', 'hc:Claim'),
  (102, 'ACME', 'Lookup ACME Service Code without Modifier', 'hc:Claim')
RETURNING key;

INSERT INTO jetsapi.process_input (key, process_key, input_type, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (110, 101, 0, 'test1', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER'),
  (120, 102, 0, 'test1', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER')
;

INSERT INTO jetsapi.process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (110, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL),
  (110, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (110, 'DOS', 'hc:date_of_service', NULL, NULL, NULL, NULL),
  (110, 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL, NULL),
  (110, 'MODIFIER', 'hc:modifier', NULL, NULL, NULL, NULL),
  (110, 'NDAYS', 'hc:ndays', 'apply_regex', '(\d)+.*', '-1', NULL),
  (110, 'SUBMITTED_AMT', 'hc:submitted_amount', 'parse_amount', '10', '99', NULL),
  (110, 'ALLOWED_AMT'  , 'hc:allowed_amount', 'parse_amount', '1', NULL, 'Input amounts cannot be null'),
  (120, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL),
  (120, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (120, 'DOS', 'hc:date_of_service', NULL, NULL, NULL, NULL),
  (120, 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL, NULL),
  (120, 'MODIFIER', 'hc:modifier', NULL, NULL, NULL, NULL),
  (120, 'NDAYS', 'hc:ndays', 'apply_regex', '(\d)+.*', '-1', NULL),
  (120, 'SUBMITTED_AMT', 'hc:submitted_amount', 'parse_amount', '10', '99', NULL),
  (120, 'ALLOWED_AMT'  , 'hc:allowed_amount', 'parse_amount', '1', NULL, 'Input amounts cannot be null')
;

INSERT INTO jetsapi.rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (101, 'jets:iState', 'lk:withModifier', 'true', 'bool'),
  (102, 'jets:iState', 'lk:withModifier', 'false', 'bool')
;


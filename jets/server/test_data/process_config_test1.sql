TRUNCATE TABLE process_config, process_input, process_mapping, rule_config, process_merge;

INSERT INTO process_config (key, client, description, main_entity_rdf_type) VALUES
  (1, 'ACME', 'Lookup ACME Service Code with Modifier', 'hc:Claim'),
  (3, 'ACME', 'Lookup ACME Service Code without Modifier', 'hc:Claim'),
  (2, 'M2C', 'Test M2C file', 'm2c:Claim')
RETURNING key;

INSERT INTO process_input (key, process_key, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (1, 1, 'test1', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER'),
  (3, 3, 'test1', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER'),
  (2, 2, 'm2c__input1', 'm2c:Claim', 'COL2', NULL)
;

INSERT INTO process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value) VALUES
  (1, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL),
  (1, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL),
  (1, 'DOS', 'hc:date_of_service', NULL, NULL, NULL),
  (1, 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL),
  (1, 'MODIFIER', 'hc:modifier', NULL, NULL, NULL),
  (1, 'NDAYS', 'hc:ndays', NULL, NULL, NULL),
  (3, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL),
  (3, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL),
  (3, 'DOS', '_0:date_of_service', NULL, NULL, NULL),
  (3, 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL),
  (3, 'MODIFIER', 'hc:modifier', NULL, NULL, NULL),
  (3, 'NDAYS', 'hc:ndays', NULL, NULL, NULL),
  (2, 'COL1', 'm2c:P1', 'regex', '\d{3}', NULL),
  (2, 'COL2', 'm2c:P2', NULL, NULL, NULL),
  (2, 'COL2', 'm2c:P3', 'parse_amount', '100', '0')
;

INSERT INTO rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (1, 'jets:iState', 'lk:withModifier', 'true', 'bool'),
  (3, 'jets:iState', 'lk:withModifier', 'false', 'bool'),
  (2, 'subject2', 'predicate1', 'object1', 'int'),
  (2, 'subject2', 'predicate2', 'object2', 'date')
;

INSERT INTO process_merge (process_key, entity_rdf_type, query_rdf_property_list, grouping_rdf_property) VALUES
  (2, 'm2c:Claim', 'm2c:P1,m2c:P2', 'm2c:P2')
;


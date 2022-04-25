TRUNCATE TABLE process_config, process_input, process_mapping, rule_config, process_merge;

INSERT INTO process_config (key, client, description, main_entity_rdf_type) VALUES
  (1, 'ACME', 'Test ACME file', 'hc:Claim'),
  (2, 'M2C', 'Test M2C file', 'm2c:Claim')
RETURNING key;

INSERT INTO process_input (key, process_key, input_table, entity_rdf_type, grouping_column) VALUES
  (1, 1, 'test1', 'hc:Claim', 'MEMBER_NUMBER'),
  (2, 2, 'm2c__input1', 'm2c:Claim', 'COL2')
;

INSERT INTO process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value) VALUES
  (1, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL),
  (1, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL),
  (1, 'DOS', 'hc:date_of_service', NULL, NULL, NULL),
  (1, 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL),
  (1, 'MODIFIER', 'hc:modifier', NULL, NULL, NULL),
  (2, 'COL1', 'm2c:P1', 'regex', '\d{3}', NULL),
  (2, 'COL2', 'm2c:P2', NULL, NULL, NULL),
  (2, 'COL2', 'm2c:P3', 'parse_amount', '100', '0')
;

INSERT INTO rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (1, 'iState', 'lk:withModifier', 'true', 'bool'),
  (2, 'subject2', 'predicate1', 'object1', 'int'),
  (2, 'subject2', 'predicate2', 'object2', 'date')
;

INSERT INTO process_merge (process_key, entity_rdf_type, query_rdf_property_list, grouping_rdf_property) VALUES
  (2, 'm2c:Claim', 'm2c:P1,m2c:P2', 'm2c:P2')
;


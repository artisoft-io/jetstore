-- TRUNCATE TABLE process_config, process_input, process_mapping, rule_config, process_merge;
DELETE FROM process_config WHERE key in (201);

INSERT INTO process_config (key, client, description, main_entity_rdf_type) VALUES
  (201, 'ACME', 'Testing exist_not with looping and negation', 'hc:Claim')
RETURNING key;

INSERT INTO process_input (key, process_key, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (201, 201, 'test2', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER')
;

INSERT INTO process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (201, 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL),
  (201, 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (201, 'ZIP', 'hc:zip', NULL, NULL, NULL, NULL)
;

INSERT INTO rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (201, 'asim:KEY001', 'rdf:type', 'aspec:Entity', 'resource'),
  (201, 'asim:KEY001', 'aspec:patient_persona_lk_key', 'lk:BasePatientPersona', 'text'),
  (201, 'asim:KEY001', 'aspec:nbr_patients', '1000', 'int'),
  (201, 'asim:KEY001', 'aspec:patient_key_prefix', '02001', 'text')
;

-- INSERT INTO process_merge (process_key, entity_rdf_type, query_rdf_property_list, grouping_rdf_property) VALUES
--   (2, 'm2c:Claim', 'm2c:P1,m2c:P2', 'm2c:P2')
-- ;


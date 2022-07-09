-- TRUNCATE TABLE jetsapi.process_config, jetsapi.process_input, jetsapi.process_mapping, jetsapi.rule_config, jetsapi.process_merge;
DELETE FROM jetsapi.process_config WHERE key in (201, 202, 203);

INSERT INTO jetsapi.process_config (key, client, description, main_entity_rdf_type) VALUES
  (201, 'ACME', 'Input mapping for aspec:Simulator', 'aspec:Simulator'),
  (202, 'ACME', 'Entity from hc:SimulatedPatient', 'hc:SimulatedPatient'),
  (203, 'ACME', 'Entity Merge from hc:SimulatedPatient', 'hc:SimulatedPatient')
;

INSERT INTO jetsapi.process_input (key, process_key, input_type, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (221, 201, 0, 'test2', 'aspec:Simulator', 'key', 'key'),
  (222, 202, 1, 'hc:SimulatedPatient', 'hc:SimulatedPatient', 'hc:patient_number', 'jets:key'),
  (230, 203, 1, 'hc:SimulatedPatient', 'hc:SimulatedPatient', 'hc:patient_number', 'jets:key'),
  (231, 203, 1, 'hc:ProfessionalClaim', 'hc:ProfessionalClaim', 'hc:member_number', 'jets:key'),
  (232, 203, 1, 'hc:InstitutionalClaim', 'hc:InstitutionalClaim', 'hc:member_number', 'jets:key')
;

INSERT INTO jetsapi.process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (221, 'key', 'jets:key', NULL, NULL, NULL, NULL),
  (221, 'anchor_date', 'aspec:anchor_date', NULL, NULL, NULL, NULL),
  (221, 'nbr_entities', 'aspec:nbr_entities', NULL, NULL, NULL, NULL),
  (221, 'entity_key_prefix', 'aspec:entity_key_prefix', NULL, NULL, NULL, NULL),
  (221, 'entity_persona_lk', 'aspec:entity_persona_lk', NULL, NULL, NULL, NULL),
  (222, 'asim:anchor_date', 'asim:anchor_date', NULL, NULL, NULL, NULL),
  (222, 'asim:persona_key', 'asim:persona_key', NULL, NULL, NULL, NULL),
  (222, 'asim:demographic_group_key', 'asim:demographic_group_key', NULL, NULL, NULL, NULL),
  (222, 'asim:event_group1_lk', 'asim:event_group1_lk', NULL, NULL, NULL, NULL),
  (222, 'asim:description', 'asim:description', NULL, NULL, NULL, NULL),
  (222, 'hc:patient_number', 'hc:patient_number', NULL, NULL, NULL, NULL),
  (222, 'hc:dob', 'hc:dob', NULL, NULL, NULL, NULL),
  (222, 'hc:gender', 'hc:gender', NULL, NULL, NULL, NULL),
  (222, 'asim:claim_group_lk', 'asim:claim_group_lk', NULL, NULL, NULL, NULL),
  (222, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (222, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL),
  (230, 'hc:patient_number', 'hc:patient_number', NULL, NULL, NULL, NULL),
  (230, 'hc:dob', 'hc:dob', NULL, NULL, NULL, NULL),
  (230, 'hc:gender', 'hc:gender', NULL, NULL, NULL, NULL),
  (230, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (230, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL),
  (231, 'hc:member_number', 'hc:member_number', NULL, NULL, NULL, NULL),
  (231, 'hc:claim_number', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (231, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (231, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL),
  (232, 'hc:member_number', 'hc:member_number', NULL, NULL, NULL, NULL),
  (232, 'hc:claim_number', 'hc:claim_number', NULL, NULL, NULL, NULL),
  (232, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (232, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL)
;

INSERT INTO jetsapi.rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (201, 'jets:iState', 'rdf:type', 'jets:State', 'resource')
;

-- INSERT INTO jetsapi.process_merge (process_key, entity_rdf_type, query_rdf_property_list, grouping_rdf_property) VALUES
--   (2, 'm2c:Claim', 'm2c:P1,m2c:P2', 'm2c:P2')
-- ;


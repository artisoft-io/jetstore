DELETE FROM jetsapi.pipeline_config WHERE key in (201, 202, 203);
DELETE FROM jetsapi.process_config WHERE key in (251, 252, 255);
DELETE FROM jetsapi.process_input WHERE table_name in ('test2');
DELETE FROM jetsapi.rule_config WHERE process_config_key in (251, 252, 255);

INSERT INTO jetsapi.pipeline_config (key, client, description, process_config_key, process_name, main_table_name, merged_in_table_names, user_email) VALUES
  (201, 'Zeme', 'Input mapping for aspec:Simulator', 251, 'Gen02', 'test2',                   '{}', 'user@mail.com'),
  (202, 'Zeme', 'Entity from hc:SimulatedPatient', 252, 'Gen02', 'hc:SimulatedPatient',       '{}', 'user@mail.com'),
  (203, 'Zeme', 'Entity Merge from hc:SimulatedPatient', 253, 'Gen02', 'hc:SimulatedPatient', '{hc:ProfessionalClaim,hc:InstitutionalClaim}', 'user@mail.com'),
  (205, 'Zeme', 'From simulation to inferrence', 255, 'Gen02', 'test2', '{}', 'user@mail.com')
;

INSERT INTO jetsapi.process_config (key, process_name, main_rules, is_rule_set, output_tables, user_email) VALUES
  (251, 'PROC02-1', 'test2_ruleset1.jr', 1, '{hc:SimulatedPatient}', 'user@mail.com'),
  (252, 'PROC02-2', 'test2_ruleset2.jr', 1, '{hc:SimulatedClaim,hc:Claim,hc:ProfessionalClaim,hc:InstitutionalClaim}', 'user@mail.com'),
  (253, 'PROC02-3', 'test2_ruleset3.jr', 1, '{hc:PatientAdum}', 'user@mail.com'),
  (255, 'PROC02-ALL', 'all_uuid', 0, '{hc:PatientAdum,hc:SimulatedClaim,hc:Claim,hc:ProfessionalClaim,hc:InstitutionalClaim,hc:SimulatedPatient,hc:Patient}', 'user@mail.com')
;

INSERT INTO jetsapi.process_input (table_name, client, source_type, entity_rdf_type, grouping_column, key_column, user_email) VALUES
  ('test2',                'Zeme', 'file', 'aspec:Simulator', 'key', 'key', 'user@mail.com'),
  ('hc:SimulatedPatient',  'Zeme', 'domain_table', 'hc:SimulatedPatient', 'hc:patient_number', 'jets:key', 'user@mail.com'),
  ('hc:ProfessionalClaim', 'Zeme', 'domain_table', 'hc:ProfessionalClaim', 'hc:member_number', 'jets:key', 'user@mail.com'),
  ('hc:InstitutionalClaim','Zeme', 'domain_table', 'hc:InstitutionalClaim', 'hc:member_number', 'jets:key', 'user@mail.com')
;

INSERT INTO jetsapi.process_mapping (table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) VALUES
  ('test2', 'key', 'jets:key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test2', 'anchor_date', 'aspec:anchor_date', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test2', 'nbr_entities', 'aspec:nbr_entities', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test2', 'entity_key_prefix', 'aspec:entity_key_prefix', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test2', 'entity_persona_lk', 'aspec:entity_persona_lk', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:anchor_date', 'asim:anchor_date', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:persona_key', 'asim:persona_key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:demographic_group_key', 'asim:demographic_group_key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:event_group1_lk', 'asim:event_group1_lk', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:description', 'asim:description', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'hc:patient_number', 'hc:patient_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'hc:dob', 'hc:dob', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'hc:gender', 'hc:gender', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'asim:claim_group_lk', 'asim:claim_group_lk', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'jets:key', 'jets:key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient', 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim', 'hc:member_number', 'hc:member_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim', 'hc:claim_number', 'hc:claim_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim', 'jets:key', 'jets:key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim', 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'hc:member_number', 'hc:member_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'hc:claim_number', 'hc:claim_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'jets:key', 'jets:key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL, 'user@mail.com')
;

INSERT INTO jetsapi.rule_config (process_config_key, process_name, client, subject, predicate, object, rdf_type) VALUES
  (251, 'PROC02-1', 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource'),
  (252, 'PROC02-2', 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource'),
  (255, 'PROC02-ALL', 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource')
;


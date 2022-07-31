DELETE FROM jetsapi.source_config WHERE key in (291);
DELETE FROM jetsapi.process_input WHERE key in (271, 272, 273, 274);
DELETE FROM jetsapi.process_mapping WHERE process_input_key in (271, 272, 273, 274);
DELETE FROM jetsapi.process_config WHERE key in (251, 252, 253, 254);
DELETE FROM jetsapi.rule_config    WHERE process_config_key in (251, 252, 253, 254);
DELETE FROM jetsapi.pipeline_config WHERE key in (201, 202, 203, 204);
DELETE FROM jetsapi.pipeline_execution_status WHERE pipeline_config_key in (201, 202, 203, 204);

INSERT INTO jetsapi.source_config (key, object_type, client, table_name, grouping_column, user_email) VALUES
  (291, 'Simulator', 'Zeme', 'test2', NULL, 'user@mail.com')
;

INSERT INTO jetsapi.process_input (key, client, object_type, table_name, source_type, entity_rdf_type, grouping_column, key_column, user_email) VALUES
  (271, 'Zeme', 'Simulator'         , 'test2'                , 'file',         'aspec:Simulator'      , NULL               , NULL , 'user@mail.com'),
  (272, 'Zeme', 'SimulatedPatient'  , 'hc:SimulatedPatient'  , 'domain_table', 'hc:SimulatedPatient'  , 'hc:patient_number', NULL , 'user@mail.com'),
  (273, 'Zeme', 'ProfessionalClaim' , 'hc:ProfessionalClaim' , 'domain_table', 'hc:ProfessionalClaim' , 'hc:member_number' , NULL , 'user@mail.com'),
  (274, 'Zeme', 'InstitutionalClaim', 'hc:InstitutionalClaim', 'domain_table', 'hc:InstitutionalClaim', 'hc:member_number' , NULL , 'user@mail.com')
;

INSERT INTO jetsapi.process_mapping (process_input_key, table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) VALUES
  (271, 'test2'                , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (271, 'test2'                , 'anchor_date'               , 'aspec:anchor_date'         , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (271, 'test2'                , 'nbr_entities'              , 'aspec:nbr_entities'        , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (271, 'test2'                , 'entity_key_prefix'         , 'aspec:entity_key_prefix'   , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (271, 'test2'                , 'entity_persona_lk'         , 'aspec:entity_persona_lk'   , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:anchor_date'          , 'asim:anchor_date'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:persona_key'          , 'asim:persona_key'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:demographic_group_key', 'asim:demographic_group_key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:event_group1_lk'      , 'asim:event_group1_lk'      , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:description'          , 'asim:description'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'hc:patient_number'         , 'hc:patient_number'         , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'hc:dob'                    , 'hc:dob'                    , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'hc:gender'                 , 'hc:gender'                 , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'asim:claim_group_lk'       , 'asim:claim_group_lk'       , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (272, 'hc:SimulatedPatient'  , 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (273, 'hc:ProfessionalClaim' , 'hc:member_number'          , 'hc:member_number'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (273, 'hc:ProfessionalClaim' , 'hc:claim_number'           , 'hc:claim_number'           , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (273, 'hc:ProfessionalClaim' , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (273, 'hc:ProfessionalClaim' , 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (274, 'hc:InstitutionalClaim', 'hc:member_number'          , 'hc:member_number'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (274, 'hc:InstitutionalClaim', 'hc:claim_number'           , 'hc:claim_number'           , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (274, 'hc:InstitutionalClaim', 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  (274, 'hc:InstitutionalClaim', 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com')
;

INSERT INTO jetsapi.process_config (key, process_name, main_rules, is_rule_set, output_tables, user_email) VALUES
  (251, 'PatientGen' , 'test2_ruleset1.jr', 1, '{aspec:Simulator,hc:SimulatedPatient}', 'user@mail.com'),
  (252, 'ClaimGen'   , 'test2_ruleset2.jr', 1, '{hc:SimulatedClaim,hc:Claim,hc:ProfessionalClaim,hc:InstitutionalClaim}', 'user@mail.com'),
  (253, 'PatientAdum', 'test2_ruleset3.jr', 1, '{hc:PatientAdum}', 'user@mail.com'),
  (254, 'AllE2E'     , 'all_uuid'         , 0, '{hc:PatientAdum,hc:SimulatedClaim,hc:Claim,hc:ProfessionalClaim,hc:InstitutionalClaim,hc:SimulatedPatient,hc:Patient}', 'user@mail.com')
;

INSERT INTO jetsapi.rule_config (process_config_key, process_name, client, subject, predicate, object, rdf_type) VALUES
  (251, 'PatientGen' , 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource'),
  (252, 'ClaimGen'   , 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource'),
  (253, 'PatientAdum', 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource'),
  (254, 'AllE2E'     , 'Zeme', 'jets:iState', 'rdf:type', 'jets:State', 'resource')
;

INSERT INTO jetsapi.pipeline_config (key, process_name, client, process_config_key, main_process_input_key, merged_process_input_keys, main_table_name, user_email) VALUES
  (201, 'PatientGen' , 'Zeme', 251, 271, '{}'        , 'test2'              , 'user@mail.com'),
  (202, 'ClaimGen'   , 'Zeme', 252, 272, '{}'        , 'hc:SimulatedPatient', 'user@mail.com'),
  (203, 'PatientAdum', 'Zeme', 253, 272, '{273, 274}', 'hc:SimulatedPatient', 'user@mail.com'),
  (204, 'AllE2E'     , 'Zeme', 254, 271, '{}'        , 'test2'              , 'user@mail.com')
;

INSERT INTO jetsapi.pipeline_execution_status (key, pipeline_config_key, process_name, client, main_input_registry_key, merged_input_registry_keys, input_session_id, session_id, status, user_email) VALUES
  (281, 204, 'AllE2E', 'Zeme', 1, '{}', NULL, '1230789', 'in progress', 'user@mail.com')
;


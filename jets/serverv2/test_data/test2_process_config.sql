DELETE FROM jetsapi.source_config WHERE key in (291);
DELETE FROM jetsapi.process_input WHERE key in (271, 272, 273, 274);
DELETE FROM jetsapi.process_mapping WHERE table_name in ('Zeme_Simulator','hc:SimulatedPatient','hc:ProfessionalClaim','hc:InstitutionalClaim');
DELETE FROM jetsapi.process_config WHERE key in (251, 252, 253, 254);
DELETE FROM jetsapi.rule_config    WHERE process_config_key in (251, 252, 253, 254);
DELETE FROM jetsapi.pipeline_config WHERE key in (201, 202, 203, 204);
DELETE FROM jetsapi.pipeline_execution_status WHERE pipeline_config_key in (201, 202, 203, 204);

-- INSERT INTO jetsapi.source_config (key, object_type, client, table_name, grouping_column, user_email) VALUES
--   (291, 'Simulator', 'Zeme', 'Zeme_Simulator', NULL, 'user@mail.com')
-- ;

TRUNCATE jetsapi.client_registry;
INSERT INTO jetsapi.client_registry (client, details) VALUES
  ('Zeme', 'Unit test client'),
  ('cHealth', NULL),
  ('dHealth', 'Some client'),
  ('pHealth', NULL)
;


TRUNCATE jetsapi.object_type_registry;
INSERT INTO jetsapi.object_type_registry (object_type, entity_rdf_type, details) VALUES
  ('Simulator', 'aspec:Simulator', 'Healthcare Simulated Data Generation'),
  ('SimulatedPatient', 'hc:SimulatedPatient', 'Healthcare SimulatedPatient'),
  ('ProfessionalClaim', 'hc:ProfessionalClaim', 'Healthcare ProfessionalClaim'),
  ('InstitutionalClaim', 'hc:InstitutionalClaim', 'Healthcare InstitutionalClaim'),
  ('Claim', 'hc:Claim', 'Healthcare Claim'),
  ('Network', 'hc:NetworkTransparency', 'Network Transparency file')
;

TRUNCATE jetsapi.object_type_mapping_details;
INSERT INTO jetsapi.object_type_mapping_details (object_type, data_property, is_required) VALUES
  ('Claim', 'hc:serviceDate', '1'),
  ('Claim', 'hc:procedureCode', '1'),
  ('Simulator'         , 'jets:key'                  , '1'),
  ('Simulator'         , 'aspec:anchor_date'         , '1'),
  ('Simulator'         , 'aspec:nbr_entities'        , '1'),
  ('Simulator'         , 'aspec:entity_key_prefix'   , '0'),
  ('Simulator'         , 'aspec:entity_persona_lk'   , '0'),
  ('SimulatedPatient'  , 'asim:anchor_date'          , '1'),
  ('SimulatedPatient'  , 'asim:persona_key'          , '1'),
  ('SimulatedPatient'  , 'asim:demographic_group_key', '1'),
  ('SimulatedPatient'  , 'asim:event_group1_lk'      , '0'),
  ('SimulatedPatient'  , 'asim:description'          , '1'),
  ('SimulatedPatient'  , 'hc:patient_number'         , '1'),
  ('SimulatedPatient'  , 'hc:dob'                    , '1'),
  ('SimulatedPatient'  , 'hc:gender'                 , '0'),
  ('SimulatedPatient'  , 'asim:claim_group_lk'       , '1'),
  ('SimulatedPatient'  , 'jets:key'                  , '1'),
  ('SimulatedPatient'  , 'rdf:type'                  , '1'),
  ('ProfessionalClaim' , 'hc:member_number'          , '1'),
  ('ProfessionalClaim' , 'hc:claim_number'           , '1'),
  ('ProfessionalClaim' , 'jets:key'                  , '1'),
  ('ProfessionalClaim' , 'rdf:type'                  , '1'),
  ('InstitutionalClaim', 'hc:member_number'          , '1'),
  ('InstitutionalClaim', 'hc:claim_number'           , '1'),
  ('InstitutionalClaim', 'jets:key'                  , '1'),
  ('InstitutionalClaim', 'rdf:type'                  , '1')
;

TRUNCATE jetsapi.file_key_staging;
INSERT INTO jetsapi.file_key_staging (client, object_type, file_key) VALUES
  ('Zeme', 'Simulator', 'client=Zeme/ot=Simulator/object.csv'),
  ('Zeme', 'SimulatedPatient', 'client=Zeme/ot=SimulatedPatient/object.csv'),
  ('Zeme', 'ProfessionalClaim', 'client=Zeme/ot=ProfessionalClaim/object.csv'),
  ('Zeme', 'InstitutionalClaim', 'client=Zeme/ot=InstitutionalClaim/object.csv'),
  ('Zeme', 'Claim', 'client=Zeme/ot=Claim/object.csv'),
  ('Zeme', 'Network', 'client=Zeme/ot=Network/object.csv'),
  ('cHealth', 'Simulator',        'client=cHealth/ot=Simulator/object.csv'),
  ('cHealth', 'SimulatedPatient', 'client=cHealth/ot=SimulatedPatient/object.csv'),
  ('dHealth', 'Simulator',        'dlient=cHealth/ot=Simulator/object.csv'),
  ('dHealth', 'SimulatedPatient', 'dlient=cHealth/ot=SimulatedPatient/object.csv')
;

INSERT INTO jetsapi.process_input (key, client, object_type, table_name, source_type, entity_rdf_type, grouping_column, key_column, user_email) VALUES
  (271, 'Zeme', 'Simulator'         , 'Zeme_Simulator'       , 'file',         'aspec:Simulator'      , NULL               , NULL , 'user@mail.com'),
  (272, 'Zeme', 'SimulatedPatient'  , 'hc:SimulatedPatient'  , 'domain_table', 'hc:SimulatedPatient'  , 'hc:patient_number', NULL , 'user@mail.com'),
  (273, 'Zeme', 'ProfessionalClaim' , 'hc:ProfessionalClaim' , 'domain_table', 'hc:ProfessionalClaim' , 'hc:member_number' , NULL , 'user@mail.com'),
  (274, 'Zeme', 'InstitutionalClaim', 'hc:InstitutionalClaim', 'domain_table', 'hc:InstitutionalClaim', 'hc:member_number' , NULL , 'user@mail.com')
;

INSERT INTO jetsapi.process_mapping (table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) VALUES
  ('Zeme_Simulator'       , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('Zeme_Simulator'       , 'anchor_date'               , 'aspec:anchor_date'         , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('Zeme_Simulator'       , 'nbr_entities'              , 'aspec:nbr_entities'        , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('Zeme_Simulator'       , 'entity_key_prefix'         , 'aspec:entity_key_prefix'   , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('Zeme_Simulator'       , 'entity_persona_lk'         , 'aspec:entity_persona_lk'   , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:anchor_date'          , 'asim:anchor_date'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:persona_key'          , 'asim:persona_key'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:demographic_group_key', 'asim:demographic_group_key', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:event_group1_lk'      , 'asim:event_group1_lk'      , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:description'          , 'asim:description'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'hc:patient_number'         , 'hc:patient_number'         , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'hc:dob'                    , 'hc:dob'                    , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'hc:gender'                 , 'hc:gender'                 , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'asim:claim_group_lk'       , 'asim:claim_group_lk'       , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:SimulatedPatient'  , 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim' , 'hc:member_number'          , 'hc:member_number'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim' , 'hc:claim_number'           , 'hc:claim_number'           , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim' , 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:ProfessionalClaim' , 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'hc:member_number'          , 'hc:member_number'          , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'hc:claim_number'           , 'hc:claim_number'           , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'jets:key'                  , 'jets:key'                  , NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('hc:InstitutionalClaim', 'rdf:type'                  , 'rdf:type'                  , NULL, NULL, NULL, NULL, 'user@mail.com')
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

INSERT INTO jetsapi.pipeline_config (key, process_name, client, process_config_key, main_process_input_key, merged_process_input_keys, main_object_type, user_email) VALUES
  (201, 'PatientGen' , 'Zeme', 251, 271, '{}'        , 'Simulator'        , 'user@mail.com'),
  (202, 'ClaimGen'   , 'Zeme', 252, 272, '{}'        , 'SimulatedPatient' , 'user@mail.com'),
  (203, 'PatientAdum', 'Zeme', 253, 272, '{273,274}' , 'SimulatedPatient' , 'user@mail.com'),
  (204, 'AllE2E'     , 'Zeme', 254, 271, '{}'        , 'Simulator'        , 'user@mail.com')
;

INSERT INTO jetsapi.pipeline_execution_status (key, pipeline_config_key, process_name, client, main_object_type, main_input_registry_key, merged_input_registry_keys, input_session_id, session_id, status, user_email) VALUES
  (281, 204, 'AllE2E', 'Zeme', 'Simulator', 1, '{}', NULL, '1230789', 'in progress', 'user@mail.com')
;


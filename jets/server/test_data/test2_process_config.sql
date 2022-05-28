-- TRUNCATE TABLE process_config, process_input, process_mapping, rule_config, process_merge;
DELETE FROM process_config WHERE key in (201, 202);

INSERT INTO process_config (key, client, description, main_entity_rdf_type) VALUES
  (201, 'ACME', 'Entity from aspec:Simulator', 'aspec:Simulator'),
  (202, 'ACME', 'Entity from hc:SimulatedPatient', 'hc:SimulatedPatient')
;

INSERT INTO process_input (key, process_key, input_type, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (221, 201, 1, 'aspec:Simulator', 'aspec:Simulator', 'jets:key', 'jets:key'),
  (222, 202, 1, 'hc:SimulatedPatient', 'hc:SimulatedPatient', 'jets:key', 'jets:key')
;

INSERT INTO process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (221, 'aspec:anchor_date', 'aspec:anchor_date', NULL, NULL, NULL, NULL),
  (221, 'aspec:nbr_entities', 'aspec:nbr_entities', NULL, NULL, NULL, NULL),
  (221, 'aspec:entity_key_prefix', 'aspec:entity_key_prefix', NULL, NULL, NULL, NULL),
  (221, 'aspec:entity_persona_lk', 'aspec:entity_persona_lk', NULL, NULL, NULL, NULL),
  (221, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (221, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL),
  (222, 'asim:persona_key', 'asim:persona_key', NULL, NULL, NULL, NULL),
  (222, 'asim:demographic_group_key', 'asim:demographic_group_key', NULL, NULL, NULL, NULL),
  (222, 'asim:event_group1_lk', 'asim:event_group1_lk', NULL, NULL, NULL, NULL),
  (222, 'asim:description', 'asim:description', NULL, NULL, NULL, NULL),
  (222, 'hc:patient_number', 'hc:patient_number', NULL, NULL, NULL, NULL),
  (222, 'hc:dob', 'hc:dob', NULL, NULL, NULL, NULL),
  (222, 'hc:gender', 'hc:gender', NULL, NULL, NULL, NULL),
  (222, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL),
  (222, 'rdf:type', 'rdf:type', NULL, NULL, NULL, NULL)
;

INSERT INTO rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (201, 'jets:iState', 'rdf:type', 'jets:State', 'resource')
;

DROP TABLE IF EXISTS public."aspec:Simulator";
CREATE TABLE IF NOT EXISTS public."aspec:Simulator" (
  "rdf:type" text ARRAY NOT NULL,
  "session_id" text  ,
  "shard_id" INTEGER  ,
  "last_update" TIMESTAMP  ,
  "jets:key" text  ,
  "aspec:anchor_date" text  ,
  "aspec:nbr_entities" INTEGER  ,
  "aspec:entity_key_prefix" text  ,
  "aspec:entity_persona_lk" text  ,
  PRIMARY KEY ("jets:key")
);

INSERT INTO "aspec:Simulator" (
  "rdf:type","jets:key","aspec:anchor_date","aspec:nbr_entities","aspec:entity_key_prefix", "aspec:entity_persona_lk") VALUES
  ('{"aspec:Simulator"}', 'K:001', '2020-12-31', 2, '01001', 'lk:BasePatientPersona'),
  ('{"aspec:Simulator"}', 'K:002', '2020-12-31', 2, '01002', 'lk:BasePatientPersona')
;
-- INSERT INTO process_merge (process_key, entity_rdf_type, query_rdf_property_list, grouping_rdf_property) VALUES
--   (2, 'm2c:Claim', 'm2c:P1,m2c:P2', 'm2c:P2')
-- ;


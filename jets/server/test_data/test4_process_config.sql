

-- TRUNCATE TABLE jetsapi.pipeline_config, jetsapi.process_input, jetsapi.process_mapping, jetsapi.rule_config, jetsapi.process_merge;
DELETE FROM jetsapi.pipeline_config WHERE key in (400);

INSERT INTO jetsapi.pipeline_config (key, client, description, main_entity_rdf_type) VALUES
  (400, 'TEST3', 'TEST for not operator', 'acme:AIUSIClaim')
;

INSERT INTO jetsapi.process_input (key, process_key, input_type, table_name, entity_rdf_type, grouping_column, key_column) VALUES
  (401, 400, 0, 'test4', 'acme:AIUSIClaim', 'jets:key', 'jets:key')
;

INSERT INTO jetsapi.process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (401, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL)
;

INSERT INTO jetsapi.rule_config (process_key, subject, predicate, object, rdf_type) VALUES
  (400, '77f1b3429fed','_0:medicareRateObj261','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826', 'resource'),
  (400, '77f1b3429fed','_0:medicareRateObj262','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726', 'resource'),
  (400, '77f1b3429fed','_0:medicareRateObjTC1','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC', 'resource'),
  (400, '77f1b3429fed','_0:medicareRateObjTC2','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC', 'resource'),
  (400, '77f1b3429fed','_0:addendumGLookup','jets:ADDENDUMG_LOOKUP_CONFIG:737212018', 'resource'),
  (400, '77f1b3429fed','_0:carrierLocality','14312:40', 'resource'),
  (400, '77f1b3429fed','_0:carrierLocalityGeoZip','jets:GEOZIP_CARRIER_LOOKUP_CONFIG:3302018', 'resource'),
  (400, '77f1b3429fed','_0:carrierLocalityZip','null', 'null'),
  (400, '77f1b3429fed','acme:placeOfServiceCode','11', 'text'),
  (400, '77f1b3429fed','acme:procedureCode','73721', 'text'),
  (400, '77f1b3429fed','acme:procedureCodeModifier','RT', 'text'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726','NF_FEE','70.47', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726','ONF_FEE','0', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC','NF_FEE','179.27', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC','ONF_FEE','237.76', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826','NF_FEE','70.81', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826','ONF_FEE','0', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC','NF_FEE','180.2', 'double'),
  (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC','ONF_FEE','243.03', 'double'),
  (400, 'jets:ADDENDUMG_LOOKUP_CONFIG:737212018','YEAR','2018','text')
;

-- INSERT INTO jetsapi.rule_config (process_key, subject, predicate, object, rdf_type) VALUES
--   (400, 'RR_ACME001', 'rdf:type', 'RRAcme', 'resource'),
--   (400, 'RR_ACME001','_0:medicareRateObj261','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826', 'resource'),
--   (400, 'RR_ACME001','_0:medicareRateObj262','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726', 'resource'),
--   (400, 'RR_ACME001','_0:medicareRateObjTC1','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC', 'resource'),
--   (400, 'RR_ACME001','_0:medicareRateObjTC2','jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC', 'resource'),
--   (400, 'RR_ACME001','_0:addendumGLookup','jets:ADDENDUMG_LOOKUP_CONFIG:737212018', 'resource'),
--   (400, 'RR_ACME001','_0:carrierLocality','14312:40', 'resource'),
--   (400, 'RR_ACME001','_0:carrierLocalityGeoZip','jets:GEOZIP_CARRIER_LOOKUP_CONFIG:3302018', 'resource'),
--   (400, 'RR_ACME001','_0:carrierLocalityZip','null', 'null'),
--   (400, 'RR_ACME001','acme:placeOfServiceCode','11', 'text'),
--   (400, 'RR_ACME001','acme:procedureCode','73721', 'text'),
--   (400, 'RR_ACME001','acme:procedureCodeModifier','RT', 'text'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726','NF_FEE','70.47', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201726','ONF_FEE','0', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC','NF_FEE','179.27', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212017TC','ONF_FEE','237.76', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826','NF_FEE','70.81', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:4073721201826','ONF_FEE','0', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC','NF_FEE','180.2', 'double'),
--   (400, 'jets:PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG:14312:40737212018TC','ONF_FEE','243.03', 'double'),
--   (400, 'jets:ADDENDUMG_LOOKUP_CONFIG:737212018','YEAR','2018','text')
-- ;

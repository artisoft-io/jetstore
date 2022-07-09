-- TRUNCATE TABLE jetsapi.process_config, jetsapi.process_input, jetsapi.process_mapping, jetsapi.rule_config, jetsapi.process_merge;
DELETE FROM jetsapi.process_config WHERE key in (500);

INSERT INTO jetsapi.process_config (key, client, description, main_entity_rdf_type) VALUES
  (500, 'TEST5', 'TEST for Trigger', 'acme:Claim')
;

INSERT INTO jetsapi.process_input (key, process_key, input_type, input_table, entity_rdf_type, grouping_column, key_column) VALUES
  (501, 500, 0, 'test5', 'acme:Claim', 'jets:key', 'jets:key')
;

INSERT INTO jetsapi.process_mapping (process_input_key, input_column, data_property, function_name, argument, default_value, error_message) VALUES
  (501, 'jets:key', 'jets:key', NULL, NULL, NULL, NULL)
;

INSERT INTO jetsapi.rule_config (process_key, subject, predicate, object, rdf_type) VALUES
(500, 'RR_ACME001', 'rdf:type', 'acme:Claim', 'resource'),
(500, 'RR_ACME001','_0:YN_ROWS','jets:YN:1', 'resource'),
(500, 'RR_ACME001','_0:A_ROWS','jets:A:1', 'resource'),
(500, 'RR_ACME001','acme:a','A', 'text'),
(500, 'RR_ACME001','acme:b','B', 'text'),
(500, 'RR_ACME001','acme:c','C', 'text'),
(500, 'RR_ACME001','acme:d','D', 'text'),
(500, 'RR_ACME001','acme:e','E', 'text'),
(500, 'RR_ACME001','acme:f','E', 'text'),
(500, 'jets:YN:1','YES','Y', 'text'),
(500, 'jets:YN:1','NO','N', 'text'),
(500, 'jets:A:1','UPPER_A','A','text')
;
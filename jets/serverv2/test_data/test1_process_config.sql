DELETE FROM jetsapi.pipeline_config WHERE key in (101, 102);
DELETE FROM jetsapi.process_config WHERE key in (151, 152);
DELETE FROM jetsapi.process_input WHERE table_name in ('test1');
DELETE FROM jetsapi.rule_config WHERE process_config_key in (151, 152);

INSERT INTO jetsapi.pipeline_config (key, client, description, process_config_key, process_name, main_table_name, user_email) VALUES
  (101, 'ACME', 'Lookup ACME Service Code with Modifier', 151, 'PROC01', 'test1', 'user@mail.com'),
  (102, 'ACME', 'Lookup ACME Service Code without Modifier', 152, 'PROC01', 'test1', 'user@mail.com')
RETURNING key;

INSERT INTO jetsapi.process_config (key, process_name, main_rules, is_rule_set, output_tables, user_email) VALUES
  (151, 'PROC01', 'test1_ruleset1.jr', 1, '{hc:Claim}', 'user@mail.com'),
  (152, 'PROC01', 'test1_ruleset1.jr', 1, '{hc:Claim}', 'user@mail.com')
;

INSERT INTO jetsapi.process_input (table_name, client, source_type, entity_rdf_type, grouping_column, key_column, user_email) VALUES
  ('test1', 'ACME', 'file', 'hc:Claim', 'MEMBER_NUMBER', 'CLAIM_NUMBER', 'user@mail.com')
;

INSERT INTO jetsapi.process_mapping (table_name, input_column, data_property, function_name, argument, default_value, error_message, user_email) VALUES
  ('test1', 'MEMBER_NUMBER', 'hc:member_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test1', 'CLAIM_NUMBER', 'hc:claim_number', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test1', 'DOS', 'hc:date_of_service', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test1', 'SERVICE_CODE', 'hc:service_code', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test1', 'MODIFIER', 'hc:modifier', NULL, NULL, NULL, NULL, 'user@mail.com'),
  ('test1', 'NDAYS', 'hc:ndays', 'apply_regex', '(\d)+.*', '-1', NULL, 'user@mail.com'),
  ('test1', 'SUBMITTED_AMT', 'hc:submitted_amount', 'parse_amount', '10', '99', NULL, 'user@mail.com'),
  ('test1', 'ALLOWED_AMT'  , 'hc:allowed_amount', 'parse_amount', '1', NULL, 'Input amounts cannot be null', 'user@mail.com')
;

INSERT INTO jetsapi.rule_config (process_config_key, process_name, client, subject, predicate, object, rdf_type) VALUES
  (151, 'PROC01', 'ACME', 'jets:iState', 'lk:withModifier', 'true', 'bool'),
  (152, 'PROC01', 'ACME', 'jets:iState', 'lk:withModifier', 'false', 'bool')
;


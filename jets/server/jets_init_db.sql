
TRUNCATE jetsapi.mapping_function_registry;
INSERT INTO jetsapi.mapping_function_registry (function_name, is_argument_required) VALUES
  ('to_upper', '0'),
  ('parse_amount', '1'),
  ('reformat0', '1'),
  ('apply_regex', '1'),
  ('scale_units', '1'),
  ('to_zip5', '0')
;

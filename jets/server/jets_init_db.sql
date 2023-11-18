
TRUNCATE jetsapi.mapping_function_registry;
INSERT INTO jetsapi.mapping_function_registry (function_name, is_argument_required) VALUES
  ('apply_regex',            '1'),
  ('concat',                 '1'),
  ('concat_with',            '1'),
  ('format_phone',           '0'),
  ('overpunch_number',       '1'),
  ('parse_amount',           '1'),
  ('reformat0',              '1'),
  ('scale_units',            '1'),
  ('to_upper',               '0'),
  ('to_zip5',                '0'),
  ('to_zipext4_from_zip9',   '0'),
  ('to_zipext4',             '0'),
  ('trim',                   '0'),
  ('validate_date',          '0')
;

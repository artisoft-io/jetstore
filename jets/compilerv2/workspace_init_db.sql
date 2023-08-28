INSERT INTO domain_classes (key, name, source_file_key) VALUES (0, 'owl:Thing', -1)
ON CONFLICT (key) DO NOTHING;

INSERT INTO schema_info (version_major, version_minor) VALUES (1, 0)
ON CONFLICT (version_major, version_minor) DO NOTHING;


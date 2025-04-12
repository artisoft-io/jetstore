DROP TABLE IF EXISTS test_table;
CREATE TABLE IF NOT EXISTS test_table (
    "hc:claim_number" text  ,
    "hc:member_number" text  ,
    "jets:key" text  ,
    "shard_id" integer  ,
    "rdf:type" text ARRAY 
);
CREATE INDEX IF NOT EXISTS test_table_shard_id_idx ON test_table (shard_id);

INSERT INTO test_table (
"hc:claim_number", 
"hc:member_number", 
"jets:key", 
"shard_id", 
"rdf:type") 
  VALUES 
    ('12345678901','mbr0012','123-123-123',30,'{"{jets}:\"Entity\"","hc:PharmacyClaim"}'),
    ('12345678902','mbr0013','123-123-333',45,'{"jets:\\Entity","hc:PharmacyClaim"}')
;

-- COPY test_table  TO '/work/test_table.copy' WITH (DELIMITER '|');

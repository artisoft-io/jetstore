-- initial schema for database apiserver tables
CREATE SCHEMA IF NOT EXISTS jetsapi;

-- DROP TABLE IF EXISTS jetsapi.users;
-- CREATE TABLE IF NOT EXISTS jetsapi.users (
--   user_id SERIAL PRIMARY KEY, 
--   name TEXT NOT NULL, 
--   email TEXT NOT NULL, 
--   password TEXT NOT NULL, 
--   last_update timestamp without time zone DEFAULT now() NOT NULL,
--   UNIQUE (email)
-- );

-- Test Data
INSERT INTO jetsapi.client_registry (client) VALUES
  ('ACME'),
  ('Zeme'),
  ('Latour')
;

-- TEST Table
DROP TABLE IF EXISTS jetsapi.pipelines;
CREATE TABLE IF NOT EXISTS jetsapi.pipelines (
  key SERIAL PRIMARY KEY, 
  user_name TEXT NOT NULL, 
  client TEXT, 
  process TEXT NOT NULL, 
  status TEXT NOT NULL DEFAULT 'submitted', 
  submitted_at timestamp without time zone DEFAULT now() NOT NULL
);

-- some test data
INSERT INTO jetsapi.pipelines (key,user_name,client,process,status,submitted_at) VALUES
  ('1001', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-01'),
  ('1002', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-30'),
  ('1003', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-02'),
  ('1004', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-01'),
  ('1005', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-01'),
  ('1006', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-30'),
  ('1007', 'Michel', NULL, 'PROC3', 'completed', '2021-05-15'),
  ('1008', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-15'),
  ('1009', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-15'),
  ('1000', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-15'),
  ('1021', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-04'),
  ('1022', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-04'),
  ('1023', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-04'),
  ('1024', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-04'),
  ('1025', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-04'),
  ('1026', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-04'),
  ('1027', 'Michel', NULL, 'PROC3', 'completed', '2021-05-04'),
  ('1028', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-04'),
  ('1029', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-04'),
  ('1020', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-04'),
  ('1031', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-05'),
  ('1032', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-05'),
  ('1033', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-05'),
  ('1034', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-05'),
  ('1035', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-05'),
  ('1036', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-05'),
  ('1037', 'Michel', NULL, 'PROC3', 'completed', '2021-05-05'),
  ('1038', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-05'),
  ('1039', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-05'),
  ('1030', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-05'),
  ('1041', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-06'),
  ('1042', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-06'),
  ('1043', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-06'),
  ('1044', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-06'),
  ('1045', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-06'),
  ('1046', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-06'),
  ('1047', 'Michel', NULL, 'PROC3', 'completed', '2021-05-06'),
  ('1048', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-06'),
  ('1049', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-06'),
  ('1040', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-06'),
  ('1051', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-07'),
  ('1052', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-07'),
  ('1053', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-07'),
  ('1054', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-07'),
  ('1055', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-07'),
  ('1056', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-07'),
  ('1057', 'Michel', NULL, 'PROC3', 'completed', '2021-05-07'),
  ('1058', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-07'),
  ('1059', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-07'),
  ('1050', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-07'),
  ('1061', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-08'),
  ('1062', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-08'),
  ('1063', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-08'),
  ('1064', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-08'),
  ('1065', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-08'),
  ('1066', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-08'),
  ('1067', 'Michel', NULL, 'PROC3', 'completed', '2021-05-08'),
  ('1068', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-08'),
  ('1069', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-08'),
  ('1060', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-08'),
  ('1011', 'Michel', 'ACME', 'PROC1', 'submitted', '2022-07-09'),
  ('1012', 'Manuel', 'ACME', 'PROC1', 'completed', '2021-06-09'),
  ('1013', 'Manuel', 'ZenHealth', 'PROC2', 'completed', '2022-04-09'),
  ('1014', 'Michel', 'ZenHealth', 'PROC2', 'completed', '2021-09-09'),
  ('1015', 'Michel', 'ACME', 'PROC1', 'in-progress', '2022-07-09'),
  ('1016', 'Manuel', NULL, 'PROC3', 'in-progress', '2022-06-09'),
  ('1017', 'Michel', NULL, 'PROC3', 'completed', '2021-05-09'),
  ('1018', 'Manuel', NULL, 'PROC3', 'completed', '2021-06-09'),
  ('1019', 'Michel', 'AACP', 'PROC4', 'completed', '2021-07-09'),
  ('1010', 'Manuel', 'AACP', 'PROC4', 'completed', '2021-08-09')
;

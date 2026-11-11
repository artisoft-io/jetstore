-- 00_base.sql: the data every screenshot shows.
--
-- Applied by the screenshot stack after jets_ws's CI client configuration
-- (process_config/ci_workspace_init_db.sql), so the client CI, its org D1, the
-- three source configurations and the pipelines already exist. This file adds
-- data only, all of it from the invented 22 Patients set in
-- docs/screenshots/data/, which the stack copies into the database container
-- as /fixture-data/:
--
--   1. three files loaded on 2026-01-05, as the loader leaves them: a row each
--      in input_loader_status, input_registry and session_registry, and their
--      rows in the staging tables CI_D1_hc:Eligibility, CI_D1_hc:MedicalClaim
--      and CI_D1_hc:PharmacyClaim;
--   2. the three Map_ pipelines run on them, as they leave the domain tables
--      hc:Eligibility, hc:MedicalClaim and hc:PharmacyClaim, and registered;
--   3. Patient_Profile_Template runs in each status, the completed run's
--      Patient_Profile rows carrying the data set's sample profiles;
--   4. two archived runs, one of them of a session a user deleted.
--
-- Every date and every key is fixed, so the screens are the same at every run.
-- Later fixtures may rely on everything named here. Explicit keys below 10000
-- are this file's; a later fixture takes its keys from its own range (10_ from
-- 10000, 20_ from 20000, 30_ from 30000), and the sequences continue at 100000,
-- so a row a spec creates through the app never takes one of them.

\set ON_ERROR_STOP on
SET client_min_messages = warning;

-- ---------------------------------------------------------------------------
-- Source periods: the day a file was received, as InsertSourcePeriod computes
-- them (jets/datatable/source_period.go).
INSERT INTO jetsapi.source_period (key, year, month, day, month_period, week_period, day_period)
SELECT k, y, m, d,
       (y - 1970) * 12 + m,
       (extract(epoch FROM make_date(y, m, d))::bigint / 604800 + 1)::int,
       (extract(epoch FROM make_date(y, m, d))::bigint / 86400 + 1)::int
  FROM (VALUES (101, 2025, 11, 3), (102, 2025, 12, 1), (103, 2026, 1, 5)) AS p(k, y, m, d);

-- ---------------------------------------------------------------------------
-- 1. The three files, loaded.
--
-- The loader (Jets_Loader, jets/workspace_assets/pipes_config/jets_loader.pc.json)
-- copies a file into the staging table its source configuration names: one TEXT
-- column per header, a repeated header made unique with _2, then session_id.
-- Values are trimmed and an empty field is NULL.
CREATE TEMP TABLE shots_file (
  object_type text, table_name text, file_name text, session_id text,
  ilr_key int, ir_key int, rows int, loaded_at timestamp);
INSERT INTO shots_file VALUES
  ('hc:Eligibility',   'CI_D1_hc:Eligibility',   'M021-M040_M007_M008_eligibility.csv',      '1767603600101', 101, 101,  22, '2026-01-05 09:00:12'),
  ('hc:MedicalClaim',  'CI_D1_hc:MedicalClaim',  'M021-M040_M007_M008_medical_claims.csv',   '1767603600102', 102, 102, 301, '2026-01-05 09:01:05'),
  ('hc:PharmacyClaim', 'CI_D1_hc:PharmacyClaim', 'M021-M040_M007_M008_pharmacy_claims.csv',  '1767603600103', 103, 103, 257, '2026-01-05 09:01:48');

CREATE OR REPLACE FUNCTION pg_temp.shots_file_key(f shots_file) RETURNS text LANGUAGE sql AS $$
  SELECT format('jetstore/input/client=CI/org=D1/object_type=%s/year=2026/month=1/day=5/%s',
                f.object_type, f.file_name)
$$;

CREATE TABLE public."CI_D1_hc:Eligibility" ("Member_Address_Line_1" TEXT, "Member_Address_Line_2" TEXT, "Member_City" TEXT, "Member_DOB" TEXT, "Member_First_Name" TEXT, "Member_Gender" TEXT, "Member_ID" TEXT, "Member_Last_Name" TEXT, "Member_Relationship" TEXT, "Member_State_Code" TEXT, "Member_Zip" TEXT, "Subscriber_Address_Line_1" TEXT, "Subscriber_Address_Line_2" TEXT, "Subscriber_City" TEXT, "Subscriber_DOB" TEXT, "Subscriber_First_Name" TEXT, "Subscriber_Gender" TEXT, "Subscriber_ID" TEXT, "Subscriber_Last_Name" TEXT, "Subscriber_State_Code" TEXT, "Subscriber_Zip" TEXT, "Group_Number" TEXT, "Plan_Name" TEXT, "Plan_Type" TEXT, "Medicare_Indicator" TEXT, "Effective_Date" TEXT, "Termination_Date" TEXT);
CREATE TABLE public."CI_D1_hc:MedicalClaim" ("Member_ID" TEXT, "Claim_Type" TEXT, "Service_Date_From" TEXT, "Member_DOB" TEXT, "Member_Gender" TEXT, "Patient_Pay_Amount" TEXT, "Total_Paid_Amount" TEXT, "Claim_ID" TEXT, "Diagnosis_Code1" TEXT, "Diagnosis_Code2" TEXT, "Diagnosis_Code3" TEXT, "Diagnosis_Code4" TEXT, "Service_Date_To" TEXT, "Place_Of_Service_Code" TEXT, "Procedure_Code1" TEXT, "Procedure_Code2" TEXT, "Procedure_Code3" TEXT, "Procedure_Code4" TEXT, "Billing_Address_Line_1" TEXT, "Billing_Address_Line_2" TEXT, "Billing_City" TEXT, "Billing_First_Name" TEXT, "Billing_Last_Name" TEXT, "Billing_NPI" TEXT, "Billing_State_Code" TEXT, "Billing_Zip" TEXT, "Claim_Line_Number" TEXT, "Claim_Type_2" TEXT, "Member_Address_Line_1" TEXT, "Member_Address_Line_2" TEXT, "Member_City" TEXT, "Member_First_Name" TEXT, "Member_Last_Name" TEXT, "Member_Relationship" TEXT, "Member_State_Code" TEXT, "Member_Zip" TEXT, "Patient_Pay_Amount_2" TEXT, "Primary_Diagnosis_Code" TEXT, "Procedure_Code_Modifier1" TEXT, "Procedure_Code_Modifier2" TEXT, "Provider_Address_Line_1" TEXT, "Provider_Address_Line_2" TEXT, "Provider_City" TEXT, "Provider_First_Name" TEXT, "Provider_Last_Name" TEXT, "Provider_NPI" TEXT, "Provider_State_Code" TEXT, "Provider_Zip" TEXT, "Subscriber_Address_Line_1" TEXT, "Subscriber_Address_Line_2" TEXT, "Subscriber_City" TEXT, "Subscriber_DOB" TEXT, "Subscriber_First_Name" TEXT, "Subscriber_Gender" TEXT, "Subscriber_ID" TEXT, "Subscriber_Last_Name" TEXT, "Subscriber_State_Code" TEXT, "Subscriber_Zip" TEXT, "Total_Paid_Amount_2" TEXT);
CREATE TABLE public."CI_D1_hc:PharmacyClaim" ("Member_ID" TEXT, "Claim_Type" TEXT, "Service_Date" TEXT, "Member_DOB" TEXT, "Member_Gender" TEXT, "Patient_Pay_Amount" TEXT, "Total_Paid_Amount" TEXT, "Claim_ID" TEXT, "DAW_Code" TEXT, "Days_Supply" TEXT, "Drug_Name" TEXT, "Drug_Strength" TEXT, "Drug_Type_Code" TEXT, "Quantity_Dispensed" TEXT, "NDC" TEXT, "Billing_Address_Line_1" TEXT, "Billing_Address_Line_2" TEXT, "Billing_City" TEXT, "Billing_First_Name" TEXT, "Billing_Last_Name" TEXT, "Billing_NPI" TEXT, "Billing_State_Code" TEXT, "Billing_Zip" TEXT, "Claim_Type_2" TEXT, "Drug_Name_2" TEXT, "Drug_Strength_2" TEXT, "Drug_Type_Code_2" TEXT, "Member_Address_Line_1" TEXT, "Member_Address_Line_2" TEXT, "Member_City" TEXT, "Member_First_Name" TEXT, "Member_Last_Name" TEXT, "Member_Relationship" TEXT, "Member_State_Code" TEXT, "Member_Zip" TEXT, "Patient_Pay_Amount_2" TEXT, "Provider_Address_Line_1" TEXT, "Provider_Address_Line_2" TEXT, "Provider_City" TEXT, "Provider_First_Name" TEXT, "Provider_Last_Name" TEXT, "Provider_NPI" TEXT, "Provider_State_Code" TEXT, "Provider_Zip" TEXT, "Subscriber_Address_Line_1" TEXT, "Subscriber_Address_Line_2" TEXT, "Subscriber_City" TEXT, "Subscriber_DOB" TEXT, "Subscriber_First_Name" TEXT, "Subscriber_Gender" TEXT, "Subscriber_ID" TEXT, "Subscriber_Last_Name" TEXT, "Subscriber_State_Code" TEXT, "Subscriber_Zip" TEXT, "Total_Paid_Amount_2" TEXT);

\copy public."CI_D1_hc:Eligibility" FROM '/fixture-data/M021-M040_M007_M008_eligibility.csv' WITH (FORMAT csv, HEADER true)
\copy public."CI_D1_hc:MedicalClaim" FROM '/fixture-data/M021-M040_M007_M008_medical_claims.csv' WITH (FORMAT csv, HEADER true)
\copy public."CI_D1_hc:PharmacyClaim" FROM '/fixture-data/M021-M040_M007_M008_pharmacy_claims.csv' WITH (FORMAT csv, HEADER true)

-- The loader's last column, its trimming, its empty fields, and its index.
DO $$
DECLARE
  f record;
  c record;
BEGIN
  FOR f IN SELECT * FROM shots_file LOOP
    EXECUTE format('ALTER TABLE public.%I ADD COLUMN session_id TEXT DEFAULT %L NOT NULL', f.table_name, '');
    FOR c IN SELECT column_name FROM information_schema.columns
              WHERE table_schema = 'public' AND table_name = f.table_name AND column_name <> 'session_id' LOOP
      EXECUTE format('UPDATE public.%I SET %I = NULLIF(btrim(%I), %L)', f.table_name, c.column_name, c.column_name, '');
    END LOOP;
    EXECUTE format('UPDATE public.%I SET session_id = %L', f.table_name, f.session_id);
    EXECUTE format('CREATE INDEX %I ON public.%I (session_id)', 'public_' || f.table_name || '_session_id', f.table_name);
  END LOOP;
END
$$;

INSERT INTO jetsapi.input_loader_status
  (key, object_type, table_name, client, org, file_key, source_period_key, bad_row_count, load_count,
   session_id, status, user_email, last_update)
SELECT f.ilr_key, f.object_type, f.table_name, 'CI', 'D1', pg_temp.shots_file_key(f), 103, 0, f.rows,
       f.session_id, 'completed', 'admin@localhost.test', f.loaded_at
  FROM shots_file f;

-- A file is registered under its domain key, hc:Eligibility for all three
-- (source_config.domain_keys), by the system user (RegisterFileInputSource).
INSERT INTO jetsapi.input_registry
  (key, client, org, object_type, file_key, source_period_key, table_name, source_type, session_id,
   user_email, last_update)
SELECT f.ir_key, 'CI', 'D1', 'hc:Eligibility', pg_temp.shots_file_key(f), 103, f.table_name, 'file',
       f.session_id, 'system', f.loaded_at
  FROM shots_file f;

INSERT INTO jetsapi.session_registry
  (session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
SELECT f.session_id, 'file', 'CI', 103, sp.month_period, sp.week_period, sp.day_period, f.loaded_at
  FROM shots_file f, jetsapi.source_period sp WHERE sp.key = 103;

-- ---------------------------------------------------------------------------
-- 2. The three Map_ runs, completed.
--
-- map_entity.pc.json maps each staging row through the source configuration's
-- process_mapping rows into the domain table named for the object type. Only
-- what this data needs is reproduced: a column copied, cast to the domain
-- column's type, and scrub_characters, which removes the argument's characters.
-- All three tables are keyed on the member, so their domain key is
-- hc:Eligibility's (domain_keys_registry).
CREATE OR REPLACE FUNCTION pg_temp.shots_map(staging text, domain text, sess text, at timestamp)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  m record;
  cols text := '';
  vals text := '';
  expr text;
BEGIN
  FOR m IN
    SELECT pm.data_property, pm.input_column, pm.function_name, pm.argument, dc.data_type
      FROM jetsapi.process_mapping pm
      JOIN information_schema.columns sc
        ON sc.table_schema = 'public' AND sc.table_name = staging AND sc.column_name = pm.input_column
      JOIN information_schema.columns dc
        ON dc.table_schema = 'public' AND dc.table_name = domain AND dc.column_name = pm.data_property
     WHERE pm.table_name = staging
     ORDER BY pm.data_property
  LOOP
    expr := format('s.%I', m.input_column);
    IF m.function_name = 'scrub_characters' THEN
      expr := format('translate(%s, %L, %L)', expr, m.argument, '');
    END IF;
    IF m.data_type <> 'text' THEN
      expr := format('(%s)::%s', expr, m.data_type);
    END IF;
    cols := cols || format('%I, ', m.data_property);
    vals := vals || expr || ', ';
  END LOOP;
  EXECUTE format(
    'INSERT INTO public.%I (%s"jets:client", "jets:org", "jets:key", "rdf:type", %I, %I, session_id, last_update)
     SELECT %s%L, %L, md5(%L || row_to_json(s)::text), ARRAY[%L], s."Member_ID", 0, %L, %L
       FROM public.%I s',
    domain, cols, 'hc:Eligibility:domain_key', 'hc:Eligibility:shard_id',
    vals, 'CI', 'D1', sess, domain, sess, at, staging);
END
$$;

CREATE TEMP TABLE shots_map_run (
  pes_key int, pipeline_config_key int, process_name text, file_ir_key int, file_session text,
  domain_table text, session_id text, ir_key int, started timestamp, finished timestamp, rows int);
INSERT INTO shots_map_run VALUES
  (101, 3, 'Map_Eligibility',   101, '1767603600101', 'hc:Eligibility',   '1767604500201', 111, '2026-01-05 09:15:00', '2026-01-05 09:16:41',  22),
  (102, 4, 'Map_MedicalClaim',  102, '1767603600102', 'hc:MedicalClaim',  '1767604560202', 112, '2026-01-05 09:16:00', '2026-01-05 09:17:52', 301),
  (103, 5, 'Map_PharmacyClaim', 103, '1767603600103', 'hc:PharmacyClaim', '1767604620203', 113, '2026-01-05 09:17:00', '2026-01-05 09:18:37', 257);

DO $$
BEGIN
  PERFORM pg_temp.shots_map(f.table_name, r.domain_table, r.session_id, r.finished)
     FROM shots_map_run r JOIN shots_file f ON f.ir_key = r.file_ir_key;
END
$$;

INSERT INTO jetsapi.pipeline_execution_status
  (key, pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys,
   client, process_name, main_object_type, input_session_id, session_id, source_period_key, status,
   user_email, start_time, last_update, workspace_name, workspace_version)
SELECT r.pes_key, r.pipeline_config_key, r.file_ir_key, pg_temp.shots_file_key(f), '{}',
       'CI', r.process_name, 'hc:Eligibility', r.file_session, r.session_id, 103, 'completed',
       'admin@localhost.test', r.started, r.finished, 'jets_ws', '1767225600'
  FROM shots_map_run r JOIN shots_file f ON f.ir_key = r.file_ir_key;

-- The domain table each run wrote, registered under its domain key with the
-- file's key, as RegisterDomainTables does.
INSERT INTO jetsapi.input_registry
  (key, client, org, object_type, file_key, source_period_key, table_name, source_type, session_id,
   user_email, last_update)
SELECT r.ir_key, 'CI', '', 'hc:Eligibility', pg_temp.shots_file_key(f), 103, r.domain_table, 'domain_table',
       r.session_id, 'admin@localhost.test', r.finished
  FROM shots_map_run r JOIN shots_file f ON f.ir_key = r.file_ir_key;

INSERT INTO jetsapi.session_registry
  (session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
SELECT r.session_id, 'domain_table', 'CI', 103, sp.month_period, sp.week_period, sp.day_period, r.finished
  FROM shots_map_run r, jetsapi.source_period sp WHERE sp.key = 103;

-- Two steps each: sharding the file, then mapping it, on one node.
INSERT INTO jetsapi.pipeline_execution_details
  (pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
   session_id, source_period_key, shard_id, cpipes_step_id, status, input_records_count,
   input_bad_records_count, input_files_size_mb, input_files_count, rete_sessions_count,
   output_records_count, user_email, start_time, last_update)
SELECT r.pipeline_config_key, r.pes_key, 'CI', r.process_name, r.file_session, r.session_id, 103, 0,
       s.step, 'completed', r.rows, 0, 1, 1, 0, r.rows, 'admin@localhost.test',
       r.started + s.offs, r.started + s.offs + interval '40 seconds'
  FROM shots_map_run r,
       (VALUES ('sharding', interval '5 seconds'), ('reducing00', interval '50 seconds')) AS s(step, offs)
 ORDER BY r.pes_key, s.offs;

INSERT INTO jetsapi.cpipes_execution_status (pipeline_execution_status_key, session_id)
SELECT r.pes_key, r.session_id FROM shots_map_run r ORDER BY r.pes_key;

INSERT INTO jetsapi.cpipes_execution_status_details
  (session_id, process_name, cpipes_step_id, nbr_nodes, total_input_files_size_mb,
   total_input_records_count, total_input_bad_records_count, total_output_records_count)
SELECT ped.session_id, ped.process_name, ped.cpipes_step_id, count(*), sum(ped.input_files_size_mb),
       sum(ped.input_records_count), sum(ped.input_bad_records_count), sum(ped.output_records_count)
  FROM jetsapi.pipeline_execution_details ped
 WHERE ped.pipeline_execution_status_key BETWEEN 101 AND 103
 GROUP BY ped.session_id, ped.process_name, ped.cpipes_step_id
 ORDER BY ped.session_id, ped.cpipes_step_id;

-- ---------------------------------------------------------------------------
-- 3. Patient_Profile_Template, in each status.
--
-- Its main input is the eligibility domain table and it merges the two claim
-- tables (pipeline_config 8). Seven steps: three shard, three sort, and one
-- that runs the rules and writes Patient_Profile and Briefing records.
CREATE TEMP TABLE shots_ppt_run (
  pes_key int, session_id text, status text, started timestamp, finished timestamp,
  last_step int, failure_details text, failure_class text, failure_source text);
INSERT INTO shots_ppt_run VALUES
  (111, '1767608400301', 'completed',   '2026-01-05 10:20:00', '2026-01-05 10:24:37', 6, NULL, NULL, NULL),
  (112, '1767778200302', 'errors',      '2026-01-07 09:30:00', '2026-01-07 09:34:58', 6, NULL, NULL, NULL),
  (113, '1767868200303', 'failed',      '2026-01-08 10:30:00', '2026-01-08 10:33:12', 3,
   'step reducing03 failed on node 0: while reading the stage files of PharmacyClaim.sharded: the request timed out',
   'timeout', 'reducing03'),
  (114, '1767949200304', 'interrupted', '2026-01-09 09:00:00', '2026-01-09 09:02:20', 2,
   'the run was interrupted: the apiserver stopped while the run was in progress', 'interrupted', 'apiserver'),
  (115, '1768039200305', 'submitted',   '2026-01-10 10:00:00', '2026-01-10 10:02:05', 4, NULL, NULL, NULL),
  (116, '1768039500306', 'pending',     '2026-01-10 10:05:00', '2026-01-10 10:05:00', -1, NULL, NULL, NULL);

INSERT INTO jetsapi.pipeline_execution_status
  (key, pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys,
   client, process_name, main_object_type, input_session_id, session_id, source_period_key, status,
   failure_details, failure_class, failure_source, user_email, start_time, last_update,
   workspace_name, workspace_version)
SELECT r.pes_key, 8, 111, pg_temp.shots_file_key(f), '{112,113}',
       'CI', 'Patient_Profile_Template', 'hc:Eligibility', '1767604500201', r.session_id, 103, r.status,
       r.failure_details, r.failure_class, r.failure_source, 'admin@localhost.test', r.started, r.finished,
       'jets_ws', '1767225600'
  FROM shots_ppt_run r, shots_file f WHERE f.ir_key = 101;

INSERT INTO jetsapi.session_registry
  (session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
SELECT r.session_id, 'domain_table', 'CI', 103, sp.month_period, sp.week_period, sp.day_period, r.finished
  FROM shots_ppt_run r, jetsapi.source_period sp
 WHERE sp.key = 103 AND r.status IN ('completed', 'errors');

-- The steps, each on one node, up to the run's last step; the last step of a
-- run that did not complete carries its status.
CREATE TEMP TABLE shots_ppt_step (n int, step text, input_session text, records int, output_records int);
INSERT INTO shots_ppt_step VALUES
  (0, 'sharding',   '1767604500201',  22,  22),
  (1, 'reducing00', '1767604620203', 257, 257),
  (2, 'reducing01', '1767604560202', 301, 301),
  (3, 'reducing02', '1767604500201',  22,  22),
  (4, 'reducing03', '1767604620203', 257, 257),
  (5, 'reducing04', '1767604560202', 301, 301),
  (6, 'reducing05', '1767604500201',  22,  44);

INSERT INTO jetsapi.pipeline_execution_details
  (pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
   session_id, source_period_key, shard_id, cpipes_step_id, status, error_message, input_records_count,
   input_bad_records_count, input_files_size_mb, input_files_count, rete_sessions_count,
   output_records_count, user_email, start_time, last_update)
SELECT 8, r.pes_key, 'CI', 'Patient_Profile_Template', '1767604500201', r.session_id, 103, 0, s.step,
       CASE WHEN s.n < r.last_step OR r.status IN ('completed', 'errors') THEN 'completed'
            WHEN r.status = 'submitted' THEN 'in progress'
            ELSE r.status END,
       CASE WHEN s.n = r.last_step AND r.status IN ('failed', 'interrupted') THEN r.failure_details END,
       s.records, 0, 1, 1,
       CASE WHEN s.step = 'reducing05' THEN 22 ELSE 0 END,
       CASE WHEN s.n = r.last_step AND r.status NOT IN ('completed', 'errors') THEN 0 ELSE s.output_records END,
       'admin@localhost.test',
       r.started + s.n * interval '35 seconds' + interval '4 seconds',
       r.started + s.n * interval '35 seconds' + interval '32 seconds'
  FROM shots_ppt_run r JOIN shots_ppt_step s ON s.n <= r.last_step
 ORDER BY r.pes_key, s.n;

INSERT INTO jetsapi.cpipes_execution_status (pipeline_execution_status_key, session_id)
SELECT r.pes_key, r.session_id FROM shots_ppt_run r WHERE r.last_step >= 0 ORDER BY r.pes_key;

INSERT INTO jetsapi.cpipes_execution_status_details
  (session_id, process_name, cpipes_step_id, nbr_nodes, total_input_files_size_mb,
   total_input_records_count, total_input_bad_records_count, total_output_records_count)
SELECT ped.session_id, ped.process_name, ped.cpipes_step_id, count(*), sum(ped.input_files_size_mb),
       sum(ped.input_records_count), sum(ped.input_bad_records_count), sum(ped.output_records_count)
  FROM jetsapi.pipeline_execution_details ped
  JOIN shots_ppt_run r ON r.pes_key = ped.pipeline_execution_status_key
 WHERE r.status IN ('completed', 'errors', 'failed')
 GROUP BY ped.session_id, ped.process_name, ped.cpipes_step_id
 ORDER BY ped.session_id, ped.cpipes_step_id;

-- What the two finished runs wrote, edge by edge.
INSERT INTO jetsapi.pipeline_execution_channel_details
  (pipeline_execution_details_key, session_id, input_channel, output_channel, output_channel_spec,
   output_type, output_entity, output_location, output_sinks_count, output_records_count, parts_count,
   last_update)
SELECT ped.key, ped.session_id, 'Eligibility.merged', o.ch, o.spec, 'stage', o.entity,
       format('jetstore/stage/process_name=Patient_Profile_Template/session_id=%s/step_id=%s/', ped.session_id, o.step),
       1, 22, 1, ped.last_update
  FROM jetsapi.pipeline_execution_details ped
  JOIN shots_ppt_run r ON r.pes_key = ped.pipeline_execution_status_key
 CROSS JOIN (VALUES ('Patient_Profile.writer', 'Patient_Profile', 'cintel:Patient_Profile', 'Patient_Profile'),
                    ('Briefing.writer', 'Briefing', 'cintel:Briefing', 'Briefing')) AS o(ch, spec, entity, step)
 WHERE r.status IN ('completed', 'errors') AND ped.cpipes_step_id = 'reducing05'
 ORDER BY ped.key, o.ch;

-- The completed run's Patient_Profile records: each member's details, and as
-- the claim summary the profile the data set carries for that member, written
-- by an earlier version of the pipeline (patient_profiles_expected.csv).
CREATE TEMP TABLE shots_expected (member_id text, profile_prose text);
\copy shots_expected FROM '/fixture-data/patient_profiles_expected.csv' WITH (FORMAT csv, HEADER true)

INSERT INTO public."cintel:Patient_Profile"
  ("hc:Member_DOB", "hc:Member_First_Name", "hc:Member_Gender", "hc:Member_ID", "hc:Member_Last_Name",
   "hc:Member_Relationship", "hc:Member_Zip", "cintel:Claim_Summary", "cintel:Model_Name",
   "jets:client", "jets:key", "rdf:type", ":domain_key", ":shard_id", session_id, last_update)
SELECT e."hc:Member_DOB", e."hc:Member_First_Name", e."hc:Member_Gender", e."hc:Member_ID",
       e."hc:Member_Last_Name", e."hc:Member_Relationship", e."hc:Member_Zip", x.profile_prose, 'template',
       'CI', md5(r.session_id || e."hc:Member_ID"), ARRAY['cintel:Patient_Profile'], e."hc:Member_ID", 0,
       r.session_id, r.finished
  FROM public."hc:Eligibility" e
  JOIN shots_expected x ON x.member_id = e."hc:Member_ID"
  JOIN shots_ppt_run r ON r.status = 'completed'
 ORDER BY e."hc:Member_ID";

-- The run with errors: the rules found no description for two diagnosis codes.
-- (M54.50 and M54.51 are newer than the jets_ws diagnosis lookup.)
INSERT INTO jetsapi.process_errors
  (pipeline_execution_status_key, session_id, grouping_key, row_jets_key, input_column, error_message,
   shard_id, cpipes_step_id, error_channel, operator_type, last_update)
SELECT 112, '1767778200302', c."hc:Member_ID", c."jets:key", 'hc:Seconday_Diagnosis_Code1',
       format('DiagnosisDescriptionLookup: no row for key %s (claim %s)',
              c."hc:Seconday_Diagnosis_Code1", c."hc:Claim_ID"),
       0, 'reducing05', 'jetrules', 'jetrules', timestamp '2026-01-07 09:34:40'
  FROM public."hc:MedicalClaim" c
 WHERE c."hc:Seconday_Diagnosis_Code1" IN ('M5450', 'M5451')
 ORDER BY c."hc:Member_ID", c."hc:Claim_ID";

-- ---------------------------------------------------------------------------
-- 4. Archived runs: one moved by the retention purge, one by a user's deletion
-- of its session, which leaves a tombstone in session_deletion.
INSERT INTO jetsapi.pipeline_execution_status_archive
  (key, pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys,
   client, process_name, main_object_type, input_session_id, session_id, source_period_key, status,
   user_email, start_time, last_update, workspace_name, workspace_version, archived_at, archive_reason)
VALUES
  (91, 8, 91, 'jetstore/input/client=CI/org=D1/object_type=hc:Eligibility/year=2025/month=11/day=3/M021-M040_M007_M008_eligibility.csv',
   '{92,93}', 'CI', 'Patient_Profile_Template', 'hc:Eligibility', '1762162200191', '1762164000401', 101, 'completed',
   'admin@localhost.test', '2025-11-03 10:00:00', '2025-11-03 10:04:51', 'jets_ws', '1761955200',
   '2026-01-02 02:00:00', 'retention'),
  (92, 8, 94, 'jetstore/input/client=CI/org=D1/object_type=hc:Eligibility/year=2025/month=12/day=1/M021-M040_M007_M008_eligibility.csv',
   '{95,96}', 'CI', 'Patient_Profile_Template', 'hc:Eligibility', '1764581400194', '1764583200402', 102, 'completed',
   'admin@localhost.test', '2025-12-01 10:00:00', '2025-12-01 10:04:22', 'jets_ws', '1761955200',
   '2026-01-06 14:12:09', 'user');

INSERT INTO jetsapi.session_deletion
  (session_id, client, source_type, month_period, reason, requested_by, requested_at, status, completed_at)
SELECT '1764583200402', 'CI', 'domain_table', sp.month_period, 'user', 'admin@localhost.test',
       '2026-01-06 14:12:05', 'done', '2026-01-06 14:12:09'
  FROM jetsapi.source_period sp WHERE sp.key = 102;

-- ---------------------------------------------------------------------------
-- The sequences continue after every fixture's keys.
DO $$
BEGIN
  PERFORM setval(pg_get_serial_sequence('jetsapi.' || t, 'key'), 100000)
     FROM unnest(ARRAY['source_period', 'input_loader_status', 'input_registry', 'pipeline_execution_status',
                   'pipeline_execution_details', 'pipeline_execution_channel_details', 'cpipes_execution_status',
                   'cpipes_execution_status_details', 'process_errors']) AS t;
END
$$;

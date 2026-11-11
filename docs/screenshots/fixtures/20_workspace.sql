-- 20_workspace.sql: the data the workspace, agent-supervision and administration
-- screens show (docs/screenshots/specs/workspace/).
--
-- Applied after 00_base.sql. Data only, and every value invented:
--
--   1. fixed dates on the configuration rows the stack loads at start-up, so
--      the screens that list them (the Workspaces registry, the configuration
--      flows, User Administration) are the same at every run;
--   2. three more user accounts, at localhost.test addresses;
--   3. two rule configurations for the client CI;
--   4. three agent change proposals, their runs, decisions and one transcript;
--   5. three incidents raised over the seeded Patient_Profile_Template runs,
--      with their hypotheses and transitions;
--   6. two detection task runs;
--   7. three rules unit test runs of jets_ws's own tests, with their checks.
--
-- Explicit keys are 20000 and above. The sequences of the tables this file
-- writes continue at 100000, so a row a spec creates through the app never
-- takes one of them.

\set ON_ERROR_STOP on
SET client_min_messages = warning;

-- ---------------------------------------------------------------------------
-- 1. Fixed dates. The configuration is loaded when the stack starts, so its
-- last_update columns hold the time of the run (client_registry, process_input,
-- process_mapping, process_config, workspace_registry, roles, users and the
-- others). They are set to 2026-01-02 08:00:00, the value 10_data.sql gives
-- source_config and pipeline_config. Rows the earlier fixtures wrote carry
-- dates in 2026-01 and are left alone.
DO $$
DECLARE
  t record;
BEGIN
  FOR t IN
    SELECT c.table_name
      FROM information_schema.columns c
      JOIN information_schema.tables tb
        ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
     WHERE c.table_schema = 'jetsapi' AND c.column_name = 'last_update'
       AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('UPDATE jetsapi.%I SET last_update = %L WHERE last_update > %L',
                   t.table_name, '2026-01-02 08:00:00', '2026-03-01');
  END LOOP;
END
$$;

-- The jets_ws registry row. The apiserver registers its active workspace when
-- it starts, after the fixtures, with no repository (the stack compiles an
-- export) and the time of the run. A trigger gives the row the repository an
-- installation registers, which Pull Workspace needs, a description, and a
-- fixed date, whenever it is written.
CREATE OR REPLACE FUNCTION public.shots_fix_workspace_registry() RETURNS trigger AS $$
BEGIN
  IF NEW.workspace_name = 'jets_ws' THEN
    NEW.workspace_uri := coalesce(nullif(NEW.workspace_uri, ''), 'https://git.localhost.test/jetstore/jets_ws.git');
    NEW.description := coalesce(nullif(NEW.description, ''), 'The example workspace: the CI client and its patient profiles');
    NEW.last_update := '2026-01-02 08:00:00';
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS shots_fix_workspace_registry ON jetsapi.workspace_registry;
CREATE TRIGGER shots_fix_workspace_registry BEFORE INSERT OR UPDATE ON jetsapi.workspace_registry
  FOR EACH ROW EXECUTE FUNCTION public.shots_fix_workspace_registry();
UPDATE jetsapi.workspace_registry SET workspace_uri = workspace_uri WHERE workspace_name = 'jets_ws';

-- ---------------------------------------------------------------------------
-- 2. Users. The stack sets every address in a user_email column to its admin
-- after the fixtures; this trigger keeps the localhost.test accounts below as
-- they are, since an account's address is its key. Their passwords are not
-- password hashes, so none of them can sign in.
CREATE OR REPLACE FUNCTION public.shots_keep_fixture_users() RETURNS trigger AS $$
BEGIN
  IF OLD.user_email LIKE '%@localhost.test' THEN
    NEW.user_email := OLD.user_email;
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS shots_keep_fixture_users ON jetsapi.users;
CREATE TRIGGER shots_keep_fixture_users BEFORE UPDATE OF user_email ON jetsapi.users
  FOR EACH ROW EXECUTE FUNCTION public.shots_keep_fixture_users();

-- The apiserver writes the admin's row when it starts, after the fixtures,
-- with the time of the run; this trigger gives it a fixed date instead.
CREATE OR REPLACE FUNCTION public.shots_fix_admin_date() RETURNS trigger AS $$
BEGIN
  IF NEW.user_email = 'admin@localhost.test' THEN
    NEW.last_update := '2026-01-02 08:00:00';
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS shots_fix_admin_date ON jetsapi.users;
CREATE TRIGGER shots_fix_admin_date BEFORE INSERT OR UPDATE ON jetsapi.users
  FOR EACH ROW EXECUTE FUNCTION public.shots_fix_admin_date();

INSERT INTO jetsapi.users (user_email, name, password, encrypted_roles, is_active, last_update) VALUES
  ('dana.reyes@localhost.test',  'Dana Reyes',  'not-a-password-hash', '{ops_user}',           1, '2026-01-03 14:20:00'),
  ('kim.okafor@localhost.test',  'Kim Okafor',  'not-a-password-hash', '{knowledge_engineer}', 1, '2026-01-03 14:25:00'),
  ('lee.marsh@localhost.test',   'Lee Marsh',   'not-a-password-hash', '{}',                   0, '2026-01-12 08:40:00');

-- ---------------------------------------------------------------------------
-- 3. Rule configurations (Rules Configuration screen): per process and client,
-- the objects the rules read at the start of each session.
INSERT INTO jetsapi.rule_configv2 (key, process_config_key, process_name, client, rule_config_json, user_email, last_update) VALUES
  (20001, 50105, 'Patient_Profile_Template', 'CI',
   '[{"jets:key":{"value":"jets:iState","type":"text"},"rdf:type":{"value":"jets:State","type":"resource"},"jets:max_vertex_visits":{"value":"500000","type":"int"}}]',
   'admin@localhost.test', '2026-01-04 11:00:00'),
  (20002, 50101, 'Patient_Profile', 'CI',
   '[{"jets:key":{"value":"iRuleConfig001","type":"text"},"rdf:type":{"value":"cintel:RuleConfig","type":"resource"},"cintel:breakOnMemberID":{"value":"M007","type":"text"}}]',
   'admin@localhost.test', '2026-01-04 11:05:00');

-- ---------------------------------------------------------------------------
-- 4. Agent change proposals (Proposals screen). Three authoring runs, each
-- with the proposal it wrote; the first has a transcript in the audit chain.
INSERT INTO jetsapi.agent_run (run_id, agent_id, agent_version, model_id, prompt_version, tier,
                               started_at, ended_at, run_status, triggered_by, domain_model_version,
                               iteration_cap, wall_clock_cap_seconds, token_spend) VALUES
  ('run_20001', 'authoring', '1.0.0', 'example-model-7b', 'authoring-v3', 'T2',
   '2026-01-06 08:00:00+00', '2026-01-06 08:04:10+00', 'completed', 'admin@localhost.test', 'jets_ws@789dd9b2', 12, 900, 18400),
  ('run_20002', 'authoring', '1.0.0', 'example-model-7b', 'authoring-v3', 'T2',
   '2026-01-06 09:10:00+00', '2026-01-06 09:13:30+00', 'completed', 'admin@localhost.test', 'jets_ws@789dd9b2', 12, 900, 15200),
  ('run_20003', 'authoring', '1.0.0', 'example-model-7b', 'authoring-v3', 'T2',
   '2026-01-08 13:00:00+00', '2026-01-08 13:02:45+00', 'completed', 'admin@localhost.test', 'jets_ws@789dd9b2', 12, 900, 9800);

INSERT INTO jetsapi.change_proposal (proposal_id, trigger, trigger_ref, affected_pipelines, rationale,
                                     generated_tests, impact_affected_assets, impact_clinical_relevance_touched,
                                     approval_state, proposal_model_version) VALUES
  ('prop_20001', 'authoring_run', 'run_20001', ARRAY['Map_PharmacyClaim'],
   E'Map_PharmacyClaim: give hc:Days_Supply a default of 30 when Days_Supply is empty.\n\n' ||
   E'No line of the delivery of 2026-01-05 is empty, but nothing guards the next one:\n' ||
   E'Patient_Profile_Template counts days of therapy from it, so an empty value counts as zero.\n\n' ||
   E'Mapping row, CI_D1_hc:PharmacyClaim:\n' ||
   E'  data_property  hc:Days_Supply\n' ||
   E'  input_column   Days_Supply\n' ||
   E'  default_value  30',
   ARRAY['map_pharmacy_claim_days_supply_default'], ARRAY['hc:PharmacyClaim', 'cintel:Patient_Profile'], true,
   'awaiting_human_approval', 'example-model-7b'),
  ('prop_20002', 'authoring_run', 'run_20002', ARRAY['Patient_Profile_Template'],
   E'lookups/diagnosis_info_lookup.csv: add the codes M5450 and M5451.\n\n' ||
   E'Run 1767778200302 of Patient_Profile_Template wrote 27 process errors, each a\n' ||
   E'DiagnosisDescriptionLookup with no row for one of these two codes.',
   ARRAY['diagnosis_lookup_m5450_m5451'], ARRAY['lookups/diagnosis_info_lookup.csv', 'cintel:Patient_Profile'], false,
   'approved', 'example-model-7b'),
  ('prop_20003', 'authoring_run', 'run_20003', ARRAY['Patient_Profile_Template'],
   E'Patient_Profile_Template: lower jets:max_vertex_visits from 1000000 to 1000.',
   ARRAY[]::text[], ARRAY['cintel:Patient_Profile'], false,
   'rejected', 'example-model-7b');

INSERT INTO jetsapi.approval_event (approval_event_id, run_ref, subject_ref, from_state, to_state, actor,
                                    tier_at_event, decided_at, decision_rationale) VALUES
  ('apr_20001', 'run_20001', 'prop_20001', 'draft', 'validated', 'authoring', 'T2', '2026-01-06 08:04:00+00', 'generated test passes'),
  ('apr_20002', 'run_20001', 'prop_20001', 'validated', 'agent_reviewed', 'authoring', 'T2', '2026-01-06 08:04:05+00', 'review found no other mapping of hc:Days_Supply'),
  ('apr_20003', 'run_20001', 'prop_20001', 'agent_reviewed', 'awaiting_human_approval', 'authoring', 'T2', '2026-01-06 08:04:10+00', 'touches a column the profile reads'),
  ('apr_20004', 'run_20002', 'prop_20002', 'draft', 'validated', 'authoring', 'T2', '2026-01-06 09:13:20+00', 'generated test passes'),
  ('apr_20005', 'run_20002', 'prop_20002', 'validated', 'agent_reviewed', 'authoring', 'T2', '2026-01-06 09:13:25+00', NULL),
  ('apr_20006', 'run_20002', 'prop_20002', 'agent_reviewed', 'awaiting_human_approval', 'authoring', 'T2', '2026-01-06 09:13:30+00', NULL),
  ('apr_20007', 'run_20002', 'prop_20002', 'awaiting_human_approval', 'approved', 'kim.okafor@localhost.test', 'T2', '2026-01-07 10:02:00+00', 'Same cleansing as NDC; fine.'),
  ('apr_20008', 'run_20003', 'prop_20003', 'draft', 'validated', 'authoring', 'T2', '2026-01-08 13:02:30+00', NULL),
  ('apr_20009', 'run_20003', 'prop_20003', 'validated', 'agent_reviewed', 'authoring', 'T2', '2026-01-08 13:02:40+00', NULL),
  ('apr_20010', 'run_20003', 'prop_20003', 'agent_reviewed', 'awaiting_human_approval', 'authoring', 'T2', '2026-01-08 13:02:45+00', NULL),
  ('apr_20011', 'run_20003', 'prop_20003', 'awaiting_human_approval', 'rejected', 'kim.okafor@localhost.test', 'T2', '2026-01-08 15:30:00+00', 'Too low for 22 members; no generated test.');

-- The first run's transcript. The table's trigger numbers each event and
-- chains its hash to the one before, so the screen can verify the chain.
INSERT INTO jetsapi.agent_audit (run_id, seq, event_type, actor, tier, tool_name, payload, created_at) VALUES
  ('run_20001', 0, 'intent', 'authoring', 'T2', NULL,
   '{"goal": "Guard hc:Days_Supply against an empty Days_Supply in CI pharmacy claims"}', '2026-01-06 08:00:00+00'),
  ('run_20001', 0, 'tool_call', 'authoring', 'T2', 'query',
   '{"sql": "SELECT count(*) FROM \"CI_D1_hc:PharmacyClaim\" WHERE coalesce(\"Days_Supply\", '''') = ''''", "rows": 1, "result": {"count": 0}}', '2026-01-06 08:01:10+00'),
  ('run_20001', 0, 'tool_call', 'authoring', 'T2', 'read_mapping',
   '{"table_name": "CI_D1_hc:PharmacyClaim", "data_property": "hc:Days_Supply", "default_value": null}', '2026-01-06 08:02:00+00'),
  ('run_20001', 0, 'decision', 'authoring', 'T2', NULL,
   '{"proposal_id": "prop_20001", "change": "default_value 30 on hc:Days_Supply"}', '2026-01-06 08:03:30+00'),
  ('run_20001', 0, 'outcome', 'authoring', 'T2', NULL,
   '{"proposal_id": "prop_20001", "state": "awaiting_human_approval"}', '2026-01-06 08:04:10+00');

-- ---------------------------------------------------------------------------
-- 5. Incidents (Incidents screen), over the seeded Patient_Profile_Template
-- runs of 00_base.sql: the failed, the interrupted and the one with errors.
INSERT INTO jetsapi.agent_run (run_id, agent_id, agent_version, model_id, prompt_version, tier,
                               started_at, ended_at, run_status, triggered_by, domain_model_version,
                               iteration_cap, wall_clock_cap_seconds, token_spend) VALUES
  ('run_20010', 'shadow_triage', '1.0.0', 'rules', 'triage-v2', 'T0',
   '2026-01-09 06:00:00+00', '2026-01-09 06:01:20+00', 'completed', 'schedule', 'jets_ws@789dd9b2', 1, 600, 0);

INSERT INTO jetsapi.incident (incident_id, incident_session_id, incident_run_ref, incident_detected_at,
                              incident_locus, classification, severity, status, incident_step_ref,
                              incident_shard_ref, incident_confounders, incident_model_version) VALUES
  ('inc_20001', '1767868200303', 'run_20010', '2026-01-09 06:01:00+00',
   'worker_failed', NULL, 'high', 'diagnosed', NULL, 2, ARRAY[]::text[], 'triage-v2'),
  ('inc_20002', '1767949200304', 'run_20010', '2026-01-09 06:01:10+00',
   'worker_not_terminated', NULL, 'medium', 'triaged', NULL, NULL, ARRAY['stall_cause_unknown'], 'triage-v2'),
  ('inc_20003', '1767778200302', NULL, '2026-01-07 11:00:00+00',
   'per_record_failures_reported', 'source_content_change', 'low', 'verified', NULL, NULL, ARRAY[]::text[], 'triage-v2');

INSERT INTO jetsapi.hypothesis (hypothesis_id, hypothesis_incident_ref, cause, cause_category, confidence, rank,
                                supporting_evidence, contradicting_evidence, hypothesis_locus, basis) VALUES
  ('hyp_20001', 'inc_20001', 'The rules worker of shard 2 ran out of memory', 'capacity_or_cost_deviation', 0.67, 1,
   '[{"statement": "Shard 2 stopped while shards 0, 1 and 3 completed", "source": "cpipes_execution_status_details", "source_ref": "1767868200303"},
     {"statement": "The run failed in the jetrules step", "source": "pipeline_execution_status", "source_ref": "1767868200303"}]',
   '[{"statement": "The same input completed in run 1767608400301 two days earlier", "source": "pipeline_execution_status", "source_ref": "1767608400301"}]',
   'worker_failed', '{"supporting_count": 2, "contradicting_count": 1, "evidenceability": "conditional"}'),
  ('hyp_20002', 'inc_20001', 'A rule loops on one member until the vertex visit limit', 'transformation_defect', 0.5, 2,
   '[{"statement": "max_vertex_visits is set for this process", "source": "pipeline_config", "source_ref": "CI/Patient_Profile_Template"}]',
   '[{"statement": "No process error names the vertex visit limit", "source": "process_errors", "source_ref": "1767868200303"}]',
   'worker_failed', '{"supporting_count": 1, "contradicting_count": 1, "evidenceability": "coarse"}');

INSERT INTO jetsapi.incident_event (incident_event_id, event_incident_ref, from_status, to_status, event_actor,
                                    event_actor_kind, transitioned_at, event_run_ref, classification_before,
                                    classification_after, transition_rationale) VALUES
  ('ie_20001', 'inc_20001', 'detected', 'triaged', 'shadow_triage', 'agent', '2026-01-09 06:01:05+00', 'run_20010', NULL, NULL,
   'Locus worker_failed is present in the execution record'),
  ('ie_20002', 'inc_20001', 'triaged', 'diagnosed', 'shadow_triage', 'agent', '2026-01-09 06:01:15+00', 'run_20010', NULL, NULL,
   'Two hypotheses ranked by evidence for and against'),
  ('ie_20003', 'inc_20002', 'detected', 'triaged', 'shadow_triage', 'agent', '2026-01-09 06:01:12+00', 'run_20010', NULL, NULL,
   'Locus worker_not_terminated is present in the execution record'),
  ('ie_20004', 'inc_20003', 'detected', 'triaged', 'admin@localhost.test', 'human', '2026-01-07 11:30:00+00', NULL, NULL, NULL,
   '27 process errors, each a diagnosis code missing from the lookup'),
  ('ie_20005', 'inc_20003', 'triaged', 'diagnosed', 'admin@localhost.test', 'human', '2026-01-07 11:35:00+00', NULL, NULL, 'source_content_change',
   'Codes M5450 and M5451 are new in this delivery'),
  ('ie_20006', 'inc_20003', 'diagnosed', 'remediation_proposed', 'admin@localhost.test', 'human', '2026-01-07 11:40:00+00', NULL, NULL, NULL,
   'Add the two codes to the diagnosis lookup'),
  ('ie_20007', 'inc_20003', 'remediation_proposed', 'awaiting_approval', 'admin@localhost.test', 'human', '2026-01-07 11:41:00+00', NULL, NULL, NULL, NULL),
  ('ie_20008', 'inc_20003', 'awaiting_approval', 'remediating', 'admin@localhost.test', 'human', '2026-01-07 14:00:00+00', NULL, NULL, NULL, NULL),
  ('ie_20009', 'inc_20003', 'remediating', 'resolved', 'admin@localhost.test', 'human', '2026-01-08 09:00:00+00', NULL, NULL, NULL,
   'Lookup updated and the workspace compiled'),
  ('ie_20010', 'inc_20003', 'resolved', 'verified', 'admin@localhost.test', 'human', '2026-01-08 09:30:00+00', NULL, NULL, NULL,
   'The next run had no process error');

-- ---------------------------------------------------------------------------
-- 6. Detection task runs (Detection screen): one scheduled, one started from
-- the screen. Each task runs anomaly detection, then shadow triage.
INSERT INTO jetsapi.agentic_detection_run (key, task_ref, command, trigger, requested_by, status, since_window,
                                           requested_at, started_at, finished_at, summary, error_message, last_update) VALUES
  (20001, 'task-20001', 'anomaly_detect', 'schedule', '', 'ok', '26h',
   NULL, '2026-01-09 06:00:05', '2026-01-09 06:00:40',
   '{"dryRun": false, "purgeRegime": "none", "channelDetails": true, "workerRows": 48, "headerlessWorkerRows": 0, "truncated": false, "baselineRead": true, "baselines": 6, "anomaliesFound": 2, "anomaliesWritten": 2, "anomaliesAlreadyRecorded": 0, "byDetector": {"worker_stall": 1, "duration": 1}}',
   '', '2026-01-09 06:00:40'),
  (20002, 'task-20001', 'shadow_triage', 'schedule', '', 'ok', '26h',
   NULL, '2026-01-09 06:00:41', '2026-01-09 06:01:20',
   '{"dryRun": false, "attestOnly": false, "purgeRegime": "none", "channelDetails": true, "runsClassified": 3, "incidentsWritten": 2, "hypothesesWritten": 2, "alreadyRaised": 0, "unmappedLoci": [], "census": {"worker_failed": {"present": 1, "absent": 2, "notEvaluable": 0}, "worker_not_terminated": {"present": 1, "absent": 2, "notEvaluable": 0}, "rows_lost_silently": {"present": 0, "absent": 2, "notEvaluable": 1}}, "attestationRead": true, "attestationHeld": true, "attestation": "Shadow mode holds: no incident has passed diagnosed, and no remediation has been written."}',
   '', '2026-01-09 06:01:20'),
  (20003, 'task-20003', 'anomaly_detect', 'manual', 'admin@localhost.test', 'ok', '168h',
   '2026-01-10 08:30:00', '2026-01-10 08:31:10', '2026-01-10 08:31:50',
   '{"dryRun": false, "purgeRegime": "none", "channelDetails": true, "workerRows": 61, "headerlessWorkerRows": 0, "truncated": false, "baselineRead": true, "baselines": 6, "anomaliesFound": 2, "anomaliesWritten": 0, "anomaliesAlreadyRecorded": 2, "byDetector": {"worker_stall": 1, "duration": 1}}',
   '', '2026-01-10 08:31:50'),
  (20004, 'task-20003', 'shadow_triage', 'manual', 'admin@localhost.test', 'ok', '168h',
   '2026-01-10 08:30:00', '2026-01-10 08:31:51', '2026-01-10 08:32:30',
   '{"dryRun": false, "attestOnly": false, "purgeRegime": "none", "channelDetails": true, "runsClassified": 5, "incidentsWritten": 0, "hypothesesWritten": 0, "alreadyRaised": 2, "unmappedLoci": [], "census": {"worker_failed": {"present": 1, "absent": 4, "notEvaluable": 0}, "worker_not_terminated": {"present": 1, "absent": 4, "notEvaluable": 0}}, "attestationRead": true, "attestationHeld": true, "attestation": "Shadow mode holds: no incident has passed diagnosed, and no remediation has been written."}',
   '', '2026-01-10 08:32:30');

-- ---------------------------------------------------------------------------
-- 7. Rules unit test runs (Rules Unit Tests flow) of three of jets_ws's tests.
INSERT INTO jetsapi.rules_unit_test_execution (key, workspace_name, test_path, test_name, engine, legacy_mode, status,
                                               reason, load_ms, run_ms, duration_ms, sessions, loops, log, user_email, last_update) VALUES
  (20001, 'jets_ws', 'jet_rules/unit_tests/salience_shared_consequent/salience_shared_consequent.tst.json',
   'salience, shared consequent', 'native', 'fixed', 'pass', '', 41, 3, 52, 1, 1,
   'session one: 3 checks, 3 passed', 'admin@localhost.test', '2026-01-04 15:10:00'),
  (20002, 'jets_ws', 'jet_rules/unit_tests/retraction_defect1/retraction_defect1.tst.json',
   'retraction defect 1', 'native', 'fixed', 'pass', '', 38, 4, 50, 2, 1,
   E'session control: 1 check, 1 passed\nsession withdrawn: 1 check, 1 passed', 'admin@localhost.test', '2026-01-04 15:10:05'),
  (20003, 'jets_ws', 'jet_rules/unit_tests/salience_shared_consequent/salience_shared_consequent.tst.json',
   'salience, shared consequent', 'native', 'fixed', 'fail', '1 of 3 checks failed', 40, 3, 51, 1, 1,
   'session one: 3 checks, 2 passed, 1 failed', 'admin@localhost.test', '2026-01-04 14:55:00');

INSERT INTO jetsapi.rules_unit_test_check (key, execution_key, session_name, kind, "check", outcome, found, engine) VALUES
  (20001, 20001, 'one', 'expected', '{"s":"main1","p":"ut_ssal:value","o":{"value":"7","type":"int"}}', 'pass', '7', ''),
  (20002, 20001, 'one', 'expected', '{"s":"main1","p":"ut_ssal:late","o":{"value":"7","type":"int"}}', 'pass', '7', ''),
  (20003, 20001, 'one', 'expected', '{"s":"main1","p":"ut_ssal:count","o":{"value":"1","type":"int"}}', 'pass', '1', ''),
  (20004, 20003, 'one', 'expected', '{"s":"main1","p":"ut_ssal:value","o":{"value":"7","type":"int"}}', 'pass', '7', ''),
  (20005, 20003, 'one', 'expected', '{"s":"main1","p":"ut_ssal:late","o":{"value":"7","type":"int"}}', 'pass', '7', ''),
  (20006, 20003, 'one', 'expected', '{"s":"main1","p":"ut_ssal:count","o":{"value":"1","type":"int"}}', 'fail', '0', '');

-- ---------------------------------------------------------------------------
-- The sequences of the tables above continue at 100000.
DO $$
BEGIN
  PERFORM setval(pg_get_serial_sequence('jetsapi.' || t, 'key'), 100000)
     FROM unnest(ARRAY['rule_configv2', 'agentic_detection_run', 'rules_unit_test_execution',
                       'rules_unit_test_check', 'runbook_run', 'runbook_step_run']) AS t;
END
$$;

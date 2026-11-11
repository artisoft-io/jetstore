-- 10_data.sql: what the data and pipeline screens need beyond 00_base.sql.
--
-- The screens of the User Guide's data and pipeline pages (Home, its detail
-- screens, the Query Tool and the flows that load files and start pipelines)
-- are drawn from 00_base.sql's rows. This file adds no row. It fixes one thing
-- those screens would otherwise change at every run:
--
--   the configuration's timestamps. update_db loads jets_ws's CI configuration
--   at the stack's start, so source_config and pipeline_config carry the time
--   of the run in last_update. The Load Files and Start Pipeline flows show that
--   column (Last Updated, Loaded At), so their images would differ every run.
--   Both are set to one fixed date, before the files of 00_base.sql arrived.
--
-- Keys from 10000 are this file's, should a later version need rows of its own.

\set ON_ERROR_STOP on
SET client_min_messages = warning;

UPDATE jetsapi.source_config   SET last_update = timestamp '2026-01-02 08:00:00';
UPDATE jetsapi.pipeline_config SET last_update = timestamp '2026-01-02 08:00:00';

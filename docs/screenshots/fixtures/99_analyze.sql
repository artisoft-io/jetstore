-- Runs last, after every other fixture: gathers the planner's statistics once.
--
-- A data table sorts by one column with LIMIT and OFFSET, and rows that tie on
-- it come back in the order the chosen plan produces. Without this, whether
-- autovacuum has analysed a table by the time a spec reads it decides the plan,
-- so the same rows could appear in a different order from one run to the next,
-- and an image of them would be rewritten for no change in the UI. Analysing
-- here makes the plan, and so the order, the same on every run.
ANALYZE;

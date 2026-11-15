package datatable

// InsertSourcePeriod, and the failure row a register-key hook records with the
// period it returns, against a real PostgreSQL (scratch_postgres_test.go).
//
// A workspace's register-key hook that fails a folder before registering it
// records the failure in pipeline_execution_status itself, filed under the
// period the folder's file key names, which it takes from InsertSourcePeriod
// as RegisterFileKeys does. The hook's own tests assert its statement as text,
// with no database; that the statement is accepted by the shipped tables, and
// that the period it is filed under exists even in a deployment that has never
// seen that date, was verified by hand. This is that verification, committed
// on this side, where both the function and the tables are.

import (
	"context"
	"testing"
)

// InsertSourcePeriod inserts a period it does not find, with the month, week
// and day periods CalculatePeriod gives, and returns the same key for it after.
func TestInsertSourcePeriodInsertsOnceAndReturnsTheKey(t *testing.T) {
	pool := createTablesFromSchema(t, "source_period")
	ctx := context.Background()
	key, err := InsertSourcePeriod(pool, 2024, 8, 2)
	if err != nil {
		t.Fatalf("inserting a period into an empty source_period: %v", err)
	}
	again, err := InsertSourcePeriod(pool, 2024, 8, 2)
	if err != nil || again != key {
		t.Fatalf("the same period again: key %d, %v; want %d", again, err, key)
	}
	other, err := InsertSourcePeriod(pool, 2026, 8, 22)
	if err != nil || other == key {
		t.Fatalf("another period: key %d, %v; want a key other than %d", other, err, key)
	}
	var n int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.source_period").Scan(&n); err != nil || n != 2 {
		t.Fatalf("source_period holds %d rows, %v; want 2", n, err)
	}
	sp, err := LoadSourcePeriod(pool, key)
	if err != nil {
		t.Fatal(err)
	}
	month, week, day := CalculatePeriod(2024, 8, 2)
	if sp.Year != 2024 || sp.Month != 8 || sp.Day != 2 ||
		sp.MonthPeriod != month || sp.WeekPeriod != week || sp.DayPeriod != day {
		t.Errorf("period %d is %+v, want 2024-8-2 with periods %d %d %d", key, sp, month, week, day)
	}
}

// The failure row a register-key hook records for a folder it fails before
// registering it: no pipeline configuration, no input registry, the sentinel
// session ids, and the period of the folder's own date. The shipped
// pipeline_execution_status accepts it with nothing else supplied, and it is
// filed under that period rather than the latest one.
func TestARegisterKeyFailureRowIsFiledUnderItsOwnPeriod(t *testing.T) {
	pool := createTablesFromSchema(t, "source_period", "pipeline_execution_status")
	ctx := context.Background()
	// A later period exists, as it does in any deployment that has run since.
	if _, err := InsertSourcePeriod(pool, 2026, 8, 22); err != nil {
		t.Fatal(err)
	}
	periodKey, err := InsertSourcePeriod(pool, 2024, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	const folder = "client=ACME/org=Org1/object_type=Claims/year=2024/month=8/day=2/run=1"
	_, err = pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status (
  pipeline_config_key, main_input_registry_key, main_input_file_key,
  client, process_name, main_object_type, input_session_id, session_id,
  source_period_key, status, failure_details, user_email) VALUES
  (0, 0, $1, $2, $3, $4, 'F000000000000', 'F000000000000',
   $6, 'failed', $5, 'system')`,
		folder, "ACME", "Claims_Process", "Claims", "error: input folder contains no data files", periodKey)
	if err != nil {
		t.Fatalf("the failure row was refused: %v", err)
	}
	var year, month, day int
	var status string
	err = pool.QueryRow(ctx, `SELECT sp.year, sp.month, sp.day, pe.status
		FROM jetsapi.pipeline_execution_status pe JOIN jetsapi.source_period sp ON sp.key = pe.source_period_key
		WHERE pe.main_input_file_key = $1`, folder).Scan(&year, &month, &day, &status)
	if err != nil {
		t.Fatal(err)
	}
	if year != 2024 || month != 8 || day != 2 || status != "failed" {
		t.Errorf("filed under %d-%d-%d with status %q, want 2024-8-2 and failed", year, month, day, status)
	}
}

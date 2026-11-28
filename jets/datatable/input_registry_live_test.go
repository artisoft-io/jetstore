package datatable

// Registrations a session deletion marked deleted (input_registry.deleted_at):
// a live reader does not see them, a reader of what a run read does. Needs
// JETS_TEST_DSN; skipped otherwise.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/inputregistry"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const registryLiveDB = "jm_input_registry_live"

func markInputOf(t *testing.T, pool *pgxpool.Pool, peKey int) {
	t.Helper()
	purgetest.Exec(t, pool, `UPDATE jetsapi.input_registry ir SET deleted_at = now()
		FROM jetsapi.pipeline_execution_status pe WHERE pe.key = $1 AND ir.key = pe.main_input_registry_key`, peKey)
}

// Live: a run whose input's session was deleted (its registration marked) is
// not resubmitted. History: the run's input still resolves its schema provider.
func TestMarkedInputIsNotResubmittedAndStillResolves(t *testing.T) {
	pool := purgetest.DB(t, registryLiveDB)
	ctx := context.Background()
	// purgetest.DB writes source_period key 1, which seedRun's insert would
	// meet on the column's sequence.
	purgetest.Exec(t, pool, `SELECT setval(pg_get_serial_sequence('jetsapi.source_period', 'key'), 100)`)
	peKey := seedRun(t, pool, "sess-marked", []string{"completed"})
	purgetest.Exec(t, pool, `UPDATE jetsapi.input_registry ir SET schema_provider_json = '{"request_id":"r1"}'
		FROM jetsapi.pipeline_execution_status pe WHERE pe.key = $1 AND ir.key = pe.main_input_registry_key`, peKey)
	if err := CheckResubmittable(ctx, pool, "sess-marked"); err != nil {
		t.Fatalf("a run whose input is live: %v", err)
	}
	markInputOf(t, pool, peKey)
	err := CheckResubmittable(ctx, pool, "sess-marked")
	if err == nil || !strings.Contains(err.Error(), "no longer registered") {
		t.Fatalf("a run whose input is marked deleted: got %v, want the purged-input refusal", err)
	}
	sp, sid, err := GetSchemaProviderJsonFromPipelineKey(pool, peKey)
	if err != nil || sp != `{"request_id":"r1"}` || sid != "sess-marked" {
		t.Fatalf("by key, input marked: got (%q, %q, %v), want its schema provider", sp, sid, err)
	}
	if sp, err = GetSchemaProviderJsonFromPipelineSession(pool, "sess-marked"); err != nil || sp != `{"request_id":"r1"}` {
		t.Fatalf("by session, input marked: got (%q, %v), want its schema provider", sp, err)
	}
}

// A screen reading the view lists the live registrations only; on a database
// without the view the table is read under the view's name, so the columns and
// where clauses that name it still resolve.
func TestReadActionReadsTheLiveRegistrations(t *testing.T) {
	pool := purgetest.DB(t, registryLiveDB)
	ctx := context.Background()
	t.Cleanup(inputregistry.Forget)
	for _, s := range []string{"live", "marked"} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
			(client, object_type, source_period_key, table_name, source_type, session_id, user_email)
			VALUES ('c', 'o', 1, 't', 'file', $1, 'u')`, s)
	}
	purgetest.Exec(t, pool, `UPDATE jetsapi.input_registry SET deleted_at = now() WHERE session_id = 'marked'`)
	read := func() (*DataTableAction, []string) {
		dtq := &DataTableAction{
			FromClauses:   []FromClause{{Schema: "jetsapi", Table: "input_registry_live"}},
			Columns:       []Column{{Table: "input_registry_live", Column: "session_id"}},
			WhereClauses:  []WhereClause{{Table: "input_registry_live", Column: "client", Values: []string{"c"}}},
			SortColumn:    "session_id",
			SortAscending: true,
			Limit:         10,
		}
		if err := dtq.resolveLiveRegistry(ctx, pool); err != nil {
			t.Fatal(err)
		}
		query, _ := dtq.buildQuery()
		rows, err := pool.Query(ctx, query)
		if err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			got = append(got, s)
		}
		return dtq, got
	}
	dtq, got := read()
	if dtq.FromClauses[0].Table != "input_registry_live" || !slices.Equal(got, []string{"live"}) {
		t.Errorf("with the view: read %s, got %v; want the view and [live]", dtq.FromClauses[0].Table, got)
	}

	purgetest.Exec(t, pool, `DROP VIEW jetsapi.input_registry_live; ALTER TABLE jetsapi.input_registry DROP COLUMN deleted_at`)
	inputregistry.Forget()
	dtq, got = read()
	fc := dtq.FromClauses[0]
	if fc.Table != "input_registry" || fc.AsTable != "input_registry_live" || !slices.Equal(got, []string{"live", "marked"}) {
		t.Errorf("without the view: read %s AS %s, got %v; want the table under the view's name and every row",
			fc.Table, fc.AsTable, got)
	}
}

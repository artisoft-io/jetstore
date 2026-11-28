package main

// jetsapi.input_registry_live, which jets_init_db.sql creates and this program
// runs: the live registrations, those of input_registry not marked deleted.
//
// TestInputRegistryLiveColumns needs no database. TestInputRegistryLive runs
// the statement on a scratch PostgreSQL named by JETS_TEST_DSN, skipped when it
// is unset. Run with -count=1: both read jets_init_db.sql and jets_schema.json.

import (
	"context"
	"regexp"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/inputregistry"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

// The view selects every column of input_registry but deleted_at, in
// jets_schema.json's order. No database needed.
func TestInputRegistryLiveColumns(t *testing.T) {
	var want []string
	for _, d := range purgetest.SchemaDefinitions(t) {
		if d.TableName == "input_registry" {
			for _, c := range d.Columns {
				if !c.Deleted && c.ColumnName != inputregistry.Column {
					want = append(want, c.ColumnName)
				}
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("jets_schema.json has no table input_registry")
	}
	m := regexp.MustCompile(`(?s)SELECT\n(.*?)\nFROM jetsapi\.input_registry\n`).FindStringSubmatch(purgetest.LiveViewStatement(t))
	if m == nil {
		t.Fatal("no select list in the view")
	}
	if got := selectNames(m[1]); !slices.Equal(got, want) {
		t.Errorf("input_registry_live selects\n%v\nwant input_registry's columns but deleted_at\n%v", got, want)
	}
}

// The view holds the rows whose deleted_at is NULL, and jets_init_db.sql
// recreates it over a view already deployed.
func TestInputRegistryLive(t *testing.T) {
	pool := purgetest.DB(t, "srg_input_registry_live")
	ctx := context.Background()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
		(client, object_type, source_period_key, table_name, source_type, session_id, user_email, deleted_at)
		VALUES ('c', 'o', 1, 't', 'file', 'live', 'u', NULL), ('c', 'o', 1, 't', 'file', 'marked', 'u', now())`)
	purgetest.Exec(t, pool, `DROP VIEW IF EXISTS jetsapi.input_registry_live;`+purgetest.LiveViewStatement(t))
	var sessions []string
	rows, err := pool.Query(ctx, `SELECT session_id FROM jetsapi.input_registry_live ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		sessions = append(sessions, s)
	}
	rows.Close()
	if !slices.Equal(sessions, []string{"live"}) {
		t.Errorf("the view holds %v, want [live]", sessions)
	}
}

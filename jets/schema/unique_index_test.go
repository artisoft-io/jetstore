// UpdateTable and a UNIQUE INDEX declared in the schema.
//
// The DB test needs JETS_TEST_DSN, as the tests beside it; the statement tests
// need nothing.
package schema

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Every index definition that is not a UNIQUE INDEX gets the statement it got
// before the fix: the shipped jets_schema.json's and the shapes the workspace
// domain tables use (DomainTableDefinitions, jets/workspace).
func TestCreateIndexIfNotExistsUnchanged(t *testing.T) {
	before := func(def string) string {
		return "CREATE INDEX IF NOT EXISTS " + strings.TrimPrefix(strings.TrimPrefix(def, "INDEX"), "index")
	}
	defs := []string{
		`INDEX t_domain_key_idx ON "s"."t"  (session_id, "jets:key" ASC)`,
		`INDEX t_shard_idx ON "s"."t"  (session_id, "shard_id")`,
		`index lower_case_idx ON jetsapi.t (a)`,
	}
	n := 0
	for _, td := range loadAllTableDefinitions(t) {
		for _, idx := range td.Indexes {
			defs = append(defs, idx.IndexDef)
			n++
		}
	}
	if n == 0 {
		t.Fatal("jets_schema.json declares no index")
	}
	for _, def := range defs {
		if uniqueIndexDef.MatchString(def) {
			continue
		}
		if got, want := createIndexIfNotExists(def), before(def); got != want {
			t.Errorf("%q:\n got %q\nwant %q", def, got, want)
		}
	}
}

func TestCreateIndexIfNotExistsUnique(t *testing.T) {
	for def, want := range map[string]string{
		"UNIQUE INDEX t_uidx ON jetsapi.t (a, b)":   "CREATE UNIQUE INDEX IF NOT EXISTS t_uidx ON jetsapi.t (a, b)",
		"unique index t_uidx ON jetsapi.t (a)":      "CREATE UNIQUE INDEX IF NOT EXISTS t_uidx ON jetsapi.t (a)",
		"UNIQUE  INDEX t_uidx ON jetsapi.t (a)":     "CREATE UNIQUE INDEX IF NOT EXISTS t_uidx ON jetsapi.t (a)",
		"INDEX unique_name_idx ON jetsapi.t (a)":    "CREATE INDEX IF NOT EXISTS  unique_name_idx ON jetsapi.t (a)",
		"UNIQUE INDEXES_ARE_NOT_A_KEYWORD ON x (a)": "CREATE INDEX IF NOT EXISTS UNIQUE INDEXES_ARE_NOT_A_KEYWORD ON x (a)",
	} {
		if got := createIndexIfNotExists(def); got != want {
			t.Errorf("%q:\n got %q\nwant %q", def, got, want)
		}
	}
}

// A table with a UNIQUE INDEX migrates twice: CreateTable the first time,
// UpdateTable the second, which failed with a syntax error before the fix.
func TestUniqueIndexSurvivesASecondMigration(t *testing.T) {
	pool := migrationTestPool(t)
	ctx := context.Background()
	const table = "unique_index_migration_test"
	drop := func() {
		if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi."+table); err != nil {
			t.Fatal(err)
		}
	}
	drop()
	t.Cleanup(drop)
	td := TableDefinition{
		SchemaName: "jetsapi",
		TableName:  table,
		Columns: []ColumnDefinition{
			{ColumnName: "key", DataType: "int", IsPK: true},
			{ColumnName: "name", DataType: "text", IsNotNull: true},
			{ColumnName: "version", DataType: "text"},
		},
		Indexes: []IndexDefinition{
			{IndexName: table + "_name_uidx", IndexDef: fmt.Sprintf("UNIQUE INDEX %s_name_uidx ON jetsapi.%s (name)", table, table)},
			{IndexName: table + "_version_idx", IndexDef: fmt.Sprintf("INDEX %s_version_idx ON jetsapi.%s (version)", table, table)},
		},
	}
	for pass := 1; pass <= 3; pass++ {
		if err := td.UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("migration %d: %v", pass, err)
		}
	}
	var unique bool
	err := pool.QueryRow(ctx, `SELECT i.indisunique FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = $1`, table+"_name_uidx").Scan(&unique)
	if err != nil || !unique {
		t.Fatalf("the index is not unique (%v)", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO jetsapi."+table+" (name) VALUES ('a')"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO jetsapi."+table+" (name) VALUES ('a')"); err == nil {
		t.Fatal("a duplicate name was accepted")
	}
}

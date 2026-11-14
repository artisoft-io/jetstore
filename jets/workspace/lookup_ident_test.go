package workspace

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestSqliteIdent(t *testing.T) {
	for name, want := range map[string]string{
		"acme:DrugLookup": `"acme:DrugLookup"`,
		`say "hi"`:        `"say ""hi"""`,
		`back\slash`:      `"back\slash"`,
		"":                `""`,
	} {
		if got := sqliteIdent(name); got != want {
			t.Errorf("sqliteIdent(%q) = %s, want %s", name, got, want)
		}
	}
}

// A lookup table and a column whose names hold a double quote and a backslash
// package under exactly those names. With Go's %q quoting, the table name was
// written "odd\"name\\x", which SQLite reads as the identifier odd\ followed
// by stray text, and the compile failed.
func TestLookupNamesWithAQuoteAndABackslash(t *testing.T) {
	const tblName, colName = `acme:odd"name\x`, `fee"x\y`
	wsDir := t.TempDir()
	writeFile(t, wsDir, "lookups/odd.csv", "CODE,"+`"fee""x\y"`+"\nA1,12.5\n")
	tables := []*rete.LookupTableNode{{
		Name:    tblName,
		CsvFile: "lookups/odd.csv",
		Key:     []string{"CODE"},
		Columns: []rete.LookupTableColumn{{Name: colName, Type: "double"}},
	}}
	ensure(t, tables, wsDir)

	dbPath := filepath.Join(wsDir, "lookup.db")
	found := false
	for _, n := range tableNames(t, dbPath) {
		found = found || n == tblName
	}
	if !found {
		t.Fatalf("lookup.db tables %q; want one named %s", tableNames(t, dbPath), tblName)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var fee float64
	q := "SELECT " + sqliteIdent(colName) + " FROM " + sqliteIdent(tblName) + ` WHERE "jets:key" = 'A1'`
	if err := db.QueryRow(q).Scan(&fee); err != nil || fee != 12.5 {
		t.Fatalf("%s: %v, %v; want 12.5", q, fee, err)
	}
	var indexes int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ?", tblName).Scan(&indexes); err != nil || indexes != 2 {
		t.Fatalf("indexes on %s: %d, %v; want 2", tblName, indexes, err)
	}
}

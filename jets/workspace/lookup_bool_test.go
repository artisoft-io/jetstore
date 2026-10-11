package workspace

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// A lookup's bool column (rules_unit_test_harness I-45, row 2): the accepted
// spellings package as 1 or 0, an empty cell as NULL, which both engines serve
// as rdf null, and anything else stops the compile, naming where it is.
func TestLookupBoolColumnPackaging(t *testing.T) {
	wsDir := t.TempDir()
	writeFile(t, wsDir, "lookups/drug.csv", "CODE,NAME,MAINT\nA1,aspirin, YES \nB2,insulin,\nC3,statin,n\n")
	writeFile(t, wsDir, "lookups/fee.csv", "CPT,YEAR,FEE\n99213,2015,73.97\n")
	tables := newLookupTables()
	ensure(t, tables, wsDir)

	db, err := sql.Open("sqlite", "file:"+filepath.Join(wsDir, "lookup.db")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var table string
	for _, name := range tableNames(t, filepath.Join(wsDir, "lookup.db")) {
		if strings.Contains(name, "Drug") {
			table = name
		}
	}
	if table == "" {
		t.Fatal("no drug lookup table in lookup.db")
	}
	rows, err := db.Query(`SELECT "jets:key", "MAINT" FROM "` + table + `" ORDER BY _rowid_`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var key string
		var maint sql.NullInt64
		if err := rows.Scan(&key, &maint); err != nil {
			t.Fatal(err)
		}
		if maint.Valid {
			got[key] = map[int64]string{0: "0", 1: "1"}[maint.Int64]
		} else {
			got[key] = "NULL"
		}
	}
	want := map[string]string{"A1": "1", "B2": "NULL", "C3": "0"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("MAINT for %s = %s, want %s (all: %v)", k, got[k], v, got)
		}
	}
}

func TestLookupBoolColumnRefusesOtherValues(t *testing.T) {
	wsDir := t.TempDir()
	writeFile(t, wsDir, "lookups/drug.csv", "CODE,NAME,MAINT\nA1,aspirin,true\nB2,insulin,maybe\n")
	writeFile(t, wsDir, "lookups/fee.csv", "CPT,YEAR,FEE\n99213,2015,73.97\n")
	_, err := ensureLookupDb(newLookupTables(), wsDir, filepath.Join(wsDir, "lookup.db"))
	if err == nil {
		t.Fatal("a bool cell reading maybe packaged; want an error")
	}
	for _, part := range []string{"acme:DrugLookup", "row 3", "column MAINT", `"maybe" is not a bool`} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not name %s", err, part)
		}
	}
}

package workspace

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// lookupFixture is a workspace root holding two lookup CSVs, and the lookup
// definitions a compile of it would collect.
func lookupFixture(t *testing.T) (wsDir string, tables []*rete.LookupTableNode) {
	t.Helper()
	wsDir = t.TempDir()
	writeFile(t, wsDir, "lookups/drug.csv", "CODE,NAME,MAINT\nA1,aspirin,true\nB2,insulin,false\nB2,insulin pen,true\n")
	writeFile(t, wsDir, "lookups/fee.csv", "CPT,YEAR,FEE\n99213,2015,73.97\n99214,2015,108.92\n")
	return wsDir, newLookupTables()
}

func newLookupTables() []*rete.LookupTableNode {
	return []*rete.LookupTableNode{
		{
			Name:    "acme:DrugLookup",
			CsvFile: "lookups/drug.csv",
			Key:     []string{"CODE"},
			Columns: []rete.LookupTableColumn{{Name: "NAME", Type: "text"}, {Name: "MAINT", Type: "bool"}},
		},
		{
			Name:    "acme:FeeLookup",
			CsvFile: "lookups/fee.csv",
			Key:     []string{"CPT", "YEAR"},
			Columns: []rete.LookupTableColumn{{Name: "FEE", Type: "double"}},
		},
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readBytes(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func ensure(t *testing.T, tables []*rete.LookupTableNode, wsDir string) LookupDbResult {
	t.Helper()
	res, err := ensureLookupDb(tables, wsDir, filepath.Join(wsDir, "lookup.db"))
	if err != nil {
		t.Fatalf("ensureLookupDb: %v", err)
	}
	return res
}

func tableNames(t *testing.T, dbPath string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	return names
}

func TestEnsureLookupDbBuildsWhenMissingThenSkips(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	dbPath := filepath.Join(wsDir, "lookup.db")

	first := ensure(t, tables, wsDir)
	if !first.Rebuilt || first.Reason != "no lookup.db" {
		t.Fatalf("first compile: got %+v, want a rebuild because there is no lookup.db", first)
	}
	got := strings.Join(tableNames(t, dbPath), ",")
	if want := "__jets_lookup_db_meta__,acme:DrugLookup,acme:FeeLookup"; got != want {
		t.Fatalf("tables: got %s, want %s", got, want)
	}
	built := readBytes(t, dbPath)

	second := ensure(t, newLookupTables(), wsDir)
	if second.Rebuilt {
		t.Fatalf("unchanged inputs: got a rebuild (%s)", second.Reason)
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatalf("fingerprint moved with unchanged inputs: %s then %s", first.Fingerprint, second.Fingerprint)
	}
	if !bytes.Equal(built, readBytes(t, dbPath)) {
		t.Fatal("a skipped compile changed lookup.db")
	}
	if !strings.Contains(second.String(), "packaging skipped") {
		t.Fatalf("log line does not say it skipped: %q", second.String())
	}
}

// The property the skip rests on: a lookup.db that was kept is exactly the
// file a rebuild would have written, so skipping is unobservable.
func TestSkippedLookupDbIsByteIdenticalToARebuild(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	dbPath := filepath.Join(wsDir, "lookup.db")
	ensure(t, tables, wsDir)
	if res := ensure(t, tables, wsDir); res.Rebuilt {
		t.Fatalf("expected a skip, got a rebuild (%s)", res.Reason)
	}
	kept := readBytes(t, dbPath)

	// A forced rebuild over the kept file, in place.
	if err := forceRebuildLookupDb(tables, wsDir); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kept, readBytes(t, dbPath)) {
		t.Fatal("rebuilding over the kept lookup.db produced different bytes")
	}

	// And a build in a second workspace that never had a lookup.db.
	otherDir, _ := lookupFixture(t)
	ensure(t, newLookupTables(), otherDir)
	if !bytes.Equal(kept, readBytes(t, filepath.Join(otherDir, "lookup.db"))) {
		t.Fatal("a fresh build elsewhere produced different bytes from the kept lookup.db")
	}
}

// forceRebuildLookupDb is the forced rebuild, for a workspace root
// other than the one the package globals name.
func forceRebuildLookupDb(lookupTables []*rete.LookupTableNode, wsDir string) error {
	tables := uniqueLookupTables(lookupTables)
	v, err := sqliteLibraryVersion()
	if err != nil {
		return err
	}
	fp, err := lookupDbFingerprint(tables, wsDir, v)
	if err != nil {
		return err
	}
	return packageLookupTables(tables, wsDir, filepath.Join(wsDir, "lookup.db"), fp, v)
}

func TestEnsureLookupDbRebuildsWhenAnInputChanges(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, wsDir string, tables []*rete.LookupTableNode)
	}{
		{"csv row appended", func(t *testing.T, wsDir string, _ []*rete.LookupTableNode) {
			writeFile(t, wsDir, "lookups/fee.csv", "CPT,YEAR,FEE\n99213,2015,73.97\n99214,2015,108.92\n99215,2015,145.00\n")
		}},
		{"csv byte changed, same length", func(t *testing.T, wsDir string, _ []*rete.LookupTableNode) {
			writeFile(t, wsDir, "lookups/fee.csv", "CPT,YEAR,FEE\n99213,2015,73.98\n99214,2015,108.92\n")
		}},
		{"column type", func(_ *testing.T, _ string, tables []*rete.LookupTableNode) {
			tables[1].Columns[0].Type = "text"
		}},
		{"column added", func(_ *testing.T, _ string, tables []*rete.LookupTableNode) {
			tables[1].Columns = append(tables[1].Columns, rete.LookupTableColumn{Name: "YEAR", Type: "int"})
		}},
		{"key", func(_ *testing.T, _ string, tables []*rete.LookupTableNode) {
			tables[1].Key = []string{"CPT"}
		}},
		{"table renamed", func(_ *testing.T, _ string, tables []*rete.LookupTableNode) {
			tables[0].Name = "acme:DrugLookup2"
		}},
		{"csv file repointed at identical content", func(t *testing.T, wsDir string, tables []*rete.LookupTableNode) {
			writeFile(t, wsDir, "lookups/fee_copy.csv", string(readBytes(t, filepath.Join(wsDir, "lookups/fee.csv"))))
			tables[1].CsvFile = "lookups/fee_copy.csv"
		}},
		{"table order", func(_ *testing.T, _ string, tables []*rete.LookupTableNode) {
			tables[0], tables[1] = tables[1], tables[0]
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wsDir, tables := lookupFixture(t)
			first := ensure(t, tables, wsDir)
			tables = newLookupTables()
			c.change(t, wsDir, tables)
			res := ensure(t, tables, wsDir)
			if !res.Rebuilt {
				t.Fatal("input changed and lookup.db was kept")
			}
			if res.Fingerprint == first.Fingerprint {
				t.Fatal("input changed and the fingerprint did not")
			}
			if res.Reason != "lookup definitions, csv files or sqlite version changed" {
				t.Fatalf("reason: %q", res.Reason)
			}
			// And the change is in the file: a rebuild is a real one.
			if again := ensure(t, tables, wsDir); again.Rebuilt {
				t.Fatalf("second compile after the change rebuilt again (%s)", again.Reason)
			}
		})
	}
}

func TestEnsureLookupDbRebuildsOnFormatVersionBump(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	ensure(t, tables, wsDir)
	saved := lookupDbFormatVersion
	lookupDbFormatVersion = saved + 1
	defer func() { lookupDbFormatVersion = saved }()
	res := ensure(t, tables, wsDir)
	if !res.Rebuilt || !strings.Contains(res.Reason, "packager format version changed") {
		t.Fatalf("format version bump: got %+v", res)
	}
}

func TestLookupDbFingerprintCoversSqliteVersion(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	a, err := lookupDbFingerprint(tables, wsDir, "3.50.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := lookupDbFingerprint(tables, wsDir, "3.50.1")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("a sqlite library upgrade does not change the fingerprint")
	}
}

// A lookup the compiler collects twice (imported by two main rule files) is
// packaged once, from its first definition, and fingerprinted the same way.
func TestLookupDbFingerprintUsesTheFirstDefinition(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	dup := *tables[1]
	dup.Columns = []rete.LookupTableColumn{{Name: "FEE", Type: "text"}}
	withDup := append(newLookupTables(), &dup)
	a, err := lookupDbFingerprint(uniqueLookupTables(tables), wsDir, "v")
	if err != nil {
		t.Fatal(err)
	}
	b, err := lookupDbFingerprint(uniqueLookupTables(withDup), wsDir, "v")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("a later duplicate definition changed the fingerprint")
	}
}

func TestEnsureLookupDbRebuildsAnUnusableFile(t *testing.T) {
	cases := []struct {
		name   string
		damage func(t *testing.T, wsDir, dbPath string)
		reason string
	}{
		{"not a sqlite file", func(t *testing.T, _, dbPath string) {
			if err := os.WriteFile(dbPath, []byte("this is not a database, just text that is long enough"), 0644); err != nil {
				t.Fatal(err)
			}
		}, "existing lookup.db not reusable: file is not a database"},
		{"truncated", func(t *testing.T, _, dbPath string) {
			b := readBytes(t, dbPath)
			if err := os.WriteFile(dbPath, b[:len(b)/2], 0644); err != nil {
				t.Fatal(err)
			}
		}, "existing lookup.db not reusable: database disk image is malformed"},
		{"built before the fingerprint existed", func(t *testing.T, wsDir, dbPath string) {
			tables := uniqueLookupTables(newLookupTables())
			if err := packageLookupTables(tables, wsDir, dbPath, "", "v"); err != nil {
				t.Fatal(err)
			}
		}, "existing lookup.db not reusable: no input fingerprint"},
		{"empty file", func(t *testing.T, _, dbPath string) {
			if err := os.WriteFile(dbPath, nil, 0644); err != nil {
				t.Fatal(err)
			}
		}, "existing lookup.db not reusable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wsDir, tables := lookupFixture(t)
			dbPath := filepath.Join(wsDir, "lookup.db")
			ensure(t, tables, wsDir)
			good := readBytes(t, dbPath)
			c.damage(t, wsDir, dbPath)
			res := ensure(t, newLookupTables(), wsDir)
			if !res.Rebuilt || !strings.HasPrefix(res.Reason, c.reason) {
				t.Fatalf("got %+v, want a rebuild with reason %q", res, c.reason)
			}
			if !bytes.Equal(good, readBytes(t, dbPath)) {
				t.Fatal("the rebuild did not restore the original bytes")
			}
		})
	}
}

// Today's in-place rebuild dropped and recreated each table it knew about, so
// a lookup removed from the rules stayed in lookup.db. A fresh build drops it.
func TestRebuildDropsALookupRemovedFromTheRules(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	dbPath := filepath.Join(wsDir, "lookup.db")
	ensure(t, tables, wsDir)
	ensure(t, newLookupTables()[:1], wsDir)
	got := strings.Join(tableNames(t, dbPath), ",")
	if want := "__jets_lookup_db_meta__,acme:DrugLookup"; got != want {
		t.Fatalf("tables: got %s, want %s", got, want)
	}
}

// A build that fails leaves the previous lookup.db as it was, and no
// half-written file beside it.
func TestFailedRebuildKeepsThePreviousLookupDb(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	dbPath := filepath.Join(wsDir, "lookup.db")
	ensure(t, tables, wsDir)
	before := readBytes(t, dbPath)

	tables = newLookupTables()
	tables[1].Key = []string{"NO_SUCH_COLUMN"}
	if _, err := ensureLookupDb(tables, wsDir, dbPath); err == nil {
		t.Fatal("expected the build to fail on a key column the csv lacks")
	}
	if !bytes.Equal(before, readBytes(t, dbPath)) {
		t.Fatal("a failed build changed lookup.db")
	}
	if _, err := os.Stat(dbPath + ".building"); !os.IsNotExist(err) {
		t.Fatalf("a failed build left %s.building behind (%v)", dbPath, err)
	}
}

func TestEnsureLookupDbRebuildsWhenACsvIsMissing(t *testing.T) {
	wsDir, tables := lookupFixture(t)
	ensure(t, tables, wsDir)
	if err := os.Remove(filepath.Join(wsDir, "lookups/fee.csv")); err != nil {
		t.Fatal(err)
	}
	// The fingerprint cannot be taken, so the packager runs and reports the
	// missing file in its own words rather than the old lookup.db being kept.
	_, err := ensureLookupDb(newLookupTables(), wsDir, filepath.Join(wsDir, "lookup.db"))
	if err == nil || !strings.Contains(err.Error(), "while opening lookup table csv file") {
		t.Fatalf("got %v, want the packager's missing-csv error", err)
	}
}

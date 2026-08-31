package compute_pipes

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The cold start of 2026-08-31, reproduced.
//
// **This is a real regression test and it is worth saying so**, because the two
// defects before it in this area were not testable at that level: a directory
// mode needs a second uid to observe, and this needs only two goroutines.
//
// The failing shape was a double-checked lock with the check on the wrong side
// of the work — `domainTablesMap = make(...)` published an empty map, then
// filled it. A goroutine reaching the unsynchronised outer test in that window
// saw non-nil, skipped the lock, and returned an **empty cache with a nil
// error**, so its caller reported the class "not found in the local workspace"
// against a workspace that was fine.
//
// In production it was six workers and five losers. Here it is `n` goroutines
// released together, and any one of them seeing an empty map fails the test.
func TestDomainCachesAreNotPublishedBeforeTheyAreFull(t *testing.T) {
	ws := t.TempDir()
	build := filepath.Join(ws, "testws", "build")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	// Two entries is enough; what is under test is emptiness, not content.
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(build, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tables.json", `{"cintel:Patient_Profile":{"name":"t1"},"hc:Eligibility":{"name":"t2"}}`)
	write("classes.json", `{"cintel:Patient_Profile":{"name":"c1"},"hc:Eligibility":{"name":"c2"}}`)
	write("properties.json", `{"cintel:has_x":{"name":"p1"},"cintel:has_y":{"name":"p2"}}`)

	oldHome, oldPrefix := workspaceHome, wsPrefix
	workspaceHome, wsPrefix = ws, "testws"
	t.Cleanup(func() {
		workspaceHome, wsPrefix = oldHome, oldPrefix
		ClearJetrulesCaches()
	})

	for _, tc := range []struct {
		name string
		get  func() (int, error)
	}{
		{"domain tables", func() (int, error) { m, err := GetWorkspaceDomainTables(); return len(m), err }},
		{"domain classes", func() (int, error) { m, err := GetWorkspaceDomainClasses(); return len(m), err }},
		{"data properties", func() (int, error) { m, err := GetWorkspaceDataProperties(); return len(m), err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ClearJetrulesCaches()
			const n = 16
			var start sync.WaitGroup
			var done sync.WaitGroup
			start.Add(1)
			counts := make([]int, n)
			errs := make([]error, n)
			for i := range n {
				done.Add(1)
				go func(i int) {
					defer done.Done()
					start.Wait() // release them together, to widen the window
					counts[i], errs[i] = tc.get()
				}(i)
			}
			start.Done()
			done.Wait()
			for i := range n {
				if errs[i] != nil {
					t.Fatalf("goroutine %d: unexpected error: %v", i, errs[i])
				}
				if counts[i] != 2 {
					t.Errorf("goroutine %d saw %d entries, want 2 — an empty cache returned with a "+
						"nil error is the production failure: the caller then reports the class "+
						"missing from a workspace that is intact", i, counts[i])
				}
			}
		})
	}
}

// The flat record's columns are the class's data properties.
//
// **This is the defect that nulled `cintel:has_Medical_Events`**, one layer up
// from where it was reported. The column reached `extractLiteralValue` because
// `GetDomainProperties` put it there, and `GetMultiValueDataProperties` excludes
// object properties from the multi-value set — so a multi-valued object property
// looked single-valued and its data was dropped with a warning. Both halves came
// from the commit that introduced object properties.
//
// The fixture is `cintel:Patient_Profile`'s real shape: a text data property
// that receives the `toon` serialisation, two `resource` object properties that
// are walked to build it, and a multi-valued data property that must survive.
func TestFlatRecordColumnsAreDataPropertiesOnly(t *testing.T) {
	ws := t.TempDir()
	build := filepath.Join(ws, "testws", "build")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	tables := `{"cintel:Patient_Profile":{"table_name":"cintel:Patient_Profile",` +
		`"class_name":"cintel:Patient_Profile","columns":[` +
		`{"column_name":"cintel:Claim_Summary","type":"text"},` +
		`{"column_name":"cintel:has_Medical_Events","type":"resource","is_object":true,"as_array":true},` +
		`{"column_name":"hc:Member_ID","type":"text"},` +
		`{"column_name":"jets:ruleTag","type":"text","as_array":true}]}}`
	if err := os.WriteFile(filepath.Join(build, "tables.json"), []byte(tables), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome, oldPrefix := workspaceHome, wsPrefix
	workspaceHome, wsPrefix = ws, "testws"
	t.Cleanup(func() {
		workspaceHome, wsPrefix = oldHome, oldPrefix
		ClearJetrulesCaches()
	})
	ClearJetrulesCaches()

	columns, err := GetDomainProperties("cintel:Patient_Profile", false)
	if err != nil {
		t.Fatalf("GetDomainProperties: %v", err)
	}
	got := make(map[string]bool, len(columns))
	for _, c := range columns {
		got[c] = true
	}
	if got["cintel:has_Medical_Events"] {
		t.Error("an object property must not be a column of a flat record — " +
			"this is what reached extractLiteralValue and had its values nulled")
	}
	for _, want := range []string{
		"jets:key", "rdf:type", "jets:source_period_sequence",
		"cintel:Claim_Summary", "hc:Member_ID", "jets:ruleTag",
	} {
		if !got[want] {
			t.Errorf("%s should be a column, got %v", want, columns)
		}
	}
	// The column that receives the toon serialisation is a *data* property and
	// survives, which is why the filter needs no knowledge of column_encodings.
	if !got["cintel:Claim_Summary"] {
		t.Error("the encoded column is a data property and must survive the filter")
	}
}

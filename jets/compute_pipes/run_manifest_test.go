package compute_pipes

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// twoTableRun is the declaration half used by most of these: two tables and one
// file, which is the smallest configuration that can be sliced.
func twoTableRun() *ComputePipesConfig {
	return &ComputePipesConfig{
		OutputTables: []*TableSpec{
			{Key: "claims_out", Name: "jetsapi.claims"},
			{Key: "members_out", Name: "jetsapi.members"},
		},
		OutputFiles: []OutputFileSpec{
			{Key: "export", FileName2: "export.csv"},
		},
	}
}

func buildOrFail(t *testing.T, cfg *ComputePipesConfig, observed []ObservedChannel) *CpipesRunManifest {
	t.Helper()
	m, err := BuildRunManifest(cfg, observed, RunManifestMeta{
		SessionId: "s1", ProcessName: "p1", PipelineExecutionKey: 7, Status: "completed",
		WrittenAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRunManifest: %v", err)
	}
	return m
}

func TestRunManifestFileKeyIsRunScoped(t *testing.T) {
	got := RunManifestFileKey("stage", "myprocess", "sess-1")
	want := "stage/process_name=myprocess/session_id=sess-1/run_manifest.json"
	if got != want {
		t.Fatalf("RunManifestFileKey = %q, want %q", got, want)
	}
	// The ruling's whole point about the key: it names the run and nothing
	// finer. A step or partition component would make the document one copy
	// per slice, which is the storage choice defeating the content.
	for _, forbidden := range []string{"step_id=", "jets_partition=", "node_id="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the manifest key carries %q, so it is not run-scoped: %s", forbidden, got)
		}
	}
}

// describesEveryDeclaration is the criterion "whole, never sliced" written as a
// function, so that the test below can drive it in both directions. A manifest
// that describes one table of a two-table run must fail it.
func describesEveryDeclaration(cfg *ComputePipesConfig, m *CpipesRunManifest) error {
	declared := make([]string, 0, len(cfg.OutputTables)+len(cfg.OutputFiles))
	for _, s := range cfg.OutputTables {
		declared = append(declared, DeclaredInOutputTables+"/"+s.Key)
	}
	for i := range cfg.OutputFiles {
		declared = append(declared, DeclaredInOutputFiles+"/"+cfg.OutputFiles[i].Key)
	}
	got := make(map[string]bool, len(m.Entries))
	for _, e := range m.Entries {
		got[e.DeclaredIn+"/"+e.Channel] = true
	}
	missing := make([]string, 0)
	for _, d := range declared {
		if !got[d] {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("the manifest describes %d of %d declared deliverables; missing %v",
			len(got), len(declared), missing)
	}
	return nil
}

// TestManifestIsWholeNeverSliced is acceptance criterion 10, and it is written
// so that it proves itself: the same assertion that passes on the manifest a
// two-table run produces must fail on a manifest describing one of them.
func TestManifestIsWholeNeverSliced(t *testing.T) {
	cfg := twoTableRun()
	observed := []ObservedChannel{
		{OutputType: SinkDbTable, OutputChannel: "claims_out", OutputLocation: "sql://jetsapi.claims",
			LocationCount: 1, SinksCount: 4, RecordsCount: 1200},
		{OutputType: SinkDbTable, OutputChannel: "members_out", OutputLocation: "sql://jetsapi.members",
			LocationCount: 1, SinksCount: 4, RecordsCount: 340},
		{OutputType: SinkOutputFile, OutputChannel: "export", OutputLocation: "s3://acme/out/export.csv",
			LocationCount: 1, SinksCount: 1, RecordsUnknown: true, PartsCount: 1},
	}
	m := buildOrFail(t, cfg, observed)
	if err := describesEveryDeclaration(cfg, m); err != nil {
		t.Fatalf("a whole manifest failed the whole-manifest assertion: %v", err)
	}

	// The mutation. A manifest that lost one declaration must fail, or the
	// assertion above says nothing.
	sliced := *m
	sliced.Entries = m.Entries[:1]
	if err := describesEveryDeclaration(cfg, &sliced); err == nil {
		t.Fatal("a manifest describing 1 of 3 declared deliverables passed the whole-manifest assertion, " +
			"so the assertion detects nothing")
	}
}

// TestManifestOmitsUndeclaredEdges pins the subject rule: the manifest lists
// what the document declared, not what the graph did. A jets_partition edge is
// a shard and is not a deliverable.
func TestManifestOmitsUndeclaredEdges(t *testing.T) {
	cfg := twoTableRun()
	observed := []ObservedChannel{
		{OutputType: SinkDbTable, OutputChannel: "claims_out", OutputLocation: "sql://jetsapi.claims",
			LocationCount: 1, SinksCount: 1, RecordsCount: 10},
		// The splitter's own edge: a real row of the table, and not a
		// deliverable. Its channel is not in output_tables or output_files.
		{OutputType: SinkJetsPartition, OutputChannel: "split_by_year", OutputLocation: "s3://acme/parts",
			LocationCount: 1, SinksCount: 12, RecordsCount: 5000, PartsCount: 12},
		// An intermediate table nothing declared.
		{OutputType: SinkDbTable, OutputChannel: "scratch", OutputLocation: "sql://jetsapi.scratch",
			LocationCount: 1, SinksCount: 1, RecordsCount: 99},
	}
	m := buildOrFail(t, cfg, observed)
	if len(m.Entries) != 3 {
		t.Fatalf("got %d entries, want the 3 declarations", len(m.Entries))
	}
	buf, _ := json.Marshal(m)
	for _, forbidden := range []string{"split_by_year", "scratch", SinkJetsPartition} {
		if strings.Contains(string(buf), forbidden) {
			t.Errorf("the manifest names %q, which the document did not declare: %s", forbidden, buf)
		}
	}
}

// TestNotMeasuredIsNotZero is the distinction the whole nullable column exists
// for: a merge step records NULL deliberately, and reporting it as 0 would hand
// a consumer a measurement nobody made.
func TestNotMeasuredIsNotZero(t *testing.T) {
	cfg := &ComputePipesConfig{
		OutputTables: []*TableSpec{{Key: "empty_table", Name: "jetsapi.empty"}},
		OutputFiles:  []OutputFileSpec{{Key: "merged"}},
	}
	observed := []ObservedChannel{
		// A table that genuinely wrote no rows: 0 is a measurement.
		{OutputType: SinkDbTable, OutputChannel: "empty_table", OutputLocation: "sql://jetsapi.empty",
			LocationCount: 1, SinksCount: 1, RecordsCount: 0},
		// A merge: bytes moved, no record parsed.
		{OutputType: SinkOutputFile, OutputChannel: "merged", OutputLocation: "s3://acme/out/m.csv",
			LocationCount: 1, SinksCount: 1, RecordsUnknown: true, PartsCount: 1},
	}
	m := buildOrFail(t, cfg, observed)
	if m.Entries[0].RecordsCount == nil || *m.Entries[0].RecordsCount != 0 {
		t.Errorf("a measured zero must stay 0, got %v", m.Entries[0].RecordsCount)
	}
	if m.Entries[1].RecordsCount != nil {
		t.Errorf("an unmeasurable sink must be null, got %v", *m.Entries[1].RecordsCount)
	}
	buf, _ := json.MarshalIndent(m, "", " ")
	if !strings.Contains(string(buf), `"records_count": null`) {
		t.Errorf("the json must carry an explicit null rather than omitting the field: %s", buf)
	}
}

// TestSqlEntryCarriesNoParts is R14's discriminator doing its job: one entry
// shape, and the location URI says how to read it.
func TestSqlEntryCarriesNoParts(t *testing.T) {
	cfg := twoTableRun()
	observed := []ObservedChannel{
		{OutputType: SinkDbTable, OutputChannel: "claims_out", OutputLocation: "sql://jetsapi.claims",
			LocationCount: 1, SinksCount: 1, RecordsCount: 5},
		{OutputType: SinkOutputFile, OutputChannel: "export", OutputLocation: "s3://acme/out/export.csv",
			LocationCount: 1, SinksCount: 1, RecordsUnknown: true, PartsCount: 3},
	}
	m := buildOrFail(t, cfg, observed)
	for _, e := range m.Entries {
		switch {
		case strings.HasPrefix(e.Location, "sql://") && e.PartsCount != nil:
			t.Errorf("a sql:// entry carries parts_count %d", *e.PartsCount)
		case strings.HasPrefix(e.Location, "s3://") && e.PartsCount == nil:
			t.Errorf("an s3:// entry carries no parts_count: %+v", e)
		}
	}
}

// TestDeclaredButNotWritten: a declaration the run produced nothing for is in
// the manifest and says so. That is a fact about the run; omitting it would be
// a gap in the manifest, and the two must not look alike.
func TestDeclaredButNotWritten(t *testing.T) {
	cfg := twoTableRun()
	m := buildOrFail(t, cfg, []ObservedChannel{
		{OutputType: SinkDbTable, OutputChannel: "claims_out", OutputLocation: "sql://jetsapi.claims",
			LocationCount: 1, SinksCount: 1, RecordsCount: 5},
	})
	if err := describesEveryDeclaration(cfg, m); err != nil {
		t.Fatalf("%v", err)
	}
	for _, e := range m.Entries {
		if e.Channel == "members_out" {
			if e.Written || e.Location != "" || e.RecordsCount != nil {
				t.Errorf("an unwritten declaration must say so plainly: %+v", e)
			}
		}
	}
}

// TestUnresolvedBucketRefusesTheManifest is acceptance criterion 12's half.
// Recording the location would produce a document naming a bucket nobody can
// open, asserted by the one artefact a consumer cannot check.
func TestUnresolvedBucketRefusesTheManifest(t *testing.T) {
	cfg := &ComputePipesConfig{OutputFiles: []OutputFileSpec{{Key: "export"}}}
	for _, location := range []string{
		"s3://${CORPUS_OUT_BUCKET}/out/export.csv",
		"s3://$CORPUS_OUT_BUCKET/out/export.csv",
	} {
		observed := []ObservedChannel{
			{OutputType: SinkOutputFile, OutputChannel: "export", OutputLocation: location,
				LocationCount: 1, SinksCount: 1, PartsCount: 1},
		}
		m, err := BuildRunManifest(cfg, observed, RunManifestMeta{SessionId: "s1", Status: "completed"})
		if err == nil {
			t.Errorf("location %q produced a manifest rather than a refusal: %+v", location, m)
			continue
		}
		if m != nil {
			t.Errorf("a refused manifest must be nil, got %+v", m)
		}
		if !strings.Contains(err.Error(), "unresolved") {
			t.Errorf("the refusal must say what is wrong, got: %v", err)
		}
	}

	// And the control: a resolved bucket is written without complaint. Without
	// this, a builder that refused everything would pass the test above.
	observed := []ObservedChannel{
		{OutputType: SinkOutputFile, OutputChannel: "export", OutputLocation: "s3://acme-out/out/export.csv",
			LocationCount: 1, SinksCount: 1, PartsCount: 1},
	}
	m, err := BuildRunManifest(cfg, observed, RunManifestMeta{SessionId: "s1", Status: "completed"})
	if err != nil {
		t.Fatalf("a resolved bucket was refused: %v", err)
	}
	if m.Entries[0].Note != "" {
		t.Errorf("a resolved external bucket needs no note, got %q", m.Entries[0].Note)
	}
}

// TestSentinelBucketIsNotedRatherThanRefused: the sentinel is a legal
// configuration value and every writer in the tree resolves it before
// recording, so a manifest carrying it is a surprise rather than a failure --
// but it is unresolvable by a consumer, which is what the note is for.
func TestSentinelBucketIsNotedRatherThanRefused(t *testing.T) {
	cfg := &ComputePipesConfig{OutputFiles: []OutputFileSpec{{Key: "export"}}}
	observed := []ObservedChannel{
		{OutputType: SinkOutputFile, OutputChannel: "export",
			OutputLocation: "s3://" + JetStoreBucketSentinel + "/out/export.csv",
			LocationCount:  1, SinksCount: 1, PartsCount: 1},
	}
	m, err := BuildRunManifest(cfg, observed, RunManifestMeta{SessionId: "s1", Status: "completed"})
	if err != nil {
		t.Fatalf("the sentinel must not refuse the manifest: %v", err)
	}
	if !strings.Contains(m.Entries[0].Note, "sentinel") {
		t.Errorf("the entry must say the location is not resolvable outside the deployment, got %q",
			m.Entries[0].Note)
	}
}

// TestManifestCarriesItsSchemaVersion: the document is read outside this
// repository, beside another document of the same file name, so it says what
// it is.
func TestManifestCarriesItsSchemaVersion(t *testing.T) {
	m := buildOrFail(t, twoTableRun(), nil)
	if m.Schema != RunManifestSchema {
		t.Errorf("Schema = %q, want %q", m.Schema, RunManifestSchema)
	}
	if m.Status != "completed" {
		t.Errorf("Status = %q; a manifest exists only for a completed run and says so", m.Status)
	}
}

func TestBuildRunManifestRefusesNoConfig(t *testing.T) {
	if _, err := BuildRunManifest(nil, nil, RunManifestMeta{}); err == nil {
		t.Fatal("a manifest built from no configuration has no declared half and must be refused")
	}
}

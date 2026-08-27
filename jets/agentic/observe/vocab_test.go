package observe

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The generated DDL is the authority for the three vocabularies. These tests
// need no database: they read the CHECK constraints out of the SQL the audit
// package embeds and assert the Go constants against them, so regenerating the
// model with a member added or removed fails here rather than on an insert.
//
// The file is read by path rather than embedded because it belongs to the
// audit package and is generated into it; a second //go:embed of the same file
// would be a second copy to keep in step, which is the drift this test exists
// to catch.
const ddlPath = "../audit/agent_audit.sql"

func readDDL(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(ddlPath)
	if err != nil {
		t.Fatalf("reading the generated DDL at %s: %v", ddlPath, err)
	}
	return string(b)
}

var quoted = regexp.MustCompile(`'([a-z0-9_]+)'`)

// vocabularyIn extracts the quoted members of the named CHECK constraint.
func vocabularyIn(t *testing.T, ddl, constraint string) []string {
	t.Helper()
	i := strings.Index(ddl, constraint)
	if i < 0 {
		t.Fatalf("constraint %s is not in %s; has the model been regenerated?", constraint, ddlPath)
	}
	// A constraint clause runs to the next one or to the end of the CREATE
	// TABLE. Cutting at the newline instead would truncate the array
	// constraint, whose CHECK is on the line after its name; cutting at the
	// cast alone would let a single-line constraint swallow the next.
	rest := ddl[i:]
	end := len(rest)
	for _, delim := range []string{"\n  CONSTRAINT", "\n);"} {
		if j := strings.Index(rest, delim); j >= 0 && j < end {
			end = j
		}
	}
	rest = rest[:end]
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(rest, -1) {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		t.Fatalf("no members found in constraint %s", constraint)
	}
	return out
}

func assertSameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Errorf("%s drifted from the generated DDL:\n  Go:  %v\n  DDL: %v", what, g, w)
	}
}

func TestVocabulariesMatchDDL(t *testing.T) {
	ddl := readDDL(t)
	assertSameSet(t, "signalTypes", signalTypes, vocabularyIn(t, ddl, "anomaly_signal_type_ck"))
	assertSameSet(t, "subjectTypes", subjectTypes, vocabularyIn(t, ddl, "anomaly_subject_type_ck"))
	assertSameSet(t, "confounders", confounders, vocabularyIn(t, ddl, "anomaly_confounders_ck"))
}

// RecordConfounders is a claim about the boundary this package draws: these are
// the members the execution record can establish, and the five it omits are
// properties of a pipeline's configuration. If a new member is added to the
// vocabulary it belongs on one side or the other, and this test is what asks.
func TestRecordConfoundersIsASubsetAndTheRestAreConfigOnly(t *testing.T) {
	configOnly := []string{
		ConfounderParquetInput, ConfounderOnErrorDrop, ConfounderMaxInputCount,
		ConfounderSamplingCap, ConfounderDeviceWriterOutput,
	}
	for _, c := range RecordConfounders {
		if !slices.Contains(confounders, c) {
			t.Errorf("RecordConfounders holds %q, which is not in the vocabulary", c)
		}
		if slices.Contains(configOnly, c) {
			t.Errorf("%q is a configuration property and cannot be read off the record", c)
		}
	}
	if got := len(RecordConfounders) + len(configOnly); got != len(confounders) {
		t.Errorf("the two halves are %d members and the vocabulary is %d: a new member has been added and not classified",
			got, len(confounders))
	}
}

func TestAnomalyValidate(t *testing.T) {
	good := func() *Anomaly {
		return &Anomaly{
			AnomalyId:     "a1",
			SessionId:     "s1",
			SubjectType:   SubjectWorker,
			SubjectRef:    "42",
			SignalType:    SignalVolume,
			ObservedValue: "0",
			ExpectedBasis: "12 prior runs",
			DetectorRef:   "volume_collapse@1",
		}
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("a complete anomaly did not validate: %v", err)
	}

	for name, mutate := range map[string]func(*Anomaly){
		"no expected basis": func(a *Anomaly) { a.ExpectedBasis = "" },
		"no detector ref":   func(a *Anomaly) { a.DetectorRef = "" },
		"bad signal":        func(a *Anomaly) { a.SignalType = "vollume" },
		"bad subject":       func(a *Anomaly) { a.SubjectType = "shard" },
		"bad confounder":    func(a *Anomaly) { a.Confounders = []string{"probably_fine"} },
	} {
		a := good()
		mutate(a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

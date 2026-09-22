package compute_pipes

import (
	"testing"
)

// prepareCpipesEnv applies the document's `context` entries after the main
// schema provider's env is in place, so every type that existed before
// default_value overwrites what the deployment supplied. That is the whole of
// healthcare_corpus's P9-I115 (read 2026-09-20) -- "no context entry can
// default it" -- and it is a statement about the three types rather than about
// the mechanism, which has carried a fill-behind for $DATE_FILE_KEY all along.
//
// These tests pin both halves: the new arm fills in behind, and the three old
// arms still assign.

func envFor(t *testing.T, providerEnv map[string]any, context []ContextSpec) map[string]any {
	t.Helper()
	startup := &CpipesStartup{
		ProcessName: "a_process",
		MainInputSchemaProviderConfig: &SchemaProviderSpec{
			Key:  "_main_input_",
			Env:  providerEnv,
			Type: "default",
		},
		// ClusterConfig is dereferenced unqualified at the end of
		// prepareCpipesEnv, so the helper supplies one. The contract calls
		// cluster_config optional and the engine does not -- see the issue this
		// track reports; this test is not the place to repair it.
		CpConfig: ComputePipesConfig{Context: context, ClusterConfig: &ClusterSpec{}},
	}
	args := &StartComputePipesArgs{
		FileKey:   "client/object_type/year=2024/month=8/day=2/input.csv",
		SessionId: "session1",
	}
	env, err := prepareCpipesEnv(args, startup)
	if err != nil {
		t.Fatalf("prepareCpipesEnv: %v", err)
	}
	return env
}

// Criterion 16, first half: it supplies a value the schema event omitted.
func TestContextDefaultValueSuppliesWhatTheEventOmitted(t *testing.T) {
	env := envFor(t, map[string]any{}, []ContextSpec{
		{Type: "default_value", Key: "${CORPUS_OUT_BUCKET}", Expr: "jetstore_bucket"},
	})
	if got := env["${CORPUS_OUT_BUCKET}"]; got != "jetstore_bucket" {
		t.Errorf("with no value on the schema event the document's default must be taken, got %v", got)
	}
}

// Criterion 16, second half, and the half that makes the type worth having: the
// deployment wins. A default_value that overwrote would be `value` with a
// longer name.
func TestContextDefaultValueDoesNotOverwriteTheEvent(t *testing.T) {
	env := envFor(t, map[string]any{"${CORPUS_OUT_BUCKET}": "acme-corpus-out"}, []ContextSpec{
		{Type: "default_value", Key: "${CORPUS_OUT_BUCKET}", Expr: "jetstore_bucket"},
	})
	if got := env["${CORPUS_OUT_BUCKET}"]; got != "acme-corpus-out" {
		t.Errorf("the schema event's value must survive a default_value entry, got %v", got)
	}
}

// The three pre-existing types keep their behaviour exactly. This is the
// regression the change could plausibly cause -- the arm was added inside the
// switch that drives all four -- and it is the claim the plan makes in so many
// words.
func TestContextValueStillOverwritesTheEvent(t *testing.T) {
	env := envFor(t, map[string]any{"$CLIENT": "from_the_event"}, []ContextSpec{
		{Type: "value", Key: "$CLIENT", Expr: "from_the_document"},
	})
	if got := env["$CLIENT"]; got != "from_the_document" {
		t.Errorf("`value` must still assign unconditionally, got %v", got)
	}
}

func TestContextFileKeyComponentStillOverwritesTheEvent(t *testing.T) {
	env := envFor(t, map[string]any{"$YEAR": "from_the_event"}, []ContextSpec{
		{Type: "file_key_component", Key: "$YEAR", Expr: "year"},
	})
	if got := env["$YEAR"]; got != 2024 {
		t.Errorf("`file_key_component` must still assign unconditionally, got %v", got)
	}
}

// A key the event carries with a nil value counts as absent, which is the
// meaning $DATE_FILE_KEY's own guard has always had. Pinned because `== nil` is
// not `not present` in Go and a reader is entitled to wonder which was meant.
func TestContextDefaultValueTreatsANilEventValueAsAbsent(t *testing.T) {
	env := envFor(t, map[string]any{"${CORPUS_OUT_BUCKET}": nil}, []ContextSpec{
		{Type: "default_value", Key: "${CORPUS_OUT_BUCKET}", Expr: "jetstore_bucket"},
	})
	if got := env["${CORPUS_OUT_BUCKET}"]; got != "jetstore_bucket" {
		t.Errorf("a key present with a nil value is absent for this purpose, got %v", got)
	}
}

// The default arm still refuses what it does not know. A new token must not
// have widened the switch into accepting anything.
func TestContextUnknownTypeIsStillRefused(t *testing.T) {
	startup := &CpipesStartup{
		MainInputSchemaProviderConfig: &SchemaProviderSpec{Key: "_main_input_"},
		CpConfig: ComputePipesConfig{
			ClusterConfig: &ClusterSpec{},
			Context: []ContextSpec{
				{Type: "default", Key: "$X", Expr: "y"},
			},
		},
	}
	args := &StartComputePipesArgs{FileKey: "a/b.csv", SessionId: "s"}
	if _, err := prepareCpipesEnv(args, startup); err == nil {
		t.Error("an unknown ContextSpec type must still be an error")
	}
}

// The two arms compose the way P9-I115 needs them to: the document defaults the
// bucket, the gate then finds it resolved, and the same document with neither
// is refused. This is the pair the phase is actually delivering, so it is
// asserted as a pair rather than left to be inferred from two passing units.
func TestContextDefaultValueSatisfiesTheBucketGate(t *testing.T) {
	context := []ContextSpec{
		{Type: "default_value", Key: "${CORPUS_OUT_BUCKET}", Expr: JetStoreBucketSentinel},
	}
	withDefault := envFor(t, map[string]any{}, context)
	if err := ValidateResolvedBuckets(schemaProviderWithBucket("${CORPUS_OUT_BUCKET}"), nil, withDefault); err != nil {
		t.Errorf("a default_value entry must be enough to satisfy the startup gate, got: %v", err)
	}
	without := envFor(t, map[string]any{}, nil)
	if err := ValidateResolvedBuckets(schemaProviderWithBucket("${CORPUS_OUT_BUCKET}"), nil, without); err == nil {
		t.Error("without the default entry the same document must be refused, or the test above proves nothing")
	}
}

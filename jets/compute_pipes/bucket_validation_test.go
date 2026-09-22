package compute_pipes

import (
	"strings"
	"testing"
)

// The gate this file covers exists because the failure it prevents has no
// other signal: an unresolved ${...} bucket writes successfully, to a bucket
// spelled with a dollar sign in it. So the tests assert a refusal, and the
// negative controls assert that the two spellings of the JetStore bucket are
// not one.

func schemaProviderWithBucket(bucket string) *ComputePipesConfig {
	sp := &SchemaProviderSpec{Key: "_main_input_", Type: "default"}
	sp.Bucket = bucket
	return &ComputePipesConfig{SchemaProviders: []*SchemaProviderSpec{sp}}
}

func stepWritingToBucket(bucket string) []PipeSpec {
	out := OutputChannelConfig{Type: "output", Name: "deliverable"}
	out.Bucket = bucket
	return []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Type: "memory", Name: "input_row"},
		Apply: []TransformationSpec{{
			Type:          "partition_writer",
			OutputChannel: out,
		}},
	}}
}

func TestValidateResolvedBucketsRefusesUnresolvedSchemaProvider(t *testing.T) {
	cpConfig := schemaProviderWithBucket("${CORPUS_OUT_BUCKET}")
	err := ValidateResolvedBuckets(cpConfig, nil, map[string]any{"$SESSIONID": "s1"})
	if err == nil {
		t.Fatal("a schema provider naming ${CORPUS_OUT_BUCKET} with no such env var must be refused at startup")
	}
	// The diagnostic has to say which bucket: a document with twelve output
	// channels and one bad one is not served by "a bucket is unresolved".
	if !strings.Contains(err.Error(), "schema_providers[0].bucket") {
		t.Errorf("the error must locate the offending field, got: %v", err)
	}
	if !strings.Contains(err.Error(), "${CORPUS_OUT_BUCKET}") {
		t.Errorf("the error must quote the unresolved name, got: %v", err)
	}
}

func TestValidateResolvedBucketsRefusesUnresolvedOutputChannel(t *testing.T) {
	err := ValidateResolvedBuckets(&ComputePipesConfig{}, stepWritingToBucket("${CORPUS_OUT_BUCKET}"),
		map[string]any{"$SESSIONID": "s1"})
	if err == nil {
		t.Fatal("an output channel naming an unresolved bucket must be refused at startup")
	}
	if !strings.Contains(err.Error(), "pipes_config[0].apply[0].output_channel.bucket") {
		t.Errorf("the error must locate the offending channel, got: %v", err)
	}
}

func TestValidateResolvedBucketsRefusesBareDollarForm(t *testing.T) {
	// Three spellings, and the third is the one the gate missed until
	// IsUnresolvedBucket was widened: nine of the ten keys prepareCpipesEnv
	// writes are bare-dollar, so a document parameterising a bucket is more
	// likely to write corpus-out-$CLIENT than either of the first two.
	for _, bucket := range []string{"$CORPUS_OUT_BUCKET", "${CORPUS_OUT_BUCKET}", "corpus-out-$CLIENT"} {
		if err := ValidateResolvedBuckets(&ComputePipesConfig{}, stepWritingToBucket(bucket), nil); err == nil {
			t.Errorf("bucket %q carries an unresolved variable and must be refused", bucket)
		}
	}
}

// Criterion 15's negative controls. Both spellings of "the deployment's own
// bucket" must pass: an engine that refused either would refuse most of the
// corpus, and a gate nobody can run green is worse than no gate.
func TestValidateResolvedBucketsAcceptsTheJetStoreBucket(t *testing.T) {
	for _, bucket := range []string{"", JetStoreBucketSentinel, "an-external-bucket"} {
		t.Run("bucket="+bucket, func(t *testing.T) {
			if err := ValidateResolvedBuckets(schemaProviderWithBucket(bucket), nil, nil); err != nil {
				t.Errorf("bucket %q must be accepted, got: %v", bucket, err)
			}
		})
	}
}

// The check is about the environment, not about the spelling: the same document
// that is refused above is accepted when the deployment supplies the variable.
// Without this the gate would be "never write ${...} in a bucket", which is not
// the rule and would break every document that legitimately parameterises one.
func TestValidateResolvedBucketsAcceptsASuppliedVariable(t *testing.T) {
	env := map[string]any{"${CORPUS_OUT_BUCKET}": "acme-corpus-out"}
	if err := ValidateResolvedBuckets(schemaProviderWithBucket("${CORPUS_OUT_BUCKET}"), nil, env); err != nil {
		t.Errorf("a bucket variable the env supplies must be accepted, got: %v", err)
	}
}

// $SHARD_ID and $JETS_PARTITION_LABEL are assigned per node in
// actions_coordinate_cp.go, after the startup path has persisted the env, so
// they are absent here and present at the write. Refusing them would be a false
// negative produced by checking early rather than by anything being wrong.
func TestValidateResolvedBucketsToleratesNodeScopedKeys(t *testing.T) {
	for _, bucket := range []string{"corpus-out-$SHARD_ID", "corpus-out-$JETS_PARTITION_LABEL"} {
		if err := ValidateResolvedBuckets(schemaProviderWithBucket(bucket), nil, nil); err != nil {
			t.Errorf("bucket %q names a key the worker supplies; it must not be refused at startup, got: %v",
				bucket, err)
		}
	}
}

// The walk matches the json key rather than an enumerated list of types, which
// is what makes it complete. anonymized_columns_output_file is the one bucket
// that reaches no FileConfig, so it is the case that would be missed by an
// enumeration written from the four embedders.
func TestValidateResolvedBucketsReachesTheAnonymizeColumnFile(t *testing.T) {
	pipeConfig := []PipeSpec{{
		InputChannel: InputChannelConfig{Type: "memory", Name: "input_row"},
		Apply: []TransformationSpec{{
			Type: "anonymize",
			AnonymizeConfig: &AnonymizeSpec{
				AnonymizedColumnsOutputFile: &ColumnFileSpec{Bucket: "${KEYS_BUCKET}"},
			},
		}},
	}}
	err := ValidateResolvedBuckets(&ComputePipesConfig{}, pipeConfig, nil)
	if err == nil {
		t.Fatal("anonymized_columns_output_file.bucket is a bucket and must be checked")
	}
	if !strings.Contains(err.Error(), "anonymized_columns_output_file.bucket") {
		t.Errorf("the error must locate it by its own path, got: %v", err)
	}
}

// Every unresolved bucket in the step is reported, not the first. A run refused
// three times in a row for one bucket each is a worse deployment experience
// than a run refused once naming three.
func TestValidateResolvedBucketsReportsEveryOffender(t *testing.T) {
	cpConfig := schemaProviderWithBucket("${A_BUCKET}")
	outputFile := OutputFileSpec{Key: "of"}
	outputFile.Bucket = "${B_BUCKET}"
	cpConfig.OutputFiles = []OutputFileSpec{outputFile}
	err := ValidateResolvedBuckets(cpConfig, stepWritingToBucket("${C_BUCKET}"), nil)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, want := range []string{"${A_BUCKET}", "${B_BUCKET}", "${C_BUCKET}", "3 bucket name(s)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must mention %s, got: %v", want, err)
		}
	}
}

// src_bucket and dest_bucket are outside the gate deliberately -- see
// ValidateResolvedBuckets' scope note. This test pins that boundary so a later
// reader finds it stated rather than inferring it from silence, and so that
// widening the gate is a deliberate act that breaks a test.
func TestValidateResolvedBucketsLeavesReportCmdBucketsAlone(t *testing.T) {
	sp := &SchemaProviderSpec{Key: "_main_input_"}
	sp.ReportCmds = []ReportCmdSpec{{
		Type: "s3_copy_file",
		S3CopyFileConfig: &S3CopyFileSpec{
			SourceBucket:      "${SRC_BUCKET}",
			DestinationBucket: "${DEST_BUCKET}",
		},
	}}
	cpConfig := &ComputePipesConfig{SchemaProviders: []*SchemaProviderSpec{sp}}
	if err := ValidateResolvedBuckets(cpConfig, nil, nil); err != nil {
		t.Errorf("report_cmds buckets are not on this path and are not this gate's to refuse, got: %v", err)
	}
}

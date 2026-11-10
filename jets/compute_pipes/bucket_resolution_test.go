package compute_pipes

import (
	"strings"
	"testing"
)

func TestClassifyBucket(t *testing.T) {
	tests := []struct {
		name    string
		bucket  string
		want    BucketKind
		wantErr bool
	}{
		{"empty is the JetStore bucket", "", JetStoreBucket, false},
		{"the sentinel is the JetStore bucket", "jetstore_bucket", JetStoreBucket, false},
		{"whitespace around the sentinel", "  jetstore_bucket ", JetStoreBucket, false},
		{"a name is an external bucket", "my-client-bucket", ExternalBucket, false},
		{"a braced reference is unresolved", "${CORPUS_OUT_BUCKET}", JetStoreBucket, true},
		{"a braced reference inside a name is unresolved", "prefix-${ENV}-suffix", JetStoreBucket, true},
		{"a bare leading dollar is unresolved", "$CORPUS_OUT_BUCKET", JetStoreBucket, true},
		// Reversed from the first version of this table, which read "a dollar
		// mid-name is not this function's complaint" and expected
		// ExternalBucket. The complaint it deferred to is the bucket API's,
		// which makes it at the write -- see IsUnresolvedBucket.
		{"a dollar mid-name is unresolved too", "corpus-out-$CLIENT", JetStoreBucket, true},
		{"a dollar is never part of a bucket name", "weird$name", JetStoreBucket, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClassifyBucket(tc.bucket)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ClassifyBucket(%q) error = %v, wantErr %v", tc.bucket, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("ClassifyBucket(%q) = %v, want %v", tc.bucket, got, tc.want)
			}
		})
	}
}

// The six existing call sites all spell `bucket == "" || bucket ==
// "jetstore_bucket"`. This pins that ClassifyBucket agrees with them on every
// input they can see, so replacing one of them is a refactor rather than a
// change of behaviour.
func TestClassifyBucketAgreesWithTheExistingSentinelTest(t *testing.T) {
	for _, bucket := range []string{"", "jetstore_bucket", "some-bucket", "a", "JETSTORE_BUCKET"} {
		existing := bucket == "" || bucket == "jetstore_bucket"
		kind, err := ClassifyBucket(bucket)
		if err != nil {
			t.Fatalf("ClassifyBucket(%q) unexpected error: %v", bucket, err)
		}
		if (kind == JetStoreBucket) != existing {
			t.Fatalf("ClassifyBucket(%q) = %v, existing sentinel test says isJetStore=%v", bucket, kind, existing)
		}
	}
}

func TestResolveBucket(t *testing.T) {
	const deployment = "acme-jetstore-bucket"
	tests := []struct {
		bucket  string
		want    string
		wantErr bool
	}{
		{"", deployment, false},
		{"jetstore_bucket", deployment, false},
		{"client-bucket", "client-bucket", false},
		{"${CORPUS_OUT_BUCKET}", "", true},
	}
	for _, tc := range tests {
		got, err := ResolveBucket(tc.bucket, deployment)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ResolveBucket(%q) error = %v, wantErr %v", tc.bucket, err, tc.wantErr)
		}
		if got != tc.want {
			t.Fatalf("ResolveBucket(%q) = %q, want %q", tc.bucket, got, tc.want)
		}
	}
}

// The error message is part of the contract: whoever reads it is looking at a
// deployment whose environment did not carry a variable, and the name of that
// variable is the only useful thing the message can say.
func TestUnresolvedBucketErrorNamesTheBucket(t *testing.T) {
	_, err := ClassifyBucket("${CORPUS_OUT_BUCKET}")
	if err == nil {
		t.Fatal("expected an error")
	}
	if want := "${CORPUS_OUT_BUCKET}"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not name %q", err.Error(), want)
	}
}

// The two output writers (the merge writer and the partition writer) choose
// their bucket through outputBucket. A bucket configured as a variable whose
// value is the sentinel must reach the sinks as the deployment's bucket (""),
// not as an external bucket spelled "jetstore_bucket": before JM-057 the
// writers tested the raw value for the sentinel and substituted afterwards, so
// this case slipped through as external and only the sinks' own re-tests kept
// it out of a bucket of that name.
func TestOutputBucket(t *testing.T) {
	env := map[string]any{
		"${OUT_BUCKET}":      JetStoreBucketSentinel,
		"${EXTERNAL_BUCKET}": "some-client-bucket",
		"$BARE_SENTINEL":     JetStoreBucketSentinel,
	}
	spWith := func(bucket string) SchemaProvider {
		return &DefaultSchemaProvider{spec: &SchemaProviderSpec{FileConfig: FileConfig{Bucket: bucket}}}
	}
	tests := []struct {
		name       string
		configured string
		sp         SchemaProvider
		outLoc     string
		want       string
		wantErr    bool
	}{
		{"nothing configured is the deployment's bucket", "", nil, "", "", false},
		{"the sentinel is the deployment's bucket", "jetstore_bucket", nil, "", "", false},
		{"a literal name is external", "some-client-bucket", nil, "", "some-client-bucket", false},
		{"a variable resolving to a name is external", "${EXTERNAL_BUCKET}", nil, "", "some-client-bucket", false},
		{"a variable resolving to the sentinel is the deployment's bucket", "${OUT_BUCKET}", nil, "", "", false},
		{"a bare variable resolving to the sentinel too", "$BARE_SENTINEL", nil, "", "", false},
		{"an unresolved variable is refused", "${NO_SUCH_BUCKET}", nil, "", "", true},
		{"the input provider's bucket under jetstore_s3_input", "", spWith("input-bucket"), "jetstore_s3_input", "input-bucket", false},
		{"the input provider's sentinel under jetstore_s3_input", "", spWith("jetstore_bucket"), "jetstore_s3_input", "", false},
		{"the input provider's variable resolving to the sentinel", "", spWith("${OUT_BUCKET}"), "jetstore_s3_input", "", false},
		{"the input provider is ignored elsewhere", "", spWith("input-bucket"), "jetstore_s3_output", "", false},
		{"a configured bucket wins over the input provider", "some-client-bucket", spWith("input-bucket"), "jetstore_s3_input", "some-client-bucket", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := outputBucket(tc.configured, tc.sp, tc.outLoc, env)
			if (err != nil) != tc.wantErr {
				t.Fatalf("outputBucket(%q) error = %v, wantErr %v", tc.configured, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("outputBucket(%q) = %q, want %q", tc.configured, got, tc.want)
			}
		})
	}
}

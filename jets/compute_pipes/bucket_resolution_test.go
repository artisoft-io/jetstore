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

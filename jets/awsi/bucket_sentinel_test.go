package awsi

import "testing"

func TestIsJetStoreBucket(t *testing.T) {
	tests := []struct {
		name   string
		bucket string
		want   bool
	}{
		{"empty is the deployment's bucket", "", true},
		{"the sentinel is the deployment's bucket", "jetstore_bucket", true},
		{"a name is an external bucket", "some-client-bucket", false},
		// The deployment's bucket by its real name is addressed as itself,
		// which is the same bucket; the predicate does not need to know it.
		{"the deployment's own name is not the sentinel", "acme-jetstore-bucket", false},
		{"the match is case-sensitive", "JETSTORE_BUCKET", false},
		{"the match does not trim", " jetstore_bucket", false},
		{"an unresolved reference is not the sentinel", "${OUT_BUCKET}", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsJetStoreBucket(tc.bucket); got != tc.want {
				t.Fatalf("IsJetStoreBucket(%q) = %v, want %v", tc.bucket, got, tc.want)
			}
		})
	}
}

// IsJetStoreBucket replaces a test written out by hand at every sink; this
// pins that it agrees with that test on every input, so each replacement is a
// refactor rather than a change of behaviour.
func TestIsJetStoreBucketAgreesWithTheSpelledOutTest(t *testing.T) {
	for _, bucket := range []string{"", "jetstore_bucket", "x", "jetstore_bucket ", "JETSTORE_BUCKET", "$X"} {
		spelledOut := bucket == "" || bucket == "jetstore_bucket"
		if got := IsJetStoreBucket(bucket); got != spelledOut {
			t.Fatalf("IsJetStoreBucket(%q) = %v, spelled-out test says %v", bucket, got, spelledOut)
		}
	}
}

package stack

import (
	"reflect"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// The sentinel the stack refuses is the one the runtime resolves.
func TestExternalBucketSentinelIsTheRuntimeOne(t *testing.T) {
	if jetStoreBucketSentinel != awsi.JetStoreBucketSentinel {
		t.Fatalf("stack refuses %q, runtime resolves %q", jetStoreBucketSentinel, awsi.JetStoreBucketSentinel)
	}
}

func TestExternalBucketNames(t *testing.T) {
	accepted := []struct {
		value string
		want  []string
	}{
		{"", nil},
		{"one-bucket", []string{"one-bucket"}},
		{"a-bucket,b.bucket.example,c3po", []string{"a-bucket", "b.bucket.example", "c3po"}},
		// Legacy names, which an older bucket may still carry.
		{"Legacy_Bucket", []string{"Legacy_Bucket"}},
	}
	for _, tc := range accepted {
		got, err := ExternalBucketNames(tc.value)
		if err != nil {
			t.Errorf("ExternalBucketNames(%q) refused: %v", tc.value, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ExternalBucketNames(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}

	refused := []struct {
		value string
		names []string // each refused name the error must mention
	}{
		{"jetstore_bucket", []string{"jetstore_bucket"}},
		{"a-bucket,jetstore_bucket", []string{"name 2"}},
		{"a-bucket,", []string{"name 2 is empty"}},
		{",a-bucket", []string{"name 1 is empty"}},
		{"a-bucket,,b-bucket", []string{"name 2 is empty"}},
		{"a-bucket, b-bucket", []string{`" b-bucket"`}},
		{"s3://a-bucket", []string{`"s3://a-bucket"`}},
		{"a-bucket/prefix", []string{`"a-bucket/prefix"`}},
		{"ab", []string{`"ab"`}},
		{"jetstore_bucket,,x y", []string{"name 1", "name 2 is empty", `"x y"`}},
	}
	for _, tc := range refused {
		got, err := ExternalBucketNames(tc.value)
		if err == nil {
			t.Errorf("ExternalBucketNames(%q) = %q, want it refused", tc.value, got)
			continue
		}
		for _, name := range tc.names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("ExternalBucketNames(%q): error %q does not mention %s", tc.value, err, name)
			}
		}
	}
}

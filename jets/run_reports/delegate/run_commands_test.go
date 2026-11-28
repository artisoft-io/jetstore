package delegate

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// A report command's buckets reach awsi.MultiPartCopy only as names it can
// address: a placeholder nothing substituted is refused, naming the command,
// and the deployment's own bucket is resolved before the copy.
func TestCopyBuckets(t *testing.T) {
	const own = "deployment-bucket"
	cmd := func(src, dest, comment string) *compute_pipes.ReportCmdSpec {
		return &compute_pipes.ReportCmdSpec{
			Type:    "s3_copy_file",
			Comment: comment,
			S3CopyFileConfig: &compute_pipes.S3CopyFileSpec{
				SourceBucket: src, SourceKey: "in/a.csv",
				DestinationBucket: dest, DestinationKey: "out/a.csv",
			},
		}
	}
	for _, tc := range []struct {
		name              string
		cmd               *compute_pipes.ReportCmdSpec
		own               string
		wantSrc, wantDest string
		wantErr           []string // substrings of the error, nil when none
	}{
		{"both external", cmd("src-bucket", "dest-bucket", ""), own, "src-bucket", "dest-bucket", nil},
		{"empty is the own bucket", cmd("", "dest-bucket", ""), own, own, "dest-bucket", nil},
		{"sentinel is the own bucket", cmd("src-bucket", "jetstore_bucket", ""), own, "src-bucket", own, nil},
		{"sentinel with spaces is the own bucket", cmd(" jetstore_bucket ", "dest-bucket", ""), own, own, "dest-bucket", nil},
		{"external with spaces is trimmed", cmd(" src-bucket", "dest-bucket ", ""), own, "src-bucket", "dest-bucket", nil},
		{"braced placeholder in src", cmd("${OUT_BUCKET}", "dest-bucket", ""), own, "", "",
			[]string{"report_cmds[3] (s3_copy_file)", "src_bucket", "${OUT_BUCKET}", "unresolved"}},
		{"bare placeholder in dest", cmd("src-bucket", "out-$CLIENT", "copy the extract"), own, "", "",
			[]string{`report_cmds[3] (s3_copy_file, "copy the extract")`, "dest_bucket", "out-$CLIENT", "unresolved"}},
		{"own bucket not set", cmd("jetstore_bucket", "dest-bucket", ""), "", "", "",
			[]string{"report_cmds[3] (s3_copy_file)", "src_bucket", "JETS_BUCKET is not set"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := *tc.cmd.S3CopyFileConfig
			got, err := copyBuckets(3, tc.cmd, tc.own)
			if *tc.cmd.S3CopyFileConfig != before {
				t.Errorf("copyBuckets changed the command: %+v", *tc.cmd.S3CopyFileConfig)
			}
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("copyBuckets = %+v, want an error", got)
				}
				for _, w := range tc.wantErr {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error %q does not name %q", err, w)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("copyBuckets: %v", err)
			}
			if got.SourceBucket != tc.wantSrc || got.DestinationBucket != tc.wantDest {
				t.Errorf("buckets = %q, %q, want %q, %q", got.SourceBucket, got.DestinationBucket, tc.wantSrc, tc.wantDest)
			}
			if got.SourceKey != before.SourceKey || got.DestinationKey != before.DestinationKey {
				t.Errorf("keys changed: %q, %q", got.SourceKey, got.DestinationKey)
			}
		})
	}
}

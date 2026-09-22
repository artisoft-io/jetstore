package compute_pipes

import (
	"fmt"
	"strings"
)

// One definition of "is this a resolved bucket", for every caller that has a
// configured bucket name in hand and needs to know what it is.
//
// The test `bucket == "" || bucket == "jetstore_bucket"` is written out at six
// call sites today (DownloadS3Object in actions_s3_utils.go, the merge writer
// in pipe_executor_merge_files.go, the partition writer, the s3 device worker,
// actions_get_columns_from_file.go and actions_coordinate_cp.go). All six agree
// on what an empty or sentinel bucket means and none of them says anything
// about a bucket name that still carries an unexpanded environment reference —
// so a document naming `${CORPUS_OUT_BUCKET}` is carried all the way to the
// write and creates an S3 bucket path spelled with the dollar sign in it. The
// run succeeds and the deliverables are somewhere nobody will look.
//
// This is the classification those six share, plus the case they do not make.

// BucketKind is what a configured bucket name turns out to name.
type BucketKind int

const (
	// JetStoreBucket is the deployment's own bucket: the empty string, or the
	// "jetstore_bucket" sentinel written in a document to say so explicitly.
	JetStoreBucket BucketKind = iota
	// ExternalBucket is a bucket named literally by the document.
	ExternalBucket
)

func (k BucketKind) String() string {
	switch k {
	case JetStoreBucket:
		return "jetstore_bucket"
	case ExternalBucket:
		return "external"
	}
	return fmt.Sprintf("BucketKind(%d)", int(k))
}

// JetStoreBucketSentinel is the name a document writes to say "the deployment's
// own bucket" rather than leaving the field empty.
const JetStoreBucketSentinel = "jetstore_bucket"

// ClassifyBucket says what a configured bucket name names, and returns an error
// when the name is not a bucket name at all because variable substitution did
// not reach it.
//
// An unresolved name is an error rather than a kind. The alternative — treating
// it as an external bucket, which is what every call site does today — writes
// the run's deliverables to a bucket whose name contains "${", reports success,
// and is discovered by the deliverables' absence somewhere else.
func ClassifyBucket(bucket string) (BucketKind, error) {
	trimmed := strings.TrimSpace(bucket)
	if trimmed == "" || trimmed == JetStoreBucketSentinel {
		return JetStoreBucket, nil
	}
	if IsUnresolvedBucket(trimmed) {
		return JetStoreBucket, fmt.Errorf(
			"error: bucket %q is unresolved: it still carries a variable reference, "+
				"which means the environment did not supply a value for it", bucket)
	}
	return ExternalBucket, nil
}

// IsUnresolvedBucket reports whether a bucket name still carries a variable
// reference rather than being a name.
//
// The rule is any dollar sign at all, and that is a widening of what this
// function first did. It refused "${NAME}" anywhere and a leading "$", and left
// a dollar mid-name alone on the ground that S3 bucket names cannot contain one
// "so it is the bucket API's complaint to make rather than this function's".
//
// That premise is right and the conclusion drawn from it was the wrong way
// round. A complaint the bucket API makes is a complaint made at the write, and
// moving the complaint off the write is the whole point of the gate this
// function was written for — so deferring to the API there gives back exactly
// what the rest of the design buys. And because a bucket name can never
// legitimately contain a dollar, the wider rule trades nothing for it: there is
// no name it refuses that was ever a name.
//
// The case that made this concrete is the ordinary one rather than an exotic
// one. prepareCpipesEnv writes ten env keys and nine of them are bare-dollar —
// $FILE_KEY, $SESSIONID, $PATH_FILE_KEY, $SHARD_ID and the rest — so a document
// parameterising a bucket writes "corpus-out-$CLIENT" far more naturally than
// it writes the braced form, and "corpus-out-$CLIENT" is the one spelling the
// original two tests both missed.
func IsUnresolvedBucket(bucket string) bool {
	return strings.ContainsRune(bucket, '$')
}

// ResolveBucket returns the bucket to address, substituting the deployment's
// own bucket for the JetStore kind. It is ClassifyBucket for callers that want
// a name rather than a kind.
//
// jetStoreBucket is passed rather than read, because the two producers in this
// tree differ: awsi.JetStoreBucket() and the package-level bucketName.
func ResolveBucket(bucket, jetStoreBucket string) (string, error) {
	kind, err := ClassifyBucket(bucket)
	if err != nil {
		return "", err
	}
	if kind == JetStoreBucket {
		return jetStoreBucket, nil
	}
	return strings.TrimSpace(bucket), nil
}

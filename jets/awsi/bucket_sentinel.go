package awsi

// The deployment's own bucket, named.
//
// A configured bucket name means the deployment's own bucket (JETS_BUCKET) in
// two spellings: left empty, or written out as the sentinel below. The test
// for that was spelled out by hand at every reader and writer that resolves a
// bucket, in this package and in compute_pipes, which imports this one. It
// lives here, rather than in compute_pipes beside ClassifyBucket, because this
// package is the one both can import: the S3 sinks in this file's neighbours
// apply it too, as the last line of defence before the request leaves.

// JetStoreBucketSentinel is the name a document writes to say "the
// deployment's own bucket" rather than leaving the field empty.
const JetStoreBucketSentinel = "jetstore_bucket"

// IsJetStoreBucket reports whether a bucket name, as it reaches a reader or a
// writer, means the deployment's own bucket: the empty string or the sentinel.
//
// The match is exact. The deployment's own bucket written out by its real name
// is not matched, and does not need to be: it is addressed as itself and
// reaches the same bucket. Surrounding whitespace is not trimmed, because no
// call site trimmed before this function existed; compute_pipes.ClassifyBucket
// trims first and then asks this function.
func IsJetStoreBucket(name string) bool {
	return name == "" || name == JetStoreBucketSentinel
}

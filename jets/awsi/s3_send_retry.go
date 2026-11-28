package awsi

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

// S3 sends that retry, and the ones that cannot.
//
// Several pipelines started together send to S3 at once, and S3 answers a
// burst with throttling (503 SlowDown) and the odd transient failure. The SDK
// retries each request a few times on its own, but its retries draw on a
// per-client quota, and a burst of failures empties it: the SDK then returns
// the error at once ("retry quota exceeded"). So a send that would have
// succeeded a few seconds later failed the run instead, and the workaround was
// to wait and start the run again.
//
// RetrySend retries a send above the SDK, with a jittered exponential backoff
// and a bounded number of attempts, on throttling and transient errors only,
// logging each retry. An error that will not change on a second try (access
// denied, no such bucket, a cancelled context) is returned at once.
//
// A send can be retried only if its body can be read again from the start: a
// file or an in-memory buffer can, a pipe cannot. Until 2026-11-27 the upload
// functions here and the partition writers' S3 worker retried every failed
// send with the body as the failed attempt left it, so a retry after a
// partial read sent the rest of the body as the whole object, and succeeded.
// UploadToS3 and UploadToS3FromReader now seek back to where the body started
// before each attempt, and send a body they cannot seek once, leaving the
// retries to the SDK (which retries each part it has read into memory).

// s3SendMaxAttempts bounds the attempts RetrySend makes, the first included.
const s3SendMaxAttempts = 8

// The backoff before retry n (n from 1) is drawn from [d/2, d], where d is
// s3SendBaseDelay doubled n-1 times and capped at s3SendMaxDelay: about 1, 2,
// 4, 8, 16, 30 and 30 seconds, so a send that keeps failing gives up after
// roughly 45 to 90 seconds.
var (
	s3SendBaseDelay = 1 * time.Second
	s3SendMaxDelay  = 30 * time.Second
)

// s3SendSleep waits d, or until ctx is done. A test replaces it.
var s3SendSleep = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// s3SendBackoff is the wait before retry n (n from 1).
func s3SendBackoff(n int) time.Duration {
	d := s3SendBaseDelay
	for i := 1; i < n && d < s3SendMaxDelay; i++ {
		d *= 2
	}
	d = min(d, s3SendMaxDelay)
	half := d / 2
	return half + rand.N(d-half+1)
}

// IsRetryableSendError reports whether a failed S3 send is worth another
// attempt: S3 throttled it, the SDK's own retry quota ran out, or the SDK
// classes the error as transient (a 500, 502, 503 or 504, a request timeout, a
// connection reset). A cancelled or expired context is not.
func IsRetryableSendError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var quota ratelimit.QuotaExceededError
	if errors.As(err, &quota) {
		return true
	}
	if retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary {
		return true
	}
	return retry.IsErrorRetryables(retry.DefaultRetryables).IsErrorRetryable(err) == aws.TrueTernary
}

// RetrySend calls send until it succeeds, it fails with an error
// IsRetryableSendError does not retry, s3SendMaxAttempts attempts have been
// made, or ctx is done, and returns the last error. Each retry is logged with
// what, the attempt and the error. send must be safe to call again: a send
// with a body re-reads it from the start.
func RetrySend(ctx context.Context, what string, send func() error) error {
	for attempt := 1; ; attempt++ {
		err := send()
		if err == nil {
			if attempt > 1 {
				log.Printf("S3 send %s succeeded on attempt %d", what, attempt)
			}
			return nil
		}
		if attempt >= s3SendMaxAttempts || !IsRetryableSendError(err) {
			return err
		}
		wait := s3SendBackoff(attempt)
		log.Printf("S3 send %s: attempt %d of %d failed, retrying in %v: %v",
			what, attempt, s3SendMaxAttempts, wait.Round(time.Millisecond), err)
		if s3SendSleep(ctx, wait) != nil {
			return err
		}
	}
}

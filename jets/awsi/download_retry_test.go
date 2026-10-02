package awsi

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// A stand-in for an SDK API error. Every one the SDK returns carries ErrorCode(),
// which is all IsNoSuchKey asks of it, so the test needs no S3 and no smithy-go.
type fakeAPIError struct{ code string }

func (e *fakeAPIError) Error() string     { return e.code + ": something" }
func (e *fakeAPIError) ErrorCode() string { return e.code }

// The shape the SDK actually returns: an operation error wrapping a response
// error wrapping the typed one. Two levels of %w stand in for both.
func sdkLike(code string) error {
	return fmt.Errorf("operation error S3: GetObject, %w",
		fmt.Errorf("https response error StatusCode: 404, %w", &fakeAPIError{code: code}))
}

func TestIsNoSuchKey(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"NoSuchKey, as GetObject reports it": {sdkLike("NoSuchKey"), true},
		"NotFound, as HeadObject reports it": {sdkLike("NotFound"), true},
		// Without s3:ListBucket a missing key is reported as this, and so is a
		// real refusal; it must not read as "no such object".
		"AccessDenied":      {sdkLike("AccessDenied"), false},
		"a plain error":     {errors.New("NoSuchKey in the text only"), false},
		"wrapped once more": {fmt.Errorf("outer: %w", sdkLike("NoSuchKey")), true},
		"flattened with %v": {fmt.Errorf("outer: %v", sdkLike("NoSuchKey")), false},
		"nil":               {nil, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := IsNoSuchKey(c.err); got != c.want {
				t.Errorf("IsNoSuchKey = %v, want %v", got, c.want)
			}
		})
	}
}

// The I-23 fix: a missing object costs one attempt and no sleep, where it used to
// cost seven attempts and 10.5 s, and the error that comes back still says what it
// was.
func TestDownloadWithRetryReturnsAMissingObjectAtOnce(t *testing.T) {
	attempts := 0
	var slept []time.Duration
	_, err := downloadWithRetry(func() ([]byte, error) {
		attempts++
		return nil, sdkLike("NoSuchKey")
	}, func(d time.Duration) { slept = append(slept, d) }, "s3://b/k")
	if attempts != 1 || len(slept) != 0 {
		t.Errorf("attempts = %d, sleeps = %v; a missing object is not transient", attempts, slept)
	}
	if !IsNoSuchKey(err) {
		t.Errorf("the returned error no longer says NoSuchKey: %v", err)
	}
}

// Everything else keeps the schedule it always had: seven attempts, sleeping
// 0.5 s to 3 s between them, and the last error wrapped rather than flattened.
func TestDownloadWithRetryKeepsTheScheduleForOtherErrors(t *testing.T) {
	attempts := 0
	var slept []time.Duration
	cause := errors.New("connection reset")
	_, err := downloadWithRetry(func() ([]byte, error) {
		attempts++
		return nil, cause
	}, func(d time.Duration) { slept = append(slept, d) }, "s3://b/k")
	if attempts != 7 {
		t.Errorf("attempts = %d, want 7", attempts)
	}
	want := []time.Duration{500, 1000, 1500, 2000, 2500, 3000}
	for i := range want {
		want[i] *= time.Millisecond
	}
	if fmt.Sprint(slept) != fmt.Sprint(want) {
		t.Errorf("slept %v, want %v", slept, want)
	}
	if !errors.Is(err, cause) {
		t.Errorf("the cause was flattened: %v", err)
	}
}

func TestDownloadWithRetrySucceedsAfterATransientFailure(t *testing.T) {
	attempts := 0
	buf, err := downloadWithRetry(func() ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("throttled")
		}
		return []byte("ok"), nil
	}, func(time.Duration) {}, "s3://b/k")
	if err != nil || string(buf) != "ok" || attempts != 3 {
		t.Errorf("buf = %q, err = %v, attempts = %d", buf, err, attempts)
	}
}

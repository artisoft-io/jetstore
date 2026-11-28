package awsi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// responseError is an S3 failure as the SDK reports it: an HTTP status, and
// the API error code S3 sent.
func responseError(status int, code string) error {
	return &smithy.OperationError{
		ServiceID:     "S3",
		OperationName: "PutObject",
		Err: &awshttp.ResponseError{
			ResponseError: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
				Err:      &smithy.GenericAPIError{Code: code, Message: "test"},
			},
		},
	}
}

func TestIsRetryableSendError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"throttled", responseError(503, "SlowDown"), true},
		{"internal error", responseError(500, "InternalError"), true},
		{"request timeout", responseError(400, "RequestTimeout"), true},
		{"retry quota exhausted", fmt.Errorf("failed to get rate limit token, %w",
			ratelimit.QuotaExceededError{Available: 0, Requested: 5}), true},
		{"the SDK's attempts exhausted on throttling", &retry.MaxAttemptsError{Attempt: 3,
			Err: responseError(503, "SlowDown")}, true},
		{"wrapped by a caller", fmt.Errorf("while copying: %w", responseError(503, "SlowDown")), true},
		{"access denied", responseError(403, "AccessDenied"), false},
		{"no such bucket", responseError(404, "NoSuchBucket"), false},
		{"cancelled", context.Canceled, false},
		{"deadline", fmt.Errorf("upload: %w", context.DeadlineExceeded), false},
		{"plain error", errors.New("while rewinding the body to send: bad seek"), false},
	} {
		if got := IsRetryableSendError(tc.err); got != tc.want {
			t.Errorf("%s: IsRetryableSendError = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// stubSleep records the waits RetrySend asks for, without waiting.
func stubSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	var waits []time.Duration
	saved := s3SendSleep
	s3SendSleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	t.Cleanup(func() { s3SendSleep = saved })
	return &waits
}

func TestRetrySend(t *testing.T) {
	throttled := responseError(503, "SlowDown")

	t.Run("retries throttling until it succeeds", func(t *testing.T) {
		waits := stubSleep(t)
		calls := 0
		err := RetrySend(context.Background(), "test", func() error {
			calls++
			if calls < 3 {
				return throttled
			}
			return nil
		})
		if err != nil || calls != 3 || len(*waits) != 2 {
			t.Errorf("err %v, calls %d, waits %v; want nil, 3, two waits", err, calls, *waits)
		}
	})

	t.Run("bounded", func(t *testing.T) {
		waits := stubSleep(t)
		calls := 0
		err := RetrySend(context.Background(), "test", func() error { calls++; return throttled })
		if !errors.Is(err, throttled) || calls != s3SendMaxAttempts || len(*waits) != s3SendMaxAttempts-1 {
			t.Errorf("err %v, calls %d, waits %d; want the last error, %d calls, %d waits",
				err, calls, len(*waits), s3SendMaxAttempts, s3SendMaxAttempts-1)
		}
	})

	t.Run("an error that will not change is returned at once", func(t *testing.T) {
		waits := stubSleep(t)
		denied := responseError(403, "AccessDenied")
		calls := 0
		err := RetrySend(context.Background(), "test", func() error { calls++; return denied })
		if !errors.Is(err, denied) || calls != 1 || len(*waits) != 0 {
			t.Errorf("err %v, calls %d, waits %v; want the error, 1 call, no wait", err, calls, *waits)
		}
	})

	t.Run("a cancelled context stops the retries", func(t *testing.T) {
		stubSleep(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		err := RetrySend(ctx, "test", func() error { calls++; return throttled })
		if !errors.Is(err, throttled) || calls != 1 {
			t.Errorf("err %v, calls %d; want the send's error after 1 call", err, calls)
		}
	})
}

// The backoff doubles from s3SendBaseDelay, is capped at s3SendMaxDelay, and is
// drawn from the upper half of each step.
func TestS3SendBackoff(t *testing.T) {
	for n := 1; n < s3SendMaxAttempts; n++ {
		d := s3SendBaseDelay << (n - 1)
		if d > s3SendMaxDelay || d <= 0 {
			d = s3SendMaxDelay
		}
		for range 50 {
			got := s3SendBackoff(n)
			if got < d/2 || got > d {
				t.Fatalf("s3SendBackoff(%d) = %v, want within [%v, %v]", n, got, d/2, d)
			}
		}
	}
}

// fakeS3 answers PUTs with the statuses given, in order (200 once they run
// out), and records each body it received.
type fakeS3 struct {
	statuses []int
	bodies   []string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.bodies = append(f.bodies, string(body))
	status := http.StatusOK
	if len(f.statuses) > 0 {
		status, f.statuses = f.statuses[0], f.statuses[1:]
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>SlowDown</Code><Message>Please reduce your request rate.</Message></Error>`)
		return
	}
	w.Header().Set("ETag", `"etag"`)
	w.WriteHeader(http.StatusOK)
}

// testS3Client is an S3 client for the fake, with the SDK's own retries off so
// that every failure reaches RetrySend.
func testS3Client(url string) *s3.Client {
	return s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(url),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		Retryer:      aws.NopRetryer{},
	})
}

// A body that can seek is sent from where it started on every attempt, and
// one that cannot is sent once. Until 2026-11-27 every body was retried as the
// failed attempt left it.
func TestUploadToS3RetriesFromTheStart(t *testing.T) {
	waits := stubSleep(t)
	fake := &fakeS3{statuses: []int{http.StatusServiceUnavailable}}
	server := httptest.NewServer(fake)
	defer server.Close()
	client := testS3Client(server.URL)

	r := bytes.NewReader([]byte("headerPAYLOAD"))
	if _, err := r.Seek(6, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	body, canRetry := readerBody(r)
	if err := uploadToS3(client, "test-bucket", "out/part-0", body, canRetry); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(fake.bodies) != 2 || fake.bodies[0] != "PAYLOAD" || fake.bodies[1] != "PAYLOAD" || len(*waits) != 1 {
		t.Errorf("bodies %q, waits %v; want PAYLOAD twice and one wait", fake.bodies, *waits)
	}

	// A file, from its current position.
	f, err := os.CreateTemp(t.TempDir(), "upload")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString("skipFILE BODY"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	fake.statuses, fake.bodies = []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable}, nil
	body, canRetry = fileBody(f)
	if err := uploadToS3(client, "test-bucket", "out/part-1", body, canRetry); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(fake.bodies) != 3 || fake.bodies[2] != "FILE BODY" || fake.bodies[0] != "FILE BODY" {
		t.Errorf("bodies %q; want FILE BODY three times", fake.bodies)
	}

	// A pipe cannot be read again: one attempt, and its error.
	fake.statuses, fake.bodies = []int{http.StatusServiceUnavailable}, nil
	pin, pout := io.Pipe()
	go func() { pout.Write([]byte("STREAM")); pout.Close() }()
	body, canRetry = readerBody(pin)
	if canRetry {
		t.Fatalf("a pipe is retried")
	}
	err = uploadToS3(client, "test-bucket", "out/part-2", body, canRetry)
	if err == nil || len(fake.bodies) != 1 {
		t.Errorf("err %v, bodies %q; want an error after one attempt", err, fake.bodies)
	}
}

package compute_pipes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// A partition's local file is sent from its start on every attempt. Until
// 2026-11-27 a retry sent the body as the failed attempt had left it.
func TestS3DeviceWorkerRetriesFromTheStartOfTheFile(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	failures := 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		bodies = append(bodies, string(body))
		if failures > 0 {
			failures--
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>SlowDown</Code><Message>Please reduce your request rate.</Message></Error>`)
			return
		}
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(server.URL),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		Retryer:      aws.NopRetryer{}, // every failure reaches awsi.RetrySend
	})

	local := filepath.Join(t.TempDir(), "part-0.csv")
	if err := os.WriteFile(local, []byte("a,b\n1,2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	errCh := make(chan error, 4)
	resultCh := make(chan ComputePipesResult, 4)
	worker := NewS3DeviceWorker(manager.NewUploader(client), done, errCh)
	task := &S3Object{ExternalBucket: "test-bucket", FileKey: "out/part-0.csv", LocalFilePath: local}
	if err := worker.processTask(task, nil, resultCh); err != nil {
		t.Fatalf("processTask: %v", err)
	}
	if len(bodies) != 2 || bodies[0] != "a,b\n1,2\n" || bodies[1] != "a,b\n1,2\n" {
		t.Errorf("bodies sent %q, want the whole file twice", bodies)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Errorf("the local file was not removed: %v", err)
	}
}

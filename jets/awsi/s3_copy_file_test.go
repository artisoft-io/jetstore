package awsi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// The multipart copy must encrypt with JETS_S3_KMS_KEY_ARN, as the single part
// copy does. Without it s3 falls back to the destination bucket's default key.
func TestCreateMultipartUploadInputUsesKmsKey(t *testing.T) {
	saved := kmsKeyArn
	defer func() { kmsKeyArn = saved }()

	kmsKeyArn = "arn:aws:kms:us-east-1:111122223333:key/test-key"
	input := newCreateMultipartUploadInput("dest-bucket", "dest/key.txt")
	if *input.Bucket != "dest-bucket" || *input.Key != "dest/key.txt" {
		t.Errorf("got bucket %q key %q, want dest-bucket dest/key.txt", *input.Bucket, *input.Key)
	}
	if input.ServerSideEncryption != types.ServerSideEncryptionAwsKms {
		t.Errorf("got ServerSideEncryption %q, want %q", input.ServerSideEncryption, types.ServerSideEncryptionAwsKms)
	}
	if input.SSEKMSKeyId == nil || *input.SSEKMSKeyId != kmsKeyArn {
		t.Errorf("got SSEKMSKeyId %v, want %s", input.SSEKMSKeyId, kmsKeyArn)
	}

	// No key configured: leave the encryption to the bucket default
	kmsKeyArn = ""
	input = newCreateMultipartUploadInput("dest-bucket", "dest/key.txt")
	if input.ServerSideEncryption != "" || input.SSEKMSKeyId != nil {
		t.Errorf("got ServerSideEncryption %q SSEKMSKeyId %v, want both unset", input.ServerSideEncryption, input.SSEKMSKeyId)
	}
}

// fakeMultipartS3 serves the requests a multipart copy of two small objects
// makes: the listing, the upload's creation, a part copy per object, which
// answers with partStatuses in order (200 once they run out), and the
// completion or the abort.
type fakeMultipartS3 struct {
	mu           sync.Mutex
	partStatuses []int
	partCopies   int
	completed    bool
	aborted      bool
}

func (f *fakeMultipartS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = io.ReadAll(r.Body)
	q := r.URL.Query()
	w.Header().Set("Content-Type", "application/xml")
	switch {
	case r.Method == http.MethodGet && q.Get("list-type") == "2":
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Name>src</Name><IsTruncated>false</IsTruncated>`+
			`<Contents><Key>in/part-0</Key><Size>10</Size></Contents><Contents><Key>in/part-1</Key><Size>10</Size></Contents></ListBucketResult>`)
	case r.Method == http.MethodPost && q.Has("uploads"):
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult><Bucket>dest</Bucket><Key>out/all</Key><UploadId>up-1</UploadId></InitiateMultipartUploadResult>`)
	case r.Method == http.MethodPut && q.Get("partNumber") != "":
		f.partCopies++
		status := http.StatusOK
		if len(f.partStatuses) > 0 {
			status, f.partStatuses = f.partStatuses[0], f.partStatuses[1:]
		}
		switch status {
		case http.StatusOK:
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><CopyPartResult><ETag>"etag-`+q.Get("partNumber")+`"</ETag></CopyPartResult>`)
		case http.StatusServiceUnavailable:
			w.WriteHeader(status)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>SlowDown</Code><Message>Please reduce your request rate.</Message></Error>`)
		default:
			w.WriteHeader(status)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`)
		}
	case r.Method == http.MethodPost && q.Get("uploadId") != "":
		f.completed = true
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><CompleteMultipartUploadResult><Bucket>dest</Bucket><Key>out/all</Key><ETag>"done"</ETag></CompleteMultipartUploadResult>`)
	case r.Method == http.MethodDelete && q.Get("uploadId") != "":
		f.aborted = true
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// A part copy is retried as every S3 send is (RetrySend): a throttled one
// again after a backoff, one that will not change on a second try (access
// denied) not at all, the upload then aborted. Until 2026-11-27 the part copy
// kept its own loop, which retried any error four times.
func TestMultiPartCopyRetriesAPartAsRetrySendDoes(t *testing.T) {
	waits := stubSleep(t)
	fake := &fakeMultipartS3{partStatuses: []int{http.StatusServiceUnavailable}}
	server := httptest.NewServer(fake)
	defer server.Close()
	client := testS3Client(server.URL)

	if err := MultiPartCopy(context.Background(), client, 1, "src", "in", "dest", "out/all", false); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if fake.partCopies != 3 || len(*waits) != 1 || !fake.completed || fake.aborted {
		t.Errorf("part copies %d, waits %v, completed %v, aborted %v; want 3, one wait, completed",
			fake.partCopies, *waits, fake.completed, fake.aborted)
	}

	*waits = nil
	fake2 := &fakeMultipartS3{partStatuses: []int{http.StatusForbidden}}
	server2 := httptest.NewServer(fake2)
	defer server2.Close()
	err := MultiPartCopy(context.Background(), testS3Client(server2.URL), 1, "src", "in", "dest", "out/all", false)
	// The abort comes before the error is returned; what follows it may still
	// be under way, so only these are read.
	fake2.mu.Lock()
	partCopies, aborted := fake2.partCopies, fake2.aborted
	fake2.mu.Unlock()
	if err == nil || partCopies != 1 || len(*waits) != 0 || !aborted {
		t.Errorf("err %v, part copies %d, waits %v, aborted %v; want an error after one attempt and the abort",
			err, partCopies, *waits, aborted)
	}
}

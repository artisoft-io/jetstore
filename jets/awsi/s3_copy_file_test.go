package awsi

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// The multipart copy must encrypt with JETS_S3_KMS_KEY_ARN, as the single part
// copy does. Without it s3 falls back to the destination bucket's default key (CPED-180).
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

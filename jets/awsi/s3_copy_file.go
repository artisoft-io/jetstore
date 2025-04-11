package awsi

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var fileSizeCutoff int64 = 500 * 1024 * 1024 // file less than 500 MB using single shot copy
var fileSizeMidPoint int64 = 10 * 1024 * 1024 * 1024
var smallChunk int64 = 25 * 1024 * 1024 // multi part: part size of 25 MB for file size < 10 GB
var bigChunk int64 = 100 * 1024 * 1024  // multi part: part size of 100 MB for files > 10 GB

// Copy a file from s3 to s3.
// Do a copy in a single action if the file is less than fileSizeCutoff, otherwise do a multi-part copy.
func CopyS3File(ctx context.Context, s3Client *s3.Client, srcBucket, srcKey, destBucket, destKey string) error {
	// Get the size of the source file
	fileSize, err := GetObjectSize(s3Client, srcBucket, srcKey)
	if err != nil {
		return fmt.Errorf("while getting the file size: %v", err)
	}
	if fileSize < fileSizeCutoff {
		// Do the copy in one shot
		copyInput := &s3.CopyObjectInput{
			CopySource: aws.String(url.QueryEscape(fmt.Sprintf("%s/%s", srcBucket, srcKey))),
			Bucket:     aws.String(destBucket),
			Key:        aws.String(destKey),
		}
		if len(kmsKeyArn) > 0 {
			copyInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			copyInput.SSEKMSKeyId = aws.String(kmsKeyArn)
		}
		_, err = s3Client.CopyObject(ctx, copyInput)
		return err
	}

	// Copy using a multi-part copy action
	copyInput := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(srcBucket),
		Key:    aws.String(srcKey),
	}
	if len(kmsKeyArn) > 0 {
		copyInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		copyInput.SSEKMSKeyId = aws.String(kmsKeyArn)
	}
	// Initiate the multi-part upload / copy
	uploader, err := s3Client.CreateMultipartUpload(ctx, copyInput)
	if err != nil {
		return fmt.Errorf("while CreateMultipartUpload: %v", err)
	}
	partSize := smallChunk
	if fileSize > fileSizeMidPoint {
		partSize = bigChunk
	}
	var bytePosition int64
	var partNbr int32
	var uploadOutput *s3.UploadPartCopyOutput
	var uploadErr error
	var sleepDuration time.Duration = 500 * time.Millisecond
	copyResponses := make([]types.CompletedPart, 0, fileSize/partSize)

	defer func() {
		if uploadErr != nil {
			// Cancel the whole thing
			_, err := s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(destBucket),
				Key:      aws.String(destKey),
				UploadId: uploader.UploadId,
			})
			if err != nil {
				log.Println("WARNING: while AbortMultipartUpload:", err)
			}
		}
	}()

	// Upload each part
	for bytePosition < fileSize {
		// The last part might be smaller than partSize, so check to make sure
		// that lastByte isn't beyond the end of the object.
		lastByte := bytePosition + partSize - 1
		if lastByte > fileSize-1 {
			lastByte = fileSize - 1
		}
		partNbr++
		uploadInput := &s3.UploadPartCopyInput{
			CopySource:      aws.String(url.QueryEscape(fmt.Sprintf("%s/%s", srcBucket, srcKey))),
			Bucket:          aws.String(destBucket),
			Key:             aws.String(destKey),
			CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", bytePosition, lastByte)),
			PartNumber:      aws.Int32(partNbr),
			UploadId:        uploader.UploadId,
		}
		retry := 0
	do_retry:
		uploadOutput, uploadErr = s3Client.UploadPartCopy(ctx, uploadInput)
		if uploadErr != nil {
			if retry < 4 {
				retry++
				time.Sleep(sleepDuration)
				sleepDuration *= 2
				goto do_retry
			}
			return uploadErr
		}
		bytePosition += partSize
		copyResponses = append(copyResponses, types.CompletedPart{
			ETag:       aws.String(strings.Trim(*uploadOutput.CopyPartResult.ETag, "\"")),
			PartNumber: aws.Int32(partNbr),
		})
	}

	_, uploadErr = s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(destBucket),
		Key:      aws.String(destKey),
		UploadId: uploader.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: copyResponses,
		},
	})
	return uploadErr
}

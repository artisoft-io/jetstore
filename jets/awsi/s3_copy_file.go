package awsi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ***
// var fileSizeCutoff int64 = 500 * 1024 * 1024 // file less than 500 MB using single shot copy
var fileSizeCutoff int64 = 5 * 1024 * 1024 // file less than 5 MB using single shot copy
var fileSizeMidPoint int64 = 10 * 1024 * 1024 * 1024

// var smallChunk int64 = 25 * 1024 * 1024 // multi part: part size of 25 MB for file size < 10 GB
var smallChunk int64 = 5 * 1024 * 1024 // multi part: part size of 25 MB for file size < 10 GB
var bigChunk int64 = 100 * 1024 * 1024 // multi part: part size of 100 MB for files > 10 GB

// helper function to build the string for the range of bits to copy
func buildCopySourceRange(start, partSize, objectSize int64) string {
	end := start + partSize - 1
	if end > objectSize {
		end = objectSize - 1
	}
	return fmt.Sprintf("bytes=%d-%d", start, end)
}

// function that starts, perform each part upload, and completes the copy
func MultiPartCopy(ctx context.Context, svc *s3.Client, srcBucket string, srcKey string, destBucket string, destKey string) error {

	// Get the size of the source file
	fileSize, err := GetObjectSize(svc, srcBucket, srcKey)
	if err != nil {
		return fmt.Errorf("while getting the file size: %v", err)
	}
	copySource := url.QueryEscape(fmt.Sprintf("/%s/%s", srcBucket, srcKey))

	if fileSize < fileSizeCutoff {
		// Do the copy in one shot
		log.Printf("Copying using single part for file %s of size %d", srcKey, fileSize)
		copyInput := &s3.CopyObjectInput{
			CopySource: &copySource,
			Bucket:     &destBucket,
			Key:        &destKey,
		}
		if len(kmsKeyArn) > 0 {
			copyInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			copyInput.SSEKMSKeyId = &kmsKeyArn
		}
		_, err = svc.CopyObject(ctx, copyInput)
		return err
	}

	// Copy using a multi-part copy action
	log.Printf("Copying using a multi-part copy for file %s of size %d", srcKey, fileSize)

	// Create the multipart upload: get the upload id as it is needed later
	var uploadId string
	createOutput, err := svc.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &destBucket,
		Key:    &destKey,
	})
	if err != nil {
		return err
	}
	if createOutput != nil && createOutput.UploadId != nil {
		uploadId = *createOutput.UploadId
	}
	if uploadId == "" {
		return errors.New("no upload id found in start upload request")
	}

	var i int64
	var partNumber int32 = 1
	maxRetry := 4
	parts := make([]types.CompletedPart, 0)
	partSize := smallChunk
	if fileSize > fileSizeMidPoint {
		partSize = bigChunk
	}
	numUploads := fileSize/partSize + 1
	abort := func(errMsg string) {
		log.Printf("%s, attempting to abort upload\n", errMsg)
		abortIn := s3.AbortMultipartUploadInput{
			Bucket:   &destBucket,
			Key:      &destKey,
			UploadId: &uploadId,
		}
		// ignoring any errors with aborting the copy
		_, err := svc.AbortMultipartUpload(context.TODO(), &abortIn)
		if err != nil {
			log.Printf("WARNING: Abort upload failed: %v\n", err)
		} else {
			log.Printf("Upload aborted")
		}
	}
	log.Printf("Will attempt upload in %d number of parts to %s", numUploads, destKey)

	for i = 0; i < fileSize; i += partSize {
		copyRange := buildCopySourceRange(i, partSize, fileSize)
		partNum := partNumber
		partInput := s3.UploadPartCopyInput{
			Bucket:          &destBucket,
			CopySource:      &copySource,
			CopySourceRange: &copyRange,
			Key:             &destKey,
			PartNumber:      &partNum,
			UploadId:        &uploadId,
		}

		log.Printf("Attempting to upload part %d with range: %s", partNumber, copyRange)
		sleepDuration := 500 * time.Millisecond
		retry := 0
	do_retry:
		partResp, err := svc.UploadPartCopy(ctx, &partInput)
		//*** TESTING
		if err == nil && strings.HasSuffix(destKey, "1.csv") && retry == 0 {
			err = fmt.Errorf("error: simulated part upload error")
		}
		if err != nil {
			if retry < maxRetry {
				log.Printf("Got error in s3Client.UploadPartCopy '%v' for part %d (retrying)", err, partNum)
				retry++
				time.Sleep(sleepDuration)
				sleepDuration *= 2
				goto do_retry
			}
			abort(fmt.Sprintf("Upload part %d failed", partNum))
			return fmt.Errorf("while uploading part %d: %v", partNumber, err)
		}

		//copy etag and part number from response as it is needed for completion
		if partResp != nil && partResp.CopyPartResult != nil {
			etag := *partResp.CopyPartResult.ETag
			cPart := types.CompletedPart{
				ETag:       &etag,
				PartNumber: &partNum,
			}
			parts = append(parts, cPart)
		} else {
			return fmt.Errorf("error: upload part had no error but did not returned CopyPartResult")
		}
		log.Printf("Successfully upload part %d of %s", partNumber, uploadId)
		partNumber++
	}

	//complete actual upload
	//does not actually copy if the complete command is not received
	complete := s3.CompleteMultipartUploadInput{
		Bucket:   &destBucket,
		Key:      &destKey,
		UploadId: &uploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	}
	compOutput, err := svc.CompleteMultipartUpload(ctx, &complete)
	if err != nil {
		abort("error while completing the upload")
		return fmt.Errorf("while completing upload: %v", err)
	}
	if compOutput != nil {
		log.Printf("Successfully copied Bucket: %s Key: %s to Bucket: %s Key: %s", srcBucket, srcKey, destBucket, destKey)
	}
	return nil
}

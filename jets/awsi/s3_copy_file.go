package awsi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var fileSizeCutoff int64 = 500 * 1024 * 1024 // file less than 500 MB using single shot copy
var fileSizeMidPoint int64 = 10 * 1024 * 1024 * 1024

var smallChunk int64 = 25 * 1024 * 1024 // multi part: part size of 25 MB for file size < 10 GB
var bigChunk int64 = 100 * 1024 * 1024  // multi part: part size of 100 MB for files > 10 GB

// helper function to build the string for the range of bits to copy
func buildCopySourceRange(start, partSize, objectSize int64) string {
	end := start + partSize - 1
	if end > objectSize {
		end = objectSize - 1
	}
	return fmt.Sprintf("bytes=%d-%d", start, end)
}

// function that starts, perform each part upload, and completes the copy
func MultiPartCopy(ctx context.Context, svc *s3.Client, maxPoolSize int,
	srcBucket string, srcKey string, destBucket string, destKey string) error {

	// Get the size of the source file
	fileSize, err := GetObjectSize(svc, srcBucket, srcKey)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			log.Printf(
				"warning: in MultiPartCopy, source file key %s/%s does not exist, skipping file copy",
				srcBucket, srcKey)
			return nil
		}
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

	maxRetry := 4
	partSize := smallChunk
	if fileSize > fileSizeMidPoint {
		partSize = bigChunk
	}
	numUploads := int(fileSize/partSize + 1)
	poolSize := min(maxPoolSize, numUploads)
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

	// Use a channel to distribute the part upload to a pool of workers
	tasksCh := make(chan s3.UploadPartCopyInput, 1)
	taskResultsCh := make(chan types.CompletedPart, 1)
	errCh := make(chan error, 10)
	done := make(chan struct{})
	sendError := func(err error) {
		if err == nil {
			return
		}
		errCh <- err
		// Interrupt the process, avoid closing a closed channel
		select {
		case <-done:
		default:
			close(done)
		}
	}

	// Set the the worker pool
	go func() {
		defer close(taskResultsCh)
		log.Printf("Uploading %d parts using a pool size of %d to %s", numUploads, poolSize, destKey)
		var wg sync.WaitGroup
		for i := range poolSize {
			wg.Add(1)
			go func(iworker int) {
				defer wg.Done()
				// Do work - upload the part
				for partInput := range tasksCh {

					sleepDuration := 500 * time.Millisecond
					retry := 0
				do_retry:
					partResp, err := svc.UploadPartCopy(ctx, &partInput)
					if err != nil {
						if retry < maxRetry && !strings.Contains(err.Error(), "context canceled") {
							log.Printf(
								"Got error in s3Client.UploadPartCopy '%v' for part %d (retrying)", err, *partInput.PartNumber)
							retry++
							time.Sleep(sleepDuration)
							sleepDuration *= 2
							goto do_retry
						}
						abort(fmt.Sprintf("Upload worker %d, part %d failed", iworker, *partInput.PartNumber))
						sendError(fmt.Errorf("while uploading worker %d, part %d: %v", iworker, *partInput.PartNumber, err))
						return
					}

					// send out etag and part number from response as it is needed for completion
					if partResp != nil && partResp.CopyPartResult != nil {
						eTag := *partResp.CopyPartResult.ETag
						partNum := *partInput.PartNumber
						taskResultsCh <- types.CompletedPart{
							ETag:       &eTag,
							PartNumber: &partNum,
						}
					} else {
						sendError(fmt.Errorf(
							"error: worker %d,upload part had no error but did not returned CopyPartResult", iworker))
						return
					}
					// log.Printf(
					// 	"***Successfully upload worker %d, part %d of %s", iworker, *partInput.PartNumber, uploadId)
				}
				// log.Println("***All done for part upload worker", iworker)
			}(i)
		}
		// log.Printf("***Waiting on part upload workers task (pool of size %d) to complete", poolSize)
		wg.Wait()
		// log.Printf("***DONE - Part upload workers task (pool of size %d) completed", poolSize)
	}()

	// Prepare a task for each part to upload/copy
	numberOfParts := int(fileSize/partSize + 1)
	go func() {
		defer close(tasksCh)
		var i int64
		var partNumber int32 = 1
		// log.Printf("*** Preparing %d copy tasks", numberOfParts)
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
			// send the task to the worker pool
			select {
			case tasksCh <- partInput:
			case <-done:
				log.Println("sending tasks to pool worker interrupted")
				return
			}
			partNumber++
		}
	}()

	// Collect the tasks results
	go func() {
		defer close(errCh)
		// log.Println("*** Collecting tasks results")
		parts := make([]types.CompletedPart, 0, numberOfParts)
		for result := range taskResultsCh {
			// log.Printf("***Got from taskResultsCh OK (copy part) for part %d", *result.PartNumber)
			parts = append(parts, result)
		}
		// Sort the result my part number
		slices.SortFunc(parts, func(lhs, rhs types.CompletedPart) int {
			a := *lhs.PartNumber
			b := *rhs.PartNumber
			switch {
			case a < b:
				return -1
			case a > b:
				return 1
			default:
				return 0
			}
		})
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
			sendError(fmt.Errorf("while completing upload: %v", err))
			return
		}
		if compOutput != nil {
			// log.Printf("*** Successfully copied Bucket: %s Key: %s to Bucket: %s Key: %s", srcBucket, srcKey, destBucket, destKey)
		}
	}()

	// Collect if there were any errors
	for err := range errCh {
		if err != nil {
			err = fmt.Errorf("while multipart copy file: %v", err)
			log.Println(err)
			return err
		}
	}
	return nil
}

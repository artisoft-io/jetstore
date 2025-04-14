package awsi

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var fileSizeCutoff int64 = 500 * 1024 * 1024 // file less than 500 MB using single shot copy
var fileSizeMidPoint int64 = 10 * 1024 * 1024 * 1024
var smallChunk int64 = 25 * 1024 * 1024 // multi part: part size of 25 MB for file size < 10 GB
var bigChunk int64 = 100 * 1024 * 1024  // multi part: part size of 100 MB for files > 10 GB

type completedTask struct {
	completedPart *types.CompletedPart
	err           error
}

// Copy a file from s3 to s3.
// Do a copy in a single action if the file is less than fileSizeCutoff, otherwise do a multi-part copy.
func CopyS3File(ctx context.Context, s3Client *s3.Client, poolSize int, done chan struct{}, srcBucket, srcKey, destBucket, destKey string) error {
	// Get the size of the source file
	fileSize, err := GetObjectSize(s3Client, srcBucket, srcKey)
	if err != nil {
		return fmt.Errorf("while getting the file size: %v", err)
	}
	if fileSize < fileSizeCutoff {
		// Do the copy in one shot
		log.Printf("Copying using single part for file %s of size %d", srcKey, fileSize)
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
	log.Printf("Copying using a multi-part copy for file %s of size %d", srcKey, fileSize)
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
	var uploadErr error

	defer func() {
		if uploadErr != nil {
			// Cancel the whole thing
			log.Printf("Get error: %v, aborting multipart upload", uploadErr)
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

	// Use a channel to distribute the part upload to a pool of workers
	tasksCh := make(chan s3.UploadPartCopyInput, 1)
	taskResultsCh := make(chan completedTask, 1)

	// Create a pool of workers
	log.Printf("Creating a part upload worker pool of size %d", poolSize)
	go func() {
		var wg sync.WaitGroup
		for i := range poolSize {
			wg.Add(1)
			go func(iworker int) {
				defer wg.Done()
				// Do work - upload the part
				for task := range tasksCh {
					sleepDuration := 500 * time.Millisecond
					retry := 0
				do_retry:
					uploadOutput, err := s3Client.UploadPartCopy(ctx, &task)
					if err != nil {
						if retry < 4 {
							log.Printf("Got error in s3Client.UploadPartCopy '%v' for part %d (retrying)", err, *task.PartNumber)
							retry++
							time.Sleep(sleepDuration)
							sleepDuration *= 2
							goto do_retry
						}
						// Unable to complete, send the err and bail
						log.Printf("*** Got error in s3Client.UploadPartCopy '%v' for part %d (too many tries)", err, *task.PartNumber)
						select {
						case taskResultsCh <- completedTask{
							completedPart: nil,
							err:           err,
						}:
						case <-done:
							log.Println("CopyS3File pool worker interrupted")
						}
						return
					}
					select {
					case taskResultsCh <- completedTask{
						completedPart: &types.CompletedPart{
							ETag:       aws.String(*uploadOutput.CopyPartResult.ETag),
							PartNumber: aws.Int32(partNbr),
						},
						err: nil,
					}:
					case <-done:
						log.Println("CopyS3File pool worker interrupted (2)")
						return
					}
				}
				log.Println("All done for part upload worker", iworker)
			}(i)
		}
		log.Printf("Waiting on part upload workers task (pool of size %d) to complete", poolSize)
		wg.Wait()
		log.Printf("DONE - Part upload workers task (pool of size %d) completed", poolSize)
		close(taskResultsCh)
	}()

	// Prepare a task for each part
	log.Printf("Preparing %d copy tasks", fileSize/partSize)
	go func() {
		defer close(tasksCh)
		for bytePosition < fileSize {
			// The last part might be smaller than partSize, so check to make sure
			// that lastByte isn't beyond the end of the object.
			lastByte := min(bytePosition + partSize - 1, fileSize - 1)
			partNbr++
			uploadInput := s3.UploadPartCopyInput{
				CopySource:      aws.String(url.QueryEscape(fmt.Sprintf("%s/%s", srcBucket, srcKey))),
				Bucket:          aws.String(destBucket),
				Key:             aws.String(destKey),
				CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", bytePosition, lastByte)),
				PartNumber:      aws.Int32(partNbr),
				UploadId:        uploader.UploadId,
			}
			bytePosition += partSize

			// send the task to the worker pool
			select {
			case tasksCh <- uploadInput:
			case <-done:
				log.Println("sending tasks to pool worker interrupted")
				return
			}
		}
	}()

	// Collect the tasks results
	log.Println("Collecting tasks results")
	copyResponses := make([]types.CompletedPart, 0, fileSize/partSize)
	for result := range taskResultsCh {
		if result.err != nil {
			log.Printf("*** Got error from taskResultsCh (copy part): %v, for part %d", result.err, *result.completedPart.PartNumber)
			uploadErr = err
			return uploadErr
		}
		copyResponses = append(copyResponses, *result.completedPart)
	}

	// Complete the multi part upload / copy
	log.Println("Completing multi part copy")
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

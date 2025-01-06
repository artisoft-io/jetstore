package compute_pipes

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3DeviceWorker is the component that actually put a local file to s3
// The worker gets it's tasks from a channel managed by S3DeviceManager

type S3DeviceWorker struct {
	s3Uploader *manager.Uploader
	done       chan struct{}
	errCh      chan error
}

func NewS3DeviceWorker(s3Uploader *manager.Uploader, done chan struct{}, errCh chan error) *S3DeviceWorker {
	return &S3DeviceWorker{
		s3Uploader: s3Uploader,
		done:       done,
		errCh:      errCh,
	}
}

func (ctx *S3DeviceWorker) DoWork(mgr *S3DeviceManager, resultCh chan ComputePipesResult) {
	var count int64
	// log.Printf("S3Device Worker Entering DoWork")
	for task := range mgr.WorkersTaskCh {
		err := ctx.processTask(&task, mgr, resultCh)
		if err != nil {
			return
		}
		count += 1
	}
	resultCh <- ComputePipesResult{PartsCount: count}
}

func (ctx *S3DeviceWorker) processTask(task *S3Object, _ *S3DeviceManager, resultCh chan ComputePipesResult) error {
	var cpErr error
	var putObjInput *s3.PutObjectInput
	var retry int

	// log.Println("S3DeviceWorker: Put file to s3 key:",task.FileKey)
	// open the local temp file for the writer
	fileHd, err := os.Open(task.LocalFilePath)
	if err != nil {
		cpErr = fmt.Errorf("while opening local file for read %v", err)
		goto gotError
	}
	defer func() {
		fileHd.Close()
		os.Remove(task.LocalFilePath)
	}()

	if task.ExternalBucket == "" {
		task.ExternalBucket = bucketName
	}
	putObjInput = &s3.PutObjectInput{
		Bucket: &task.ExternalBucket,
		Key:    &task.FileKey,
		Body:   bufio.NewReader(fileHd),
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	retry = 0
do_retry:
	_, err = ctx.s3Uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		cpErr = fmt.Errorf("while copying file to s3: %v", err)
		goto gotError
	}
	return nil
gotError:
	log.Println(cpErr)
	resultCh <- ComputePipesResult{Err: cpErr}
	ctx.errCh <- cpErr
	// Avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
	return cpErr
}

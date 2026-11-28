package compute_pipes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
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

// processTask puts one local file to s3, then removes it.
//
// The send is retried with awsi.RetrySend, from the start of the file each
// time. Until 2026-11-27 it was retried six times at 500 ms on any error, with
// a body that the failed attempt had already read from: a retry after a
// partial read sent the rest of the file as the whole object.
func (ctx *S3DeviceWorker) processTask(task *S3Object, _ *S3DeviceManager, resultCh chan ComputePipesResult) error {
	var cpErr error

	// log.Printf("*** S3DeviceWorker: Put s3 key %s, from local file %s\n",task.FileKey, task.LocalFilePath)
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

	if awsi.IsJetStoreBucket(task.ExternalBucket) {
		task.ExternalBucket = bucketName
	}
	err = awsi.RetrySend(context.TODO(), fmt.Sprintf("s3://%s/%s", task.ExternalBucket, task.FileKey), func() error {
		if _, err := fileHd.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("while rewinding local file %s: %w", task.LocalFilePath, err)
		}
		putObjInput := &s3.PutObjectInput{
			Bucket: &task.ExternalBucket,
			Key:    &task.FileKey,
			Body:   bufio.NewReader(fileHd),
		}
		if len(kmsKeyArn) > 0 {
			putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			putObjInput.SSEKMSKeyId = &kmsKeyArn
		}
		_, err := ctx.s3Uploader.Upload(context.TODO(), putObjInput)
		return err
	})
	if err != nil {
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

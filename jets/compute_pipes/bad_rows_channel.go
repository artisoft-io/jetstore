package compute_pipes

import (
	"fmt"
	"log"
	"os"
)

type BadRowsChannel struct {
	s3DeviceManager *S3DeviceManager
	s3BasePath      string
	OutputCh        chan []any
	doneCh          chan struct{}
	errCh           chan error
}

func NewBadRowChannel(s3DeviceManager *S3DeviceManager, s3BasePath string,
	doneCh chan struct{}, errCh chan error) *BadRowsChannel {
	return &BadRowsChannel{
		s3DeviceManager: s3DeviceManager,
		s3BasePath:      s3BasePath,
		OutputCh:        make(chan []any, 2),
		doneCh:          doneCh,
		errCh:           errCh,
	}
}

func (ctx *BadRowsChannel) Write(nodeId int) {
	var cpErr, err error
	var fout *os.File
	var tempFileName, s3FileName, fileName string

	// Create a local temp dir to save the file partition for writing to s3
	localTempDir, err2 := os.MkdirTemp("", "bad_rows")
	if err2 != nil {
		cpErr = fmt.Errorf("while creating temp dir (in BadRowsChannel.Write) %v", err2)
		goto gotError
	}

	// Register as a client to S3DeviceManager
	if ctx.s3DeviceManager.ClientsWg != nil {
		ctx.s3DeviceManager.ClientsWg.Add(1)
	} else {
		log.Panicln("ERROR Expecting ClientsWg not nil")
	}

	fileName = fmt.Sprintf("part%04d-%07d.%s", nodeId, 1, ".txt")

	// Write the data to a local temp file and then copy it to s3
	tempFileName = fmt.Sprintf("%s/%s", localTempDir, fileName)
	s3FileName = fmt.Sprintf("%s/%s", ctx.s3BasePath, fileName)
	// fmt.Println("**&@@ WritePartition *1: fileName:", *ctx.fileName)
	if ctx.s3DeviceManager == nil {
		cpErr = fmt.Errorf("error: s3DeviceManager is nil")
		goto gotError
	}

	// open the local temp file
	fout, err = os.Create(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("opening output file failed: %v", err)
		goto gotError
	}
	defer fout.Close()

	// Write the partition
	//*TODO
	// writer(fout)

	// fmt.Println("**&@@ WritePartition: DONE writing local file for fileName:", *ctx.fileName)
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		FileKey:       s3FileName,
		LocalFilePath: tempFileName,
	}:
	case <-ctx.doneCh:
		log.Printf("WritePartition: sending file to S3DeviceManager interrupted")
	}

	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)

}

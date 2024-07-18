package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3DeviceManager manages a pool of S3DeviceWorker to put local files
// to s3

var bucketName, regionName string

func init() {
	bucketName = os.Getenv("JETS_BUCKET")
	regionName = os.Getenv("JETS_REGION")
}

// S3DeviceManager manage a pool of workers to put file to s3.
// ClientWg is a wait group of the partition writers created during 
// buildComputeGraph function. The WorkersTaskCh is closed in process_file.go
type S3DeviceManager struct {
	cpConfig         *ComputePipesConfig
	s3WorkerPoolSize int
	WorkersTaskCh    chan S3Object
	ClientsWg        *sync.WaitGroup
}

// S3Object is the worker's task payload to put a file to s3
type S3Object struct {
	FileKey       string
	LocalFilePath string
}

// Create the S3DeviceManager, it will be set to the receiving BuilderContext
func (ctx *BuilderContext) NewS3DeviceManager() error {

	// Create the s3 uploader that will be used by all the workers
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(regionName))
	if err != nil {
		return fmt.Errorf("while loading aws configuration (in NewS3DeviceManager): %v", err)
	}
	// Create the uploader
	s3Uploader := manager.NewUploader(s3.NewFromConfig(cfg))

	// Create the s3 device manager
	ctx.s3DeviceManager = &S3DeviceManager{
		cpConfig:         ctx.cpConfig,
		s3WorkerPoolSize: ctx.cpConfig.ClusterConfig.S3WorkerPoolSize,
		WorkersTaskCh:    make(chan S3Object, 10),
	}

	// Create a channel for the workers to report results
	// Collect the results from all the workers
	s3WorkersResultCh := make(chan ComputePipesResult)
	go func() {
		var partCount int64
		var err error
		for workerResult := range s3WorkersResultCh {
			partCount += workerResult.PartsCount
			if workerResult.Err != nil {
				err = workerResult.Err
				break
			}
		}
		// Send out the collected result
		select {
		case ctx.chResults.S3PutObjectResultCh <- ComputePipesResult{
			PartsCount: partCount,
			Err:        err}:
			if err != nil {
				// Interrupt the whole process, there's been an error writing a file part
				close(ctx.done)
			}
		case <-ctx.done:
			log.Printf("Collecting results from S3DeviceWorker interrupted")
		}
		close(ctx.chResults.S3PutObjectResultCh)
	}()

	// Set up all the workers, use a wait group to track when they are all done
	var wg sync.WaitGroup
	for i := 0; i < ctx.s3DeviceManager.s3WorkerPoolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := NewS3DeviceWorker(s3Uploader, ctx.done, ctx.errCh)
			worker.DoWork(ctx.s3DeviceManager, s3WorkersResultCh)
		}()
	}
	wg.Wait()
	close(s3WorkersResultCh)
	XXX
}

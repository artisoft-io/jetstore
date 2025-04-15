package delegate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v4/pgxpool"
)

var s3CopyFileTotalPoolSize int = 40
var commandWorkerMaxPoolSize int = 10

type CommandWorker struct {
	ctx      context.Context
	s3Client *s3.Client
	done     chan struct{}
	errCh    chan<- error
}

func NewCommandWorker(ctx context.Context, s3Client *s3.Client, 
	done chan struct{},	errCh chan<- error) *CommandWorker {
	return &CommandWorker{
		ctx:      ctx,
		s3Client: s3Client,
		done:     done,
		errCh:    errCh,
	}
}

func (ctx *CommandWorker) DoWork(workersTaskCh <-chan any) {
	for task := range workersTaskCh {
		switch vv := task.(type) {
		case compute_pipes.S3CopyFileSpec:
			// Do work here
			awsi.CopyS3File(ctx.ctx, ctx.s3Client, vv.WorkerPoolSize, ctx.done, ctx.errCh, vv.SourceBucket,
				vv.SourceKey, vv.DestinationBucket, vv.DestinationKey)

		default:
			// Unknown task type
			ctx.sendError(fmt.Errorf("error: unknown CommandWorker task type: %T", task))
			return
		}
	}
}

func (ctx *CommandWorker) sendError(err error) {
	ctx.errCh <- err
	// Interrupt the process, avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
}

// Run report commands specified by the schema provider of the main input source
// Note the errCh will be closed by this func either synchronously or async when the worker pool completes
func (ca *CommandArguments) RunSchemaProviderReportsCmds(ctx context.Context, dbpool *pgxpool.Pool,
	errCh chan<- error) (err error) {

	closeErrCh := true
	defer func() {
		if closeErrCh {
			log.Println("Closing errCh as no report commands to execute")
			close(errCh)
		}
	}()

	// Validate we have a pipeline execution uc
	if len(ca.SessionId) == 0 {
		return nil
	}
	var schemaProviderJson string
	log.Println("Getting the schema provider of the main input source")
	stmt := `
	SELECT	ir.schema_provider_json
	FROM 
		jetsapi.pipeline_execution_status pe,
		jetsapi.input_registry ir
	WHERE pe.main_input_registry_key = ir.key
		AND pe.session_id = $1`
	err = dbpool.QueryRow(ctx, stmt, ca.SessionId).Scan(&schemaProviderJson)
	if err != nil {
		return fmt.Errorf("query pipeline_execution_status failed: %v", err)
	}
	if len(schemaProviderJson) == 0 {
		// Nothing to do here
		return nil
	}
	type SchemaProviderShort struct {
		ReportCmds []compute_pipes.ReportCmdSpec `json:"report_cmds"`
	}
	schemaProvider := SchemaProviderShort{}
	err = json.Unmarshal([]byte(schemaProviderJson), &schemaProvider)
	if err != nil {
		return fmt.Errorf("while unmarshaling schema_provider_json: %s", err)
	}
	if len(schemaProvider.ReportCmds) == 0 {
		// No Report Command to Execute
		return nil
	}
	s3Client, err := awsi.NewS3Client()
	if err != nil {
		return err
	}
	log.Println("Starting the execution of the Schema Provider's report commands")

	// Create a channel to send work to worker pool
	workersTaskCh := make(chan any)
	defer close(workersTaskCh)
	closeErrCh = false // the errCh will be closed in the go func below
	done := make(chan struct{})

	// Create a pool of workers
	workerPoolSize := min(len(schemaProvider.ReportCmds), commandWorkerMaxPoolSize)
	go func() {
		var wg sync.WaitGroup
		for range workerPoolSize {
			wg.Add(1)
			go func() {
				defer wg.Done()
				NewCommandWorker(ctx, s3Client, done, errCh).DoWork(workersTaskCh)
			}()
		}
		log.Printf("Waiting on report command workers task (pool of size %d) to complete", workerPoolSize)
		wg.Wait()
		log.Printf("Done waiting on report command workers task (pool of size %d)", workerPoolSize)
		close(errCh)
	}()

	// Execute the report commands
	for i := range schemaProvider.ReportCmds {
		switch schemaProvider.ReportCmds[i].Type {
		case "s3_copy_file":
			copyConfig := schemaProvider.ReportCmds[i].S3CopyFileConfig
			if copyConfig == nil {
				return fmt.Errorf("error: report command 's3_copy_file' is missing s3_copy_file_config")
			}
			if copyConfig.WorkerPoolSize == 0 {
				copyConfig.WorkerPoolSize = s3CopyFileTotalPoolSize / workerPoolSize
			}
			select {
			case workersTaskCh <- *copyConfig:
			case <-done:
				log.Println("reportCommand pool worker interrupted")
				return
			}
		default:
			return fmt.Errorf("error: unknown report command type: %s", schemaProvider.ReportCmds[i].Type)
		}
	}
	return nil
}

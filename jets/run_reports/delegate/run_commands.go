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
	done chan struct{}, errCh chan<- error) *CommandWorker {
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
			err := awsi.MultiPartCopy(ctx.ctx, ctx.s3Client, vv.WorkerPoolSize,
				vv.SourceBucket, vv.SourceKey, vv.DestinationBucket, vv.DestinationKey, false)
			if err != nil {
				ctx.sendError(err)
			}

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
	errCh chan<- error) {
	var err error
	closeErrCh := true
	defer func() {
		if closeErrCh {
			log.Println("Closing errCh as no report commands to execute")
			close(errCh)
		}
	}()

	// Validate we have a pipeline execution uc
	if len(ca.SessionId) == 0 {
		return
	}
	if len(ca.SchemaProviderJson) == 0 {
		// Nothing to do here
		log.Println("Input surce has no schema provider, bailing out")
		return
	}
	type SchemaProviderShort struct {
		Env        map[string]any                `json:"env"`
		ReportCmds []compute_pipes.ReportCmdSpec `json:"report_cmds"`
	}
	schemaProvider := SchemaProviderShort{}
	err = json.Unmarshal([]byte(ca.SchemaProviderJson), &schemaProvider)
	if err != nil {
		errCh <- fmt.Errorf("while unmarshaling schema_provider_json: %s", err)
		return
	}
	// Determine if there are report commands to execute
	reportCommands := make([]*compute_pipes.ReportCmdSpec, 0, len(schemaProvider.ReportCmds))
	for i := range schemaProvider.ReportCmds {
		if schemaProvider.ReportCmds[i].When != nil {
			// Evaluate the when expression
			builderContext := compute_pipes.ExprBuilderContext(schemaProvider.Env)
			evaluator, err := builderContext.BuildExprNodeEvaluator("report_cmds", nil, schemaProvider.ReportCmds[i].When)
			if err != nil {
				errCh <- fmt.Errorf("while building evaluator for report command %d: %v", i, err)
				return
			}
			v, err := evaluator.Eval(schemaProvider.Env)
			if err != nil {
				errCh <- fmt.Errorf("while evaluating report command %d when expression: %v", i, err)
				return
			}
			if !compute_pipes.ToBool(v) {
				continue
			}
		}
		reportCommands = append(reportCommands, &schemaProvider.ReportCmds[i])
	}

	if len(reportCommands) == 0 {
		// No Report Command to Execute
		log.Println("Schema provider has no Report Commands to execute, bailing out")
		return
	}
	s3Client, err := awsi.NewS3Client()
	if err != nil {
		errCh <- err
		return
	}
	log.Println("Starting the execution of the Schema Provider's report commands")

	// Create a channel to send work to worker pool
	workersTaskCh := make(chan any)
	defer close(workersTaskCh)
	closeErrCh = false // the errCh will be closed in the go func below
	done := make(chan struct{})
	sendError := func(err error) {
		errCh <- err
		// Interrupt the process, avoid closing a closed channel
		select {
		case <-done:
		default:
			close(done)
		}
	}

	// Create a pool of workers
	workerPoolSize := min(len(schemaProvider.ReportCmds), commandWorkerMaxPoolSize)
	// //*** TESTING
	// workerPoolSize = 1
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
				sendError(fmt.Errorf("error: report command 's3_copy_file' is missing s3_copy_file_config"))
				return
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
			sendError(fmt.Errorf("error: unknown report command type: %s", schemaProvider.ReportCmds[i].Type))
			return
		}
	}
}

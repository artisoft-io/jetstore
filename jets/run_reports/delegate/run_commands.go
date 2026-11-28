package delegate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
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

// copyBuckets is the s3_copy_file command i with its src_bucket and dest_bucket
// resolved to the names awsi.MultiPartCopy is to address, or an error naming
// the command when one of them cannot be.
//
// A report command's buckets are not on the path compute_pipes checks
// (ValidateResolvedBuckets): they come from the schema provider as written, and
// no variable is substituted in them anywhere, so a bucket carrying a "$" is
// certain to be a placeholder nothing filled in, and is refused here rather
// than sent to S3 as a bucket name. A bucket left empty or written as the
// jetstore_bucket sentinel, with or without surrounding spaces, is the
// deployment's own bucket (JETS_BUCKET, jetStoreBucket); when that is not set
// either, the command is refused too. Anything else is an external bucket,
// addressed as written less surrounding spaces.
func copyBuckets(i int, cmd *compute_pipes.ReportCmdSpec, jetStoreBucket string) (compute_pipes.S3CopyFileSpec, error) {
	spec := *cmd.S3CopyFileConfig
	name := fmt.Sprintf("report_cmds[%d] (s3_copy_file)", i)
	if c := strings.TrimSpace(cmd.Comment); c != "" {
		name = fmt.Sprintf("report_cmds[%d] (s3_copy_file, %q)", i, c)
	}
	resolve := func(field, bucket string) (string, error) {
		trimmed := strings.TrimSpace(bucket)
		switch {
		case compute_pipes.IsUnresolvedBucket(trimmed):
			return "", fmt.Errorf("error: %s: %s %q is unresolved: it carries a variable reference, "+
				"and report commands substitute no variables", name, field, bucket)
		case awsi.IsJetStoreBucket(trimmed):
			if jetStoreBucket == "" {
				return "", fmt.Errorf("error: %s: %s %q names the deployment's own bucket, "+
					"and JETS_BUCKET is not set", name, field, bucket)
			}
			return jetStoreBucket, nil
		}
		return trimmed, nil
	}
	var err error
	if spec.SourceBucket, err = resolve("src_bucket", spec.SourceBucket); err != nil {
		return spec, err
	}
	if spec.DestinationBucket, err = resolve("dest_bucket", spec.DestinationBucket); err != nil {
		return spec, err
	}
	return spec, nil
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
	// Resolve every s3_copy_file command's buckets before any copy starts, so a
	// command that cannot be run stops the run with nothing copied. The commands
	// checked are the ones the loop below dispatches.
	copySpecs := make([]compute_pipes.S3CopyFileSpec, len(schemaProvider.ReportCmds))
	for i := range schemaProvider.ReportCmds {
		cmd := &schemaProvider.ReportCmds[i]
		if cmd.Type != "s3_copy_file" || cmd.S3CopyFileConfig == nil {
			continue
		}
		copySpecs[i], err = copyBuckets(i, cmd, awsi.JetStoreBucket())
		if err != nil {
			errCh <- err
			return
		}
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
			if schemaProvider.ReportCmds[i].S3CopyFileConfig == nil {
				sendError(fmt.Errorf("error: report command 's3_copy_file' is missing s3_copy_file_config"))
				return
			}
			copyConfig := copySpecs[i]
			if copyConfig.WorkerPoolSize == 0 {
				copyConfig.WorkerPoolSize = s3CopyFileTotalPoolSize / workerPoolSize
			}
			select {
			case workersTaskCh <- copyConfig:
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

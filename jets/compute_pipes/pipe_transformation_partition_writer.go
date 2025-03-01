package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/fraugster/parquet-go/parquet"
	"github.com/jackc/pgx/v4/pgxpool"
)

var jetsS3InputPrefix string
var jetsS3StagePrefix string
var jetsS3OutputPrefix string

func init() {
	jetsS3InputPrefix = os.Getenv("JETS_s3_INPUT_PREFIX")
	jetsS3StagePrefix = os.Getenv("JETS_s3_STAGE_PREFIX")
	jetsS3OutputPrefix = os.Getenv("JETS_s3_OUTPUT_PREFIX")
}

// partition_writer TransformationSpec implementing PipeTransformationEvaluator interface
// partition_writer: bundle input records into fixed-sized partitions.
// The outputCh column spec correspond to the intermediate channel to the actual
// device writer.
// currentDeviceCh is the physical ch to the device writer.
// If the TransformationSpec.PartitionSize is nil or 0 then there is a single partion.
// Replace the underlying channel to have a buffered channel and create one for each partition.
// Currently supporting writing to s3 jetstore stage path

type PartitionWriterTransformationPipe struct {
	cpConfig             *ComputePipesConfig
	dbpool               *pgxpool.Pool
	spec                 *TransformationSpec
	schemaProvider       SchemaProvider
	deviceWriterType     string
	localTempDir         *string
	externalBucket       string
	baseOutputPath       *string
	jetsPartitionLabel   string
	rowCountPerPartition int64
	partitionRowCount    int64
	totalRowCount        int64
	filePartitionNumber  int
	samplingRate         int
	samplingMaxCount     int
	samplingCount        int
	outputCh             *OutputChannel
	currentDeviceCh      chan []interface{}
	parquetSchema        *ParquetSchemaInfo
	columnEvaluators     []TransformationColumnEvaluator
	doneCh               chan struct{}
	errCh                chan error
	copy2DeviceResultCh  chan<- ComputePipesResult
	sessionId            string
	nodeId               int
	s3DeviceManager      *S3DeviceManager
	env                  map[string]interface{}
}

func MakeJetsPartitionLabel(jetsPartitionKey interface{}) string {
	key, ok := jetsPartitionKey.(string)
	if ok {
		return key
	}
	return fmt.Sprintf("%vp", jetsPartitionKey)
}

// Implementing interface PipeTransformationEvaluator
func (ctx *PartitionWriterTransformationPipe) Apply(input *[]interface{}) error {
	var err error
	if input == nil {
		err = fmt.Errorf("error: input record is nil in PartitionWriterTransformationPipe.Apply")
		log.Println(err)
		return err
	}

	// Check if we got the max sample records
	if ctx.samplingMaxCount > 0 && ctx.totalRowCount >= int64(ctx.samplingMaxCount) {
		return nil
	}

	// Check if partition is complete, if so close current output channel and start a new one
	if (ctx.rowCountPerPartition > 0 && ctx.partitionRowCount >= ctx.rowCountPerPartition) ||
		(ctx.samplingMaxCount > 0 && ctx.totalRowCount+ctx.partitionRowCount >= int64(ctx.samplingMaxCount)) {
		close(ctx.currentDeviceCh)
		ctx.currentDeviceCh = nil
		ctx.totalRowCount += ctx.partitionRowCount
		ctx.partitionRowCount = 0
		// Check again if we got the max nbr of sample record to avoid opening another partition
		if ctx.samplingMaxCount > 0 && ctx.totalRowCount >= int64(ctx.samplingMaxCount) {
			return nil
		}
	}

	// Check if this is the first call or the start of a new file partition, if so setup the device writer channel
	if ctx.currentDeviceCh == nil {
		// replace the underlying channel of outputCh with a buffered one
		ctx.currentDeviceCh = make(chan []interface{}, 10)
		ctx.outputCh.channel = ctx.currentDeviceCh

		// Start the device writter for the partition
		ctx.filePartitionNumber += 1

		var partitionFileName string
		if len(ctx.spec.OutputChannel.FileName) > 0 {
			// APPLY substitutions
			partitionFileName = doSubstitution(ctx.spec.OutputChannel.FileName, ctx.jetsPartitionLabel, "", ctx.env)
		}
		if partitionFileName == "" {
			var fileEx string
			switch ctx.deviceWriterType {
			case "csv_writer":
				fileEx = "csv"
			case "parquet_writer":
				fileEx = "parquet"
			case "fixed_width_writer":
				fileEx = "fixed_width"
			}
			partitionFileName = fmt.Sprintf("part%04d-%07d.%s", ctx.nodeId, ctx.filePartitionNumber, fileEx)
		}
		s3DeviceWriter := &S3DeviceWriter{
			s3DeviceManager: ctx.s3DeviceManager,
			source: &InputChannel{
				channel: ctx.currentDeviceCh,
				columns: ctx.outputCh.columns,
				config: &ChannelSpec{
					Name:      fmt.Sprintf("input channel for partition_writer for %s", partitionFileName),
					ClassName: ctx.outputCh.config.ClassName},
			},
			spec:           ctx.spec,
			schemaProvider: ctx.schemaProvider,
			outputCh:       ctx.outputCh,
			parquetSchema:  ctx.parquetSchema,
			localTempDir:   ctx.localTempDir,
			externalBucket: &ctx.externalBucket,
			s3BasePath:     ctx.baseOutputPath,
			fileName:       &partitionFileName,
			doneCh:         ctx.doneCh,
			errCh:          ctx.errCh,
		}
		ctx.s3DeviceManager.ClientsWg.Add(1)
		go func() {

			defer func() {
				// Catch the panic that might be generated downstream
				if r := recover(); r != nil {
					var buf strings.Builder
					buf.WriteString(fmt.Sprintf("s3DeviceWriter: recovered error: %v\n", r))
					buf.WriteString(string(debug.Stack()))
					cpErr := errors.New(buf.String())
					log.Println(cpErr)
					ctx.errCh <- cpErr
					// Avoid closing a closed channel
					select {
					case <-ctx.doneCh:
					default:
						close(ctx.doneCh)
					}
				}
				// fmt.Println("**@= Defer called done writing, calling ClientsWg.Done()")
				ctx.s3DeviceManager.ClientsWg.Done()
			}()

			switch ctx.deviceWriterType {
			case "csv_writer":
				s3DeviceWriter.WriteCsvPartition()
			case "parquet_writer":
				s3DeviceWriter.WriteParquetPartition()
			case "fixed_width_writer":
				s3DeviceWriter.WriteFixedWidthPartition()
			}
		}()
	}

	// Check if we are sampling records on the output
	if ctx.totalRowCount+ctx.partitionRowCount > 0 && ctx.samplingRate > 0 {
		ctx.samplingCount += 1
		if ctx.samplingCount < ctx.samplingRate {
			return nil
		}
	}
	ctx.samplingCount = 0

	// currentValue is either the input row or a new row based on ctx.NewRecord flag
	var currentValues *[]interface{}
	if ctx.spec.NewRecord {
		v := make([]interface{}, len(ctx.outputCh.config.Columns))
		currentValues = &v
		// initialize the column evaluators
		for i := range ctx.columnEvaluators {
			ctx.columnEvaluators[i].InitializeCurrentValue(currentValues)
		}
	} else {
		currentValues = input
	}

	// Apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err = ctx.columnEvaluators[i].Update(currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from partition_writer: %v", err)
			log.Println(err)
			return err
		}
	}
	// Notify the column evaluator that we're done
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Done(currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from partition_writer: %v", err)
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.outputCh.config.Columns) {
			*currentValues = (*currentValues)[:len(ctx.outputCh.config.Columns)]
		}
	}
	// Send the result to output
	// log.Printf("PARTITION WRITER (%s) ROW %v", ctx.outputCh.name, *currentValues)
	select {
	case ctx.outputCh.channel <- *currentValues:
	case <-ctx.doneCh:
		log.Printf("PartitionWriterTransformationPipe writing to '%s' interrupted", ctx.outputCh.name)
		return nil
	}
	ctx.partitionRowCount += 1

	return nil
}

// Done writing the splitter partition
//   - Close the current ctx.currentDeviceCh to flush the data, update totalRowCount
//   - Write to db the nodeId of this partition: session_id, shard, jets_partition
//   - Send the total row count to ctx.copy2DeviceResultCh
//
// Not called if the process has error upstream (see pipe_executor_splitter.go)
func (ctx *PartitionWriterTransformationPipe) Done() error {

	// Write to db the jets_partition and nodeId of this partition w/ session_id
	stmt := `INSERT INTO jetsapi.compute_pipes_partitions_registry 
	  (session_id, step_id, jets_partition) 
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`
	if _, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId,
		ctx.spec.OutputChannel.WriteStepId, ctx.jetsPartitionLabel); err != nil {
		return fmt.Errorf("error inserting in jetsapi.compute_pipes_partitions_registry table: %v", err)
	}

	return nil
}

// Always called, if error or not upstream
func (ctx *PartitionWriterTransformationPipe) Finally() {
	if ctx == nil || ctx.s3DeviceManager == nil {
		return
	}
	// Flush the current partition
	if ctx.currentDeviceCh != nil {
		close(ctx.currentDeviceCh)
		ctx.currentDeviceCh = nil
		ctx.totalRowCount += ctx.partitionRowCount
	}
	// Send the total row count to ctx.copy2DeviceResultCh
	ctx.copy2DeviceResultCh <- ComputePipesResult{
		TableName:    fmt.Sprintf("jets_partition=%s", ctx.jetsPartitionLabel),
		CopyRowCount: ctx.totalRowCount,
		PartsCount:   int64(ctx.filePartitionNumber),
	}

	// Indicate to S3DeviceManager that we're done using it
	if ctx.s3DeviceManager.ClientsWg != nil {
		// fmt.Println("**@= Finally called, calling ClientsWg.Done()")
		ctx.s3DeviceManager.ClientsWg.Done()
	} else {
		log.Panicln("ERROR expecting ctx.s3DeviceManager.ClientsWg not nil")
	}
}

// Create a new jets_partition writer, the partition is identified by the jetsPartition
func (ctx *BuilderContext) NewPartitionWriterTransformationPipe(source *InputChannel, jetsPartitionKey interface{},
	outputCh *OutputChannel, copy2DeviceResultCh chan ComputePipesResult, spec *TransformationSpec) (*PartitionWriterTransformationPipe, error) {

	// Validation
	var err error
	if ctx.s3DeviceManager == nil {
		err = fmt.Errorf("error:  ctx.s3DeviceManager == nil in NewPartitionWriterTransformationPipe")
		log.Println(err)
		return nil, err
	}
	var parquetSchema *ParquetSchemaInfo
	config := spec.PartitionWriterConfig
	// log.Println("NewPartitionWriterTransformationPipe called for partition key:",jetsPartitionKey)
	if jetsPartitionKey == nil && config.JetsPartitionKey != nil {
		lc := 0
		for strings.Contains(*config.JetsPartitionKey, "$") && lc < 5 && ctx.env != nil {
			lc += 1
			for k, v := range ctx.env {
				value, ok := v.(string)
				if ok {
					*config.JetsPartitionKey = strings.ReplaceAll(*config.JetsPartitionKey, k, value)
				}
			}
		}
		jetsPartitionKey = *config.JetsPartitionKey
	}
	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewPartitionWriterTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}

	// close the underlying channel of outputCh since it will be replaced
	ctx.channelRegistry.CloseChannel(outputCh.name)

	// NOTE: parquet schema -- saving data as text
	// NOTE (future) To write parquet using typed data, get the data type from the schema provider.
	// Check if the DeviceWriterType is specified by the Schema Provider
	var sp SchemaProvider
	if spec.OutputChannel.SchemaProvider != "" {
		sp = ctx.schemaManager.GetSchemaProvider(spec.OutputChannel.SchemaProvider)
		if sp == nil {
			err = fmt.Errorf("schema provider %s not found (in NewPartitionWriterTransformationPipe)",
				spec.OutputChannel.SchemaProvider)
			log.Println(err)
			return nil, err
		}
	}
	// DeviceWriterType is required, may have been taken from schema provider in ValidatePipeSpecConfig
	if config.DeviceWriterType == "" {
		err = fmt.Errorf("unexpected error:  spec.DeviceWriterType == nil in NewPartitionWriterTransformationPipe")
		log.Println(err)
		return nil, err
	}

	// Verify that the device writer supports the file format
	switch config.DeviceWriterType {
	case "csv_writer":
		switch spec.OutputChannel.Format {
		case "csv", "headerless_csv", "xlsx", "headerless_xlsx":
		default:
			return nil, fmt.Errorf("error: csv_writer does not support file format '%s'", spec.OutputChannel.Format)
		}
	case "parquet_writer":
		switch spec.OutputChannel.Format {
		case "parquet", "parquet_select":
		default:
			return nil, fmt.Errorf("error: parquet_writer does not support file format '%s'", spec.OutputChannel.Format)
		}
	case "fixed_width_writer":
		switch spec.OutputChannel.Format {
		case "fixed_width":
		default:
			return nil, fmt.Errorf("error: fixed_width_writer does not support file format '%s'", spec.OutputChannel.Format)
		}
	}

	// Use the column specified from the output channel, if none are specified, look at the schema provider
	// Note this does not apply to output channel with dynamic columns since they have placeholder at config time
	if outputCh.config.HasDynamicColumns && config.DeviceWriterType == "parquet_writer" {
		err = fmt.Errorf("error: parquet writer is not supported with output_channel with dynamic columns")
		log.Println(err)
		return nil, err
	}
	if !outputCh.config.HasDynamicColumns {
		if len(outputCh.config.Columns) == 0 && sp != nil {
			outputCh.config.Columns = sp.ColumnNames()
		}
		if len(outputCh.config.Columns) == 0 {
			//*TODO Cannot use parquet with output_channel with dynamic columns, need to defer the construction of the schema
			return nil, fmt.Errorf("error: output channel '%s' have no columns specified", outputCh.name)
		}
		//*TODO Cannot use parquet with output_channel with dynamic columns, need to defer the construction of the schema
		switch config.DeviceWriterType {
		case "parquet_writer":
			if spec.OutputChannel.UseInputParquetSchema {
				parquetSchema = ctx.inputParquetSchema
			} else {
				var buf strings.Builder
				buf.WriteString(fmt.Sprintf("message %s {\n", spec.OutputChannel.Name))
				for i := range outputCh.config.Columns {
					buf.WriteString(fmt.Sprintf("optional binary %s (UTF8);\n", outputCh.config.Columns[i]))
				}
				buf.WriteString("}\n")
				var compression string
				switch spec.OutputChannel.Compression {
				case "snappy":
					compression = parquet.CompressionCodec_SNAPPY.String()
				case "none":
					compression = parquet.CompressionCodec_UNCOMPRESSED.String()
				default:
					compression = parquet.CompressionCodec_UNCOMPRESSED.String()
				}
				parquetSchema = &ParquetSchemaInfo{
					Schema:      buf.String(),
					Compression: compression,
				}
			}
		}
	}

	jetsPartitionLabel := MakeJetsPartitionLabel(jetsPartitionKey)
	var baseOutputPath string
	var externalBucket string
	switch spec.OutputChannel.Type {
	case "stage":
		// s3 partitioning, write the partition files in the JetStore's stage path defined by the env var JETS_s3_STAGE_PREFIX
		// baseOutputPath structure is: <JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reduce01/jets_partition=22p/
		baseOutputPath = fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
			jetsS3StagePrefix, ctx.processName, ctx.sessionId, spec.OutputChannel.WriteStepId, jetsPartitionLabel)
	case "output":
		switch {
		case len(spec.OutputChannel.Bucket) > 0:
			if spec.OutputChannel.Bucket != "jetstore_bucket" {
				externalBucket = spec.OutputChannel.Bucket
			}
		case sp != nil && spec.OutputChannel.OutputLocation == "jetstore_s3_input":
			externalBucket = sp.Bucket()
		}
		if len(externalBucket) > 0 {
			externalBucket = doSubstitution(externalBucket, "", "", ctx.env)
		}
		if len(spec.OutputChannel.KeyPrefix) > 0 {
			baseOutputPath = doSubstitution(spec.OutputChannel.KeyPrefix, jetsPartitionLabel,
				spec.OutputChannel.OutputLocation, ctx.env)
		} else {
			baseOutputPath = doSubstitution("$PATH_FILE_KEY", jetsPartitionLabel,
				spec.OutputChannel.OutputLocation, ctx.env)
		}
	}

	// Check if we limit the file part size
	var rowCountPerPartition int64
	if config.PartitionSize > 0 {
		rowCountPerPartition = int64(config.PartitionSize)
	}

	// Create a local temp dir to save the file partition for writing to s3
	localTempDir, err2 := os.MkdirTemp("", "jets_partition")
	if err2 != nil {
		err = fmt.Errorf("while creating temp dir (in NewPartitionWriterTransformationPipe) %v", err2)
		log.Println(err)
		return nil, err
	}

	// Register as a client to S3DeviceManager
	if ctx.s3DeviceManager.ClientsWg != nil {
		ctx.s3DeviceManager.ClientsWg.Add(1)
	} else {
		log.Panicln("ERROR Expecting ClientsWg not nil")
	}

	return &PartitionWriterTransformationPipe{
		cpConfig:             ctx.cpConfig,
		dbpool:               ctx.dbpool,
		spec:                 spec,
		schemaProvider:       sp,
		deviceWriterType:     config.DeviceWriterType,
		externalBucket:       externalBucket,
		baseOutputPath:       &baseOutputPath,
		localTempDir:         &localTempDir,
		jetsPartitionLabel:   jetsPartitionLabel,
		rowCountPerPartition: rowCountPerPartition,
		samplingRate:         config.SamplingRate,
		samplingMaxCount:     config.SamplingMaxCount,
		outputCh:             outputCh,
		parquetSchema:        parquetSchema,
		columnEvaluators:     columnEvaluators,
		errCh:                ctx.errCh,
		doneCh:               ctx.done,
		copy2DeviceResultCh:  copy2DeviceResultCh,
		sessionId:            ctx.sessionId,
		nodeId:               ctx.nodeId,
		s3DeviceManager:      ctx.s3DeviceManager,
		env:                  ctx.env,
	}, nil
}

func doSubstitution(value, jetsPartitionLabel string, s3OutputLocation string,
	env map[string]interface{}) string {
	lc := 0
	for strings.Contains(value, "$") && lc < 5 && env != nil {
		lc += 1
		for key, v := range env {
			vv, ok := v.(string)
			if ok {
				value = strings.ReplaceAll(value, key, vv)
			}
			if !strings.Contains(value, "$") {
				break
			}
		}
		value = strings.ReplaceAll(value, "$CURRENT_PARTITION_LABEL", jetsPartitionLabel)
	}
	if s3OutputLocation == "jetstore_s3_output" {
		value = strings.ReplaceAll(value, jetsS3InputPrefix, jetsS3OutputPrefix)
	}
	return value
}

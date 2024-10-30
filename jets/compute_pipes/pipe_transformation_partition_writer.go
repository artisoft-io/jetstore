package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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
	columnNames          []string
	deviceWriterType     string
	localTempDir         *string
	baseOutputPath       *string
	jetsPartitionLabel   string
	rowCountPerPartition int64
	partitionRowCount    int64
	totalRowCount        int64
	filePartitionNumber  int
	outputCh             *OutputChannel
	currentDeviceCh      chan []interface{}
	parquetSchema        []string
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
func (ctx *PartitionWriterTransformationPipe) apply(input *[]interface{}) error {
	var err error
	if input == nil {
		err = fmt.Errorf("error: input record is nil in PartitionWriterTransformationPipe.apply")
		log.Println(err)
		return err
	}

	// Check if partition is complete, if so close current output channel and start a new one
	if ctx.rowCountPerPartition > 0 && ctx.partitionRowCount >= ctx.rowCountPerPartition {
		close(ctx.currentDeviceCh)
		ctx.currentDeviceCh = nil
		ctx.totalRowCount += ctx.partitionRowCount
		ctx.partitionRowCount = 0
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
			partitionFileName = doSubstitution(ctx.spec.OutputChannel.FileName, ctx.jetsPartitionLabel, ctx.env)
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
				config:  &ChannelSpec{Name: fmt.Sprintf("input channel for partition_writer for %s", partitionFileName)},
			},
			spec:           ctx.spec,
			schemaProvider: ctx.schemaProvider,
			columnNames:    ctx.columnNames,
			outputCh:       ctx.outputCh,
			parquetSchema:  ctx.parquetSchema,
			localTempDir:   ctx.localTempDir,
			s3BasePath:     ctx.baseOutputPath,
			fileName:       &partitionFileName,
			doneCh:         ctx.doneCh,
			errCh:          ctx.errCh,
		}
		ctx.s3DeviceManager.ClientsWg.Add(1)
		go func() {
			defer ctx.s3DeviceManager.ClientsWg.Done()
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
	// currentValue is either the input row or a new row based on ctx.NewRecord flag
	var currentValues *[]interface{}
	if ctx.spec.NewRecord {
		v := make([]interface{}, len(ctx.columnNames))
		currentValues = &v
		// initialize the column evaluators
		for i := range ctx.columnEvaluators {
			ctx.columnEvaluators[i].initializeCurrentValue(currentValues)
		}
	} else {
		currentValues = input
	}

	// apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err = ctx.columnEvaluators[i].update(currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from partition_writer: %v", err)
			log.Println(err)
			return err
		}
	}
	// Notify the column evaluator that we're done
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].done(currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from partition_writer: %v", err)
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.columnNames) {
			*currentValues = (*currentValues)[:len(ctx.columnNames)]
		}
	}
	// Send the result to output
	select {
	case ctx.outputCh.channel <- *currentValues:
	case <-ctx.doneCh:
		log.Printf("PartitionWriterTransformationPipe writing to '%s' interrupted", ctx.outputCh.config.Name)
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
func (ctx *PartitionWriterTransformationPipe) done() error {
	// Flush the current partition
	if ctx.currentDeviceCh != nil {
		close(ctx.currentDeviceCh)
		ctx.currentDeviceCh = nil
		ctx.totalRowCount += ctx.partitionRowCount
	}

	// Write to db the jets_partition and nodeId of this partition w/ session_id
	stmt := `INSERT INTO jetsapi.compute_pipes_partitions_registry 
	  (session_id, step_id, jets_partition) 
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`
	if _, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId,
		ctx.spec.OutputChannel.WriteStepId, ctx.jetsPartitionLabel); err != nil {
		return fmt.Errorf("error inserting in jetsapi.compute_pipes_partitions_registry table: %v", err)
	}

	// Send the total row count to ctx.copy2DeviceResultCh
	ctx.copy2DeviceResultCh <- ComputePipesResult{
		TableName:    fmt.Sprintf("jets_partition=%s", ctx.jetsPartitionLabel),
		CopyRowCount: ctx.totalRowCount,
		PartsCount:   int64(ctx.filePartitionNumber),
	}
	return nil
}

// Always called, if error or not upstream
func (ctx *PartitionWriterTransformationPipe) finally() {
	if ctx == nil || ctx.s3DeviceManager == nil {
		return
	}
	// Indicate to S3DeviceManager that we're done using it
	if ctx.s3DeviceManager.ClientsWg != nil {
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
	var parquetSchema []string
	// log.Println("NewPartitionWriterTransformationPipe called for partition key:",jetsPartitionKey)
	if jetsPartitionKey == nil && spec.JetsPartitionKey != nil {
		jetsPartitionKey = *spec.JetsPartitionKey
	}
	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewPartitionWriterTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}

	// close the underlying channel of outputCh since it will be replaced
	ctx.channelRegistry.CloseChannel(outputCh.config.Name)

	// NOTE: parquet schema -- saving data as text
	// NOTE (future) To write parquet using typed data, get the data type from the schema provider.
	// Check if the DeviceWriterType is specified by the Schema Provider
	var sp SchemaProvider
	if spec.OutputChannel.SchemaProvider != "" {
		sp = ctx.schemaManager.GetSchemaProvider(spec.OutputChannel.SchemaProvider)
		if sp == nil {
			err = fmt.Errorf("schema provider %s not found (in NewPartitionWriterTransformationPipe)", spec.OutputChannel.SchemaProvider)
			log.Println(err)
			return nil, err
		}
	}
	var deviceWriterType string
	if spec.DeviceWriterType != nil {
		deviceWriterType = *spec.DeviceWriterType
	}
	if deviceWriterType == "" {
		switch sp.InputFormat() {
		case "csv", "headerless_csv":
			deviceWriterType = "csv_writer"
		case "parquet", "parquet_select":
			deviceWriterType = "parquet_writer"
		case "fixed_width":
			deviceWriterType = "fixed_width_writer"
		default:
			err = fmt.Errorf("unsupported output file format: %s (in NewPartitionWriterTransformationPipe)", sp.InputFormat())
			log.Println(err)
			return nil, err
		}
	}
	var columnNames []string
	if sp != nil {
		columnNames = sp.ColumnNames()
	}
	// Note: column may or may not be provided by the schema provider
	if len(columnNames) == 0 {
		columnNames = outputCh.config.Columns
	}

	switch deviceWriterType {
	case "parquet_writer":
		parquetSchema = make([]string, len(columnNames))
		for i := range columnNames {
			parquetSchema[i] = fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY", columnNames[i])
		}
	}

	jetsPartitionLabel := MakeJetsPartitionLabel(jetsPartitionKey)
	var baseOutputPath string
	switch spec.OutputChannel.Type {
	case "stage":
		// s3 partitioning, write the partition files in the JetStore's stage path defined by the env var JETS_s3_STAGE_PREFIX
		// baseOutputPath structure is: <JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reduce01/jets_partition=22p/
		baseOutputPath = fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
			jetsS3StagePrefix, ctx.processName, ctx.sessionId, spec.OutputChannel.WriteStepId, jetsPartitionLabel)
	case "output":
		baseOutputPath = doSubstitution(spec.OutputChannel.KeyPrefix, jetsPartitionLabel, ctx.env)
	default:
		return nil, fmt.Errorf("error: unknown output channel type for partition_writer: %s", spec.OutputChannel.Type)
	}

	// Check if we limit the file part size
	var rowCountPerPartition int64
	if spec.PartitionSize != nil && *spec.PartitionSize > 0 {
		rowCountPerPartition = int64(*spec.PartitionSize)
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
		columnNames:          columnNames,
		deviceWriterType:     deviceWriterType,
		baseOutputPath:       &baseOutputPath,
		localTempDir:         &localTempDir,
		jetsPartitionLabel:   jetsPartitionLabel,
		rowCountPerPartition: rowCountPerPartition,
		outputCh:             outputCh,
		parquetSchema:        parquetSchema,
		columnEvaluators:     columnEvaluators,
		doneCh:               ctx.done,
		copy2DeviceResultCh:  copy2DeviceResultCh,
		sessionId:            ctx.sessionId,
		nodeId:               ctx.nodeId,
		s3DeviceManager:      ctx.s3DeviceManager,
		env:                  ctx.env,
	}, nil
}

func doSubstitution(value, jetsPartitionLabel string, env map[string]interface{}) string {
	if strings.Contains(value, "$") {
		for key, v := range env {
			vv, ok := v.(string)
			if ok {
				value = strings.ReplaceAll(value, key, vv)
			}
			if !strings.Contains(value, "$") {
				break
			}
		}
		if strings.Contains(value, "$") {
			value = strings.ReplaceAll(value, "$CURRENT_PARTITION_LABEL", jetsPartitionLabel)
		}
	}
	value = strings.ReplaceAll(value, jetsS3InputPrefix, jetsS3OutputPrefix)
	return value
}

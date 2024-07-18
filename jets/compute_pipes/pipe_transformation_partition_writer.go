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

func init() {
	jetsS3InputPrefix = os.Getenv("JETS_s3_INPUT_PREFIX")
	jetsS3StagePrefix = os.Getenv("JETS_s3_STAGE_PREFIX")
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
	cpConfig                   *ComputePipesConfig
	dbpool                     *pgxpool.Pool
	spec                       *TransformationSpec
	localTempDir               *string
	baseOutputPath             *string
	jetsPartitionLabel         string
	rowCountPerPartition       int64
	partitionRowCount          int64
	totalRowCount              int64
	filePartitionNumber        int
	outputCh                   *OutputChannel
	currentDeviceCh            chan []interface{}
	parquetSchema              []string
	columnEvaluators           []TransformationColumnEvaluator
	doneCh                     chan struct{}
	errCh                      chan error
	copy2DeviceResultCh        chan<- ComputePipesResult
	sessionId                  string
	nodeId                     int
	s3DeviceManager            *S3DeviceManager
}

func MakeJetsPartitionLabel(jetsPartitionKey interface{}) string {
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
		isParquetWriter := ctx.spec.DeviceWriterType == nil || *ctx.spec.DeviceWriterType == "parquet_writer"
		fileEx := "csv"
		if isParquetWriter {
			fileEx = "parquet"
		}
		partitionFileName := fmt.Sprintf("part%03d-%07d.%s", ctx.nodeId, ctx.filePartitionNumber, fileEx)
		s3DeviceWriter := &S3DeviceWriter{
			s3DeviceManager: ctx.s3DeviceManager,
			source: &InputChannel{
				channel: ctx.currentDeviceCh,
				columns: ctx.outputCh.columns,
				config:  &ChannelSpec{Name: fmt.Sprintf("input channel for partition_writer for %s", partitionFileName)},
			},
			parquetSchema: ctx.parquetSchema,
			localTempDir:  ctx.localTempDir,
			s3BasePath:    ctx.baseOutputPath,
			fileName:      &partitionFileName,
			doneCh:        ctx.doneCh,
			errCh:         ctx.errCh,
		}
		if isParquetWriter {
			go s3DeviceWriter.WriteParquetPartition()
		} else {
			go s3DeviceWriter.WriteCsvPartition()
		}
	}

	currentValues := make([]interface{}, len(ctx.outputCh.config.Columns))
	// initialize the column evaluators
	for i := range ctx.columnEvaluators {
		ctx.columnEvaluators[i].initializeCurrentValue(&currentValues)
	}
	// apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err = ctx.columnEvaluators[i].update(&currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from partition_writer: %v", err)
			log.Println(err)
			return err
		}
	}
	// Notify the column evaluator that we're done
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].done(&currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from partition_writer: %v", err)
		}
	}
	// Send the result to output
	select {
	case ctx.outputCh.channel <- currentValues:
	case <-ctx.doneCh:
		log.Printf("PartitionWriterTransformationPipe writing to '%s' interrupted", ctx.outputCh.config.Name)
		return nil
	}
	ctx.partitionRowCount += 1

	return nil
}

// Done writing the splitter partition
//   - Close the current ctx.currentDeviceCh to flush the data, update totalRowCount
//   - Write to db the nodeId of this partition: session_id, file_key, shard
//     Here the file_key is ctx.baseOutputPath
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
	stepId := "reducing0"
	if ctx.spec.StepId != nil {
		stepId = *ctx.spec.StepId
	}
	stmt := `INSERT INTO jetsapi.compute_pipes_partitions_registry 
	  (session_id, step_id, file_key, jets_partition, shard_id) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT ON CONSTRAINT compute_pipes_partitions_registry_unique_cstraint_v5 
		DO UPDATE SET (step_id, jets_partition) =	(EXCLUDED.step_id, EXCLUDED.jets_partition)`
	_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, stepId, *ctx.baseOutputPath,
		ctx.jetsPartitionLabel, ctx.nodeId)
	if err != nil {
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
	// Indicate to S3DeviceManager that we're done using it
	ctx.s3DeviceManager.ClientsWg.Done()
}

// Create a new jets_partition writer, the partition is identified by the jetsPartition
func (ctx *BuilderContext) NewPartitionWriterTransformationPipe(source *InputChannel, jetsPartitionKey interface{},
	outputCh *OutputChannel, copy2DeviceResultCh chan ComputePipesResult, spec *TransformationSpec) (*PartitionWriterTransformationPipe, error) {

	// Prepare the column evaluators
	var err error
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewPartitionWriterTransformationPipe) %v", err)
			fmt.Println(err)
			return nil, err
		}
	}

	// close the underlying channel of outputCh since it will be replaced
	ctx.channelRegistry.CloseChannel(outputCh.config.Name)

	// Prepare the parquet schema -- saving rows based on specified data type
	schema := make(map[string]string)
	if spec.DataSchema != nil {
		for i := range *spec.DataSchema {
			schema[(*spec.DataSchema)[i].Columns] = (*spec.DataSchema)[i].RdfType
		}
	}
	parquetSchema := make([]string, len(outputCh.config.Columns))
	if spec.DeviceWriterType == nil || *spec.DeviceWriterType == "parquet_writer" {
		for i := range outputCh.config.Columns {
			//*$1
			// rdfType := schema[outputCh.config.Columns[i]]
			// switch rdfType {
			// case "int", "int32":
			// 	parquetSchema[i] = fmt.Sprintf("name=%s, type=INT32, repetitiontype=OPTIONAL",	outputCh.config.Columns[i])
			// case "long", "int64", "timestamp":
			// 	parquetSchema[i] = fmt.Sprintf("name=%s, type=INT64, repetitiontype=OPTIONAL",	outputCh.config.Columns[i])
			// case "double", "float64":
			// 	parquetSchema[i] = fmt.Sprintf("name=%s, type=DOUBLE, repetitiontype=OPTIONAL",	outputCh.config.Columns[i])
			// default:
			// 	parquetSchema[i] = fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY, repetitiontype=OPTIONAL",
			// 	outputCh.config.Columns[i])
			// }
			parquetSchema[i] = fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY",
				outputCh.config.Columns[i])
		}
	}

	// s3 partitioning, write the partition files in the JetStore's stage path defined by the env var JETS_s3_STAGE_PREFIX
	// baseOutputPath structure is: <JETS_s3_STAGE_PREFIX>/<original file key folder>/session_id=123456789/jets_partition=22p/
	// The original file key folder is prepended with the jets partition (it replace the first path component with the partion number)
	p := ctx.env["$FILE_KEY_FOLDER"].(string)
	// Write the partition files in the jetstore stage folder of s3
	p = strings.Replace(p, jetsS3InputPrefix, jetsS3StagePrefix, 1)
	if spec.FilePathSubstitutions != nil {
		for _, ps := range *spec.FilePathSubstitutions {
			p = strings.Replace(p, ps.Replace, ps.With, 1)
		}
	}
	// if $FILE_KEY_FOLDER is a input partition, i.e. we're reducing a second time
	// remove the /session_id... from path
	ipos := strings.Index(p, "/session_id=")
	if ipos > 0 {
		p = p[:ipos]
	}
	session_id := ctx.SessionId()
	jetsPartitionLabel := MakeJetsPartitionLabel(jetsPartitionKey)
	baseOutputPath := fmt.Sprintf("%s/session_id=%s/jets_partition=%s", p, session_id, jetsPartitionLabel)

	// Check if we limit the file part size
	var rowCountPerPartition int64
	if spec.PartitionSize != nil && *spec.PartitionSize > 0 {
		rowCountPerPartition = int64(*spec.PartitionSize)
	}

	// Create a local temp dir to save the file partition for writing to s3
	localTempDir, err2 := os.MkdirTemp("", "jets_partition")
	if err2 != nil {
		err = fmt.Errorf("while creating temp dir (in NewPartitionWriterTransformationPipe) %v", err2)
		fmt.Println(err)
		return nil, err
	}

	// Register as a client to S3DeviceManager
	ctx.s3DeviceManager.ClientsWg.Add(1)

	return &PartitionWriterTransformationPipe{
		cpConfig:                   ctx.cpConfig,
		dbpool:                     ctx.dbpool,
		spec:                       spec,
		baseOutputPath:             &baseOutputPath,
		localTempDir:               &localTempDir,
		jetsPartitionLabel:         jetsPartitionLabel,
		rowCountPerPartition:       rowCountPerPartition,
		outputCh:                   outputCh,
		parquetSchema:              parquetSchema,
		columnEvaluators:           columnEvaluators,
		doneCh:                     ctx.done,
		copy2DeviceResultCh:        copy2DeviceResultCh,
		sessionId:                  session_id,
		nodeId:                     ctx.nodeId,
		s3DeviceManager:            ctx.s3DeviceManager,
	}, nil
}

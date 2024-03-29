package compute_pipes

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

// partition_writer TransformationSpec implementing PipeTransformationEvaluator interface
// partition_writer: bundle input records into fixed-sized partitions.
// The outputCh collumn spec correspond to the intermediate channel to the actual
// device writer.
// currentDeviceCh is the physical ch to the device writer.
// If the TransformationSpec.PartitionSize is nil or 0 then there is a single partion.
// Replace the underlying channel to have a buffered channel and create one for each partition.
// Currently supporting writing to s3 jetstore input path

type PartitionWriterTransformationPipe struct {
	dbpool               *pgxpool.Pool
	spec                 *TransformationSpec
	splitterKey          *string
	localTempDir         *string
	baseOutputPath       *string
	splitterShardId      int
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
	bucketName           string
	regionName           string
	sessionId            string
}

// Implementing interface PipeTransformationEvaluator
func (ctx *PartitionWriterTransformationPipe) apply(input *[]interface{}) error {
	var err error
	doRowSize := false
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

		// Check if we need the row size
		if ctx.spec.PartitionSize != nil && *ctx.spec.PartitionSize > 0 {
			doRowSize = true
		}

		// Start the device writter for the partition
		ctx.filePartitionNumber += 1
		partitionFileName := fmt.Sprintf("part%04d.parquet", ctx.filePartitionNumber)
		s3DeviceWriter := &S3DeviceWriter{
			source: &InputChannel{
				channel: ctx.currentDeviceCh,
				columns: ctx.outputCh.columns,
				config:  &ChannelSpec{Name: fmt.Sprintf("input channel for partition_writer for %s", partitionFileName)},
			},
			parquetSchema: ctx.parquetSchema,
			localTempDir:  ctx.localTempDir,
			s3BasePath:    ctx.baseOutputPath,
			fileName:      &partitionFileName,
			bucketName:    ctx.bucketName,
			regionName:    ctx.regionName,
			doneCh:        ctx.doneCh,
			errCh:         ctx.errCh,
		}
		go s3DeviceWriter.WritePartition()
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
	if doRowSize {
		// Compute the ctx.rowCountPerPartition based on the size of the currentValues
		rowSize := 0
		for i := range currentValues {
			rowSize += len(currentValues[i].(string))
		}
		if rowSize > 0 {
			ctx.rowCountPerPartition = int64(*ctx.spec.PartitionSize / rowSize)
		}
		// fmt.Println("**!@@ partition_writer: splitterShardId", ctx.splitterShardId, "filePartitionNumber", ctx.filePartitionNumber, "rowSize:", rowSize, "rowCountPerPartition:", ctx.rowCountPerPartition)
		doRowSize = false
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
//   - Write to db the shardId of this partition: session_id, file_key, shard
//     Here the file_key is ctx.baseOutputPath
//   - write the 0-byte sentinel file (take the file name from env JETS_SENTINEL_FILE_NAME)
//   - Send the total row count to ctx.copy2DeviceResultCh
func (ctx *PartitionWriterTransformationPipe) done() error {
	// Flush the current partition
	if ctx.currentDeviceCh != nil {
		close(ctx.currentDeviceCh)
		ctx.currentDeviceCh = nil
		ctx.totalRowCount += ctx.partitionRowCount
	}

	// Write to db the shardId of this partition: session_id, file_key, shard
	stmt := `INSERT INTO jetsapi.compute_pipes_shard_registry (session_id, file_key, shard_id) 
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, *ctx.baseOutputPath, ctx.splitterShardId)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.compute_pipes_shard_registry table: %v", err)
	}

	// Write the 0-byte sentinel file (take the file name from env JETS_SENTINEL_FILE_NAME)
	// Copy file to s3 location
	sentinelFileName := os.Getenv("JETS_SENTINEL_FILE_NAME")
	if len(sentinelFileName) == 0 {
		sentinelFileName = "_DONE"
	}
	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, sentinelFileName)
	fileHd, err2 := os.OpenFile(tempFileName, os.O_RDWR|os.O_CREATE, 0644)
	if err2 != nil {
		err = fmt.Errorf("while creating sentinel file to copy to s3: %v", err2)
		log.Println(err)
		return err
	}
	defer func() {
		fileHd.Close()
		os.Remove(tempFileName)
	}()
	s3FileName := fmt.Sprintf("%s/%s", *ctx.baseOutputPath, sentinelFileName)
	if err2 = awsi.UploadToS3(ctx.bucketName, ctx.regionName, s3FileName, fileHd); err != nil {
		err = fmt.Errorf("while copying sentinel to s3: %v", err)
		return err
	}

	// Send the total row count to ctx.copy2DeviceResultCh
	ctx.copy2DeviceResultCh <- ComputePipesResult{
		TableName:    *ctx.baseOutputPath,
		CopyRowCount: ctx.totalRowCount,
	}
	return nil
}

func (ctx *PartitionWriterTransformationPipe) finally() {
	close(ctx.copy2DeviceResultCh)
}

func (ctx *BuilderContext) NewPartitionWriterTransformationPipe(source *InputChannel, splitterKey *string,
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

	// Prepare the parquet schema -- saving rows as string
	parquetSchema := make([]string, len(outputCh.config.Columns))
	for i := range outputCh.config.Columns {
		parquetSchema[i] = fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY",
			outputCh.config.Columns[i])
	}

	// Partition name (jets_partition=keyHash) is the hash of splitterKey
	h := fnv.New64a()
	h.Write([]byte(*splitterKey))
	keyHash := h.Sum64()
	nbrShard := ctx.env["$NBR_SHARDS"].(int)
	splitterShardId := keyHash % uint64(nbrShard)

	p := ctx.env["$FILE_KEY_FOLDER"].(string)
	if spec.FilePathSubstitutions != nil {
		for _, ps := range *spec.FilePathSubstitutions {
			p = strings.Replace(p, ps.Replace, ps.With, 1)
		}
	}
	session_id := ctx.env["$SESSIONID"].(string)
	baseOutputPath := fmt.Sprintf("%s/session_id=%s/jets_partition=%d", p, session_id, keyHash)

	// Create a local temp dir to save the file partition for writing to s3
	localTempDir, err2 := os.MkdirTemp("", "jets_partition")
	if err2 != nil {
		err = fmt.Errorf("while creating temp dir (in NewPartitionWriterTransformationPipe) %v", err2)
		fmt.Println(err)
		return nil, err
	}

	return &PartitionWriterTransformationPipe{
		dbpool:              ctx.dbpool,
		spec:                spec,
		splitterKey:         splitterKey,
		baseOutputPath:      &baseOutputPath,
		localTempDir:        &localTempDir,
		splitterShardId:     int(splitterShardId),
		outputCh:            outputCh,
		parquetSchema:       parquetSchema,
		columnEvaluators:    columnEvaluators,
		doneCh:              ctx.done,
		copy2DeviceResultCh: copy2DeviceResultCh,
		bucketName:          os.Getenv("JETS_BUCKET"),
		regionName:          os.Getenv("JETS_REGION"),
		sessionId:           session_id,
	}, nil
}

package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
	"github.com/artisoft-io/jetstore/jets/awsi"
)

func (cpCtx *ComputePipesContext) ReadParquetFileV2(filePath *FileName, readBatchSize int64, sp SchemaProvider,
	castToRdfTxtTypeFncs []CastToRdfTxtFnc, inputSchemaCh chan<- ParquetSchemaInfo,
	computePipesInputCh chan<- []any) (int64, error) {

	var fileHd *os.File
	var inputColumns []string
	var err error
	samplingRate := int64(cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate)
	samplingMaxCount := int64(cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingMaxCount)

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	if nbrColumns > 0 {
		// Read specified columns
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)-len(cpCtx.AddionalInputHeaders)]
	}

	// Setup the parquet reader and get the arrow schema
	pqFileReader, err := file.NewParquetReader(fileHd)
	if err != nil {
		return 0, fmt.Errorf("while opening the parquet file reader for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}
	defer pqFileReader.Close()

	if readBatchSize == 0 {
		readBatchSize = 1024
	}
	reader, err := pqarrow.NewFileReader(pqFileReader, pqarrow.ArrowReadProperties{BatchSize: readBatchSize}, memory.NewGoAllocator())
	if err != nil {
		return 0, fmt.Errorf("while opening the pqarrow file reader for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}

	schema, err := reader.Schema()
	if err != nil {
		return 0, fmt.Errorf("while getting the arrow schema for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}

	// Check if we read only some rows
	var firstRowToRead, nbrRowsToRead int64
	if filePath.InFileKeyInfo.end > 0 {
		nbrRowsToRead = reader.ParquetReader().NumRows() / int64(cpCtx.CpConfig.ClusterConfig.ShardingInfo.NbrPartitions)
		firstRowToRead = int64(cpCtx.ComputePipesNodeArgs.NodeId) * nbrRowsToRead
	}
	if cpCtx.ComputePipesNodeArgs.NodeId == cpCtx.CpConfig.ClusterConfig.ShardingInfo.NbrPartitions-1 {
		nbrRowsToRead = reader.ParquetReader().NumRows() - firstRowToRead
	}
	// log.Println("*** The parquet file contains", reader.ParquetReader().NumRows(), "rows, reading from row", firstRowToRead, "reading", nbrRowsToRead, "rows")

	parquetSchemaInfo := NewParquetSchemaInfo(schema)
	// Save the parquet schema to s3 on request
	if inputSchemaCh != nil {

		// Make the schema avail to channel registry
		inputSchemaCh <- *parquetSchemaInfo

		if cpCtx.ComputePipesArgs.NodeId == 0 {
			// save schema info to s3
			schemaInfo, err := json.Marshal(parquetSchemaInfo)
			if err != nil {
				return 0, fmt.Errorf("while making json from parquet schema info: %v", err)
			}
			fileKey := fmt.Sprintf("%s/process_name=%s/session_id=%s/input_parquet_schema.json",
				jetsS3StagePrefix, cpCtx.ProcessName, cpCtx.SessionId)
			log.Printf("Saving parquet schema to: %s", fileKey)
			err = awsi.UploadBufToS3(fileKey, schemaInfo)
			if err != nil {
				return 0, fmt.Errorf("while uploading parquet schema info to s3: %v", err)
			}
		}
	}
	// Make the list of column idx to read
	var columnIndices []int
	if len(inputColumns) > 0 {
		columnIndices = make([]int, 0, len(inputColumns))
		for _, c := range inputColumns {
			idx := schema.FieldIndices(c)
			if len(idx) > 0 {
				columnIndices = append(columnIndices, idx[0])
			} else {
				return 0, fmt.Errorf("error: column %s is not found in the parquet schema of '%s' (LoadFiles)", c, cpCtx.FileKey)
			}
		}
	}
	// Get a Record Reader
	recordReader, err := reader.GetRecordReader(context.TODO(), columnIndices, nil)
	if err != nil {
		return 0, fmt.Errorf("while creating parquet record reader: %v", err)
	}
	defer recordReader.Release()

	if nbrColumns == 0 {
		// Get the columns from the schema
		for _, fi := range schema.Fields() {
			inputColumns = append(inputColumns, fi.Name)
		}
		nbrColumns = len(inputColumns)
	}

	// Prepare the extended columns from partfile_key_component
	var extColumns []string
	if len(cpCtx.PartFileKeyComponents) > 0 {
		extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
		for i := range cpCtx.PartFileKeyComponents {
			result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKeyInfo.key)
			if len(result) > 0 {
				extColumns[i] = result[1]
			}
		}
	}

	// Determine if trim the columns
	trimColumns := false
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" && sp != nil {
		trimColumns = sp.TrimColumns()
	}

	var inputRowCount int64
	var currentRow int64
	var done bool
	for !done && recordReader.Next() {
		// read and put the rows into computePipesInputCh
		currentRow, inputRowCount, done, err = cpCtx.processRecord(computePipesInputCh, recordReader.Record(),
			nbrColumns, extColumns, trimColumns, castToRdfTxtTypeFncs,
			firstRowToRead, nbrRowsToRead, samplingRate, samplingMaxCount, currentRow, inputRowCount)
		if err != nil {
			return inputRowCount, err
		}
	}
	if recordReader.Err() == io.EOF {
		return inputRowCount, nil
	}
	return inputRowCount, recordReader.Err()
}

func (cpCtx *ComputePipesContext) processRecord(computePipesInputCh chan<- []any, arrowRecord arrow.Record,
	nbrColumns int, extColumns []string, trimColumns bool, castToRdfTxtTypeFncs []CastToRdfTxtFnc,
	firstRowToRead, nbrRowsToRead, samplingRate, samplingMaxCount, currentRow, inputRowCount int64) (int64, int64, bool, error) {
	defer arrowRecord.Release()
	var castFnc CastToRdfTxtFnc
	var errCol error
	if nbrRowsToRead > 0 && firstRowToRead > currentRow+arrowRecord.NumRows() {
		// skip this record
		// log.Println("*** SKIP Record of",arrowRecord.NumRows(),"rows")
		currentRow += arrowRecord.NumRows()
		return currentRow, inputRowCount, false, nil
	}
	// fmt.Println("*** The Arrow Record contains", arrowRecord.NumRows(), "rows")
	for irow := range int(arrowRecord.NumRows()) {
		if nbrRowsToRead > 0 {
			switch {
			case firstRowToRead > currentRow:
				currentRow++
				continue
			case currentRow > firstRowToRead+nbrRowsToRead-1:
				// we're done
				return currentRow, inputRowCount, true, nil
			}
		}
		currentRow++
		// Build a record and send it to computePipesInputCh
		cpCtx.SamplingCount += 1
		if samplingRate > 0 && int64(cpCtx.SamplingCount) < samplingRate {
			continue
		}
		if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
			return currentRow, inputRowCount, false, nil
		}
		cpCtx.SamplingCount = 0
		record := make([]any, nbrColumns, nbrColumns+len(cpCtx.AddionalInputHeaders))
		for jcol, col := range arrowRecord.Columns() {
			if col.IsValid(irow) {
				if castToRdfTxtTypeFncs != nil {
					castFnc = castToRdfTxtTypeFncs[jcol]
				}
				record[jcol], errCol = ConvertWithSchemaV1(irow, col, trimColumns, castFnc)
				if errCol != nil {
					return currentRow, inputRowCount, false, fmt.Errorf("error while reading input records (ReadParquetFile): %v", errCol)
				}
			}
		}
		// Add the columns from the partfile_key_component
		if len(extColumns) > 0 {
			// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
			for i := range extColumns {
				record[nbrColumns+i] = extColumns[i]
			}
		}
		// Add placeholders for the additional input headers/columns
		if len(cpCtx.AddionalInputHeaders) > 0 {
			for range cpCtx.AddionalInputHeaders {
				record = append(record, nil)
			}
		}

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return currentRow, inputRowCount, false, ErrKillSwitch
		}

		// Send out the record
		// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
		// log.Println("*** INPUT RECORD:",record)
		select {
		case computePipesInputCh <- record:
		case <-cpCtx.Done:
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loading input row from file interrupted")
			return currentRow, inputRowCount, true, nil
		}
		inputRowCount += 1
	}
	return currentRow, inputRowCount, false, nil
}

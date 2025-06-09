package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
	"github.com/artisoft-io/jetstore/jets/awsi"
)

func (cpCtx *ComputePipesContext) ReadParquetFileV2(filePath *FileName, saveParquetSchema bool, sp SchemaProvider,
	castToRdfTxtTypeFncs []CastToRdfTxtFnc, inputSchemaCh chan<- ParquetSchemaInfo,
	computePipesInputCh chan<- []any) (int64, error) {

	var fileHd *os.File
	var inputColumns []string
	var err error
	samplingRate := cpCtx.CpConfig.PipesConfig[0].InputChannel.SamplingRate
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
	// Read specified columns
	inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)-len(cpCtx.AddionalInputHeaders)]

	// Setup the parquet reader and get the arrow schema
	pqFileReader, err := file.NewParquetReader(fileHd)
	if err != nil {
		return 0, fmt.Errorf("while opening the parquet file reader for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}
	defer pqFileReader.Close()

	reader, err := pqarrow.NewFileReader(pqFileReader, pqarrow.ArrowReadProperties{BatchSize: 1024}, memory.NewGoAllocator())
	if err != nil {
		return 0, fmt.Errorf("while opening the pqarrow file reader for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}

	schema, err := reader.Schema()
	if err != nil {
		return 0, fmt.Errorf("while getting the arrow schema for '%s' (LoadFiles): %v", filePath.LocalFileName, err)
	}

	fmt.Println("*** The reader schema", schema)
	fmt.Println("*** The file contains", reader.ParquetReader().NumRows(), "rows")

	parquetSchemaInfo := NewParquetSchemaInfo(schema)
	// Save the parquet schema to s3 on request
	if saveParquetSchema {

		// Make the schema avail to channel registry
		inputSchemaCh <- *parquetSchemaInfo
		close(inputSchemaCh)

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
	var record []any
	var errCol error
	var castFnc CastToRdfTxtFnc
	for recordReader.Next() {
		// read and put the rows into computePipesInputCh
		err = nil
		arrowRecord := recordReader.Record()
		for irow := range int(arrowRecord.NumRows()) {
			// Build a record and send it to computePipesInputCh
			cpCtx.SamplingCount += 1
			if samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
			if samplingMaxCount > 0 && inputRowCount >= samplingMaxCount {
				recordReader.Release()
				return inputRowCount, nil
			}
			cpCtx.SamplingCount = 0
			record = make([]any, nbrColumns, nbrColumns+len(cpCtx.AddionalInputHeaders))
			for jcol, col := range arrowRecord.Columns() {
				if col.IsValid(irow) {
					if castToRdfTxtTypeFncs != nil {
						castFnc = castToRdfTxtTypeFncs[jcol]
					}
					record[jcol], errCol = ConvertWithSchemaV1(irow, col, trimColumns, castFnc)
					if errCol != nil {
						return 0, fmt.Errorf("error while reading input records (ReadParquetFile): %v", errCol)
					}
				}
			}
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
			// Add placeholders for the additional input headers/columns
			if len(cpCtx.AddionalInputHeaders) > 0 {
				for range cpCtx.AddionalInputHeaders {
					record = append(record, nil)
				}
			}
		}
		recordReader.Release()

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
			return inputRowCount, ErrKillSwitch
		}

		// Send out the record
		// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh with",len(record),"columns")
		select {
		case computePipesInputCh <- record:
		case <-cpCtx.Done:
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loading input row from file interrupted")
			return inputRowCount, nil
		}
		inputRowCount += 1

	}
	return inputRowCount, recordReader.Err()
}

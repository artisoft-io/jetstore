package compute_pipes

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet"
	"github.com/apache/arrow/go/v17/parquet/compress"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

func MergeParquetPartitions(nrowsInRec int64, columns []string, fout io.Writer, FileNamesCh <-chan FileName, gotError func(error)) {
	var cpErr, err error
	var totalRowCount, count int64
	var writer *pqarrow.FileWriter
	for filePath := range FileNamesCh {
		// Process file
		writer, count, err = mergeFile(writer, nrowsInRec, columns, fout, &filePath)
		if err != nil {
			cpErr = fmt.Errorf("while processing file %s: %v", filePath.InFileKeyInfo.key, err)
			goto gotError
		}
		totalRowCount += count
	}

	log.Println("MergeParquetPartitions: Total Row Written to Parquet:", totalRowCount)
	if writer != nil {
		err = writer.Close()
		if err != nil {
			cpErr = fmt.Errorf("while closing parquet file: %v", err)
			goto gotError
		}
	}
	// All good!
	return
gotError:
	log.Printf("Got error in MergeParquetPartitions: %v", cpErr)
	gotError(cpErr)
}

func mergeFile(writer *pqarrow.FileWriter, nrowsInRec int64, columns []string, fout io.Writer, filePath *FileName) (*pqarrow.FileWriter, int64, error) {
	var fileHd *os.File
	var err error
	var totalRowCount int64

	fileHd, err = os.Open(filePath.LocalFileName)
	if err != nil {
		return nil, 0, fmt.Errorf("while opening temp file '%s' (mergeFile): %v", filePath.LocalFileName, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(filePath.LocalFileName)
	}()

	nbrColumns := len(columns)
	// Read specified columns if nbrColumns > 0
	// Setup the parquet reader and get the arrow schema
	pqFileReader, err := file.NewParquetReader(fileHd)
	if err != nil {
		return nil, 0, fmt.Errorf("while opening the parquet file reader for '%s' (mergeFile): %v", filePath.LocalFileName, err)
	}
	defer pqFileReader.Close()

	if nrowsInRec == 0 {
		nrowsInRec = 1024
	}
	reader, err := pqarrow.NewFileReader(pqFileReader, pqarrow.ArrowReadProperties{BatchSize: nrowsInRec}, memory.NewGoAllocator())
	if err != nil {
		return nil, 0, fmt.Errorf("while opening the pqarrow file reader for '%s' (mergeFile): %v", filePath.LocalFileName, err)
	}
	// log.Println("*** MERGING File with", pqFileReader.NumRows(),"rows")
	schema, err := reader.Schema()
	if err != nil {
		return nil, 0, fmt.Errorf("while getting the arrow schema for '%s' (mergeFile): %v", filePath.LocalFileName, err)
	}

	// Make the list of column idx to read
	var columnIndices []int
	if nbrColumns > 0 {
		columnIndices = make([]int, 0, nbrColumns)
		for _, c := range columns {
			idx := schema.FieldIndices(c)
			if len(idx) > 0 {
				columnIndices = append(columnIndices, idx[0])
			} else {
				return nil, 0, fmt.Errorf("error: column %s is not found in the parquet schema (MergeFIle)", c)
			}
		}
	}
	// Get a Record Reader
	recordReader, err := reader.GetRecordReader(context.TODO(), columnIndices, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("while creating parquet record reader: %v", err)
	}
	defer recordReader.Release()

	if writer == nil {
		pool := memory.NewGoAllocator()
		props := parquet.NewWriterProperties(parquet.WithCompression(compress.Codecs.Snappy), parquet.WithAllocator(pool),
			parquet.WithBatchSize(nrowsInRec), parquet.WithMaxRowGroupLength(nrowsInRec), parquet.WithCreatedBy("jetstore"))
		writer, err = pqarrow.NewFileWriter(schema, fout, props, pqarrow.DefaultWriterProps())
		if err != nil {
			return nil, 0, fmt.Errorf("while calling pqarrow.NewFileWriter: %v", err)
		}
	}
	for recordReader.Next() {
		arrowRecord := recordReader.Record()
		// log.Println("***    got record with", arrowRecord.NumRows(),"rows")
		totalRowCount += arrowRecord.NumRows()
		err = writer.Write(arrowRecord)
		arrowRecord.Release()
		if err != nil {
			return nil, 0, fmt.Errorf("while writing parquet record: %v", err)
		}
	}

	return writer, totalRowCount, nil
}

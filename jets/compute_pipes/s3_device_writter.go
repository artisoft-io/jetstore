package compute_pipes

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/golang/snappy"
	"github.com/xitongsys/parquet-go/source"
	"github.com/xitongsys/parquet-go/writer"
)

// S3DeviceWriter is the component that reads the rows comming the PipeTransformationEvaluator
// and writes them to a local file. This component delegates to S3DeviceManager to put the files in s3

type S3DeviceWriter struct {
	s3DeviceManager *S3DeviceManager
	source          *InputChannel
	parquetSchema   []string
	localTempDir    *string
	s3BasePath      *string
	fileName        *string
	spec            *TransformationSpec
	outputCh        *OutputChannel
	doneCh          chan struct{}
	errCh           chan error
}

func (ctx *S3DeviceWriter) WriteParquetPartition() {
	var cpErr, err error
	var pw *writer.CSVWriter
	var fw source.ParquetFile

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// fmt.Println("**&@@ WriteParquetPartition *1: fileName:", *ctx.fileName)
	if ctx.s3DeviceManager == nil {
		cpErr = fmt.Errorf("error: s3DeviceManager is nil")
		goto gotError
	}

	// open the local temp file for the parquet writer
	fw, err = delegate.NewLocalFileWriter(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening local parquet file for write %v", err)
		goto gotError
	}
	defer fw.Close()

	// Create the parquet writer with the provided schema
	pw, err = writer.NewCSVWriter(ctx.parquetSchema, fw, 4)
	if err != nil {
		fw.Close()
		cpErr = fmt.Errorf("while opening local parquet csv writer %v", err)
		goto gotError
	}

	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		//*$1
		// replace null with empty string, convert to string
		for i := range inRow {
			switch vv := inRow[i].(type) {
			case string:
			case nil:
				inRow[i] = ""
			default:
				inRow[i] = fmt.Sprintf("%v", vv)
			}
		}
		if err = pw.Write(inRow); err != nil {
			// fmt.Println("ERROR")
			// for i := range inRow {
			// 	fmt.Println(inRow[i], reflect.TypeOf(inRow[i]).Kind())
			// }
			// fmt.Println("ERROR")
			cpErr = fmt.Errorf("while writing row to local parquet file: %v", err)
			goto gotError
		}
	}

	if err = pw.WriteStop(); err != nil {
		cpErr = fmt.Errorf("while writing parquet stop (trailer): %v", err)
		goto gotError
	}
	// fmt.Println("**&@@ WriteParquetPartition: DONE writing local parquet file for fileName:", *ctx.fileName)
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		FileKey:       s3FileName,
		LocalFilePath: tempFileName,
	}:
	case <-ctx.doneCh:
		log.Printf("WriteParquetPartition: sending file to S3DeviceManager interrupted")
	}
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

func (ctx *S3DeviceWriter) WriteCsvPartition() {
	var cpErr, err error
	var fileHd *os.File
	var snWriter *snappy.Writer
	var csvWriter *csv.Writer

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// fmt.Println("**&@@ WriteCsvPartition *1: fileName:", *ctx.fileName)
	if ctx.s3DeviceManager == nil {
		cpErr = fmt.Errorf("error: s3DeviceManager is nil")
		goto gotError
	}

	// open the local temp file for the writer
	fileHd, err = os.Create(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening local file for write %v", err)
		goto gotError
	}
	defer fileHd.Close()

	switch ctx.spec.OutputChannel.Compression {
	case "none":
		csvWriter = csv.NewWriter(fileHd)
	case "snappy":
		// Open a snappy compressor
		snWriter = snappy.NewBufferedWriter(fileHd)
		// Open a csv writer
		csvWriter = csv.NewWriter(snWriter)
	default:
		cpErr = fmt.Errorf("error: unknown compression %s in WriteCsvPartition",
			ctx.spec.OutputChannel.Compression)
		goto gotError
	}
	if ctx.spec.WriteHeaders {
		if err = csvWriter.Write(ctx.outputCh.config.Columns); err != nil {
			cpErr = fmt.Errorf("while writing headers to local csv file: %v", err)
			goto gotError
		}
	}
	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		//*$1
		// replace null with empty string, convert to string
		row := make([]string, len(inRow))
		for i := range inRow {
			switch vv := inRow[i].(type) {
			case string:
				row[i] = vv
			case nil:
				row[i] = ""
			default:
				row[i] = fmt.Sprintf("%v", vv)
			}
		}
		if err = csvWriter.Write(row); err != nil {
			// fmt.Println("ERROR")
			// for i := range inRow {
			// 	fmt.Println(inRow[i], reflect.TypeOf(inRow[i]).Kind())
			// }
			// fmt.Println("ERROR")
			cpErr = fmt.Errorf("while writing row to local csv file: %v", err)
			goto gotError
		}
	}

	// fmt.Println("**&@@ WriteCsvPartition: DONE writing local csv file for fileName:", *ctx.fileName)
	csvWriter.Flush()
	if snWriter != nil {
		snWriter.Flush()
	}
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		FileKey:       s3FileName,
		LocalFilePath: tempFileName,
	}:
	case <-ctx.doneCh:
		log.Printf("WriteCsvPartition: sending file to S3DeviceManager interrupted")
	}
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

package compute_pipes

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
	"github.com/golang/snappy"
)

// S3DeviceWriter is the component that reads the rows comming the PipeTransformationEvaluator
// and writes them to a local file. This component delegates to S3DeviceManager to put the files in s3

type S3DeviceWriter struct {
	s3DeviceManager *S3DeviceManager
	source          *InputChannel
	schemaProvider  SchemaProvider
	columnNames     []string
	parquetSchema    *ParquetSchemaInfo
	localTempDir    *string
	externalBucket  *string
	s3BasePath      *string
	fileName        *string
	spec            *TransformationSpec
	outputCh        *OutputChannel
	doneCh          chan struct{}
	errCh           chan error
}

func (ctx *S3DeviceWriter) WriteParquetPartition() {
	var cpErr, err error
	var fout *os.File
	var  schemaDef *parquetschema.SchemaDefinition
	var fw *goparquet.FileWriter
	var codec parquet.CompressionCodec

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// fmt.Println("**&@@ WriteParquetPartition *1: fileName:", *ctx.fileName)
	if ctx.s3DeviceManager == nil {
		cpErr = fmt.Errorf("error: s3DeviceManager is nil")
		goto gotError
	}

	// open the local temp file for the parquet writer
	fout, err = os.OpenFile(tempFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		cpErr = fmt.Errorf("opening output file failed: %v", err)
		goto gotError
	}
	defer fout.Close()

	schemaDef, err = parquetschema.ParseSchemaDefinition(ctx.parquetSchema.Schema)
	if err != nil {
		cpErr = fmt.Errorf("parsing schema definition failed: %v", err)
		goto gotError
	}

	// Create the parquet writer with the provided schema
	codec, err = parquet.CompressionCodecFromString(ctx.parquetSchema.Compression)
	if err != nil {
		cpErr = fmt.Errorf("parsing compression codec failed: %v", err)
		goto gotError
	}

	fw = goparquet.NewFileWriter(fout,
		goparquet.WithCompressionCodec(codec),
		goparquet.WithSchemaDefinition(schemaDef),
		goparquet.WithCreator("jetstore"),
	)

	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		//*$1
		// Map data into types as per the schema definition
		//HERE
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
		rowData := make(map[string]any)
		for col, pos := range ctx.source.columns {
			rowData[col] = inRow[pos]
		}
		err = fw.AddData(rowData)
		if err != nil {
			// fmt.Println("ERROR")
			// for i := range inRow {
			// 	fmt.Println(inRow[i], reflect.TypeOf(inRow[i]).Kind())
			// }
			// fmt.Println("ERROR")
			cpErr = fmt.Errorf("while writing row to local parquet file: %v", err)
			goto gotError
		}
	}

	err = fw.Close()
	if err != nil {
		cpErr = fmt.Errorf("while writing parquet stop (trailer): %v", err)
		goto gotError
	}
	// fmt.Println("**&@@ WriteParquetPartition: DONE writing local parquet file for fileName:", *ctx.fileName)
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		ExternalBucket: *ctx.externalBucket,
		FileKey:        s3FileName,
		LocalFilePath:  tempFileName,
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
	var count int
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
	if ctx.schemaProvider != nil && ctx.schemaProvider.Delimiter() != 0 {
		csvWriter.Comma = ctx.schemaProvider.Delimiter()
	}
	if ctx.spec.OutputChannel.Format == "csv" {
		err = csvWriter.Write(ctx.columnNames)
		if err != nil {
			cpErr = fmt.Errorf("while writing headers to local csv file: %v", err)
			goto gotError
		}
	}
	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		count++
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

	// log.Printf("**&@@ WriteCsvPartition: DONE writing %d records to local csv file %s", count, *ctx.fileName)
	csvWriter.Flush()
	if snWriter != nil {
		snWriter.Flush()
	}
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		ExternalBucket: *ctx.externalBucket,
		FileKey:        s3FileName,
		LocalFilePath:  tempFileName,
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

func (ctx *S3DeviceWriter) WriteFixedWidthPartition() {
	var cpErr, err error
	var fileHd *os.File
	var snWriter *snappy.Writer
	var fwWriter *bufio.Writer
	var fwColumnsInfo *[]*FixedWidthColumn
	var columnPos []int
	var value string

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// Get the FixedWidthEncodingInfo from the schema provider
	var fwEncodingInfo *FixedWidthEncodingInfo
	sp := ctx.schemaProvider
	if sp != nil {
		fwEncodingInfo = sp.FixedWidthEncodingInfo()
	}
	if fwEncodingInfo == nil {
		cpErr = fmt.Errorf("error: writing fixed_width file, no encodeding info available")
		goto gotError
	}

	//*TODO support multiple record type for writing fixed_width files
	if fwEncodingInfo.RecordTypeColumn != nil {
		cpErr = fmt.Errorf("error: writing fixed_width file, currently supporting single record type")
		goto gotError
	}
	fwColumnsInfo = fwEncodingInfo.ColumnsMap[""]
	if fwColumnsInfo == nil {
		cpErr = fmt.Errorf("error: writing fixed_width file, currently supporting single record type (2)")
		goto gotError
	}

	// Getting the column position for the output fw columns
	columnPos = make([]int, 0, len(*fwColumnsInfo))
	for _, fwColumn := range *fwColumnsInfo {
		columnPos = append(columnPos, ctx.outputCh.columns[fwColumn.ColumnName])
	}

	// fmt.Println("**&@@ WriteFixedWidthPartition *1: fileName:", *ctx.fileName)
	if ctx.s3DeviceManager == nil {
		cpErr = fmt.Errorf("error: s3DeviceManager is nil")
		goto gotError
	}

	// open the local temp file for the writer
	fileHd, err = os.Create(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening local file for fixed_width write %v", err)
		goto gotError
	}
	defer fileHd.Close()

	switch ctx.spec.OutputChannel.Compression {
	case "none":
		fwWriter = bufio.NewWriter(fileHd)
	case "snappy":
		// Open a snappy compressor
		snWriter = snappy.NewBufferedWriter(fileHd)
		// Open a buffered writer
		fwWriter = bufio.NewWriter(snWriter)
	default:
		cpErr = fmt.Errorf("error: unknown compression %s in WriteFixedWidthPartition",
			ctx.spec.OutputChannel.Compression)
		goto gotError
	}

	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		//*$1
		// replace null with empty string, convert to string
		for i, fwColumn := range *fwColumnsInfo {
			l := fwColumn.End - fwColumn.Start
			switch vv := inRow[columnPos[i]].(type) {
			case string:
				value = vv
			case nil:
				value = ""
			default:
				value = fmt.Sprintf("%v", vv)
			}
			lv := len(value)
			if lv >= l {
				_, err := fwWriter.WriteString(value[:l])
				if err != nil {
					cpErr = fmt.Errorf("while writing to local fixed_width file: %v", err)
					goto gotError
				}
			} else {
				_, err := fwWriter.WriteString(value)
				if err != nil {
					cpErr = fmt.Errorf("while writing to local fixed_width file: %v", err)
					goto gotError
				}
				_, err = fwWriter.WriteString(strings.Repeat(" ", l-lv))
				if err != nil {
					cpErr = fmt.Errorf("while writing to local fixed_width file: %v", err)
					goto gotError
				}
			}
		}
		_, err := fwWriter.WriteString("\n")
		if err != nil {
			cpErr = fmt.Errorf("while writing to local fixed_width file: %v", err)
			goto gotError
		}
	}

	// fmt.Println("**&@@ WriteFixedWidthPartition: DONE writing local file for fileName:", *ctx.fileName,
	// "...file key:",s3FileName)
	fwWriter.Flush()
	if snWriter != nil {
		snWriter.Flush()
	}
	// schedule the file to be moved to s3
	select {
	case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
		ExternalBucket: *ctx.externalBucket,
		FileKey:        s3FileName,
		LocalFilePath:  tempFileName,
	}:
	case <-ctx.doneCh:
		log.Printf("WriteFixedWidthPartition: sending file to S3DeviceManager interrupted")
	}
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

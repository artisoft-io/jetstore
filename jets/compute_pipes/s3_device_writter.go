package compute_pipes

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/golang/snappy"
)

// S3DeviceWriter is the component that reads the rows comming the PipeTransformationEvaluator
// and writes them to a local file. This component delegates to S3DeviceManager to put the files in s3

type S3DeviceWriter struct {
	s3DeviceManager *S3DeviceManager
	source          *InputChannel
	schemaProvider  SchemaProvider
	parquetSchema   *ParquetSchemaInfo
	localTempDir    *string
	externalBucket  *string
	s3BasePath      *string
	fileName        *string
	spec            *TransformationSpec
	outputCh        *OutputChannel
	doneCh          chan struct{}
	errCh           chan error
}

// WritePartition is main write function that coordinates between
// writing the partition to a temp file locally or stream the
// data directly to s3.
func (ctx *S3DeviceWriter) WritePartition(writer func(w io.Writer)) {
	var cpErr, err error
	if ctx.spec.PartitionWriterConfig.StreamDataOut {
		// Stream the data directly to s3
		s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)
		pin, pout := io.Pipe()

		go func() {
			writer(pout)
			pout.Close()
		}()

		// Write to s3 from pin
		awsi.UploadToS3FromReader(*ctx.externalBucket, s3FileName, pin)

	} else {
		// Write the data to a local temp file and then copy it to s3
		var fout *os.File
		tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
		s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)
		// fmt.Println("**&@@ WritePartition *1: fileName:", *ctx.fileName)
		if ctx.s3DeviceManager == nil {
			cpErr = fmt.Errorf("error: s3DeviceManager is nil")
			goto gotError
		}

		// open the local temp file
		fout, err = os.Create(tempFileName)
		if err != nil {
			cpErr = fmt.Errorf("opening output file failed: %v", err)
			goto gotError
		}
		defer fout.Close()

		// Write the partition
		writer(fout)

		// fmt.Println("**&@@ WritePartition: DONE writing local file for fileName:", *ctx.fileName)
		// schedule the file to be moved to s3
		select {
		case ctx.s3DeviceManager.WorkersTaskCh <- S3Object{
			ExternalBucket: *ctx.externalBucket,
			FileKey:        s3FileName,
			LocalFilePath:  tempFileName,
		}:
		case <-ctx.doneCh:
			log.Printf("WritePartition: sending file to S3DeviceManager interrupted")
		}
	}

	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)

}

func (ctx *S3DeviceWriter) WriteParquetPartitionV2(fout io.Writer) {
	gotError := func(err error) {
		log.Println(err)
		ctx.errCh <- err
		close(ctx.doneCh)
	}
	WriteParquetPartitionV3(ctx.parquetSchema, fout, ctx.source.channel, gotError)
}

func (ctx *S3DeviceWriter) WriteCsvPartition(fout io.Writer) {
	var count int
	var cpErr, err error
	var snWriter *snappy.Writer
	var csvWriter *csv.Writer

	switch ctx.spec.OutputChannel.Compression {
	case "none":
		csvWriter = csv.NewWriter(fout)
	case "snappy":
		// Open a snappy compressor
		snWriter = snappy.NewBufferedWriter(fout)
		// Open a csv writer
		csvWriter = csv.NewWriter(snWriter)
	default:
		cpErr = fmt.Errorf("error: unknown compression %s in WriteCsvPartition",
			ctx.spec.OutputChannel.Compression)
		goto gotError
	}
	if ctx.spec.OutputChannel.Delimiter != 0 {
		csvWriter.Comma = ctx.spec.OutputChannel.Delimiter
	}
	if ctx.schemaProvider != nil {
		csvWriter.QuoteAll = ctx.schemaProvider.QuoteAllRecords()
		csvWriter.NoQuotes = ctx.schemaProvider.NoQuotes()
	}
	if ctx.spec.OutputChannel.Format == "csv" {
		err = csvWriter.Write(ctx.outputCh.config.Columns)
		if err != nil {
			cpErr = fmt.Errorf("while writing headers to local csv file: %v", err)
			goto gotError
		}
	}
	// Write the rows into the temp file
	for inRow := range ctx.source.channel {
		count++
		// log.Printf("*** CSV.WRITE %d:%v\n", count, inRow)
		// replace null with empty string, convert to string
		row := make([]string, len(inRow))
		for i := range inRow {
			row[i] = encodeRdfTypeToTxt(inRow[i])
		}
		// log.Printf("*** Cast WRITE RDF TYPE %d:%v\n", count, row)
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

	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

func (ctx *S3DeviceWriter) WriteFixedWidthPartition(fout io.Writer) {
	var cpErr error
	var snWriter *snappy.Writer
	var fwWriter *bufio.Writer
	var fwColumnsInfo *[]*FixedWidthColumn
	var columnPos []int
	var value string
	var fwEncodingInfo *FixedWidthEncodingInfo

	// Get the FixedWidthEncodingInfo from the schema provider
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
		columnPos = append(columnPos, (*ctx.outputCh.columns)[fwColumn.ColumnName])
	}

	switch ctx.spec.OutputChannel.Compression {
	case "none":
		fwWriter = bufio.NewWriter(fout)
	case "snappy":
		// Open a snappy compressor
		snWriter = snappy.NewBufferedWriter(fout)
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
			value = encodeRdfTypeToTxt(inRow[columnPos[i]])
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

	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

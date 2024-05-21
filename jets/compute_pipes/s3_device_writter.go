package compute_pipes

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang/snappy"
	"github.com/xitongsys/parquet-go/source"
	"github.com/xitongsys/parquet-go/writer"
)

type S3DeviceWriter struct {
	s3Uploader    *manager.Uploader
	source        *InputChannel
	parquetSchema []string
	localTempDir  *string
	s3BasePath    *string
	fileName      *string
	bucketName    string
	regionName    string
	doneCh        chan struct{}
	errCh         chan error
}

//*TODO No need to have bucketName and regionName in ctx, can be put as globals
var kmsKeyArn string
func init() {
	kmsKeyArn = os.Getenv("JETS_S3_KMS_KEY_ARN")
}

func (ctx *S3DeviceWriter) WriteParquetPartition(s3WriterResultCh chan<- ComputePipesResult) {
	var cpErr, err error
	var pw *writer.CSVWriter
	var fileHd *os.File
	var fw source.ParquetFile
	var putObjInput *s3.PutObjectInput


	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// fmt.Println("**&@@ WriteParquetPartition *1: fileName:", *ctx.fileName)
	if ctx.s3Uploader == nil {
		cpErr = fmt.Errorf("error: s3Uploader is nil")
		goto gotError
	}

	// open the local temp file for the parquet writer
	fw, err = delegate.NewLocalFileWriter(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening local parquet file for write %v", err)
		goto gotError
	}
	defer func() {
		os.Remove(tempFileName)
	}()

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
			fw.Close()
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
		fw.Close()
		cpErr = fmt.Errorf("while writing parquet stop (trailer): %v", err)
		goto gotError
	}
	// fmt.Println("**&@@ WriteParquetPartition: DONE writing local parquet file for fileName:", *ctx.fileName)
	fw.Close()
	// //****
	// if fw != nil {
	// 	cpErr = fmt.Errorf("SIMULATED error writing parquet stop (trailer): %v", err)
	// 	fmt.Println(cpErr)
	// 	goto gotError
	// }
	// //****

	// Copy file to s3 location
	fileHd, err = os.Open(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening written file to copy to s3: %v", err)
		goto gotError
	}
	defer fileHd.Close()

	putObjInput = &s3.PutObjectInput{
		Bucket: &ctx.bucketName,
		Key:    &s3FileName,
		Body:   bufio.NewReader(fileHd),
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	_, err = ctx.s3Uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		cpErr = fmt.Errorf("while copying parquet jets_partition to s3: %v", err)
		goto gotError
	}
	s3WriterResultCh <- ComputePipesResult{PartsCount: 1}
	// All good!
	return
gotError:
	log.Println(cpErr)
	s3WriterResultCh <- ComputePipesResult{Err: cpErr}
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

func (ctx *S3DeviceWriter) WriteCsvPartition(s3WriterResultCh chan<- ComputePipesResult) {
	var cpErr, err error
	var fileHd *os.File
	var snWriter *snappy.Writer
	var csvWriter *csv.Writer
	var putObjInput *s3.PutObjectInput

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// fmt.Println("**&@@ WriteCsvPartition *1: fileName:", *ctx.fileName)
	if ctx.s3Uploader == nil {
		cpErr = fmt.Errorf("error: s3Uploader is nil")
		goto gotError
	}

	// open the local temp file for the writer
	fileHd, err = os.Create(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening local file for write %v", err)
		goto gotError
	}
	defer func() {
		os.Remove(tempFileName)
	}()

	// Open a snappy compressor
	snWriter = snappy.NewBufferedWriter(fileHd)

	// Open a csv writer
	csvWriter = csv.NewWriter(snWriter)

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
			fileHd.Close()
			// fmt.Println("ERROR")
			// for i := range inRow {
			// 	fmt.Println(inRow[i], reflect.TypeOf(inRow[i]).Kind())
			// }
			// fmt.Println("ERROR")
			cpErr = fmt.Errorf("while writing row to local csv file: %v", err)
			goto gotError
		}
	}

	// fmt.Println("**&@@ WriteCsvPartition: DONE writing local parquet file for fileName:", *ctx.fileName)
	csvWriter.Flush()
	snWriter.Flush()
	_, err = fileHd.Seek(0, 0)
	if err != nil {
		cpErr = fmt.Errorf("while opening written file to copy to s3: %v", err)
		goto gotError
	}

	defer fileHd.Close()

	putObjInput = &s3.PutObjectInput{
		Bucket: &ctx.bucketName,
		Key:    &s3FileName,
		Body:   bufio.NewReader(fileHd),
	}
	if len(kmsKeyArn) > 0 {
		putObjInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putObjInput.SSEKMSKeyId = &kmsKeyArn
	}
	_, err = ctx.s3Uploader.Upload(context.TODO(), putObjInput)
	if err != nil {
		cpErr = fmt.Errorf("while copying compressed csv jets_partition to s3: %v", err)
		goto gotError
	}
	s3WriterResultCh <- ComputePipesResult{PartsCount: 1}
	// All good!
	return
gotError:
	log.Println(cpErr)
	s3WriterResultCh <- ComputePipesResult{Err: cpErr}
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

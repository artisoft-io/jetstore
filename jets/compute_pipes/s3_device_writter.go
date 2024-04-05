package compute_pipes

import (
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/xitongsys/parquet-go/writer"
)

type S3DeviceWriter struct {
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

func (ctx *S3DeviceWriter) WritePartition() {
	var cpErr error
	var pw *writer.CSVWriter
	var fileHd *os.File

	// fmt.Println("**&@@ WritePartition: fileName:", *ctx.fileName)

	tempFileName := fmt.Sprintf("%s/%s", *ctx.localTempDir, *ctx.fileName)
	s3FileName := fmt.Sprintf("%s/%s", *ctx.s3BasePath, *ctx.fileName)

	// open the local temp file for the parquet writer
	fw, err := delegate.NewLocalFileWriter(tempFileName)
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
		// replace null with empty string
		for i := range inRow {
			switch  vv := inRow[i].(type) {
			case string:
			case nil:
				inRow[i] = ""
			default:
				inRow[i] = fmt.Sprintf("%v", vv)
			}
		}
		if err = pw.Write(inRow); err != nil {
			fw.Close()
			cpErr = fmt.Errorf("while writing row to local parquet file: %v", err)
			goto gotError
		}
	}

	if err = pw.WriteStop(); err != nil {
		fw.Close()
		cpErr = fmt.Errorf("while writing parquet stop (trailer): %v", err)
		goto gotError
	}
	// fmt.Println("**&@@ WritePartition: DONE writing local parquet file for fileName:", *ctx.fileName)
	fw.Close()

	// Copy file to s3 location
	fileHd, err = os.Open(tempFileName)
	if err != nil {
		cpErr = fmt.Errorf("while opening written file to copy to s3: %v", err)
		goto gotError
	}
	if err = awsi.UploadToS3(ctx.bucketName, ctx.regionName, s3FileName, fileHd); err != nil {
		fileHd.Close()
		cpErr = fmt.Errorf("while copying to s3: %v", err)
		goto gotError
	}
	fileHd.Close()
	// fmt.Println("**&@@ WritePartition: DONE copying to s3 for fileName:", *ctx.fileName)

	// All good!
	return
gotError:
	//* FIX THIS These error are often commin too late FIX THIS
	log.Fatalln("PANIC (FIX THIS)",cpErr)
	// log.Println(cpErr)
	// ctx.errCh <- cpErr
	// close(ctx.doneCh)
}

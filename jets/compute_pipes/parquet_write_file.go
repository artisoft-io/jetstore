package compute_pipes

import (
	"fmt"
	"io"
	"log"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet"
	"github.com/apache/arrow/go/v17/parquet/compress"
	"github.com/apache/arrow/go/v17/parquet/file"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
)

func (ctx *S3DeviceWriter) WriteParquetPartitionV2(fout io.Writer) {
	var cpErr, err error
	pool := memory.NewGoAllocator()
	schemaInfo := ctx.parquetSchema

	// Prepare the parquet writer
	props := parquet.NewWriterProperties(parquet.WithCompression(compress.Codecs.Snappy))
	writer, err := pqarrow.NewFileWriter(schemaInfo.ArrowSchema(), fout, props, pqarrow.DefaultWriterProps())
	if err != nil {
		cpErr = fmt.Errorf("while calling pqarrow.NewFileWriter: %v", err)
		goto gotError
	}
	



	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.doneCh)
}

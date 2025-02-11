package compute_pipes

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Component that merge part files into a single output file.
// Merge the file content by chunks w/o loading it as rows

// Function to merge the partfiles into a single file by streaming the content
// to s3 using a channel. This is run in the main thread, so no need to have
// a result channel back to the caller.
func (cpCtx *ComputePipesContext) StartMergeFiles(dbpool *pgxpool.Pool) (cpErr error) {

	log.Println("Entering StartMergeFiles")

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("StartMergeFiles: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			cpErr = errors.New(buf.String())
			log.Println(cpErr)
			cpCtx.ErrCh <- cpErr
			close(cpCtx.Done)
		}
	}()

	// Validate the pipe config
	pipeSpec := &cpCtx.CpConfig.PipesConfig[0]
	outputFileKey := pipeSpec.OutputFile
	if pipeSpec.Type != "merge_files" || outputFileKey == nil {
		cpErr = fmt.Errorf("error: StartMergeFiles called but the PipeConfig does not have a valid output_file component")
		return
	}
	outputFileConfig := GetOutputFileConfig(cpCtx.CpConfig, *outputFileKey)
	if outputFileConfig == nil {
		cpErr = fmt.Errorf("error: OutputFile config not found for key %s in StartMergeFiles", *outputFileKey)
		return
	}
	// outputFileConfig.KeyPrefix is the s3 output folder, when empty use:
	//     <JETS_s3_OUTPUT_PREFIX>/<input file_key dir>/
	// outputFileConfig.Name is the file name, defaults to $NAME_FILE_KEY (a file name is required)
	if outputFileConfig.OutputLocation == "" {
		outputFileConfig.OutputLocation = "jetstore_s3_output"
	}
	var fileName string
	if len(outputFileConfig.Name) > 0 {
		fileName = doSubstitution(outputFileConfig.Name, "",	"",	cpCtx.EnvSettings)
	} else {
		fileName = doSubstitution("$NAME_FILE_KEY", "",	"",	cpCtx.EnvSettings)
	}
	if len(fileName) == 0 {
		cpErr = fmt.Errorf("error: OutputFile config is missing file_name in StartMergeFile")
		return
	}

	var fileFolder string
	if len(outputFileConfig.KeyPrefix) > 0 {
		fileFolder = doSubstitution(outputFileConfig.KeyPrefix, "",	outputFileConfig.OutputLocation,
			cpCtx.EnvSettings)
	} else {
		fileFolder = doSubstitution("$PATH_FILE_KEY", "",	outputFileConfig.OutputLocation,
			cpCtx.EnvSettings)
	}
	outputS3FileKey := fmt.Sprintf("%s/%s", fileFolder, fileName)

	// Create a reader to stream the data to s3
	compression := pipeSpec.InputChannel.Compression
	inputSp := cpCtx.SchemaManager.GetSchemaProvider(pipeSpec.InputChannel.SchemaProvider)
	if len(compression) == 0 && inputSp != nil {
		compression = inputSp.Compression()
	}

	// Determine if we write the file in the source bucket of the schema provider
	var externalBucket string
	switch {
	case len(outputFileConfig.Bucket) > 0:
		if outputFileConfig.Bucket != "jetstore_bucket" {
			externalBucket = outputFileConfig.Bucket
		}
	case inputSp != nil && outputFileConfig.OutputLocation == "jetstore_s3_input":
		externalBucket = inputSp.Bucket()
	}
	if len(externalBucket) > 0 {
		externalBucket = doSubstitution(externalBucket, "", "", cpCtx.EnvSettings)
	}

	// Determine if we put a header row
	outputSp := cpCtx.SchemaManager.GetSchemaProvider(outputFileConfig.SchemaProvider)
	writeHeaders := true
	if outputSp != nil && outputSp.Format() != "csv" {
		writeHeaders = false
	}
	if len(outputFileConfig.Headers) == 0 && writeHeaders {
		if inputSp != nil {
			outputFileConfig.Headers = inputSp.ColumnNames()
		}

		if len(outputFileConfig.Headers) == 0 {
			// Get the headers from the main input source (as a fallback)
			// This is for the case where the headers are in the input file
			inputChannelName := cpCtx.CpConfig.PipesConfig[0].InputChannel.Name
			if inputChannelName == "input_row" {
				outputFileConfig.Headers =
					cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
			} else {
				for i := range cpCtx.CpConfig.Channels {
					if cpCtx.CpConfig.Channels[i].Name == inputChannelName {
						outputFileConfig.Headers = cpCtx.CpConfig.Channels[i].Columns
						break
					}
				}
			}
		}
		if len(outputFileConfig.Headers) == 0 {
			cpErr = fmt.Errorf(
				"error: merge_files operator using output_file %s, no headers avaliable",
				outputFileConfig.Key)
			return
		}
	}
	// Delimiter for the header row
	var delimit rune
	if inputSp != nil {
		delimit = inputSp.Delimiter()
	}
	inputFormat := cpCtx.CpConfig.PipesConfig[0].InputChannel.Format
	if inputFormat == "parquet" {
		//*TODO support merging parquet files into a single file
		// SPECIAL CASE = Currently only supporting single parquet file in the input
		// to be sent to s3
		if len(cpCtx.InputFileKeys) != 1 {
			cpErr = fmt.Errorf("error: merge_file operator currently cannot merge multiple parquet file, got %d input files",
				len(cpCtx.InputFileKeys))
				return
		}
	}
	r, err := cpCtx.NewMergeFileReader(inputFormat, outputSp, outputFileConfig.Headers, writeHeaders, delimit, compression)
	if err != nil {
		cpErr = err
		return
	}

	// put content of file to s3
	if err := awsi.UploadToS3FromReader(externalBucket, outputS3FileKey, r); err != nil {
		cpErr = fmt.Errorf("while copying to s3: %v", err)
		return
	}
	log.Printf("%s node %d merging files to '%s' completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
	return
}

// MergeFileReader provides a reader that conforms to io.Reader interface
// that reads content from partfiles and makes it available to the s3 manager
// via the Read interface
type MergeFileReader struct {
	currentFile    FileName
	currentFileHd  *os.File
	reader         *bufio.Reader
	headers        []byte
	compression    string
	skipHeaderLine bool
	skipHeaderFlag bool
	cpCtx          *ComputePipesContext
}

// outputSp is needed to determine if we quote all or non fields. It also provided the writeHeaders value.
func (cpCtx *ComputePipesContext) NewMergeFileReader(inputFormat string, outputSp SchemaProvider, headers []string,
	writeHeaders bool, delimit rune, compression string) (io.Reader, error) {

	var h []byte
	if len(headers) > 0 && writeHeaders {
		var sep rune = ','
		if delimit > 0 {
			sep = delimit
		}
		// Write the header into a byte slice. Using a csv.Writer to make sure
		// the delimiter is escaped correctly
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		w.Comma = sep
		if outputSp != nil {
			if outputSp.QuoteAllRecords() {
				w.QuoteAll = true
			}
			if outputSp.NoQuotes() {
				w.NoQuotes = true
			}
		}
		err := w.Write(headers)
		if err != nil {
			return nil, fmt.Errorf("while writing headers in merge_files op: %v", err)
		}
		w.Flush()
		h = buf.Bytes()
	}
	return &MergeFileReader{
		cpCtx:          cpCtx,
		headers:        h,
		compression:    compression,
		skipHeaderLine: inputFormat == "csv",
	}, nil
}

func (r *MergeFileReader) Read(buf []byte) (int, error) {
	var err error
	switch {

	case r.headers != nil:
		n := copy(buf, r.headers)
		if n < len(r.headers) {
			r.headers = r.headers[n:]
		} else {
			r.headers = nil
		}
		return n, nil

	case r.reader == nil:
		// get the next file
		r.currentFile = <-r.cpCtx.FileNamesCh
		if r.currentFile.LocalFileName == "" {
			return 0, io.EOF
		}
		// open the reader for currentFile
		r.currentFileHd, err = os.Open(r.currentFile.LocalFileName)
		if err != nil {
			return 0, fmt.Errorf("while opening temp file '%s' (MergeFileReader.Read): %v",
				r.currentFile.LocalFileName, err)
		}
		switch r.compression {
		case "snappy":
			r.reader = bufio.NewReader(snappy.NewReader(r.currentFileHd))
		case "none", "":
			r.reader = bufio.NewReader(r.currentFileHd)
		default:
			return 0, fmt.Errorf("error: unknown compression %s (merge_files)", r.compression)
		}
		// skip headerline if needed
		if r.skipHeaderLine {
			r.skipHeaderFlag = true
		}
		// read from file, delegate to itself
		return r.Read(buf)

	default:
		// Delegate to the reader
		if r.skipHeaderFlag {
			_, err = r.reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return 0, err
			}
			r.skipHeaderFlag = false
		}
		var n int
		var err2 error
		if err == nil {
			n, err2 = r.reader.Read(buf)
			if err2 != nil && err2 != io.EOF {
				return n, err2
			}
		}
		if err == io.EOF || err2 == io.EOF {
			r.currentFileHd.Close()
			os.Remove(r.currentFile.LocalFileName)
			r.currentFileHd = nil
			r.reader = nil
		}
		if n == 0 {
			// check for next file
			return r.Read(buf)
		}
		return n, nil
	}

}

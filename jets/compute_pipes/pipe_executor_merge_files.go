package compute_pipes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Component that merge part files into a single output file.
// Merge the file content by chunks w/o loading it as rows

var minPartSize int = 5*1024*1024 + 10 // 5 MB is the min part size for s3 multipart copy

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
	// outputFileConfig.OutputLocation may have 3 values:
	//	- jetstore_s3_input, to indicate to put the output file in JetStore input path.
	//	- jetstore_s3_stage, to indicate to put the output file in JetStore stage path.
	//	- jetstore_s3_output (default), to indicate to put the output file in JetStore output path.
	//	- custom file path, indicates a custom file key location (path and file name) in this case
	//    it replaces KeyPrefix and Name attributes.
	// outputFileConfig.KeyPrefix is the s3 output folder, when empty use:
	//     <JETS_s3_OUTPUT_PREFIX>/<input file_key dir>/
	// outputFileConfig.Name is the file name, defaults to $NAME_FILE_KEY (a file name is required)
	if outputFileConfig.OutputLocation() == "" {
		outputFileConfig.SetOutputLocation("jetstore_s3_output")
	}
	var fileFolder, fileName, outputS3FileKey string
	nbrFiles := len(cpCtx.InputFileKeys)
	switch outputFileConfig.OutputLocation() {
	case "jetstore_s3_input", "jetstore_s3_output", "jetstore_s3_stage":
		if len(outputFileConfig.Name()) > 0 {
			fileName = utils.ReplaceEnvVars(outputFileConfig.Name(), cpCtx.EnvSettings)
		} else {
			fileName = utils.ReplaceEnvVars("$NAME_FILE_KEY", cpCtx.EnvSettings)
		}
		if len(fileName) == 0 {
			cpErr = fmt.Errorf("error: OutputFile config is missing file_name in StartMergeFile")
			return
		}
		if outputFileConfig.OutputLocation() == "jetstore_s3_stage" {
			// put in jetstore s3 stage path
			keyPrefix := utils.ReplaceEnvVars(outputFileConfig.KeyPrefix, cpCtx.EnvSettings)
			fileFolder = fmt.Sprintf("%s/%s/%s", jetsS3StagePrefix, keyPrefix, fileName)
		} else {
			if len(outputFileConfig.KeyPrefix) > 0 {
				fileFolder = doSubstitution(outputFileConfig.KeyPrefix, "", outputFileConfig.OutputLocation(),
					cpCtx.EnvSettings)
			} else {
				fileFolder = doSubstitution("$PATH_FILE_KEY", "", outputFileConfig.OutputLocation(),
					cpCtx.EnvSettings)
			}
			outputS3FileKey = fmt.Sprintf("%s/%s", fileFolder, fileName)
		}

	default:
		outputS3FileKey = utils.ReplaceEnvVars(outputFileConfig.OutputLocation(), cpCtx.EnvSettings)
	}

	// Create a reader if stream the data to s3
	inputChannel := pipeSpec.InputChannel
	compression := inputChannel.Compression
	inputSp := cpCtx.SchemaManager.GetSchemaProvider(pipeSpec.InputChannel.SchemaProvider)

	// Determine if we write the file in the source bucket of the schema provider
	var externalBucket string
	switch {
	case len(outputFileConfig.Bucket) > 0:
		if outputFileConfig.Bucket != "jetstore_bucket" {
			externalBucket = outputFileConfig.Bucket
		}
	case inputSp != nil && outputFileConfig.OutputLocation() == "jetstore_s3_input":
		externalBucket = inputSp.Bucket()
	}
	if len(externalBucket) > 0 {
		externalBucket = utils.ReplaceEnvVars(externalBucket, cpCtx.EnvSettings)
	}

	// Determine if we put a header row
	format := "csv"
	outputSp := cpCtx.SchemaManager.GetSchemaProvider(outputFileConfig.SchemaProvider)
	switch {
	case outputSp != nil && outputSp.Format() != "":
		format = outputSp.Format()
	case outputFileConfig.Format != "":
		format = outputFileConfig.Format
	case inputChannel.Format != "":
		format = inputChannel.Format
	}
	writeHeaders := true
	if format != "csv" || (inputChannel.Format == "csv" && nbrFiles == 1) {
		writeHeaders = false
	}
	if pipeSpec.MergeFileConfig != nil && pipeSpec.MergeFileConfig.FirstPartitionHasHeaders {
		writeHeaders = false
	}

	// Determine the headers to write
	if len(outputFileConfig.Headers) == 0 && writeHeaders {
		if inputSp != nil {
			// This is always the original headers, not the uniquefied ones
			outputFileConfig.Headers = inputSp.ColumnNames()
		}

		if len(outputFileConfig.Headers) == 0 {
			// Get the headers from the main input source (as a fallback)
			// This is for the case where the headers are in the input file
			inputChannelName := cpCtx.CpConfig.PipesConfig[0].InputChannel.Name
			if inputChannelName == "input_row" {
				// Check if we need to use the original headers or the uniquefied ones
				inputChannelColumns := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
				originalHeaders := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.OriginalInputColumns
				if outputFileConfig.UseOriginalHeaders && len(originalHeaders) > 0 {
					outputFileConfig.Headers = originalHeaders
				} else {
					outputFileConfig.Headers = inputChannelColumns
				}
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
			log.Println(cpErr)
			return
		}
	}
	inputFormat := inputChannel.Format
	var delimiter rune = ','
	if inputChannel.Delimiter > 0 {
		delimiter = inputChannel.Delimiter
	}
	var fileReader io.Reader
	var err, mergeErr error
	var nrowsInRec int64
	// Check if contains multiple files to copy and make sure they are all above the min part size
	containsSmallPart := false
	if nbrFiles > 1 {
		for _, fk := range cpCtx.InputFileKeys {
			if fk.size <= minPartSize {
				containsSmallPart = true
				break
			}
		}
	}

	//*TODO Add support for xlsx
	// NOTE: Files are not downloaded locally when merging using s3 copy,
	// DOWNLOAD FILES IF: (inputFormat == "parquet" && nbrFiles > 1) ||
	//                    (compression=="snappy") || containsSmallPart || writeHeaders
	// See ComputePipesContext.startDownloadFiles() where this condition is verified.
	// This is called in ComputePipesContext.DownloadS3Files()
	switch {
	case inputFormat == "parquet" && nbrFiles > 1:
		// merge parquet files into a single file
		// Pipe the writer to a reader to content goes directly to s3
		// log.Printf("*** MERGE %d files to single parquet file\n", nbrFiles)
		pin, pout := io.Pipe()
		gotError := func(err error) {
			mergeErr = err
			pin.Close()
		}
		fileReader = pin
		go func() {
			if outputSp != nil {
				nrowsInRec = outputSp.NbrRowsInRecord()
			}
			MergeParquetPartitions(nrowsInRec, outputFileConfig.Headers, pout, cpCtx.FileNamesCh, gotError)
			pout.Close()
		}()
	case (len(compression) != 0 && compression != "none") || containsSmallPart || writeHeaders:
		// Gotta be snappy on text file or got a small part or need to write headers.
		// Not usual, do copy the old way
		log.Printf("*** MERGE %d files using text format (%s) with compression %s\n",
			nbrFiles, inputFormat, compression)
		fileReader, err = cpCtx.NewMergeFileReader(inputFormat, delimiter, outputSp, outputFileConfig.Headers, writeHeaders, compression)
		if err != nil {
			cpErr = err
			return
		}

	default:
		// Use s3 multipart file copy
		log.Printf("*** MERGE %d files using s3 multipart file copy with format %s\n", nbrFiles, inputFormat)
		s3Client, err := awsi.NewS3Client()
		if err != nil {
			cpErr = err
			return
		}

		poolSize := cpCtx.CpConfig.ClusterConfig.S3WorkerPoolSize
		sourceKey := fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s",
			jetsS3StagePrefix, cpCtx.ProcessName, cpCtx.SessionId, inputChannel.ReadStepId)
		err = awsi.MultiPartCopy(context.TODO(), s3Client, poolSize, "", sourceKey, externalBucket, outputS3FileKey,
			cpCtx.CpConfig.ClusterConfig.IsDebugMode)
		if err != nil {
			cpErr = fmt.Errorf("%s while merging files using s3 copy: %v", cpCtx.SessionId, err)
			log.Println(cpErr)
			return
		}
		log.Printf("%s node %d merging files to '%s' using s3 copy completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
		return
	}

	// put content of file to s3 using a local reader
	if err := awsi.UploadToS3FromReader(externalBucket, outputS3FileKey, fileReader); err != nil {
		cpErr = fmt.Errorf("while copying to s3: %v", err)
		return
	}
	if mergeErr != nil {
		cpErr = fmt.Errorf("%s while merging parquet files: %v", cpCtx.SessionId, mergeErr)
		return
	}
	log.Printf("%s node %d merging files to '%s' completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
	return
}

// Function to determine if need to download the input files
// returns true if:
//
//	(inputFormat == "parquet" && nbrFiles > 1) ||
//	(compression=="snappy") || containsSmallPart || (csv with first partition NOT having headers)
func (cpCtx *ComputePipesContext) startDownloadFiles() bool {
	pipeSpec := &cpCtx.CpConfig.PipesConfig[0]
	if pipeSpec.Type != "merge_files" {
		return true
	}
	nbrFiles := len(cpCtx.InputFileKeys)
	inputChannel := pipeSpec.InputChannel
	compression := inputChannel.Compression
	inputFormat := inputChannel.Format
	// If it's multi files parquet, need to download
	if inputFormat == "parquet" && nbrFiles > 1 {
		return true
	}
	// If csv, need to check if first partition has headers
	if inputFormat == "csv" && nbrFiles > 1 {
		if pipeSpec.MergeFileConfig != nil && pipeSpec.MergeFileConfig.FirstPartitionHasHeaders {
			log.Printf("%s node %d sorting part files since first file has headers", cpCtx.SessionId, cpCtx.NodeId)
			slices.SortFunc(cpCtx.InputFileKeys, func(lhs, rhs *FileKeyInfo) int {
				a := lhs.key
				b := rhs.key
				switch {
				case a < b:
					return -1
				case a > b:
					return 1
				default:
					return 0
				}
			})
		} else {
			log.Printf("%s node %d merge_files cannot use s3 multipart copy since first file does not have the headers", cpCtx.SessionId, cpCtx.NodeId)
			return true
		}
	}
	// If using snappy compression, need to download to merge the files
	if compression == "snappy" {
		return true
	}
	if nbrFiles > 1 {
		// check for small parts, need to download if a part is too small
		for _, fk := range cpCtx.InputFileKeys {
			if fk.size <= minPartSize {
				return true
			}
		}
	}
	// Able to use s3 multipart copy
	return false
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
func (cpCtx *ComputePipesContext) NewMergeFileReader(inputFormat string, delimiter rune, outputSp SchemaProvider, headers []string,
	writeHeaders bool, compression string) (io.Reader, error) {

	var h []byte
	if len(headers) > 0 && writeHeaders {
		// Write the header into a byte slice. Using a csv.Writer to make sure
		// the delimiter is escaped correctly
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if outputSp != nil {
			d := outputSp.Delimiter()
			if d > 0 {
				delimiter = d
			}
			if outputSp.QuoteAllRecords() {
				w.QuoteAll = true
			}
			if outputSp.NoQuotes() {
				w.NoQuotes = true
			}
		}
		w.Comma = delimiter
		err := w.Write(headers)
		if err != nil {
			return nil, fmt.Errorf("while writing headers in merge_files op: %v", err)
		}
		w.Flush()
		h = buf.Bytes()
	}
	if strings.HasPrefix(inputFormat, "parquet") {
		// compression does not applies to parquet file
		compression = ""
	}
	return &MergeFileReader{
		cpCtx:          cpCtx,
		headers:        h,
		compression:    compression,
		skipHeaderLine: inputFormat == "csv" && writeHeaders,
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

package compute_pipes

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
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
		cpErr = fmt.Errorf("error: StartMergeFiles called but the PipeConfig does not have a valid merge_files component")
		return
	}
	var outputFileConfig *OutputFileSpec
	for i := range cpCtx.CpConfig.OutputFiles {
		if *outputFileKey == cpCtx.CpConfig.OutputFiles[i].Key {
			outputFileConfig = &cpCtx.CpConfig.OutputFiles[i]
			break
		}
	}
	if outputFileConfig == nil {
		cpErr = fmt.Errorf("error: OutputFile config not found for key %s in StartMergeFiles", *outputFileKey)
		return
	}
	// outputFileConfig.KeyPrefix is the s3 output folder, when empty use:
	//     <JETS_s3_OUTPUT_PREFIX>/<input file_key dir>/
	// outputFileConfig.Name is the file name (required)
	var fileFolder string
	fileName := outputFileConfig.Name
	lc := 0
	for strings.Contains(fileName, "$") && lc < 5 && cpCtx.EnvSettings != nil {
		lc += 1
		for key, v := range cpCtx.EnvSettings {
			value, ok := v.(string)
			if ok {
				fileName = strings.ReplaceAll(fileName, key, value)
			}
		}
	}
	if len(outputFileConfig.KeyPrefix) > 0 {
		fileFolder = doSubstitution(
			outputFileConfig.KeyPrefix, "",
			outputFileConfig.OutputLocation,
			cpCtx.EnvSettings)
	} else {
		fileFolder = strings.Replace(cpCtx.CpConfig.CommonRuntimeArgs.FileKey,
			os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1)
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
		externalBucket = outputFileConfig.Bucket
	case inputSp != nil && outputFileConfig.OutputLocation == "jetstore_s3_input":
		externalBucket = inputSp.Bucket()
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
	r := cpCtx.NewMergeFileReader(inputFormat, outputFileConfig.Headers, writeHeaders, delimit, compression)

	// put content of file to s3
	if err := awsi.UploadToS3FromReader(externalBucket, outputS3FileKey, r); err != nil {
		cpErr = fmt.Errorf("while copying to s3: %v", err)
		return
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Printf("%s node %d merging files to '%s' completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
	}
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

func (cpCtx *ComputePipesContext) NewMergeFileReader(inputFormat string, headers []string,
	writeHeaders bool, delimit rune, compression string) io.Reader {

	var h []byte
	if len(headers) > 0 && writeHeaders {
		sep := ","
		if delimit > 0 {
			sep = string(delimit)
		}
		v := fmt.Sprintf("%s\n", strings.Join(headers, sep))
		h = []byte(v)
	}
	return &MergeFileReader{
		cpCtx:          cpCtx,
		headers:        h,
		compression:    compression,
		skipHeaderLine: inputFormat == "csv",
	}
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

package compute_pipes

import (
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
	for k, v := range cpCtx.EnvSettings {
		fileName = strings.ReplaceAll(fileName, k, fmt.Sprintf("%v", v))
	}
	if len(outputFileConfig.KeyPrefix) > 0 {
		fileFolder = outputFileConfig.KeyPrefix
		for k, v := range cpCtx.EnvSettings {
			fileFolder = strings.ReplaceAll(fileFolder, k, fmt.Sprintf("%v", v))
		}
	} else {
		fileFolder = strings.Replace(cpCtx.CpConfig.CommonRuntimeArgs.FileKey,
			os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1)
	}
	outputS3FileKey := fmt.Sprintf("%s/%s", fileFolder, fileName)

	// Create a reader to stream the data to s3
	compression := pipeSpec.InputChannel.Compression
	sp := cpCtx.SchemaManager.GetSchemaProvider(pipeSpec.InputChannel.SchemaProvider)
	if len(compression) == 0 {
		compression = sp.Compression()
	}
	if len(outputFileConfig.Headers) == 0 {
		if sp == nil {
			cpErr = fmt.Errorf(
				"error: merge_files operator using output_file %s has no headers or schema_provider defined",
				outputFileConfig.Key)
			return
		}
		outputFileConfig.Headers = sp.ColumnNames()
	}
	r := cpCtx.NewMergeFileReader(outputFileConfig.Headers, compression)

	// put content of file to s3
	if err := awsi.UploadToS3FromReader(outputS3FileKey, r); err != nil {
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
	currentFile   FileName
	currentFileHd *os.File
	reader        io.Reader
	headers       []byte
	compression   string
	cpCtx         *ComputePipesContext
}

func (cpCtx *ComputePipesContext) NewMergeFileReader(headers []string, compression string) io.Reader {
	var h []byte
	if len(headers) > 0 {
		v := fmt.Sprintf("%s\n", strings.Join(headers, ","))
		h = []byte(v)
	}
	return &MergeFileReader{
		cpCtx:       cpCtx,
		headers:     h,
		compression: compression,
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
			r.reader = snappy.NewReader(r.currentFileHd)
		case "none", "":
			r.reader = r.currentFileHd
		default:
			return 0, fmt.Errorf("error: unknown compression %s (merge_files)", r.compression)
		}
		// read from file, delegate to itself
		return r.Read(buf)

	default:
		// Delegate to the reader
		n, err2 := r.reader.Read(buf)
		if err2 == io.EOF {
			r.currentFileHd.Close()
			os.Remove(r.currentFile.LocalFileName)
			r.currentFileHd = nil
			r.reader = nil
		}
		if err2 != nil && err2 != io.EOF {
			// Got error while reading file
			return n, err2
		}
		if n == 0 {
			// check for next file
			return r.Read(buf)
		}
		return n, nil
	}

}

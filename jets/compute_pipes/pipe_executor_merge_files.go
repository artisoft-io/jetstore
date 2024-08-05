package compute_pipes

import (
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
func (cpCtx *ComputePipesContext) StartMergeFiles(dbpool *pgxpool.Pool) error {

	log.Println("Entering StartMergeFiles")

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			cpErr := fmt.Errorf("StartMergeFiles: recovered error: %v", r)
			log.Println(cpErr)
			// debug.Stack()
			debug.PrintStack()
			cpCtx.ErrCh <- cpErr
			close(cpCtx.Done)
		}
	}()
	// Validate the pipe config
	outputFileKey := cpCtx.CpConfig.PipesConfig[0].OutputFile
	if cpCtx.CpConfig.PipesConfig[0].Type != "merge_files" || outputFileKey == nil {
		return fmt.Errorf("error: StartMergeFiles called but the PipeConfig does not have a valid merge_files component")
	}
	var outputFileConfig *OutputFileSpec
	for i := range cpCtx.CpConfig.OutputFiles {
		if *outputFileKey == cpCtx.CpConfig.OutputFiles[i].Key {
			outputFileConfig = &cpCtx.CpConfig.OutputFiles[i]
			break
		}
	}
	if outputFileConfig == nil {
		return fmt.Errorf("error: OutputFile config not found for key %s in StartMergeFiles", *outputFileKey)
	}
	// Output s3 file key: <JETS_s3_OUTPUT_PREFIX>/<input file_key dir>/<outputFileConfig.Name>
	fileName := outputFileConfig.Name
	for k, v := range cpCtx.EnvSettings {
		fileName = strings.ReplaceAll(fileName, k, fmt.Sprintf("%v", v))
	}
	fileFolder := strings.Replace(cpCtx.CpConfig.CommonRuntimeArgs.FileKey,
		os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1)
	outputS3FileKey := fmt.Sprintf("%s/%s", fileFolder, fileName)

	// Create a reader to stream the data to s3
	r := cpCtx.NewMergeFileReader(outputFileConfig.Headers)

	// put content of file to s3
	if err := awsi.UploadToS3FromReader(outputS3FileKey, r); err != nil {
		return fmt.Errorf("while copying to s3: %v", err)
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Printf("%s node %d merging files to '%s' completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
	}
	return nil
}

// MergeFileReader provides a reader that conforms to io.Reader interface
// that reads content from partfiles and makes it available to the s3 manager
// via the Read interface
type MergeFileReader struct {
	currentFile   FileName
	currentFileHd *os.File
	reader        *snappy.Reader
	headers       []byte
	cpCtx         *ComputePipesContext
}

func (cpCtx *ComputePipesContext) NewMergeFileReader(headers []string) io.Reader {
	var h []byte
	if len(headers) > 0 {
		v := fmt.Sprintf("%s\n", strings.Join(headers, ","))
		h = []byte(v)
	}
	return &MergeFileReader{
		cpCtx:   cpCtx,
		headers: h,
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
		r.reader = snappy.NewReader(r.currentFileHd)
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

package compute_pipes

import (
	"bufio"
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

// Function to write transformed row to database
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

	// Create a local temp file to write the merged file
	var fileHd *os.File
	var err error
	fileHd, err = os.CreateTemp("", "jetstoreMergedFile")
	if err != nil {
		return fmt.Errorf("failed to open temp output file in StartMergeFiles: %v", err)
	}
	defer func() {
		fileName := fileHd.Name()
		fileHd.Close()
		os.Remove(fileName)
	}()

	// Write the output file headers if specified
	w := bufio.NewWriter(fileHd)
	if len(outputFileConfig.Headers) > 0 {
		_, err = w.Write([]byte(strings.Join(outputFileConfig.Headers, ",")))
		if err != nil {
			err = fmt.Errorf("while writing headers to output merged file:%v", err)
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, err)
			return err
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			err = fmt.Errorf("while writing carriage return to output merged file:%v", err)
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, err)
			return err
		}
	}
	// Merge the part files into a single output file
	// Open the destination file
	for localInFile := range cpCtx.FileNamesCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d merging file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKey)
		}
		err = copyFile(localInFile.LocalFileName, w)
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, err)
			return err
		}
	}
	err = w.Flush()
	if err != nil {
		err = fmt.Errorf("while flusing output file (StartMergeFile): %v", err)
		log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, err)
		return err
	}

	// Copy the file to s3
	fileHd.Seek(0, 0)
	if err = awsi.UploadToS3(bucketName, regionName, outputS3FileKey, fileHd); err != nil {
		return fmt.Errorf("while copying to s3: %v", err)
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Printf("%s node %d merging files to '%s' completed", cpCtx.SessionId, cpCtx.NodeId, outputS3FileKey)
	}
	return nil
}

func copyFile(source string, destination *bufio.Writer) error {
	var fileHd *os.File
	var err error
	fileHd, err = os.Open(source)
	if err != nil {
		return fmt.Errorf("while opening temp file '%s' (copyFile): %v", source, err)
	}
	defer func() {
		fileHd.Close()
		os.Remove(source)
	}()
	reader := snappy.NewReader(fileHd)
	buf := make([]byte, 4096)
	for {
		_, err = reader.Read(buf)
		switch {
		case err == io.EOF:
			// expected exit route
			return nil

		case err != nil:
			return fmt.Errorf("while reading input part file (copyFile): %v", err)

		default:
			_, err = destination.Write(buf)
			if err != nil {
				return fmt.Errorf("while writing part file to output merged file (copyFile): %v", err)
			}
		}
	}
}

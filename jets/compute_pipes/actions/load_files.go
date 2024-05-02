package actions

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	goparquet "github.com/fraugster/parquet-go"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Load multipart files to JetStore, file to load are provided by channel fileNameCh

func (cpCtx *ComputePipesContext) LoadFiles(ctx context.Context, dbpool *pgxpool.Pool) {

	// Create a channel to use as a buffer between the file loader and the copy to db
	// This gives the opportunity to use Compute Pipes to transform the data before writing to the db
	computePipesInputCh := make(chan []interface{}, 10)

	defer func() {
		// if r := recover(); r != nil {
		// 	loadFromS3FilesResultCh <- LoadFromS3FilesResult{Err: fmt.Errorf("recovered error: %v", r)}
		// 	debug.PrintStack()
		// 	close(done)
		// }
		fmt.Println("Closing computePipesInputCh **")
		close(computePipesInputCh)
	}()

	// Start the Compute Pipes async
	// Note: when nbrShards > 1, cpipes does not work in local mode in apiserver yet
	go compute_pipes.StartComputePipes(dbpool, cpCtx.InputColumns, cpCtx.Done, cpCtx.ErrCh, computePipesInputCh, cpCtx.ChResults,
		&cpCtx.CpConfig, cpCtx.EnvSettings, cpCtx.FileKeyComponents)

		// Load the files
	var totalRowCount int64
	for localInFile := range cpCtx.FileNamesCh {
		log.Printf("Loading file '%s'", localInFile)
		count, err := cpCtx.ReadFile(&localInFile, computePipesInputCh)
		totalRowCount += count
		if err != nil {
			fmt.Println("loadFile2Db returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0}
}

func (cpCtx *ComputePipesContext) ReadFile(filePath *FileName, computePipesInputCh chan<- []interface{}) (int64, error) {
	var fileHd *os.File
	var parquetReader *goparquet.FileReader
	var err error

		fileHd, err = os.Open(filePath.LocalFileName)
		if err != nil {
			return 0, fmt.Errorf("while opening temp file '%s' (loadFiles): %v", *filePath, err)
		}
		defer func() {
			fileHd.Close()
			os.Remove(filePath.LocalFileName)
		}()

		parquetReader, err = goparquet.NewFileReader(fileHd, cpCtx.InputColumns...)
		if err != nil {
			return 0, err
		}

	var inputRowCount int64
	var record []interface{}
	currentLineNumber := 0
	for {
		// read and put the rows into computePipesInputCh
		currentLineNumber += 1
		err = nil

		record = make([]interface{}, len(cpCtx.InputColumns))
		var parquetRow map[string]interface{}
		parquetRow, err = parquetReader.NextRow()
		if err == nil {
			for i := range cpCtx.InputColumns {
				rawValue := parquetRow[cpCtx.InputColumns[i]]
				if rawValue == nil {
					record[i] = ""
				} else {
					switch vv := rawValue.(type) {
					case string:
						record[i] = vv
					case []byte:
						record[i] = string(vv)
					case int:
						record[i] = strconv.Itoa(vv)
					case int32:
						record[i] = strconv.FormatInt(int64(vv), 10)
					case int64:
						record[i] = strconv.FormatInt(vv, 10)
					case float64:
						record[i] = strconv.FormatFloat(vv, 'E', -1, 32)
					case float32:
						record[i] = strconv.FormatFloat(float64(vv), 'E', -1, 32)
					case bool:
						record[i] = fmt.Sprintf("%v", vv)
					default:
						t := reflect.TypeOf(rawValue)
						if t.Kind() == reflect.Array {
							v := reflect.ValueOf(rawValue)
							bb := make([]byte, t.Len())
							for i := range bb {
								bb[i] = byte(v.Index(i).Interface().(uint8))
							}
							record[i] = string(bb)
						} else {
							record[i] = fmt.Sprintf("%v", rawValue)
						}
					}
				}
			}
		}

		switch {
		case err == io.EOF:
			// expected exit route
			// ---------------------------------------------------
			return inputRowCount, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading input records: %v", err)

		default:
			// // Remove invalid utf-8 sequence from input record
			// for i := range record {
			// 	record[i] = strings.ToValidUTF8(record[i], "")
			// }
			select {
			case computePipesInputCh <- record:
			case <-cpCtx.Done:
				log.Println("loading input row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}

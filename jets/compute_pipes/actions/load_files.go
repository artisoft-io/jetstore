package actions

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
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
		cpCtx.CpConfig, cpCtx.EnvSettings, cpCtx.FileKeyComponents)

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

	// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT",len(cpCtx.PartFileKeyComponents))
	inputColumns := cpCtx.InputColumns[:len(cpCtx.InputColumns)-len(cpCtx.PartFileKeyComponents)]
	parquetReader, err = goparquet.NewFileReader(fileHd, inputColumns...)
	if err != nil {
		return 0, err
	}

	var inputRowCount int64
	var record []interface{}
	currentLineNumber := 0
	isShardingMode := cpCtx.CpipesMode == "sharding"
	for {
		// read and put the rows into computePipesInputCh
		currentLineNumber += 1
		err = nil

		record = make([]interface{}, len(cpCtx.InputColumns))
		var parquetRow map[string]interface{}
		parquetRow, err = parquetReader.NextRow()
		if err == nil {
			for i := range inputColumns {
				rawValue := parquetRow[cpCtx.InputColumns[i]]
				if isShardingMode {
					if rawValue == nil {
						record[i] = ""
					} else {
						// Read all fields as string
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
							record[i] = fmt.Sprintf("%v", rawValue)
						}
					}
				} else {
					// Read fields and preserve their types
					// NOTES: Dates are saved as strings, must be converted to dates as needed downstream
					switch vv := rawValue.(type) {
					case []byte:
						record[i] = string(vv)
					case float32:
						record[i] = float64(vv)
					default:
						record[i] = vv
					}
				}
			}
			// Add the columns from the partfile_key_component
			if len(cpCtx.PartFileKeyComponents) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range cpCtx.PartFileKeyComponents {
					for j := range cpCtx.PartFileKeyComponents {
						if cpCtx.InputColumns[offset+j] == cpCtx.PartFileKeyComponents[i].ColumnName {
							result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKey)
							if len(result) > 0 {
								record[offset+j] = result[1]
							}
							// log.Println("**!@@ partfile_key_component Got result",result,"@column_name:",cpCtx.PartFileKeyComponents[i].ColumnName,"file_key:",filePath.InFileKey)
							break
						}
						log.Println("*** WARNING *** partfile_key_component not configure properly, column not found!!")
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

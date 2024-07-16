package actions

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	goparquet "github.com/fraugster/parquet-go"
	"github.com/golang/snappy"
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
		close(computePipesInputCh)
	}()

	// Start the Compute Pipes async
	// Note: when nbrShards > 1, cpipes does not work in local mode in apiserver yet
	go compute_pipes.StartComputePipes(dbpool, cpCtx.NodeId, cpCtx.InputColumns, cpCtx.Done, cpCtx.ErrCh, computePipesInputCh, cpCtx.ChResults,
		cpCtx.CpConfig, cpCtx.EnvSettings, cpCtx.FileKeyComponents)

	// Load the files
	var count, totalRowCount int64
	var err error
	for localInFile := range cpCtx.FileNamesCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Loading file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile)
		}
		if strings.HasSuffix(localInFile.InFileKey, ".csv") {
			count, err = cpCtx.ReadCsvFile(&localInFile, computePipesInputCh)
		} else {
			count, err = cpCtx.ReadParquetFile(&localInFile, computePipesInputCh)
		}
		totalRowCount += count
		if err != nil {
			log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "loadFile2Db returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0}
}

func (cpCtx *ComputePipesContext) ReadParquetFile(filePath *FileName, computePipesInputCh chan<- []interface{}) (int64, error) {
	var fileHd *os.File
	var parquetReader *goparquet.FileReader
	var err error
	samplingRate := cpCtx.CpConfig.ClusterConfig.SamplingRate

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
	isShardingMode := cpCtx.CpipesMode == "sharding"
	for {
		// read and put the rows into computePipesInputCh
		err = nil
		var parquetRow map[string]interface{}
		parquetRow, err = parquetReader.NextRow()
		if err == nil {
			cpCtx.SamplingCount += 1
			if samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
			cpCtx.SamplingCount = 0
			record = make([]interface{}, len(cpCtx.InputColumns))
			for i := range inputColumns {
				rawValue := parquetRow[cpCtx.InputColumns[i]]
				if isShardingMode {
					if rawValue != nil {
						// Read all fields as string
						switch vv := rawValue.(type) {
						case string:
							if len(vv) > 0 {
								record[i] = vv
							} 
						case []byte:
							if len(vv) > 0 {
								record[i] = string(vv)
							}
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
							if vv {
								record[i] = "1"	
							} else {
								record[i] = "0"
							}
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
						log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "*WARNING* partfile_key_component not configure properly, column not found!!")
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
				log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "loading input row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}

func (cpCtx *ComputePipesContext) ReadCsvFile(filePath *FileName, computePipesInputCh chan<- []interface{}) (int64, error) {
	var fileHd *os.File
	var csvReader *csv.Reader
	var err error
	samplingRate := cpCtx.CpConfig.ClusterConfig.SamplingRate

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
	csvReader = csv.NewReader(snappy.NewReader(fileHd))

	var inputRowCount int64
	var inRow []string
	var record []interface{}
	for {
		// read and put the rows into computePipesInputCh
		err = nil
		inRow, err = csvReader.Read()
		if err == nil {
			cpCtx.SamplingCount += 1
			if samplingRate > 0 && cpCtx.SamplingCount < samplingRate {
				continue
			}
			cpCtx.SamplingCount = 0
			record = make([]interface{}, len(cpCtx.InputColumns))
			for i := range inputColumns {
				if len(inRow[i]) == 0 {
					record[i] = nil
				} else {
					record[i] = inRow[i]
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

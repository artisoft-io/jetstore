package compute_pipes

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Load multipart files to JetStore, file to load are provided by channel fileNameCh
var (
	ErrKillSwitch     = errors.New("ErrKillSwitch")
	ComputePipesStart = time.Now()
)

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
		close(cpCtx.ChResults.LoadFromS3FilesResultCh)
	}()

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, computePipesInputCh)

	// Load the files
	var count, totalRowCount int64
	var err error
	inputFormat := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputFormat
	for localInFile := range cpCtx.FileNamesCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Loading file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKey)
		}
		switch inputFormat {
		case "csv", "headerless_csv", "compressed_csv", "compressed_headerless_csv":
			count, err = cpCtx.ReadCsvFile(&localInFile, inputFormat, computePipesInputCh)
		case "parquet", "parquet_select":
			count, err = cpCtx.ReadParquetFile(&localInFile, computePipesInputCh)
		default:
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "error: unsupported file format: %s", inputFormat)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
		totalRowCount += count
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loadFile2Db returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0, Err: err}
			return
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: 0}
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
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	inputColumns := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)]
	parquetReader, err = goparquet.NewFileReader(fileHd, inputColumns...)
	if err != nil {
		return 0, err
	}
	// Prepare the extended columns from partfile_key_component
	var extColumns []string
	if len(cpCtx.PartFileKeyComponents) > 0 {
		extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
		for i := range cpCtx.PartFileKeyComponents {
			result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKey)
			if len(result) > 0 {
				extColumns[i] = result[1]
			}
		}
	}

	var inputRowCount int64
	var record []interface{}
	isShardingMode := cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding"
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
			record = make([]interface{}, nbrColumns)
			for i := range inputColumns {
				rawValue := parquetRow[inputColumns[i]]
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
							record[i] = strconv.FormatFloat(vv, 'G', -1, 64)
						case float32:
							record[i] = strconv.FormatFloat(float64(vv), 'G', -1, 32)
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
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
		}

		// Kill Switch - prevent lambda timeout
		if cpCtx.CpConfig.ClusterConfig.KillSwitchMin > 0 &&
			time.Since(ComputePipesStart).Minutes() >= float64(cpCtx.CpConfig.ClusterConfig.KillSwitchMin) {
				return inputRowCount, ErrKillSwitch
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
			// log.Println(cpCtx.SessionId,"node",cpCtx.NodeId, "push record to computePipesInputCh")
			select {
			case computePipesInputCh <- record:
			case <-cpCtx.Done:
				log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "loading input row from file interrupted")
				return inputRowCount, nil
			}
			inputRowCount += 1
		}
	}
}

func (cpCtx *ComputePipesContext) ReadCsvFile(filePath *FileName, inputFormat string, computePipesInputCh chan<- []interface{}) (int64, error) {
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
	nbrColumns := len(cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
	var inputColumns []string
	var extColumns []string
	switch cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// input columns include the partfile_key_component
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns[:nbrColumns-len(cpCtx.PartFileKeyComponents)]
		// Prepare the extended columns from partfile_key_component
		if len(cpCtx.PartFileKeyComponents) > 0 {
			extColumns = make([]string, len(cpCtx.PartFileKeyComponents))
			for i := range cpCtx.PartFileKeyComponents {
				result := cpCtx.PartFileKeyComponents[i].Regex.FindStringSubmatch(filePath.InFileKey)
				if len(result) > 0 {
					extColumns[i] = result[1]
				}
			}
		}
	case "reducing":
		inputColumns = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns
	default:
		return 0, fmt.Errorf("error: unknown cpipes mode in ReadCsvFile: %s", cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode)
	}

	switch inputFormat {
	case "csv":
		csvReader = csv.NewReader(fileHd)
		// skip header row (first row)
		_, err = csvReader.Read()
	case "compressed_csv":
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
		// skip header row (first row)
		_, err = csvReader.Read()
	case "hearderless_csv":
		csvReader = csv.NewReader(fileHd)
	case "compressed_headerless_csv":
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
	default:
		return 0, fmt.Errorf("error: unknown input format in ReadCsvFile: %s", inputFormat)
	}
	if err == io.EOF {
		// empty file
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("error while reading first input records: %v", err)
	}

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
			record = make([]interface{}, nbrColumns)
			for i := range inputColumns {
				if len(inRow[i]) == 0 {
					record[i] = nil
				} else {
					record[i] = inRow[i]
				}
			}
			// Add the columns from the partfile_key_component
			if len(extColumns) > 0 {
				offset := len(inputColumns)
				// log.Println("**!@@",cpCtx.SessionId,"partfile_key_component GOT[0]",cpCtx.PartFileKeyComponents[0].ColumnName,"offset",offset,"InputColumn",cpCtx.InputColumns[offset])
				for i := range extColumns {
					record[offset+i] = extColumns[i]
				}
			}
		}

		// PUT KILL SWITCH HERE

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

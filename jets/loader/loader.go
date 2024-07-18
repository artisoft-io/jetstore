package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Main loader functions

// Define an enum indicating the encoding of the input file
type InputEncoding int64

const (
	Unspecified InputEncoding = iota
	Csv
	HeaderlessCsv
	FixedWidth
	Parquet
	ParquetSelect
	Xlsx
	HeaderlessXlsx
)

func (s InputEncoding) String() string {
	switch s {
	case Csv:
		return "csv"
	case HeaderlessCsv:
		return "headerless_csv"
	case FixedWidth:
		return "fixed_width"
	case Parquet:
		return "parquet"
	case ParquetSelect:
		return "parquet_select"
	case Xlsx:
		return "xlsx"
	case HeaderlessXlsx:
		return "headerless_xlsx"
	}
	return "unspecified"
}

func InputEncodingFromString(ie string) InputEncoding {
	switch ie {
	case "csv":
		return Csv
	case "headerless_csv":
		return HeaderlessCsv
	case "fixed_width":
		return FixedWidth
	case "parquet":
		return Parquet
	case "parquet_select":
		return ParquetSelect
	case "xlsx":
		return Xlsx
	case "headerless_xlsx":
		return HeaderlessXlsx
	case "unspecified":
		return Unspecified
	}
	return Unspecified
}

var inputFileEncoding InputEncoding

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool *pgxpool.Pool, done chan struct{}, errCh chan error, headersFileCh, fileNamesCh <-chan string,
	errFileHd *os.File) (headersDKInfo *schema.HeadersAndDomainKeysInfo, chResults *compute_pipes.ChannelResults) {
	chResults = &compute_pipes.ChannelResults{
		// NOTE: 101 is the limit of nbr of output table
		// NOTE: 10 is the limit of nbr of splitter operators
		LoadFromS3FilesResultCh: make(chan compute_pipes.LoadFromS3FilesResult, 1),
		Copy2DbResultCh:         make(chan chan compute_pipes.ComputePipesResult, 101),
		WritePartitionsResultCh: make(chan chan compute_pipes.ComputePipesResult, 10),
	}
	var rawHeaders *[]string
	var headersFile string
	var fixedWidthColumnPrefix string
	var err error

	// Setup a writer for error file (bad records)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	// Get the file name to get the headers from
	select {
	case headersFile = <-headersFileCh:
		if len(headersFile) == 0 {
			// No header file, therefore no input file
			goto doNothingExit
		}
		log.Printf("Reading headers from file %s", headersFile)
	case <-time.After(2 * time.Minute):
		err = fmt.Errorf("unable to get the header file name")
		goto gotError
	}

	switch inputFileEncoding {
	case Csv, HeaderlessCsv:
		rawHeaders, err = getRawHeadersCsv(headersFile)
		if err != nil {
			err = fmt.Errorf("while getting csv headers: %v", err)
			goto gotError
		}

		// Write raw header to error file
		for i := range *rawHeaders {
			if i > 0 {
				_, err = badRowsWriter.WriteRune(rune(sep_flag))
				if err != nil {
					err = fmt.Errorf("while writing csv headers to err file: %v", err)
					goto gotError
				}
			}
			_, err = badRowsWriter.WriteString((*rawHeaders)[i])
			if err != nil {
				err = fmt.Errorf("while writing csv headers to err file: %v", err)
				goto gotError
			}
		}
		_, err = badRowsWriter.WriteRune('\n')
		if err != nil {
			err = fmt.Errorf("while writing csv headers to err file (2): %v", err)
			goto gotError
		}

	case FixedWidth:
		// Get the headers from the header spec (input_columns_positions_csv)
		rawHeaders, fixedWidthColumnPrefix, err = getFixedWidthFileHeaders()
		if err != nil {
			goto gotError
		}
		log.Println("Input columns (rawHeaders) for fixed-width schema:", rawHeaders)

	case Parquet:
		// Get the file headers from the parquet schema
		rawHeaders, err = getRawHeadersParquet(headersFile)
		if err != nil {
			err = fmt.Errorf("while reading parquet headers: %v", err)
			goto gotError
		}

	case ParquetSelect:
		h := make([]string, 0)
		rawHeaders = &h
		err = json.Unmarshal([]byte(inputColumnsJson), rawHeaders)
		if err != nil {
			err = fmt.Errorf("while parsing inputColumnsJson using json parser: %v", err)
			goto gotError
		}
		// Make sure we don't have empty names in rawHeaders
		adjustFillers(rawHeaders)
		log.Println("Got input columns (rawHeaders) from json:", rawHeaders)

	case Xlsx, HeaderlessXlsx:
		// Parse the file type specific options
		err = parseInputFormatDataXlsx(&inputFormatDataJson)
		if err != nil {
			err = fmt.Errorf("while parsing input_data_format_json for xlsx files: %v", err)
			goto gotError
		}
		rawHeaders, err = getRawHeadersXlsx(headersFile)
		if err != nil {
			goto gotError
		}
	}

	// Contruct the domain keys based on domainKeysJson
	// ---------------------------------------
	headersDKInfo, err = schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		err = fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
		goto gotError
	}

	err = headersDKInfo.InitializeStagingTable(rawHeaders, *objectType, &domainKeysJson, fixedWidthColumnPrefix)
	if err != nil {
		err = fmt.Errorf("while calling InitializeStagingTable: %v", err)
		goto gotError
	}

	if jetsDebug >= 2 {
		fmt.Println("Domain Keys Info for table", tableName)
		fmt.Println(headersDKInfo)
	}

	// prepare staging table
	if cpipesMode == "loader" {
		err = prepareStagingTable(dbpool, headersDKInfo, tableName)
		if err != nil {
			goto gotError
		}
	}

	// read the rest of the file(s)
	// ---------------------------------------
	loadFiles(dbpool, headersDKInfo, done, errCh, fileNamesCh, chResults, badRowsWriter)
	// All good!
	return

doNothingExit:
	log.Println("processFile: no files to load, exiting ***", err)
	chResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{}
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	close(done)
	return

gotError:
	log.Println("processFile: gotError prior to loadFiles***", err)
	chResults.LoadFromS3FilesResultCh <- compute_pipes.LoadFromS3FilesResult{Err: err}
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	errCh <- err
	close(done)
	return

}

// processFileAndReportStatus is a wrapper around processFile to report error
func processFileAndReportStatus(dbpool *pgxpool.Pool,
	done chan struct{}, errCh chan error, headersFileCh, fileNamesCh <-chan string,
	downloadS3ResultCh <-chan DownloadS3Result, inFolderPath string, errFileHd *os.File) (bool, error) {

	headersDKInfo, chResults := processFile(dbpool, done, errCh, headersFileCh, fileNamesCh, errFileHd)

	downloadResult := <-downloadS3ResultCh
	err := downloadResult.err
	log.Println("Downloaded", downloadResult.InputFilesCount, "files from s3", downloadResult.err)
	if downloadResult.err != nil {
		processingErrors = append(processingErrors, downloadResult.err.Error())
	}

	log.Println("**!@@ CP RESULT = Loaded from s3:")
	loadFromS3FilesResult := <-chResults.LoadFromS3FilesResultCh
	log.Println("Loaded", loadFromS3FilesResult.LoadRowCount, "rows from s3 files with", loadFromS3FilesResult.BadRowCount, "bad rows", loadFromS3FilesResult.Err)
	if loadFromS3FilesResult.Err != nil {
		processingErrors = append(processingErrors, loadFromS3FilesResult.Err.Error())
		if err == nil {
			err = loadFromS3FilesResult.Err
		}
	}
	// log.Println("**!@@ CP RESULT = Loaded from s3: DONE")
	log.Println("**!@@ CP RESULT = Copy2DbResultCh:")
	var outputRowCount int64
	for table := range chResults.Copy2DbResultCh {
		// log.Println("**!@@ Read table results:")
		for copy2DbResult := range table {
			outputRowCount += copy2DbResult.CopyRowCount
			log.Println("**!@@ Inserted", copy2DbResult.CopyRowCount, "rows in table", copy2DbResult.TableName, "::", copy2DbResult.Err)
			if copy2DbResult.Err != nil {
				processingErrors = append(processingErrors, copy2DbResult.Err.Error())
				if err == nil {
					err = copy2DbResult.Err
				}
			}
		}
	}
	log.Println("**!@@ CP RESULT = Copy2DbResultCh: DONE")

	// registering the load
	// ---------------------------------------
	status := "completed"
	if loadFromS3FilesResult.BadRowCount > 0 || err != nil {
		status = "errors"
		processingErrors = append(processingErrors, fmt.Sprintf("File contains %d bad rows", loadFromS3FilesResult.BadRowCount))
		if err != nil {
			status = "failed"
			if cpipesMode == "loader" {
				// loader in classic mode, we don't want to fail (panic) the task
				err = nil
			}
		}
	}
	var errMessage string
	if len(processingErrors) > 0 {
		errMessage = strings.Join(processingErrors, ",")
		log.Println(errMessage)
	}

	dimentions := &map[string]string{
		"client":      *client,
		"object_type": *objectType,
	}
	if status == "failed" {
		awsi.LogMetric(*failedMetric, dimentions, 1)
	} else {
		awsi.LogMetric(*completedMetric, dimentions, 1)
	}
	objectTypes := make([]string, 0)
	if headersDKInfo != nil {
		for objType := range headersDKInfo.DomainKeysInfoMap {
			objectTypes = append(objectTypes, objType)
		}
	}
	if *pipelineExecKey == -1 {
		// Loader mode (loaderSM), register with loader_execution_status table
		log.Println("Loader mode (loaderSM), register with loader_execution_status table")
		err2 := registerCurrentLoad(loadFromS3FilesResult.LoadRowCount, loadFromS3FilesResult.BadRowCount,
			dbpool, objectTypes, tableName, status, errMessage)
		if err2 != nil {
			return false, fmt.Errorf("error while registering the load (loaderSM): %v", err2)
		}
	} else {
		// CPIPES mode (cpipesSM), register the result of this shard with pipeline_execution_details
		err2 := updatePipelineExecutionStatus(dbpool, int(loadFromS3FilesResult.LoadRowCount), int(outputRowCount), status, errMessage)
		if err2 != nil {
			return false, fmt.Errorf("error while registering the load (cpipesSM): %v", err2)
		}
	}

	return loadFromS3FilesResult.BadRowCount > 0, err
}

func coordinateWork() error {
	// open db connections
	// ---------------------------------------
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsnStr, err := awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, 10)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
		dsn = &dsnStr
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Make sure the jetstore schema exists
	// ---------------------------------------
	tblExists, err := schema.TableExists(dbpool, "jetsapi", "input_loader_status")
	if err != nil {
		return fmt.Errorf("while verifying the jetstore schema: %v", err)
	}
	if !tblExists {
		return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
	}

	// Get pipeline exec info when peKey is provided
	// ---------------------------------------
	if *pipelineExecKey > -1 {
		log.Println("CPIPES, loading pipeline configuration")
		var fkey sql.NullString
		stmt := `
		SELECT	ir.client, ir.org, ir.object_type, ir.file_key, ir.source_period_key, 
			pe.pipeline_config_key, pe.process_name, pe.input_session_id, pe.session_id, pe.user_email
		FROM 
			jetsapi.pipeline_execution_status pe,
			jetsapi.input_registry ir
		WHERE pe.main_input_registry_key = ir.key
			AND pe.key = $1`
		err = dbpool.QueryRow(context.Background(), stmt, *pipelineExecKey).Scan(client, clientOrg, objectType, &fkey, sourcePeriodKey,
			&pipelineConfigKey, &processName, &inputSessionId, sessionId, userEmail)
		if err != nil {
			return fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
		}
		if !fkey.Valid {
			return fmt.Errorf("error, file_key is NULL in input_registry table")
		}
		*inFile = fkey.String
		log.Println("Updated argument: client", *client)
		log.Println("Updated argument: org", *clientOrg)
		log.Println("Updated argument: objectType", *objectType)
		log.Println("Updated argument: sourcePeriodKey", *sourcePeriodKey)
		log.Println("Updated argument: inputSessionId", inputSessionId)
		log.Println("Updated argument: sessionId", *sessionId)
		log.Println("Updated argument: inFile", *inFile)
	}
	// Extract processing date from file key inFile
	fileKeyComponents = make(map[string]interface{})
	fileKeyComponents = datatable.SplitFileKeyIntoComponents(fileKeyComponents, inFile)
	year := fileKeyComponents["year"].(int)
	month := fileKeyComponents["month"].(int)
	day := fileKeyComponents["day"].(int)
	fileKeyDate = time.Date(year, time.Month(month), day, 14, 0, 0, 0, time.UTC)
	log.Println("fileKeyDate:", fileKeyDate)

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, *sessionId)
	if err != nil {
		return fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return fmt.Errorf("error: the session id is already used")
	}

	// Get source_config info: DomainKeysJson, tableName, input_format, is_part_files from source_config table
	// ---------------------------------------
	var dkJson, cnJson, ifJson, fwCsv, cpJson sql.NullString
	err = dbpool.QueryRow(context.Background(),
		`SELECT table_name, domain_keys_json, input_columns_json, input_columns_positions_csv ,
		input_format, is_part_files, input_format_data_json, compute_pipes_json
		  FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3`,
		*client, *clientOrg, *objectType).Scan(&tableName, &dkJson, &cnJson, &fwCsv, &inputFormat, &isPartFiles, &ifJson, &cpJson)
	if err != nil {
		return fmt.Errorf("query table_name, domain_keys_json, input_columns_json, input_columns_positions_csv, input_format_data_json from jetsapi.source_config failed: %v", err)
	}
	if cnJson.Valid && fwCsv.Valid {
		return fmt.Errorf("error, cannot specify both input_columns_json and input_columns_positions_csv")
	}
	if dkJson.Valid {
		domainKeysJson = dkJson.String
	}
	if ifJson.Valid {
		inputFormatDataJson = ifJson.String
	}
	inputFileEncoding = InputEncodingFromString(inputFormat)
	switch {
	case cnJson.Valid:
		inputColumnsJson = cnJson.String
		if inputFileEncoding == Unspecified {
			inputFileEncoding = HeaderlessCsv
		}
	case fwCsv.Valid:
		inputColumnsPositionsCsv = fwCsv.String
		if inputFileEncoding == Unspecified {
			inputFileEncoding = FixedWidth
		}
	case inputFileEncoding == Unspecified:
		// For backward compatibility
		inputFileEncoding = Csv
	}
	//* REMOVE THIS
	// if cpJson.Valid && len(cpJson.String) > 0 {
	// 	cpConfig, err = compute_pipes.UnmarshalComputePipesConfig(&cpJson.String, *shardId, *nbrShards)
	// 	if err != nil {
	// 		log.Println(fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err))
	// 		return fmt.Errorf("error while UnmarshalComputePipesConfig: %v", err)
	// 	}
	// 	log.Println("This loader contains Compute Pipes configuration")
	// 	cc := cpConfig.ClusterConfig
	// 	log.Println("CP Config: nodeId:",cc.NodeId,"scNodeId:",cc.SubClusterNodeId,"scId:", cc.SubClusterId, "nbrNodes:",cc.NbrNodes,"nbrSc:",cc.NbrSubClusters, "nbrScNodes:",cc.NbrSubClusterNodes)
	// }

	log.Printf("Input file encoding (format) is: %s", inputFileEncoding.String())

	if cpConfig != nil && *pipelineExecKey == -1 && isPartFiles == 1 {
		// Case loader mode (loaderSM) with multipart files, save the file keys to compute_pipes_shard_registry
		// and register the load to kick off cpipesSM
		nkeys, err := compute_pipes.ShardFileKeys(context.Background(), dbpool, *inFile, *sessionId, cpConfig.ClusterConfig)
		if err != nil {
			return fmt.Errorf("while sharding file keys for multipart file load: %v", err)
		}
		// Register the load and kick off the automated cpipes pipeline
		err = registerCurrentLoad(int64(nkeys), 0, dbpool, []string{*objectType}, tableName, "completed", "")
		if err != nil {
			return fmt.Errorf("error while registering the load: %v", err)
		}
		return nil
	}

	// Global var cpipesMode string,  values: loader, pre-sharding, sharding, reducing, standalone.
	// Processing file(s), invoking the loader process in loop when processing
	// multiple folders of multipart files (case cpipesSM in mode reduce)
	// Scenario:
	//	- loader classic (loaderSM) case *pipelineExecKey == -1 && computePipesJson empty
	//		Single call to processComputeGraph with inFile

	//	- loader cpipesSM standalone case *pipelineExecKey == -1 && isPartFiles == 0 && computePipesJson not empty
	//		Single call to processComputeGraph with inFile (single file for now)

	//	- loader cpipesSM pre-sharding: case *pipelineExecKey == -1 && isPartFiles == 1 && computePipesJson not empty
	//		Handled above, no invocation of processComputeGraph

	//	- loader cpipesSM sharding: case *pipelineExecKey > -1 && isPartFiles == 1 && computePipesJson not empty && jetsPartition == "",
	//		Single invokation of processComputeGraph with all file keys
	//		Note: when cpipesShardWithNoFileKeys == true, fileKeys will contain a single file to use for headers only

	//	- loader cpipesSM reducing: case *pipelineExecKey > -1 && isPartFiles == 1 && computePipesJson not empty && jetsPartition != "",
	//		Invoke of processComputeGraph for each file key, update inFile with file key to process
	//		Note: when cpipesShardWithNoFileKeys == true, fileKeys will contain a single file to use for headers only
	//		and will be set to inFile for fetching a single data file to use for headers only

	cpipesFileKeys = make([]string, 0)
	switch {
	case *pipelineExecKey == -1 && cpConfig == nil:
		// loader classic (loaderSM)
		cpipesMode = "loader"
		log.Printf("CPIPES Mode: %s", cpipesMode)
		return processComputeGraph(dbpool)

	//* REMOVE THIS
	// case *pipelineExecKey == -1 && isPartFiles == 0 && cpConfig != nil:
	// 	// loader cpipesSM standalone
	// 	cpipesMode = "standalone"
	// 	log.Printf("CPIPES Mode: %s", cpipesMode)
	// 	return processComputeGraph(dbpool)

	// case *pipelineExecKey == -1 && isPartFiles == 1 && cpConfig != nil:
	// 	// loader cpipesSM pre-sharding: handled above
	// 	return nil

	// case *pipelineExecKey > -1 && isPartFiles == 1 && cpConfig != nil:
	// 	// loader cpipes mode "sharding" (jetsPartition == "") or "reducing" (jetsPartition != "")
	// 	// Get the file keys from compute_pipes_shard_registry table
	// 	fileKeys, err := getFileKeys(dbpool, inputSessionId, cpConfig, *jetsPartition)
	// 	if err != nil || fileKeys == nil {
	// 		return fmt.Errorf("failed to get list of files from compute_pipes_shard_registry table: %v", err)
	// 	}
	// 	if cpipesShardWithNoFileKeys {
	// 		log.Printf("**!@@ Got no file keys for shardId %d, continue to participate in distribute_data", *shardId)
	// 	} else {
	// 		log.Printf("**!@@ Got %d file keys from database for shardId %d and jets_partition %s", len(fileKeys), *shardId, *jetsPartition)
	// 	}
	// 	//* TODO Cleanup now that we use cpipes_booter, always run cpipesMode as "sharding", meaning getting file keys from registry
	// 	cpipesFileKeys = fileKeys
	// 	cpipesMode = "sharding"
	// 	log.Printf("CPIPES Mode: %s", cpipesMode)
	// 	return processComputeGraph(dbpool)

	default:
		msg := "error: unexpected schenario: pipelineExecKey = %d && isPartFiles = %d && cpConfig = nil"
		log.Printf(msg, *pipelineExecKey, isPartFiles)
		return fmt.Errorf(msg, *pipelineExecKey, isPartFiles)
	}
}

func processComputeGraph(dbpool *pgxpool.Pool) (err error) {

	// Start the download of file(s) from s3 and upload to db, coordinated using channel
	done := make(chan struct{})
	errCh := make(chan error, 1)
	defer func() {
		select {
		case <-done:
			// done chan is already closed due to error
		default:
			close(done)
		}
	}()

	// Open the error file
	var errFileHd *os.File
	errFileHd, err = os.CreateTemp("", "jetstore_err")
	if err != nil {
		return fmt.Errorf("failed to open temp error file: %v", err)
	}
	// fmt.Println("Temp error file name:", errFileHd.Name())
	defer os.Remove(errFileHd.Name())

	headersFileCh, fileNamesCh, downloadS3ResultCh, inFolderPath, err := downloadS3Files(done)
	if err != nil {
		return fmt.Errorf("failed to setup the download of input file(s): %v", err)
	}
	defer os.Remove(inFolderPath)

	// Process the downloaded file(s)
	hasBadRows, err := processFileAndReportStatus(dbpool, done, errCh, headersFileCh, fileNamesCh, downloadS3ResultCh, inFolderPath, errFileHd)
	if err != nil {
		return err
	}

	if hasBadRows {

		// aws integration: Copy the error file to awsBucket
		errFileHd.Seek(0, 0)

		// Create the error file key
		dp, fn := filepath.Split(*inFile)
		errFileKey := dp + "err_" + fn
		err = awsi.UploadToS3(*awsBucket, *awsRegion, errFileKey, errFileHd)
		if err != nil {
			return fmt.Errorf("failed to upload error file: %v", err)
		}
	}
	return nil
}

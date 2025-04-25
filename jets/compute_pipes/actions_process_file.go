package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// From loader's processFileAndReportStatus

func (cpCtx *ComputePipesContext) ProcessFilesAndReportStatus(ctx context.Context, dbpool *pgxpool.Pool,
	inFolderPath string) error {

	cpCtx.ChResults = &ChannelResults{
		// NOTE: 101 is the limit of nbr of output table
		// NOTE: 10 is the limit of nbr of splitter operators
		LoadFromS3FilesResultCh: make(chan LoadFromS3FilesResult, 1),
		Copy2DbResultCh:         make(chan chan ComputePipesResult, 101),
		WritePartitionsResultCh: make(chan chan ComputePipesResult, 10),
		S3PutObjectResultCh:     make(chan ComputePipesResult, 1),
		JetrulesWorkerResultCh:  make(chan chan JetrulesWorkerResult, 99),
		ClusteringResultCh:      make(chan chan ClusteringResult, 99),
	}

	key, err := cpCtx.InsertPipelineExecutionStatus(dbpool)
	if err != nil {
		return fmt.Errorf("error while inserting the load registry (cpipesSM): %v", err)
	}

	// read the file(s) or merge them depending on the main pipe
	// --------------------
	processingErrors := make([]string, 0)
	if cpCtx.ComputePipesArgs.MergeFiles {
		// Last step, merging all the part files into a single output file
		// Special case, we're not calling StartComputePipes, so need to close
		// ChResults channels
		close(cpCtx.ChResults.LoadFromS3FilesResultCh)
		close(cpCtx.ChResults.Copy2DbResultCh)
		close(cpCtx.ChResults.WritePartitionsResultCh)
		close(cpCtx.ChResults.S3PutObjectResultCh)
		close(cpCtx.ChResults.JetrulesWorkerResultCh)
		close(cpCtx.ChResults.ClusteringResultCh)
		err = cpCtx.StartMergeFiles(dbpool)
	} else {
		err = cpCtx.LoadFiles(ctx, dbpool)
	}
	if err != nil {
		processingErrors = append(processingErrors, err.Error())
	}

	// // Collect the results of each pipes and save it to database
	// saveResultsCtx := NewSaveResultsContext(dbpool)
	// saveResultsCtx.JetsPartition = cpCtx.JetsPartitionLabel
	// saveResultsCtx.NodeId = cpCtx.NodeId
	// saveResultsCtx.SessionId = cpCtx.SessionId

	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println(cpCtx.SessionId, "**!@@ CP RESULT = Downloaded from s3:")
	}
	var totalInputFileSize int64
	var totalInputFileCount int
	for downloadResult := range cpCtx.DownloadS3ResultCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "Downloaded", downloadResult.InputFilesCount,
				"files from s3, total size:", float64(downloadResult.TotalFilesSize)/1024/1024, "MB, err:", downloadResult.Err)
		}
		totalInputFileSize += downloadResult.TotalFilesSize
		totalInputFileCount += downloadResult.InputFilesCount
		// var r *ComputePipesResult
		// r = &ComputePipesResult{
		// 	TableName:    "Downloaded files from s3",
		// 	CopyRowCount: int64(downloadResult.InputFilesCount),
		// 	Err:          downloadResult.Err,
		// }
		// saveResultsCtx.Save("S3 Download", r)
		if downloadResult.Err != nil {
			err = downloadResult.Err
			processingErrors = append(processingErrors, downloadResult.Err.Error())
		}
	}

	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CP RESULT = Loaded from s3:")
	}
	var loadedRowCount int
	for loadFromS3FilesResult := range cpCtx.ChResults.LoadFromS3FilesResultCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "Loaded", loadFromS3FilesResult.LoadRowCount,
				"rows from s3 files with", loadFromS3FilesResult.BadRowCount, "bad rows", loadFromS3FilesResult.Err)
		}
		// r = &ComputePipesResult{
		// 	TableName:    "Loaded rows from s3 files",
		// 	CopyRowCount: loadFromS3FilesResult.LoadRowCount,
		// 	Err:          loadFromS3FilesResult.Err,
		// }
		// saveResultsCtx.Save("S3 Readers", r)
		loadedRowCount += int(loadFromS3FilesResult.LoadRowCount)
		if loadFromS3FilesResult.Err != nil {
			processingErrors = append(processingErrors, loadFromS3FilesResult.Err.Error())
			if err == nil {
				err = loadFromS3FilesResult.Err
			}
		}
	}
	
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CHECKING jetrules results from JetrulesWorkerResultCh:")
	}
	// get jetrules results from JetrulesWorkerResultCh
	var reteSessionCount int64
	var reteSessionErrors int64
	for workerResultCh := range cpCtx.ChResults.JetrulesWorkerResultCh {
		for jrResults := range workerResultCh {
			reteSessionCount += jrResults.ReteSessionCount
			reteSessionErrors += jrResults.ErrorsCount
			if jrResults.Err != nil {
				processingErrors = append(processingErrors, jrResults.Err.Error())
				if err == nil {
					err = jrResults.Err
				}
			}
		}
	}
	if reteSessionErrors > 0 {
		log.Printf("WARNING: rete session got %d data errors\n", reteSessionErrors)
	}

	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CHECKING clustering results from ClusteringResultCh:")
	}
	// get clustering results from ClusteringResultCh
	for clusteringResultCh := range cpCtx.ChResults.ClusteringResultCh {
		for clusteringResult := range clusteringResultCh {
			if clusteringResult.Err != nil {
				processingErrors = append(processingErrors, clusteringResult.Err.Error())
				if err == nil {
					err = clusteringResult.Err
				}
			}
		}
	}
	// log.Println("** CHECKING clustering results DONE :: err?", err)

	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CP RESULT = Copy2DbResultCh:")
	}
	var outputRowCount int64
	for table := range cpCtx.ChResults.Copy2DbResultCh {
		// log.Println("**@= Read table results:")
		for copy2DbResult := range table {
			outputRowCount += copy2DbResult.CopyRowCount
			// saveResultsCtx.Save("DB Inserts", &copy2DbResult)
			// log.Println("**@= Inserted", copy2DbResult.CopyRowCount, "rows in table", copy2DbResult.TableName, "::", copy2DbResult.Err)
			if copy2DbResult.Err != nil {
				processingErrors = append(processingErrors, copy2DbResult.Err.Error())
				if err == nil {
					err = copy2DbResult.Err
				}
			}
		}
	}
	// log.Println("**@= CP RESULT = Copy2DbResultCh: DONE")

	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CP RESULT = WritePartitionsResultCh:")
	}
	for splitter := range cpCtx.ChResults.WritePartitionsResultCh {
		// log.Println("**@= Read SPLITTER ComputePipesResult from writePartitionsResultCh:")
		for partitionWriterResult := range splitter {
			// saveResultsCtx.Save("Jets Partition Writer", &partitionWriterResult)
			outputRowCount += partitionWriterResult.CopyRowCount
			// log.Println("**@= Wrote", partitionWriterResult.CopyRowCount, "rows in", partitionWriterResult.PartsCount, "partfiles for", partitionWriterResult.TableName, "::", partitionWriterResult.Err)
			if partitionWriterResult.Err != nil {
				processingErrors = append(processingErrors, partitionWriterResult.Err.Error())
				if err == nil {
					err = partitionWriterResult.Err
				}
			}
		}
	}
	// log.Println("**@= CP RESULT = WritePartitionsResultCh: DONE")

	// Get the result from S3DeviceManager
	// cpCtx.S3DeviceMgr == nil when cpCtx.ComputePipesArgs.MergeFiles == true
	if cpCtx.S3DeviceMgr != nil {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Println("**@= Waiting on S3DeviceMgr.ClientsWg.Wait")
		}
		cpCtx.S3DeviceMgr.ClientsWg.Wait()
		close(cpCtx.S3DeviceMgr.WorkersTaskCh)
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Println("**@= Waiting on S3DeviceMgr.ClientsWg.Wait DONE")
		}
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("**@= CP RESULT = S3PutObjectResultCh:")
	}
	for s3DeviceManagerResult := range cpCtx.ChResults.S3PutObjectResultCh {
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Put %d part files to s3", cpCtx.SessionId, cpCtx.NodeId, s3DeviceManagerResult.PartsCount)
		}
		if s3DeviceManagerResult.Err != nil {
			processingErrors = append(processingErrors, s3DeviceManagerResult.Err.Error())
			if err == nil {
				err = s3DeviceManagerResult.Err
			}
		}
	}
	// log.Println("**@= CP RESULT = S3PutObjectResultCh DONE")

	// Check for error from compute pipes
	close(cpCtx.ErrCh)
	for cpErr := range cpCtx.ErrCh {
		// got an error during compute pipes processing
		log.Printf("%s node %d got error from Compute Pipes processing: %v", cpCtx.SessionId, cpCtx.NodeId, cpErr)
		if err == nil {
			err = cpErr
		}
		// r = &ComputePipesResult{
		// 	CopyRowCount: loadFromS3FilesResult.LoadRowCount,
		// 	Err:          cpErr,
		// }
		// saveResultsCtx.Save("CP Errors", r)
		processingErrors = append(processingErrors, fmt.Sprintf("got error from Compute Pipes processing: %v", cpErr))
	}

	// registering the load
	// ---------------------------------------
	var status string
	switch {
	case err == nil:
		status = "completed"
	case err == ErrKillSwitch:
		status = "interrupted"
	default:
		status = "failed"
	}
	var errMessage string
	if len(processingErrors) > 0 {
		errMessage = strings.Join(processingErrors, ",")
		log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, errMessage)
	}

	// Register the result of this shard with pipeline_execution_details
	err2 := cpCtx.UpdatePipelineExecutionStatus(dbpool, key,
		loadedRowCount, int(totalInputFileSize/1024/1024), totalInputFileCount,
		int(reteSessionCount), int(outputRowCount), cpCtx.MainInputStepId, status, errMessage)
	if err2 != nil {
		return fmt.Errorf("error while registering the load (cpipesSM): %v", err2)
	}
	return err
}

// Register the CPIPES execution status details to pipeline_execution_details
func (cpCtx *ComputePipesContext) InsertPipelineExecutionStatus(dbpool *pgxpool.Pool) (int, error) {
	// log.Printf("Inserting status 'in progress' to pipeline_execution_details table")
	stmt := `INSERT INTO jetsapi.pipeline_execution_details (
							status, pipeline_config_key, pipeline_execution_status_key, 
							client, process_name, main_input_session_id, session_id, source_period_key,
							shard_id, jets_partition, user_email) 
							VALUES ('in progress', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10) 
							RETURNING key`
	var key int
	err := dbpool.QueryRow(context.Background(), stmt,
		cpCtx.PipelineConfigKey, cpCtx.PipelineExecKey, cpCtx.Client, cpCtx.ProcessName, cpCtx.InputSessionId, cpCtx.SessionId, cpCtx.SourcePeriodKey,
		cpCtx.NodeId, cpCtx.JetsPartitionLabel, cpCtx.UserEmail).Scan(&key)
	if err != nil {
		return 0, fmt.Errorf("error inserting in jetsapi.pipeline_execution_details table: %v", err)
	}
	return key, nil
}
func (cpCtx *ComputePipesContext) UpdatePipelineExecutionStatus(dbpool *pgxpool.Pool, key int, inputRowCount,
	totalFilesSizeMb, inputFilesCount, reteSessionCount, outputRowCount int,
	cpipesStepId, status, errMessage string) error {
	// log.Printf("Updating status '%s' to pipeline_execution_details table", status)
	stmt := `UPDATE jetsapi.pipeline_execution_details SET (
							cpipes_step_id, status, error_message, input_records_count, 
							input_files_size_mb, input_files_count, rete_sessions_count, output_records_count) 
							= ($1, $2, $3, $4, $5, $6, $7, $8) WHERE key = $9`
	_, err := dbpool.Exec(context.Background(), stmt,
		cpipesStepId, status, errMessage, inputRowCount, totalFilesSizeMb, inputFilesCount, reteSessionCount, outputRowCount, key)
	if err != nil {
		return fmt.Errorf("error updating in jetsapi.pipeline_execution_details table: %v", err)
	}
	return nil
}

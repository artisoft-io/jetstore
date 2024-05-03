package actions

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v4/pgxpool"
)

// From loader's processFileAndReportStatus

func (cpCtx *ComputePipesContext) ProcessFilesAndReportStatus(ctx context.Context, dbpool *pgxpool.Pool,
	inFolderPath string) error {

		cpCtx.ChResults = &compute_pipes.ChannelResults{
			// NOTE: 101 is the limit of nbr of output table
			// NOTE: 10 is the limit of nbr of splitter operators
			// NOTE: 5 is the limit of nbr of distribute_data operators
			LoadFromS3FilesResultCh: make(chan compute_pipes.LoadFromS3FilesResult, 1),
			Copy2DbResultCh:         make(chan chan compute_pipes.ComputePipesResult, 101),
			WritePartitionsResultCh: make(chan chan chan compute_pipes.ComputePipesResult, 10),
			MapOnClusterResultCh:    make(chan chan chan compute_pipes.ComputePipesResult, 5),
		}
	
		// read the rest of the file(s)
		// ---------------------------------------
		cpCtx.LoadFiles(ctx, dbpool)
	
		// Collect the results of each pipes and save it to database
		saveResultsCtx := compute_pipes.NewSaveResultsContext(dbpool)
  	saveResultsCtx.JetsPartition = 	cpCtx.JetsPartitionLabel
		saveResultsCtx.NodeId = cpCtx.NodeId
		saveResultsCtx.SessionId = cpCtx.SessionId
	
		downloadResult := <-cpCtx.DownloadS3ResultCh
		err := downloadResult.Err
		// log.Println("Downloaded", downloadResult.InputFilesCount, "files from s3", downloadResult.err)
		r := &compute_pipes.ComputePipesResult{
			TableName:    "Downloaded files from s3",
			CopyRowCount: int64(downloadResult.InputFilesCount),
			Err:          downloadResult.Err,
		}
		processingErrors := make([]string, 0)
		saveResultsCtx.Save("S3 Download", r)
		if downloadResult.Err != nil {
			processingErrors = append(processingErrors, downloadResult.Err.Error())
		}
	
		// log.Println("**!@@ CP RESULT = Loaded from s3:")
		loadFromS3FilesResult := <-cpCtx.ChResults.LoadFromS3FilesResultCh
		// log.Println("Loaded", loadFromS3FilesResult.LoadRowCount, "rows from s3 files with", loadFromS3FilesResult.BadRowCount, "bad rows", loadFromS3FilesResult.Err)
		r = &compute_pipes.ComputePipesResult{
			TableName:    "Loaded rows from s3 files",
			CopyRowCount: loadFromS3FilesResult.LoadRowCount,
			Err:          loadFromS3FilesResult.Err,
		}
		saveResultsCtx.Save("S3 Readers", r)
		if loadFromS3FilesResult.Err != nil {
			processingErrors = append(processingErrors, loadFromS3FilesResult.Err.Error())
			if err == nil {
				err = loadFromS3FilesResult.Err
			}
		}
		// log.Println("**!@@ CP RESULT = Loaded from s3: DONE")
		// log.Println("**!@@ CP RESULT = Copy2DbResultCh:")
		var outputRowCount int64
		for table := range cpCtx.ChResults.Copy2DbResultCh {
			// log.Println("**!@@ Read table results:")
			for copy2DbResult := range table {
				outputRowCount += copy2DbResult.CopyRowCount
				saveResultsCtx.Save("DB Inserts", &copy2DbResult)
				// log.Println("**!@@ Inserted", copy2DbResult.CopyRowCount, "rows in table", copy2DbResult.TableName, "::", copy2DbResult.Err)
				if copy2DbResult.Err != nil {
					processingErrors = append(processingErrors, copy2DbResult.Err.Error())
					if err == nil {
						err = copy2DbResult.Err
					}
				}
			}
		}
		// log.Println("**!@@ CP RESULT = Copy2DbResultCh: DONE")
	
		// log.Println("**!@@ CP RESULT = MapOnClusterResultCh:")
		for mapOn := range cpCtx.ChResults.MapOnClusterResultCh {
			// log.Println("**!@@ Read PEER from MapOnClusterResultCh:")
			for peer := range mapOn {
				// log.Println("**!@@ Read RESULT from MapOnClusterResultCh:")
				for peerResult := range peer {
					saveResultsCtx.Save("Peer Communication", &peerResult)
					// log.Printf("**!@@ PEER COMM %d Rows :: Peer %s :: %v", peerResult.CopyRowCount, peerResult.TableName, peerResult.Err)
					if peerResult.Err != nil {
						processingErrors = append(processingErrors, peerResult.Err.Error())
						if err == nil {
							err = peerResult.Err
						}
					}
				}
			}
		}
		// log.Println("**!@@ CP RESULT = MapOnClusterResultCh: DONE")
	
		// log.Println("**!@@ CP RESULT = WritePartitionsResultCh:")
		for splitter := range cpCtx.ChResults.WritePartitionsResultCh {
			// log.Println("**!@@ Read SPLITTER ComputePipesResult from writePartitionsResultCh:")
			for partition := range splitter {
				// log.Println("**!@@ Read PARTITION ComputePipesResult from writePartitionsResultCh:")
				for partitionWriterResult := range partition {
					saveResultsCtx.Save("Jets Partition Writer", &partitionWriterResult)
					outputRowCount += partitionWriterResult.CopyRowCount
					// log.Println("**!@@ Wrote", partitionWriterResult.CopyRowCount, "rows in", partitionWriterResult.PartsCount, "partfiles for", partitionWriterResult.TableName, "::", partitionWriterResult.Err)
					if partitionWriterResult.Err != nil {
						processingErrors = append(processingErrors, partitionWriterResult.Err.Error())
						if err == nil {
							err = partitionWriterResult.Err
						}
					}
				}
			}
		}
		// log.Println("**!@@ CP RESULT = WritePartitionsResultCh: DONE")
	
		// Check for error from compute pipes
		var cpErr error
		select {
		case cpErr = <-cpCtx.ErrCh:
			// got an error during compute pipes processing
			log.Printf("got error from Compute Pipes processing: %v", cpErr)
			if err == nil {
				err = cpErr
			}
			r = &compute_pipes.ComputePipesResult{
				CopyRowCount: loadFromS3FilesResult.LoadRowCount,
				Err:          cpErr,
			}
			saveResultsCtx.Save("CP Errors", r)
	
			processingErrors = append(processingErrors, fmt.Sprintf("got error from Compute Pipes processing: %v", cpErr))
		default:
			log.Println("No errors from Compute Pipes processing!")
		}
	
		// registering the load
		// ---------------------------------------
		status := "completed"
		if err != nil {
				status = "failed"
		}
		var errMessage string
		if len(processingErrors) > 0 {
			errMessage = strings.Join(processingErrors, ",")
			log.Println(errMessage)
		}

			// CPIPES mode (cpipesSM), register the result of this shard with pipeline_execution_details
			err2 := cpCtx.UpdatePipelineExecutionStatus(dbpool, int(loadFromS3FilesResult.LoadRowCount), int(outputRowCount), status, errMessage)
			if err2 != nil {
				return fmt.Errorf("error while registering the load (cpipesSM): %v", err2)
			}
	
		return err
	}

// Register the CPIPES execution status details to pipeline_execution_details
func (cpCtx *ComputePipesContext) UpdatePipelineExecutionStatus(dbpool *pgxpool.Pool, inputRowCount, outputRowCount int, status, errMessage string) error {
		log.Printf("Inserting status '%s' to pipeline_execution_details table", status)
		stmt := `INSERT INTO jetsapi.pipeline_execution_details (
							pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id, session_id, source_period_key,
							shard_id, jets_partition, status, error_message, input_records_count, rete_sessions_count, output_records_count, user_email) 
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`
		_, err := dbpool.Exec(context.Background(), stmt,
			cpCtx.PipelineConfigKey, cpCtx.PipelineExecKey, cpCtx.Client, cpCtx.ProcessName, cpCtx.InputSessionId, cpCtx.SessionId, cpCtx.SourcePeriodKey,
			cpCtx.NodeId, cpCtx.JetsPartitionLabel, status, errMessage, inputRowCount, 0, outputRowCount, cpCtx.UserEmail)
		if err != nil {
			return fmt.Errorf("error inserting in jetsapi.pipeline_execution_details table: %v", err)
		}
	return nil
}


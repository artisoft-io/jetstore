package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes Actions

func (args *ComputePipesArgs) CoordinateComputePipes(ctx context.Context, dsn string) error {
	var cpErr, err error
	var inFolderPath string
	var cpContext *ComputePipesContext
	var fileKeyComponents map[string]interface{}
	var fileKeyDate time.Time
	var fileKeys []string
	var cpipesConfigJson string
	stmt := "SELECT %s FROM jetsapi.cpipes_execution_status WHERE pipeline_execution_status_key = %d"

	// open db connection
	dbpool, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		cpErr = fmt.Errorf("while opening db connection: %v", err)
		goto gotError
	}
	defer dbpool.Close()

	// Extract processing date from file key inFile
	fileKeyComponents = make(map[string]interface{})
	fileKeyComponents = datatable.SplitFileKeyIntoComponents(fileKeyComponents, &args.FileKey)
	if len(fileKeyComponents) > 0 {
		year := fileKeyComponents["year"].(int)
		month := fileKeyComponents["month"].(int)
		day := fileKeyComponents["day"].(int)
		fileKeyDate = time.Date(year, time.Month(month), day, 14, 0, 0, 0, time.UTC)
		log.Println("fileKeyDate:", fileKeyDate)
	}
	// Get the cpipes config from cpipes_execution_status and file keys from compute_pipes_shard_registry table
	//*NOTE Case reducing, get the file keys from s3
	switch args.CpipesMode {
	case "sharding":
		err = dbpool.QueryRow(ctx, fmt.Sprintf(stmt, "sharding_config_json", args.PipelineExecKey)).Scan(&cpipesConfigJson)
		if err != nil {
			cpErr = fmt.Errorf("while reading cpipes config from cpipes_execution_status table (sharding in CoordinateComputePipes): %v", err)
			goto gotError
		}
		// Case sharding, get the file keys from compute_pipes_shard_registry
		fileKeys, err = GetFileKeys(ctx, dbpool, args.SessionId, args.NodeId)
		if err != nil {
			cpErr = fmt.Errorf("while loading aws configuration (in CoordinateComputePipes): %v", err)
			goto gotError
		}
		log.Printf("**!@@ Got %d file keys from database for nodeId %d (sharding)", len(fileKeys), args.NodeId)

	case "reducing":
		err = dbpool.QueryRow(ctx, fmt.Sprintf(stmt, "reducing_config_json", args.PipelineExecKey)).Scan(&cpipesConfigJson)
		if err != nil {
			cpErr = fmt.Errorf("while reading cpipes config from cpipes_execution_status table (reducing in CoordinateComputePipes): %v", err)
			goto gotError
		}
		// Case cpipes reducing mode, get the file keys from s3
		log.Printf("Getting file keys from s3 folder: %s", args.FileKey)
		s3Objects, err := awsi.ListS3Objects(&args.FileKey)
		if err != nil || s3Objects == nil {
			cpErr = fmt.Errorf("failed to download list of files from s3: %v", err)
			goto gotError
		}
		log.Printf("**!@@ Got %d file keys from database for nodeId %d (reducing)", len(s3Objects), args.NodeId)
		fileKeys = make([]string, 0)
		for i := range s3Objects {
			if s3Objects[i].Size > 0 {
				fileKeys = append(fileKeys, s3Objects[i].Key)
			}
		}

	default:
		cpErr = fmt.Errorf("error: invalid cpipesMode in downloadS3Files: %s", args.CpipesMode)
		goto gotError
	}

	cpContext = &ComputePipesContext{
		ComputePipesArgs: *args,
		EnvSettings: map[string]interface{}{
			"$SESSIONID":            args.SessionId,
			"$FILE_KEY_DATE":        fileKeyDate,
			"$FILE_KEY":             args.FileKey,
			"$FILE_KEY_FOLDER":      args.FileKey, // assuming always partfiles
			"$SHARD_ID":             args.NodeId,
			"$NBR_SHARDS":           args.NbrNodes,
			"$JETS_PARTITION_LABEL": args.JetsPartitionLabel,
			"$CPIPES_SERVER_ADDR":   ":8585", // not used
			"$JETSTORE_DEV_MODE":    false,
		},
		FileKeyComponents:  fileKeyComponents,
		Done:               make(chan struct{}),
		ErrCh:              make(chan error, 1),
		FileNamesCh:        make(chan FileName, 2),
		DownloadS3ResultCh: make(chan DownloadS3Result, 1),
	}
	cpContext.CpConfig, err = compute_pipes.UnmarshalComputePipesConfig(&cpipesConfigJson, args.NodeId, args.NbrNodes)
	if err != nil {
		cpErr = fmt.Errorf("failed to unmarshal cpipes config json (%s): %v", args.CpipesMode, err)
		goto gotError
	}

	defer func() {
		log.Printf("##!@@ DONE CoordinateComputePipes closing Done ch")
		select {
		case <-cpContext.Done:
			log.Printf("##!@@ Done ch was already closed!")
			// done chan is already closed due to error
		default:
			close(cpContext.Done)
			log.Printf("##!@@ Done ch closed")
		}
	}()

	// Create a local temp directory to hold the file(s)
	inFolderPath, err = os.MkdirTemp("", "jetstore")
	if err != nil {
		cpErr = fmt.Errorf("failed to create local temp directory: %v", err)
		goto gotError
	}
	defer os.Remove(inFolderPath)

	// Download files from s3
	err = cpContext.DownloadS3Files(inFolderPath, fileKeys)
	if err != nil {
		cpErr = fmt.Errorf("while DownloadingS3Files (in CoordinateComputePipes): %v", err)
		goto gotError
	}

	// Process the downloaded file(s)
	return cpContext.ProcessFilesAndReportStatus(ctx, dbpool, inFolderPath)

gotError:
	log.Println("**!@@ gotError in CoordinateComputePipes")
	log.Println(cpErr)

	//*TODO insert error in pipeline_execution_details
	return cpErr
}

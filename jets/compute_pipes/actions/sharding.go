package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes Sharding Actions

func (args *ComputePipesArgs) CoordinateComputePipes(ctx context.Context, dsn string) error {
	var cpErr, err error
	var inFolderPath string
	var cpContext *ComputePipesContext
	var fileKeyComponents map[string]interface{}
	var fileKeyDate time.Time
	var fileKeys []string

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

	cpContext = &ComputePipesContext{
		ComputePipesArgs: *args,
		EnvSettings: map[string]interface{}{
			"$SESSIONID":            args.SessionId,
			"$FILE_KEY_DATE":        fileKeyDate,
			"$FILE_KEY":             args.FileKey,
			"$FILE_KEY_FOLDER":      args.FileKey, // assuming always partfiles
			"$SHARD_ID":             args.NodeId,
			"$NBR_SHARDS":           args.CpConfig.ClusterConfig.NbrNodes,
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
	defer func() {
		select {
		case <-cpContext.Done:
			// done chan is already closed due to error
		default:
			close(cpContext.Done)
		}
	}()

	// Create a local temp directory to hold the file(s)
	inFolderPath, err = os.MkdirTemp("", "jetstore")
	if err != nil {
		cpErr = fmt.Errorf("failed to create local temp directory: %v", err)
		goto gotError

	}
	defer os.Remove(inFolderPath)

	// Get the file keys from compute_pipes_shard_registry table
	//*NOTE Case reducing, get the file keys from s3
	switch cpContext.CpConfig.ClusterConfig.CpipesMode {
	case "sharding":
		fileKeys, err = GetFileKeys(ctx, dbpool, args.InputSessionId, args.NodeId)
		if err != nil {
			cpErr = fmt.Errorf("while loading aws configuration (in CoordinateComputePipes): %v", err)
			goto gotError
		}
		log.Printf("**!@@ Got %d file keys from database for nodeId %d (sharding)", len(fileKeys), args.NodeId)

	case "reducing":
		// Case cpipes reducing mode, get the file keys from s3
		log.Printf("Getting file keys from s3 folder: %s", cpContext.FileKey)
		s3Objects, err := awsi.ListS3Objects(&cpContext.FileKey)
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
		cpErr = fmt.Errorf("error: invalid cpipesMode in downloadS3Files: %s", cpContext.CpConfig.ClusterConfig.CpipesMode)
		goto gotError
	}

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

package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes Actions

func (args *ComputePipesNodeArgs) CoordinateComputePipes(ctx context.Context, dbpool *pgxpool.Pool) error {
	var cpErr, err error
	var didSync bool
	var inFolderPath string
	var cpContext *ComputePipesContext
	var fileKeyComponents map[string]interface{}
	var fileKeyPath, fileKeyName string // Components extracted from File_Key based on is_part_file
	var fileKeyDate time.Time
	var fileKeys []*FileKeyInfo
	var cpipesConfigJson string
	var cpConfig *ComputePipesConfig
	var mainSchemaProviderConfig *SchemaProviderSpec
	var envSettings map[string]interface{}
	var schemaManager *SchemaManager
	var externalBucket string

	// Check if we need to sync the workspace files
	didSync, err = workspace.SyncComputePipesWorkspace(dbpool)
	if err != nil {
		log.Panicf("error while synching workspace files from db: %v", err)
	}
	if didSync {
		ClearJetrulesCaches()
	}

	// Make sure we have a jet partition key set
	if len(args.JetsPartitionLabel) == 0 {
		args.JetsPartitionLabel = fmt.Sprintf("%dp", args.NodeId)
	}

	stmt := "SELECT cpipes_config_json FROM jetsapi.cpipes_execution_status WHERE pipeline_execution_status_key = %d"

	// Get the cpipes config from cpipes_execution_status
	err = dbpool.QueryRow(ctx, fmt.Sprintf(stmt, args.PipelineExecKey)).Scan(&cpipesConfigJson)
	if err != nil {
		cpErr = fmt.Errorf("while reading cpipes config from cpipes_execution_status table (CoordinateComputePipes): %v", err)
		goto gotError
	}
	cpConfig, err = UnmarshalComputePipesConfig(&cpipesConfigJson)
	if err != nil {
		cpErr = fmt.Errorf("failed to unmarshal cpipes config json: %v", err)
		goto gotError
	}

	// Get file keys
	switch cpConfig.CommonRuntimeArgs.CpipesMode {
	case "sharding":
		// Case sharding, get the file keys from compute_pipes_shard_registry
		fileKeys, err = GetFileKeys(ctx, dbpool, cpConfig.CommonRuntimeArgs.SessionId, args.NodeId)
		if err != nil {
			cpErr = fmt.Errorf("while loading aws configuration (in CoordinateComputePipes): %v", err)
			goto gotError
		}
		log.Printf("%s node %d %s Got %d file keys from database for file_key: %s",
			cpConfig.CommonRuntimeArgs.SessionId, args.NodeId,
			cpConfig.CommonRuntimeArgs.MainInputStepId, len(fileKeys), cpConfig.CommonRuntimeArgs.FileKey)

	case "reducing":
		// Case cpipes reducing mode, get the file keys from s3
		fileKeys, err = GetS3FileKeys(cpConfig.CommonRuntimeArgs.ProcessName, cpConfig.CommonRuntimeArgs.SessionId,
			cpConfig.CommonRuntimeArgs.MainInputStepId, args.JetsPartitionLabel)
		if err != nil {
			cpErr = err
			goto gotError
		}
		log.Printf("%s node %d %s Got %d file keys from s3",
			cpConfig.CommonRuntimeArgs.SessionId, args.NodeId,
			cpConfig.CommonRuntimeArgs.MainInputStepId, len(fileKeys))
		if cpConfig.ClusterConfig.IsDebugMode {
			for _, k := range fileKeys {
				log.Printf("%s node %d %s Got file key from s3: %s",
					cpConfig.CommonRuntimeArgs.SessionId, args.NodeId,
					cpConfig.CommonRuntimeArgs.MainInputStepId, k.key)
			}
		}

	default:
		cpErr = fmt.Errorf("error: invalid cpipesMode in CoordinateComputePipes: %s", cpConfig.CommonRuntimeArgs.CpipesMode)
		goto gotError
	}

	// Extract processing date from file key inFile
	fileKeyComponents = make(map[string]interface{})
	fileKeyComponents = datatable.SplitFileKeyIntoComponents(fileKeyComponents, &cpConfig.CommonRuntimeArgs.FileKey)
	if len(fileKeyComponents) > 0 {
		year := fileKeyComponents["year"].(int)
		month := fileKeyComponents["month"].(int)
		day := fileKeyComponents["day"].(int)
		fileKeyDate = time.Date(year, time.Month(month), day, 14, 0, 0, 0, time.UTC)
		// log.Println("fileKeyDate:", fileKeyDate)
	}

	// Create the SchemaManager and prepare the providers
	schemaManager = NewSchemaManager(cpConfig.SchemaProviders, envSettings, cpConfig.ClusterConfig.IsDebugMode)
	err = schemaManager.PrepareSchemaProviders(dbpool)
	if err != nil {
		cpErr = fmt.Errorf("while calling schemaManager.PrepareSchemaProviders: %v", err)
		goto gotError
	}
	// Get the main_input schema provider. Don't use the key "_main_input_" as it is not guarantee to have
	// that specific key
	for i := range cpConfig.SchemaProviders {
		if cpConfig.SchemaProviders[i].SourceType == "main_input" {
			mainSchemaProviderConfig = cpConfig.SchemaProviders[i]
			break
		}
	}
	if mainSchemaProviderConfig == nil {
		// Did not find the main_input schema provider
		cpErr = fmt.Errorf("error: bug in CoordinateComputePipes, could not find the main_input schema provider")
		goto gotError
	}
	if cpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		externalBucket = mainSchemaProviderConfig.Bucket
	}

	if mainSchemaProviderConfig.IsPartFiles {
		fileKeyPath = cpConfig.CommonRuntimeArgs.FileKey
	} else {
		fileKey := cpConfig.CommonRuntimeArgs.FileKey
		idx := strings.LastIndex(fileKey, "/")
		if idx >= 0 && idx < len(fileKey)-1 {
			fileKeyName = fileKey[idx+1:]
			fileKeyPath = fileKey[0:idx]
		} else {
			fileKeyPath = fileKey
		}
	}
	//* IMPORTANT: Make sure a key is not the prefix of another key
	//  e.g. $FILE_KEY and $FILE_KEY_PATH is BAD since $FILE_KEY_PATH may get
	//  the value of $FILE_KEY with a dandling _PATH
	envSettings = PrepareCpipesEnv(cpConfig, mainSchemaProviderConfig)
	envSettings["$FILE_KEY"] = cpConfig.CommonRuntimeArgs.FileKey
	envSettings["$SESSIONID"] = cpConfig.CommonRuntimeArgs.SessionId
	envSettings["$PROCESS_NAME"] = cpConfig.CommonRuntimeArgs.ProcessName
	envSettings["$PATH_FILE_KEY"] = fileKeyPath
	envSettings["$NAME_FILE_KEY"] = fileKeyName
	envSettings["$DATE_FILE_KEY"] = fileKeyDate
	envSettings["$SHARD_ID"] = args.NodeId
	envSettings["$JETS_PARTITION_LABEL"] = args.JetsPartitionLabel

	if mainSchemaProviderConfig.Env != nil {
		for k, v := range mainSchemaProviderConfig.Env {
			envSettings[k] = v
		}
	}

	cpContext = &ComputePipesContext{
		ComputePipesArgs: ComputePipesArgs{
			ComputePipesNodeArgs:   *args,
			ComputePipesCommonArgs: *cpConfig.CommonRuntimeArgs,
		},
		CpConfig:           cpConfig,
		EnvSettings:        envSettings,
		FileKeyComponents:  fileKeyComponents,
		SchemaManager:      schemaManager,
		InputFileKeys:      fileKeys,
		KillSwitch:         make(chan struct{}),
		Done:               make(chan struct{}),
		ErrCh:              make(chan error, 1000),
		FileNamesCh:        make(chan FileName, 2),
		DownloadS3ResultCh: make(chan DownloadS3Result, 1),
	}

	// Add to envSettings based on compute pipe config
	for _, contextSpec := range cpConfig.Context {
		switch contextSpec.Type {
		case "file_key_component":
			cpContext.EnvSettings[contextSpec.Key] = cpContext.FileKeyComponents[contextSpec.Expr]
		case "value":
			cpContext.EnvSettings[contextSpec.Key] = contextSpec.Expr
		case "partfile_key_component":
		default:
			cpErr = fmt.Errorf("error: unknown ContextSpec Type: %v", contextSpec.Type)
			goto gotError
		}
	}

	if cpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		// Prepare the extended input_row columns, ie column added to input file
		cpContext.AddionalInputHeaders = GetAdditionalInputColumns(cpConfig)

		// partfile_key_component :: explained
		// ContextSpec.Type == partfile_key_component:
		//		Key is column name of input_row to put the key component (must be at end of columns comming from parquet parfiles)
		//		Expr is key in partfile file_key
		// Prepare the regex for the partfile_key_component
		cpContext.PartFileKeyComponents = make([]CompiledPartFileComponent, 0)
		for i := range cpContext.CpConfig.Context {
			if cpContext.CpConfig.Context[i].Type == "partfile_key_component" {
				regex_query := fmt.Sprintf(`%s=(.*?)\/`, cpContext.CpConfig.Context[i].Expr)
				// log.Println("**!@@",args.SessionId,"partfile_key_component Got regex_query",regex_query,"for column",cpContext.CpConfig.Context[i].Key)
				re, err := regexp.Compile(regex_query)
				if err == nil {
					cpContext.PartFileKeyComponents = append(cpContext.PartFileKeyComponents, CompiledPartFileComponent{
						ColumnName: cpContext.CpConfig.Context[i].Key,
						Regex:      re,
					})
				} else {
					log.Println("*** WARNING *** error compiling regex:", regex_query, "err:", err)
				}
			}
		}
	}

	defer func() {
		// log.Printf("##!@@ DONE CoordinateComputePipes closing Done ch")
		select {
		case <-cpContext.Done:
			// log.Printf("##!@@ Done ch was already closed!")
			// done chan is already closed due to error
		default:
			close(cpContext.Done)
			// log.Printf("##!@@ Done ch closed")
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
	err = cpContext.DownloadS3Files(inFolderPath, externalBucket, fileKeys)
	if err != nil {
		cpErr = fmt.Errorf("while DownloadingS3Files (in CoordinateComputePipes): %v", err)
		goto gotError
	}

	// Process the downloaded file(s)
	return cpContext.ProcessFilesAndReportStatus(ctx, dbpool, inFolderPath)

gotError:
	log.Println(cpConfig.CommonRuntimeArgs.SessionId, "node", args.NodeId, "error in CoordinateComputePipes:", cpErr)

	//*TODO insert error in pipeline_execution_details
	return cpErr
}

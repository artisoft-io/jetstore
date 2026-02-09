package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes Actions

func (args *ComputePipesNodeArgs) CoordinateComputePipes(ctx context.Context, dbpool *pgxpool.Pool, jrProxy JetRulesProxy) error {
	var cpErr, err error
	var didSync bool
	var inFolderPath []string
	var cpContext *ComputePipesContext
	var fileKeyComponents map[string]any
	var fileKeys [][]*FileKeyInfo
	var cpipesConfigJson string
	var cpConfig *ComputePipesConfig
	var mainSchemaProviderConfig *SchemaProviderSpec
	var envSettings map[string]any
	var schemaManager *SchemaManager
	var externalBucket string
	var fileNamesCh []chan FileName
	var inputChannelConfig *InputChannelConfig
	var nbrMergeChannels int

	// Check if we need to sync the workspace files
	didSync, err = workspace.SyncComputePipesWorkspace(dbpool)
	if err != nil {
		return fmt.Errorf("error while synching workspace files from db: %v", err)
	}
	if didSync {
		ClearJetrulesCaches()
	}

	// Make sure we have a jet partition key set
	if len(args.JetsPartitionLabel) == 0 {
		args.JetsPartitionLabel = fmt.Sprintf("%04dP", args.NodeId)
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
	envSettings = mainSchemaProviderConfig.Env

	//* IMPORTANT: Make sure a key is not the prefix of another key
	//  e.g. $FILE_KEY and $FILE_KEY_PATH is BAD since $FILE_KEY_PATH may get
	//  the value of $FILE_KEY with a dandling _PATH
	envSettings["$SHARD_ID"] = args.NodeId
	envSettings["$JETS_PARTITION_LABEL"] = args.JetsPartitionLabel

	// Allocate the MergeFileNamesCh if have merge channels
	inputChannelConfig = &cpConfig.PipesConfig[0].InputChannel
	nbrMergeChannels = len(inputChannelConfig.MergeChannels)
	fileNamesCh = make([]chan FileName, 0, 1+nbrMergeChannels)
	fileNamesCh = append(fileNamesCh, make(chan FileName, 2))
	for range nbrMergeChannels {
		fileNamesCh = append(fileNamesCh, make(chan FileName, 2))
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
			cpConfig.CommonRuntimeArgs.MainInputStepId, args.JetsPartitionLabel, inputChannelConfig, envSettings)
		if err != nil {
			cpErr = err
			goto gotError
		}
		log.Printf("%s node %d %s Got %d file keys from s3",
			cpConfig.CommonRuntimeArgs.SessionId, args.NodeId,
			cpConfig.CommonRuntimeArgs.MainInputStepId, len(fileKeys))
		if cpConfig.ClusterConfig.IsDebugMode {
			for i := range fileKeys {
				for _, k := range fileKeys[i] {
					log.Printf("%s node %d %s Got file key from s3[%d]: %s",
						cpConfig.CommonRuntimeArgs.SessionId, args.NodeId,
						cpConfig.CommonRuntimeArgs.MainInputStepId, i, k.key)
				}
			}
		}

	default:
		cpErr = fmt.Errorf("error: invalid cpipesMode in CoordinateComputePipes: %s", cpConfig.CommonRuntimeArgs.CpipesMode)
		goto gotError
	}

	// Create the SchemaManager and prepare the providers
	schemaManager = NewSchemaManager(cpConfig.SchemaProviders, envSettings, cpConfig.ClusterConfig.IsDebugMode)
	err = schemaManager.PrepareSchemaProviders(dbpool)
	if err != nil {
		cpErr = fmt.Errorf("while calling schemaManager.PrepareSchemaProviders: %v", err)
		goto gotError
	}
	if cpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		externalBucket = mainSchemaProviderConfig.Bucket
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
		JetRules:           jrProxy,
		KillSwitch:         make(chan struct{}),
		Done:               make(chan struct{}),
		ErrCh:              make(chan error, 1000),
		FileNamesCh:        fileNamesCh,
		DownloadS3ResultCh: make(chan DownloadS3Result, 1000),
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

	// Create the main temp folder for this compute pipes node
	cpContext.JetStoreTempFolder, err = os.MkdirTemp("", "jetstore")
	if err != nil {
		cpErr = fmt.Errorf("failed to create JetStore temp directory: %v", err)
		goto gotError
	}

	// Create a local temp directory to hold the file(s)

	inFolderPath = make([]string, 0, 1+nbrMergeChannels)
	for i := range 1 + nbrMergeChannels {
		var folderPath string
		folderPath, err = os.MkdirTemp(cpContext.JetStoreTempFolder, fmt.Sprintf("input_files_%d", i))
		if err != nil {
			cpErr = fmt.Errorf("failed to create local input_files directory: %v", err)
			goto gotError
		}
		inFolderPath = append(inFolderPath, folderPath)
	}
	defer func() {
		err := os.RemoveAll(cpContext.JetStoreTempFolder)
		if err != nil {
			log.Printf("%s - WARNING while calling RemoveAll in JetStore temp folder:%v", cpContext.SessionId, err)
		}
	}()

	defer cpContext.DoneAll(nil)

	// Download files from s3
	err = cpContext.DownloadS3Files(inFolderPath, externalBucket, fileKeys)
	if err != nil {
		cpErr = fmt.Errorf("while DownloadingS3Files (in CoordinateComputePipes): %v", err)
		goto gotError
	}

	// Process the downloaded file(s)
	return cpContext.ProcessFilesAndReportStatus(ctx, dbpool)

gotError:
	log.Println(cpConfig.CommonRuntimeArgs.SessionId, "node", args.NodeId, "error in CoordinateComputePipes:", cpErr)

	//*TODO insert error in pipeline_execution_details
	return cpErr
}

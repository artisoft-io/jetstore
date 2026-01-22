package compute_pipes

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point
func init() {
	gob.Register(time.Now())
}

// Function to write transformed row to database
func (cpCtx *ComputePipesContext) StartComputePipes(dbpool *pgxpool.Pool,
	inputSchemaCh <-chan ParquetSchemaInfo, computePipesInputCh <-chan []any) {

	// log.Println("Entering StartComputePipes")

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("StartComputePipes: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			cpErr := errors.New(buf.String())
			log.Println(cpErr)
			cpCtx.ErrCh <- cpErr
			close(cpCtx.Done)
			close(cpCtx.ChResults.Copy2DbResultCh)
			close(cpCtx.ChResults.WritePartitionsResultCh)
		}
	}()

	var cpErr, err error
	var channelRegistry *ChannelRegistry
	var outChannel *Channel
	var wt WriteTableSource
	var table chan ComputePipesResult
	var ctx *BuilderContext
	var inputRowChSpec *ChannelSpec
	var inputRowChannel *InputChannel
	var inputChannelName string
	var managersWg sync.WaitGroup
	var channelsSpec map[string]*ChannelSpec
	var channelsInUse map[string]*ChannelSpec
	var outputChannels []*OutputChannelConfig
	var domainKeysByClass map[string]*DomainKeysSpec

	// Create the LookupTableManager and prepare the lookups async
	lookupManager := NewLookupTableManager(cpCtx.CpConfig.LookupTables, cpCtx.EnvSettings,
		cpCtx.CpConfig.ClusterConfig.IsDebugMode)
	managersWg.Add(1)
	go func() {
		defer managersWg.Done()
		err := lookupManager.PrepareLookupTables(dbpool)
		if err != nil {
			log.Println("error in lookupManager.PrepareLookupTables:", err)
			cpCtx.ErrCh <- err
			close(cpCtx.Done)
		}
	}()

	// Prepare the channel registry
	// ----------------------------
	mainInput := cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput
	mainInputSchemaProvider := cpCtx.SchemaManager.GetSchemaProvider("_main_input_")
	// log.Printf("*** StartComputePipes: mainInput.DomainKeys: %v, mainInput.DomainClass: %v\n", *mainInput.DomainKeys, mainInput.DomainClass)
	inputParquetSchema := mainInputSchemaProvider.ParquetSchema()
	if inputSchemaCh != nil {
		// Get the parquet schema from the channel as it is being extracted from the
		// first input file
		is := <-inputSchemaCh
		inputParquetSchema = &is
		mainInputSchemaProvider.SetParquetSchema(inputParquetSchema)
		// Get the columns from the schema
		parquetColumns := inputParquetSchema.Columns()
		if len(parquetColumns) == 0 {
			cpErr = fmt.Errorf("error: input_parquet_schema has no columns")
			goto gotError
		}
		// Ensure the mainInput.InputColumns is properly initialized
		if len(mainInput.InputColumns) == 0 {
			mainInput.InputColumns = parquetColumns
			mainInput.InputColumns = append(mainInput.InputColumns, cpCtx.AddionalInputHeaders...)
			// Add the headers from the partfile_key_component
			for i := range cpCtx.CpConfig.Context {
				if cpCtx.CpConfig.Context[i].Type == "partfile_key_component" {
					mainInput.InputColumns = append(mainInput.InputColumns, cpCtx.CpConfig.Context[i].Key)
				}
			}
		}
		// Ensure the input columns are unique, if not make them unique and keep the original in InputColumnsOriginal
		headersUniquefied := schema.NewHeadersUniquefied(mainInput.InputColumns)
		mainInput.InputColumns = headersUniquefied.UniqueHeaders
		// Prepare and save the input row columns and parquet schema in the cpipes_execution_status table
		irc := InputRowColumns{
			MainInput: mainInput.InputColumns,
		}
		if headersUniquefied.Modified {
			irc.OriginalHeaders = headersUniquefied.OriginalHeaders
			mainInput.OriginalInputColumns = headersUniquefied.OriginalHeaders
		}

		if cpCtx.NodeId == 0 {
			if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
				log.Println("GOT COLUMNS FROM SCHEMA:", mainInput.InputColumns)
			}
			// Save the columns and parquet schema to db
			inputRowColumnsJson, _ := json.Marshal(irc)
			inputParquetSchemaJson, _ := json.Marshal(inputParquetSchema)

			// Update in cpipes_execution_status
			stmt := `UPDATE jetsapi.cpipes_execution_status SET (input_row_columns_json, input_parquet_schema_json) = ($1, $2) WHERE session_id = $3`
			_, err2 := dbpool.Exec(context.TODO(), stmt, string(inputRowColumnsJson), string(inputParquetSchemaJson), cpCtx.ComputePipesCommonArgs.SessionId)
			if err2 != nil {
				cpErr = fmt.Errorf("error inserting in jetsapi.cpipes_execution_status table: %v", err2)
				goto gotError
			}
		}
	}
	inputChannelName = cpCtx.CpConfig.PipesConfig[0].InputChannel.Name
	if inputChannelName == "input_row" {
		// case sharding or reducing
		// Setup the input channel for input_row
		headersPosMap := make(map[string]int)
		for i, c := range mainInput.InputColumns {
			headersPosMap[c] = i
		}
		inputRowChSpec = &ChannelSpec{
			Name:           "input_row",
			Columns:        mainInput.InputColumns,
			ClassName:      mainInput.DomainClass,
			DomainKeysInfo: mainInput.DomainKeys,
			columnsMap:     &headersPosMap,
		}
		inputRowChannel = &InputChannel{
			Name:           "input_row",
			Channel:        computePipesInputCh,
			Columns:        &headersPosMap,
			DomainKeySpec:  inputRowChSpec.DomainKeysInfo,
			Config:         inputRowChSpec,
			HasGroupedRows: cpCtx.CpConfig.PipesConfig[0].InputChannel.HasGroupedRows,
		}
	}
	// Collect all the channel that are in use in PipeConfig, looking at PipeConfig.TransformationSpec.OutputChannel
	// Make a lookup of channel spec using the channels config
	channelsSpec = make(map[string]*ChannelSpec)
	// Get the channels in used based on transformation pipe config, prime the channels using the provided channel spec
	channelsInUse = make(map[string]*ChannelSpec)
	for i := range cpCtx.CpConfig.Channels {
		chSpec := &cpCtx.CpConfig.Channels[i]
		if chSpec.Name == "input_row" {
			// Skip this one since this input_row is to indicate columns to add to the input file which is done
			// in start_sharding step
			continue
		}
		// Make the lookup of column name to pos
		cm := make(map[string]int)
		for j, c := range chSpec.Columns {
			cm[c] = j
		}
		chSpec.columnsMap = &cm
		channelsSpec[cpCtx.CpConfig.Channels[i].Name] = chSpec
		channelsInUse[cpCtx.CpConfig.Channels[i].Name] = chSpec
	}
	if inputRowChSpec != nil {
		channelsSpec[inputRowChSpec.Name] = inputRowChSpec
	}
	// Collect the output channels
	outputChannels = make([]*OutputChannelConfig, 0)
	for i := range cpCtx.CpConfig.PipesConfig {
		for j := range cpCtx.CpConfig.PipesConfig[i].Apply {
			switch cpCtx.CpConfig.PipesConfig[i].Apply[j].Type {
			case "anonymize":
				outputChannel := &cpCtx.CpConfig.PipesConfig[i].Apply[j].OutputChannel
				outputChannels = append(outputChannels, outputChannel)
				outputChannel = cpCtx.CpConfig.PipesConfig[i].Apply[j].AnonymizeConfig.KeysOutputChannel
				if outputChannel != nil {
					outputChannels = append(outputChannels, outputChannel)
				}
			case "jetrules":
				// Jetrules config overrides the outputChannel
				for k := range cpCtx.CpConfig.PipesConfig[i].Apply[j].JetrulesConfig.OutputChannels {
					outputChannel := &cpCtx.CpConfig.PipesConfig[i].Apply[j].JetrulesConfig.OutputChannels[k]
					outputChannels = append(outputChannels, outputChannel)
				}
			case "clustering":
				outputChannel := &cpCtx.CpConfig.PipesConfig[i].Apply[j].OutputChannel
				outputChannels = append(outputChannels, outputChannel)
				outputChannel = cpCtx.CpConfig.PipesConfig[i].Apply[j].ClusteringConfig.CorrelationOutputChannel
				outputChannels = append(outputChannels, outputChannel)
			default:
				outputChannel := &cpCtx.CpConfig.PipesConfig[i].Apply[j].OutputChannel
				outputChannels = append(outputChannels, outputChannel)
			}
		}
	}
	// Prepare the channels in use
	for _, outputChannel := range outputChannels {
		spec := channelsSpec[outputChannel.SpecName]
		if spec == nil {
			cpErr = fmt.Errorf("channel spec %s not found in Channel Registry", outputChannel.SpecName)
			goto gotError
		}
		channelsInUse[outputChannel.Name] = spec
	}
	domainKeysByClass = cpCtx.ComputePipesCommonArgs.DomainKeysSpecByClass
	// Use the channelsInUse map to create the Channel Registry
	channelRegistry = &ChannelRegistry{
		InputRowChannel:      inputRowChannel,
		ComputeChannels:      make(map[string]*Channel),
		OutputTableChannels:  make([]string, 0),
		ClosedChannels:       make(map[string]bool),
		DistributionChannels: make(map[string]*[]string),
	}
	for name, spec := range channelsInUse {
		channelRegistry.ComputeChannels[name] = &Channel{
			Name:          name,
			Channel:       make(chan []interface{}),
			Columns:       spec.columnsMap,
			DomainKeySpec: spec.DomainKeysInfo,
			Config:        spec,
		}
		if len(spec.ClassName) > 0 {
			// log.Printf("*** Channel '%s' for domain class %s, domain_keys: %v\n",
			// 	name, spec.ClassName, domainKeysByClass[spec.ClassName])
			if spec.DomainKeysInfo == nil && domainKeysByClass != nil {
				channelRegistry.ComputeChannels[name].DomainKeySpec = domainKeysByClass[spec.ClassName]
			}
		}
	}
	if inputChannelName != "input_row" {
		// Case reducing
		// Replace the first channel of the pipes and make it the "input_row"
		// Setup the input channel for input_row
		inChannel := channelRegistry.ComputeChannels[inputChannelName]
		if inChannel == nil {
			cpErr = fmt.Errorf("channel %s not found in Channel Registry", inputChannelName)
			goto gotError
		}
		inputRowChannel = &InputChannel{
			Name:           "input_row",
			Channel:        computePipesInputCh,
			Columns:        inChannel.Columns,
			DomainKeySpec:  inChannel.DomainKeySpec,
			Config:         inChannel.Config,
			HasGroupedRows: cpCtx.CpConfig.PipesConfig[0].InputChannel.HasGroupedRows,
		}
		cpCtx.CpConfig.PipesConfig[0].InputChannel.Name = "input_row"
		channelRegistry.InputRowChannel = inputRowChannel
	}
	// log.Println("Compute Pipes channel registry ready")
	// for name, channel := range channelRegistry.computeChannels {
	// 	log.Println("**& Channel", name, "Columns map", channel.columns)
	// }

	// Prepare the output tables
	for i := range cpCtx.CpConfig.OutputTables {
		tableName := cpCtx.CpConfig.OutputTables[i].Name
		tableName = utils.ReplaceEnvVars(tableName, cpCtx.EnvSettings)
		tableIdentifier, err := SplitTableName(tableName)
		if err != nil {
			cpErr = fmt.Errorf("while splitting table name: %s", err)
			goto gotError
		}
		if len(cpCtx.CpConfig.OutputTables[i].ChannelSpecName) == 0 {
			cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: channel_spec_name missing for Output table %s",
				cpCtx.CpConfig.OutputTables[i].Name)
			goto gotError
		}
		outChannel = channelRegistry.ComputeChannels[cpCtx.CpConfig.OutputTables[i].ChannelSpecName]
		if outChannel == nil {
			cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: channel_spec_name '%s' not found for Output table %s",
				cpCtx.CpConfig.OutputTables[i].ChannelSpecName,
				cpCtx.CpConfig.OutputTables[i].Name)
			goto gotError
		}
		channelRegistry.OutputTableChannels = append(channelRegistry.OutputTableChannels, cpCtx.CpConfig.OutputTables[i].ChannelSpecName)
		// log.Println("*** Channel for Output Table", tableIdentifier, "is:", outChannel.name)
		wt = WriteTableSource{
			source:          outChannel.Channel,
			tableIdentifier: tableIdentifier,
			columns:         outChannel.Config.Columns,
		}
		table = make(chan ComputePipesResult, 1)
		cpCtx.ChResults.Copy2DbResultCh <- table
		go wt.WriteTable(dbpool, cpCtx.Done, table)
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Println("Compute Pipes output tables ready")
	}

	ctx = &BuilderContext{
		dbpool:             dbpool,
		sessionId:          cpCtx.SessionId,
		jetsPartition:      cpCtx.JetsPartitionLabel,
		cpConfig:           cpCtx.CpConfig,
		processName:        cpCtx.ProcessName,
		channelRegistry:    channelRegistry,
		lookupTableManager: lookupManager,
		s3DeviceManager:    cpCtx.S3DeviceMgr,
		schemaManager:      cpCtx.SchemaManager,
		inputParquetSchema: inputParquetSchema,
		jetRules:           cpCtx.JetRules,
		done:               cpCtx.Done,
		errCh:              cpCtx.ErrCh,
		chResults:          cpCtx.ChResults,
		env:                cpCtx.EnvSettings,
		nodeId:             cpCtx.NodeId,
	}

	// Start the metric reporting goroutine
	if cpCtx.CpConfig.MetricsConfig != nil && ctx.cpConfig.MetricsConfig.ReportInterval > 0 {
		go func() {
			for {
				time.Sleep(time.Duration(ctx.cpConfig.MetricsConfig.ReportInterval) * time.Second)
				select {
				case <-ctx.done:
					log.Println("Metric Reporting Interrupted")
					return
				default:
					ctx.ReportMetrics()
				}
			}
		}()
	}

	// Wait until the lookup tables are ready
	managersWg.Wait()

	// log.Println("Calling ctx.BuildComputeGraph()")
	err = ctx.BuildComputeGraph()
	if err != nil {
		cpErr = fmt.Errorf("while building the compute graph: %s", err)
		goto gotError
	}
	// log.Println("Calling ctx.BuildComputeGraph() completed")

	// All done!
	close(cpCtx.ChResults.Copy2DbResultCh)
	close(cpCtx.ChResults.WritePartitionsResultCh)
	close(cpCtx.ChResults.JetrulesWorkerResultCh)
	close(cpCtx.ChResults.ClusteringResultCh)
	return

gotError:
	log.Println("error in StartComputePipes:", cpErr)
	cpCtx.ErrCh <- cpErr
	close(cpCtx.Done)
	close(cpCtx.ChResults.Copy2DbResultCh)
	close(cpCtx.ChResults.WritePartitionsResultCh)
	close(cpCtx.ChResults.JetrulesWorkerResultCh)
	close(cpCtx.ChResults.ClusteringResultCh)
	if cpCtx.S3DeviceMgr == nil {
		// Got error before the s3 device manager was created, close the chan manually
		close(cpCtx.ChResults.S3PutObjectResultCh)
	}
}

func UnmarshalComputePipesConfig(computePipesJson *string) (*ComputePipesConfig, error) {

	// unmarshall the compute graph definition
	var cpConfig ComputePipesConfig
	err := json.Unmarshal([]byte(*computePipesJson), &cpConfig)
	if err != nil {
		return nil, fmt.Errorf("while unmarshaling compute pipes json (ComputePipes): %s", err)
	}

	// validate cluster config
	if cpConfig.ClusterConfig == nil {
		return nil, fmt.Errorf("error: cluster_config is required in compute_pipes_json")
	}
	return &cpConfig, nil
}

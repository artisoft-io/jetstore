package compute_pipes

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point
func init() {
	gob.Register(time.Now())
}

// Function to write transformed row to database
func (cpCtx *ComputePipesContext) StartComputePipes(dbpool *pgxpool.Pool, computePipesInputCh <-chan []interface{}) {

	// log.Println("Entering StartComputePipes")

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			cpErr := fmt.Errorf("StartComputePipes: recovered error: %v", r)
			log.Println(cpErr)
			// debug.Stack()
			debug.PrintStack()
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
	var inputRowChannel *InputChannel

	// Create the LookupTableManager and prepare the lookups async
	lookupManager := NewLookupTableManager(cpCtx.CpConfig.LookupTables, cpCtx.EnvSettings,
		cpCtx.CpConfig.ClusterConfig.IsDebugMode)
	var lookupWg sync.WaitGroup
	lookupWg.Add(1)
	go func() {
		defer lookupWg.Done()
		err := lookupManager.PrepareLookupTables(dbpool)
		if err != nil {
			log.Println("error in lookupManager.PrepareLookupTables:", err)
			cpCtx.ErrCh <- err
			close(cpCtx.Done)
			close(cpCtx.ChResults.Copy2DbResultCh)
			close(cpCtx.ChResults.WritePartitionsResultCh)
		}
	}()

	// Prepare the channel registry
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		// Setup the input channel for input_row
		headersPosMap := make(map[string]int)
		for i, c := range cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns {
			headersPosMap[c] = i
		}
		inputRowChannel = &InputChannel{
			channel: computePipesInputCh,
			columns: headersPosMap,
			config: &ChannelSpec{
				Name:    "input_row",
				Columns: cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns,
			},
		}
	}
	channelRegistry = &ChannelRegistry{
		inputRowChannel:      inputRowChannel,
		computeChannels:      make(map[string]*Channel),
		outputTableChannels:  make([]string, 0),
		closedChannels:       make(map[string]bool),
		distributionChannels: make(map[string]*[]string),
	}
	for i := range cpCtx.CpConfig.Channels {
		cm := make(map[string]int)
		for j, c := range cpCtx.CpConfig.Channels[i].Columns {
			cm[c] = j
		}
		channelRegistry.computeChannels[cpCtx.CpConfig.Channels[i].Name] = &Channel{
			channel: make(chan []interface{}),
			columns: cm,
			config:  &cpCtx.CpConfig.Channels[i],
		}
	}
	if cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "reducing" {
		// Replace the first channel of the pipes and make it the "input_row"
		// Setup the input channel for input_row
		inChannel := channelRegistry.computeChannels[cpCtx.CpConfig.PipesConfig[0].Input]
		if inChannel == nil {
			cpErr = fmt.Errorf("channel %s not found in ChannelRegistry", cpCtx.CpConfig.PipesConfig[0].Input)
			goto gotError
		}
		inputRowChannel = &InputChannel{
			channel: computePipesInputCh,
			columns: inChannel.columns,
			config: &ChannelSpec{
				Name:    "input_row",
				Columns: inChannel.config.Columns,
			},
		}
		cpCtx.CpConfig.PipesConfig[0].Input = "input_row"
		channelRegistry.inputRowChannel = inputRowChannel
	}

	// log.Println("Compute Pipes channel registry ready")
	// for i := range cpCtx.CpConfig.Channels {
	// 	log.Println("**& Channel", cpCtx.CpConfig.Channels[i].Name, "Columns map", channelRegistry.computeChannels[cpCtx.CpConfig.Channels[i].Name].columns)
	// }

	// Prepare the output tables when in mode reducing only
	for i := range cpCtx.CpConfig.OutputTables {
		tableName := cpCtx.CpConfig.OutputTables[i].Name
		if strings.Contains(tableName, "$") {
			for k, v := range cpCtx.EnvSettings {
				value, ok := v.(string)
				if ok {
					tableName = strings.ReplaceAll(tableName, k, value)
				}
			}
		}
		tableIdentifier, err := SplitTableName(tableName)
		if err != nil {
			cpErr = fmt.Errorf("while splitting table name: %s", err)
			goto gotError
		}
		outChannel = channelRegistry.computeChannels[cpCtx.CpConfig.OutputTables[i].Key]
		channelRegistry.outputTableChannels = append(channelRegistry.outputTableChannels, cpCtx.CpConfig.OutputTables[i].Key)
		if outChannel == nil {
			cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: Output table %s does not have a channel configuration",
				cpCtx.CpConfig.OutputTables[i].Name)
			goto gotError
		}
		// log.Println("*** Channel for Output Table", tableIdentifier, "is:", outChannel.config.Name)
		wt = WriteTableSource{
			source:          outChannel.channel,
			tableIdentifier: tableIdentifier,
			columns:         outChannel.config.Columns,
		}
		table = make(chan ComputePipesResult, 1)
		cpCtx.ChResults.Copy2DbResultCh <- table
		go wt.WriteTable(dbpool, cpCtx.Done, table)
	}
	// log.Println("*** Compute Pipes output tables ready")

	ctx = &BuilderContext{
		dbpool:             dbpool,
		sessionId:          cpCtx.SessionId,
		jetsPartition:      cpCtx.JetsPartitionLabel,
		cpConfig:           cpCtx.CpConfig,
		processName:        cpCtx.ProcessName,
		channelRegistry:    channelRegistry,
		lookupTableManager: lookupManager,
		done:               cpCtx.Done,
		errCh:              cpCtx.ErrCh,
		chResults:          cpCtx.ChResults,
		env:                cpCtx.EnvSettings,
		nodeId:             cpCtx.NodeId,
	}
	err = ctx.NewS3DeviceManager()
	if err != nil {
		cpErr = err
		goto gotError
	}
	// Set the S3DeviceManager to ComputePipesContext so it's avail when cpipes wind down
	cpCtx.S3DeviceMgr = ctx.s3DeviceManager

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
	lookupWg.Wait()

	// log.Println("Calling ctx.buildComputeGraph()")
	err = ctx.buildComputeGraph()
	if err != nil {
		cpErr = fmt.Errorf("while building the compute graph: %s", err)
		goto gotError
	}
	// log.Println("Calling ctx.buildComputeGraph() completed")

	// All done!
	close(cpCtx.ChResults.Copy2DbResultCh)
	close(cpCtx.ChResults.WritePartitionsResultCh)
	return

gotError:
	log.Println("error in StartComputePipes:", cpErr)
	cpCtx.ErrCh <- cpErr
	close(cpCtx.Done)
	close(cpCtx.ChResults.Copy2DbResultCh)
	close(cpCtx.ChResults.WritePartitionsResultCh)
}

func UnmarshalComputePipesConfig(computePipesJson *string) (*ComputePipesConfig, error) {

	// unmarshall the compute graph definition
	var cpConfig ComputePipesConfig
	err := json.Unmarshal([]byte(*computePipesJson), &cpConfig)
	if err != nil {
		return nil, fmt.Errorf("while unmarshaling compute pipes json: %s", err)
	}

	// validate cluster config
	if cpConfig.ClusterConfig == nil {
		return nil, fmt.Errorf("error: cluster_config is required in compute_pipes_json")
	}
	return &cpConfig, nil
}

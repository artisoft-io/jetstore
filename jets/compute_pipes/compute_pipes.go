package compute_pipes

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point
func init() {
	gob.Register(time.Now())
}

// Function to write transformed row to database
func StartComputePipes(dbpool *pgxpool.Pool, inputHeaders []string, done chan struct{}, errCh chan error,
	computePipesInputCh <-chan []interface{}, chResults *ChannelResults,
	cpConfig *ComputePipesConfig, envSettings map[string]interface{},
	fileKeyComponents map[string]interface{}) {

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			cpErr := fmt.Errorf("StartComputePipes: recovered error: %v", r)
			log.Println(cpErr)
			// debug.Stack()
			debug.PrintStack()
			errCh <- cpErr
			close(done)
			close(chResults.Copy2DbResultCh)
			close(chResults.WritePartitionsResultCh)
			close(chResults.MapOnClusterResultCh)
		}
	}()

	var cpErr, err error
	var channelRegistry *ChannelRegistry
	var outChannel *Channel
	var wt WriteTableSource
	var table chan ComputePipesResult
	var ctx *BuilderContext
	var s3Client *s3.Client
	var s3Uploader *manager.Uploader
	var cfg aws.Config
	var inputRowChannel *InputChannel

	// Add to envSettings based on compute pipe config
	if cpConfig.Context != nil {
		for _, contextSpec := range *cpConfig.Context {
			switch contextSpec.Type {
			case "file_key_component":
				envSettings[contextSpec.Key] = fileKeyComponents[contextSpec.Expr]
			case "partfile_key_component":
			default:
				cpErr = fmt.Errorf("error: unknown ContextSpec Type: %v", contextSpec.Type)
				goto gotError
			}
		}
	}

	// Prepare the channel registry
	if cpConfig.ClusterConfig.CpipesMode == "sharding" {
		// Setup the input channel for input_row
		headersPosMap := make(map[string]int)
		for i, c := range inputHeaders {
			headersPosMap[c] = i
		}
		inputRowChannel = &InputChannel{
			channel: computePipesInputCh,
			columns: headersPosMap,
			config: &ChannelSpec{
				Name:    "input_row",
				Columns: inputHeaders,
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
	for i := range cpConfig.Channels {
		cm := make(map[string]int)
		for j, c := range cpConfig.Channels[i].Columns {
			cm[c] = j
		}
		channelRegistry.computeChannels[cpConfig.Channels[i].Name] = &Channel{
			channel: make(chan []interface{}),
			columns: cm,
			config:  &cpConfig.Channels[i],
		}
	}
	if cpConfig.ClusterConfig.CpipesMode == "reducing" {
		// Replace the first channel of the pipes and make it the "input_row"
		// Setup the input channel for input_row
		inChannel := channelRegistry.computeChannels[cpConfig.PipesConfig[0].Input]
		if inChannel == nil {
			cpErr = fmt.Errorf("channel %s not found in ChannelRegistry", cpConfig.PipesConfig[0].Input)
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
		cpConfig.PipesConfig[0].Input = "input_row"
		channelRegistry.inputRowChannel = inputRowChannel
	}

	log.Println("Compute Pipes channel registry ready")
	// for i := range cpConfig.Channels {
	// 	log.Println("**& Channel", cpConfig.Channels[i].Name, "Columns map", channelRegistry.computeChannels[cpConfig.Channels[i].Name].columns)
	// }

	// Prepare the output tables when in mode reducing only
	if cpConfig.ClusterConfig.CpipesMode == "reducing" {
		for i := range cpConfig.OutputTables {
			tableIdentifier, err := SplitTableName(cpConfig.OutputTables[i].Name)
			if err != nil {
				cpErr = fmt.Errorf("while splitting table name: %s", err)
				goto gotError
			}
			outChannel = channelRegistry.computeChannels[cpConfig.OutputTables[i].Key]
			channelRegistry.outputTableChannels = append(channelRegistry.outputTableChannels, cpConfig.OutputTables[i].Key)
			if outChannel == nil {
				cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: Output table %s does not have a channel configuration",
					cpConfig.OutputTables[i].Name)
				goto gotError
			}
			// log.Println("**& Channel for Output Table", tableIdentifier, "is:", outChannel.config.Name)
			wt = WriteTableSource{
				source:          outChannel.channel,
				tableIdentifier: tableIdentifier,
				columns:         outChannel.config.Columns,
			}
			table = make(chan ComputePipesResult, 1)
			chResults.Copy2DbResultCh <- table
			go wt.WriteTable(dbpool, done, table)
		}
		log.Println("Compute Pipes output tables ready")
	}

	// Setup the s3Uploader
	cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("JETS_REGION")))
	if err != nil {
		cpErr = fmt.Errorf("while loading aws configuration (in StartComputePipes): %v", err)
		goto gotError
	}

	// Create a s3 client
	s3Client = s3.NewFromConfig(cfg)
	// Create the uploader with the client and custom options
	s3Uploader = manager.NewUploader(s3Client)

	ctx = &BuilderContext{
		dbpool:          dbpool,
		cpConfig:        cpConfig,
		channelRegistry: channelRegistry,
		done:            done,
		errCh:           errCh,
		chResults:       chResults,
		env:             envSettings,
		s3Uploader:      s3Uploader,
		//*TODO no longer needed here. . .
		nodeId:             cpConfig.ClusterConfig.NodeId,
		subClusterId:       cpConfig.ClusterConfig.SubClusterId,
		subClusterNodeId:   cpConfig.ClusterConfig.SubClusterNodeId,
		nbrNodes:           cpConfig.ClusterConfig.NbrNodes,
		nbrSubClusters:     cpConfig.ClusterConfig.NbrSubClusters,
		nbrSubClusterNodes: cpConfig.ClusterConfig.NbrSubClusterNodes,
	}

	// Start the metric reporting goroutine
	if cpConfig.MetricsConfig != nil && ctx.cpConfig.MetricsConfig.ReportInterval > 0 {
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

	err = ctx.buildComputeGraph()
	if err != nil {
		cpErr = fmt.Errorf("while building the compute graph: %s", err)
		goto gotError
	}

	// All done!
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	close(chResults.MapOnClusterResultCh)
	return

gotError:
	log.Println(cpErr)
	log.Println("**!@@ gotError in StartComputePipes")
	errCh <- cpErr
	close(done)
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	close(chResults.MapOnClusterResultCh)
}

func UnmarshalComputePipesConfig(computePipesJson *string, nodeId, nbrNodes int) (*ComputePipesConfig, error) {

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

	// set default nbr of sub-cluster if not set
	if cpConfig.ClusterConfig.NbrSubClusters == 0 {
		cpConfig.ClusterConfig.NbrSubClusters = 1
	}
	if cpConfig.ClusterConfig.NbrNodes == 0 {
		cpConfig.ClusterConfig.NbrNodes = nbrNodes
	}
	nbrNodes = cpConfig.ClusterConfig.NbrNodes
	nbrSubClusters := cpConfig.ClusterConfig.NbrSubClusters
	subClusterId := nodeId % nbrSubClusters
	nbrSubClusterNodes := nbrNodes / nbrSubClusters
	subClusterNodeId := nodeId / nbrSubClusters

	// Make sure the sub-clusters will all contain the same number of nodes
	if nbrNodes%nbrSubClusters != 0 {
		return nil, fmt.Errorf("error: cluster has %d nodes, cannot allocate them evenly in %d sub-clusters", nbrNodes,
			nbrSubClusters)
	}
	cpConfig.ClusterConfig.NodeId = nodeId
	cpConfig.ClusterConfig.SubClusterId = subClusterId
	cpConfig.ClusterConfig.NbrSubClusterNodes = nbrSubClusterNodes
	cpConfig.ClusterConfig.SubClusterNodeId = subClusterNodeId
	log.Printf("StartComputePipes: nodeId: %d, subClusterId: %d, subClusterNodeId: %d, nbrNodes: %d, nbrSubClusters: %d, nbrSubClusterNodes: %d",
		nodeId, subClusterId, subClusterNodeId, nbrNodes, nbrSubClusters, nbrSubClusterNodes)
	return &cpConfig, nil
}

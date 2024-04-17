package compute_pipes

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the Compute Pipes runtime data structures

type ChannelRegistry struct {
	// Compute Pipes input channel, called input_row
	computePipesInputCh  <-chan []interface{}
	inputChannelSpec     *ChannelSpec
	inputColumns         map[string]int
	computeChannels      map[string]*Channel
	outputTableChannels  []string
	closedChannels       map[string]bool
	closedChMutex        sync.Mutex
	distributionChannels map[string]*[]string
}

func (r *ChannelRegistry) AddDistributionChannel(input string) string {
	channels := r.distributionChannels[input]
	if channels == nil {
		c := make([]string, 0)
		channels = &c
		r.distributionChannels[input] = channels
	}
	echo := fmt.Sprintf("%s_%d", input, len(*channels))
	*channels = append(*channels, echo)
	// create the echo channel
	r.computeChannels[echo] = &Channel{
		channel: make(chan []interface{}),
		columns: r.computeChannels[input].columns,
		config:  r.computeChannels[input].config,
	}

	return echo
}

func (r *ChannelRegistry) CloseChannel(name string) {
	r.closedChMutex.Lock()
	defer r.closedChMutex.Unlock()
	if r.closedChannels[name] {
		return
	}
	c := r.computeChannels[name]
	if c != nil {
		fmt.Println("** Closing channel", name)
		close(c.channel)
	}
	r.closedChannels[name] = true
}

func (r *ChannelRegistry) GetInputChannel(name string) (*InputChannel, error) {
	if name == "input_row" {
		return &InputChannel{
			channel: r.computePipesInputCh,
			config:  r.inputChannelSpec,
			columns: r.inputColumns,
		}, nil
	}
	ch, ok := r.computeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: input channel '%s' not found in ChannelRegistry", name)
	}
	return &InputChannel{
		channel: ch.channel,
		config:  ch.config,
		columns: ch.columns,
	}, nil
}
func (r *ChannelRegistry) GetOutputChannel(name string) (*OutputChannel, error) {
	ch, ok := r.computeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: output channel '%s' not found in ChannelRegistry", name)
	}
	return &OutputChannel{
		channel: ch.channel,
		config:  ch.config,
		columns: ch.columns,
	}, nil
}

type Channel struct {
	channel chan []interface{}
	columns map[string]int
	config  *ChannelSpec
}
type InputChannel struct {
	channel <-chan []interface{}
	columns map[string]int
	config  *ChannelSpec
}
type OutputChannel struct {
	channel chan<- []interface{}
	columns map[string]int
	config  *ChannelSpec
}

type BuilderContext struct {
	dbpool          *pgxpool.Pool
	cpConfig        *ComputePipesConfig
	channelRegistry *ChannelRegistry
	selfAddress     string
	peersAddress    []string
	done            chan struct{}
	errCh           chan error
	chResults       *ChannelResults
	env             map[string]interface{}
	s3Uploader      *manager.Uploader
}

func (ctx *BuilderContext) JetsPartition() string {
	return ctx.env["$JETS_PARTITION"].(string)
}
func (ctx *BuilderContext) NodeId() int {
	return ctx.env["$SHARD_ID"].(int)
}
func (ctx *BuilderContext) NbrNodes() int {
	return ctx.env["$NBR_SHARDS"].(int)
}
func (ctx *BuilderContext) SessionId() string {
	return ctx.env["$SESSIONID"].(string)
}
func (ctx *BuilderContext) FileKey() string {
	return ctx.env["$FILE_KEY"].(string)
}

type PipeTransformationEvaluator interface {
	apply(input *[]interface{}) error
	done() error
	finally()
}

type TransformationColumnEvaluator interface {
	initializeCurrentValue(currentValue *[]interface{})
	update(currentValue *[]interface{}, input *[]interface{}) error
	done(currentValue *[]interface{}) error
}

type PipeSet map[*PipeSpec]bool
type Input2PipeSet map[string]*PipeSet

func (ctx *BuilderContext) buildComputeGraph() error {

	// Construct the in-memory compute graph
	// Build the Pipes
	fmt.Println("**& Start ComputeGraph")
	// look for pipes with common input, need to distribute the entities
	i2p := make(Input2PipeSet)
	for i := range ctx.cpConfig.PipesConfig {
		pipeSet := i2p[ctx.cpConfig.PipesConfig[i].Input]
		if pipeSet == nil {
			p := make(PipeSet)
			pipeSet = &p
			i2p[ctx.cpConfig.PipesConfig[i].Input] = pipeSet
		}
		(*pipeSet)[&ctx.cpConfig.PipesConfig[i]] = true
	}

	for i := range ctx.cpConfig.PipesConfig {
		fmt.Println("**& PipeConfig", i, "type", ctx.cpConfig.PipesConfig[i].Type)
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		pset := i2p[pipeSpec.Input]
		input := pipeSpec.Input
		if len(*pset) > 1 {
			input = ctx.channelRegistry.AddDistributionChannel(pipeSpec.Input)
		}
		source, err := ctx.channelRegistry.GetInputChannel(input)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			fmt.Println("**& starting PipeConfig", i, "fan_out", "on source", source.config.Name)
			go ctx.StartFanOutPipe(pipeSpec, source)

		case "splitter":
			fmt.Println("**& starting PipeConfig", i, "splitter", "on source", source.config.Name)
			// Create the writePartitionResultCh that will contain the number of part files for each partition
			writePartitionsResultCh := make(chan chan ComputePipesResult, 15000) // NOTE Max number of partitions
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			go ctx.StartSplitterPipe(pipeSpec, source, writePartitionsResultCh)

		case "distribute_data":
			fmt.Println("**& starting PipeConfig", i, "distribute_data", "on source", source.config.Name)
			// Create the clusterMapResultCh to report on the outgoing peer connection
			clusterMapResultCh := make(chan chan ComputePipesResult, ctx.NbrNodes()*5)
			ctx.chResults.MapOnClusterResultCh <- clusterMapResultCh
			go ctx.StartClusterMap(pipeSpec, source, clusterMapResultCh)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	// Start the distribution channels
	for input, echos := range ctx.channelRegistry.distributionChannels {
		if echos != nil {
			go func(in string, ec *[]string) {
				echoCh := make([]chan []interface{}, len(*ec))
				for i, echo := range *ec {
					echoCh[i] = ctx.channelRegistry.computeChannels[echo].channel
				}
				// start distribution
				for item := range ctx.channelRegistry.computeChannels[in].channel {
					for i := range echoCh {
						select {
						case echoCh[i] <- item:
						case <-ctx.done:
							return
						}
					}
				}
			}(input, echos)
		}
	}
	fmt.Println("**& Start ComputeGraph DONE")
	return nil
}

// Build the PipeTransformationEvaluator: one of map_record, aggregate, or partition_writer
// The partitionResultCh argument is used only by partition_writer to return the number of part files written and
// the error that might occur
func (ctx *BuilderContext) buildPipeTransformationEvaluator(source *InputChannel, jetsPartitionKey interface{},
	partitionResultCh chan ComputePipesResult, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	// fmt.Println("**& buildPipeTransformationEvaluator for", spec.Type, "source:", source.config.Name, "jetsPartitionKey:", *jetsPartitionKey, "output:", spec.Output)

	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
	if err != nil {
		err = fmt.Errorf("while in buildPipeTransformationEvaluator for %s from source %s requesting output channel %s: %v", spec.Type, source.config.Name, spec.Output, err)
		fmt.Println(err)
		if partitionResultCh != nil {
			close(partitionResultCh)
		}
		return nil, err
	}
	switch spec.Type {
	case "map_record":
		if partitionResultCh != nil {
			close(partitionResultCh)
		}
		return ctx.NewMapRecordTransformationPipe(source, outCh, spec)

	case "aggregate":
		if partitionResultCh != nil {
			close(partitionResultCh)
		}
		return ctx.NewAggregateTransformationPipe(source, outCh, spec)

	case "partition_writer":
		return ctx.NewPartitionWriterTransformationPipe(source, jetsPartitionKey, outCh, partitionResultCh, spec)

	default:
		if partitionResultCh != nil {
			close(partitionResultCh)
		}
		return nil, fmt.Errorf("error: unknown TransformationSpec type: %s", spec.Type)
	}
}

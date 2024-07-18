package compute_pipes

import (
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the Compute Pipes runtime data structures

type ChannelRegistry struct {
	// Compute Pipes input channel (inputRowChannel), called input_row
	// Used for sharding mode only
	inputRowChannel      *InputChannel
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
		config: &ChannelSpec{
			Name:    echo,
			Columns: r.computeChannels[input].config.Columns,
		},
	}
	log.Printf("AddDistributionChannel %s -> %s", input, echo)
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
		// log.Println("** Closing channel", name)
		close(c.channel)
	}
	r.closedChannels[name] = true
}

func (r *ChannelRegistry) GetInputChannel(name string) (*InputChannel, error) {
	if name == "input_row" {
		return r.inputRowChannel, nil
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
	dbpool             *pgxpool.Pool
	cpConfig           *ComputePipesConfig
	channelRegistry    *ChannelRegistry
	done               chan struct{}
	errCh              chan error
	chResults          *ChannelResults
	env                map[string]interface{}
	s3DeviceManager    *S3DeviceManager
	nodeId             int
}

func (ctx *BuilderContext) JetsPartition() string {
	return ctx.env["$JETS_PARTITION_LABEL"].(string)
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

	for i := range ctx.cpConfig.PipesConfig {
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		input := pipeSpec.Input
		source, err := ctx.channelRegistry.GetInputChannel(input)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			// log.Println("**& starting PipeConfig", i, "fan_out", "on source", source.config.Name)
			go ctx.StartFanOutPipe(pipeSpec, source)

		case "splitter":
			// log.Println("**& starting PipeConfig", i, "splitter", "on source", source.config.Name)
			// Create the writePartitionResultCh that will contain the number of part files for each partition
			writePartitionsResultCh := make(chan chan ComputePipesResult, 15000) // NOTE Max number of partitions
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			go ctx.StartSplitterPipe(pipeSpec, source, writePartitionsResultCh)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	return nil
}

// Build the PipeTransformationEvaluator: one of map_record, aggregate, or partition_writer
// The partitionResultCh argument is used only by partition_writer to return the number of part files written and
// the error that might occur
func (ctx *BuilderContext) buildPipeTransformationEvaluator(source *InputChannel, jetsPartitionKey interface{},
	partitionResultCh chan ComputePipesResult, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	// log.Println("**& buildPipeTransformationEvaluator for", spec.Type, "source:", source.config.Name, "jetsPartitionKey:", jetsPartitionKey, "output:", spec.Output)

	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
	if err != nil {
		err = fmt.Errorf("while in buildPipeTransformationEvaluator for %s from source %s requesting output channel %s: %v", spec.Type, source.config.Name, spec.Output, err)
		log.Println(err)
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

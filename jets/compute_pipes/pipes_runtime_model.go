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
	// correspond to the input file
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

func (r *ChannelRegistry) GetInputChannel(name string, hasGroupedRows bool) (*InputChannel, error) {
	if name == "input_row" {
		if r.inputRowChannel.hasGroupedRows != hasGroupedRows {
			return &InputChannel{
				channel:        r.inputRowChannel.channel,
				config:         r.inputRowChannel.config,
				columns:        r.inputRowChannel.columns,
				hasGroupedRows: hasGroupedRows,
			}, nil
		}
		return r.inputRowChannel, nil
	}
	ch, ok := r.computeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: input channel '%s' not found in ChannelRegistry", name)
	}
	return &InputChannel{
		channel:        ch.channel,
		config:         ch.config,
		columns:        ch.columns,
		hasGroupedRows: hasGroupedRows,
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
	channel        <-chan []interface{}
	columns        map[string]int
	config         *ChannelSpec
	hasGroupedRows bool
}
type OutputChannel struct {
	channel chan<- []interface{}
	columns map[string]int
	config  *ChannelSpec
}

type BuilderContext struct {
	dbpool             *pgxpool.Pool
	sessionId          string
	jetsPartition      string
	cpConfig           *ComputePipesConfig
	processName        string
	lookupTableManager *LookupTableManager
	schemaManager      *SchemaManager
	channelRegistry    *ChannelRegistry
	done               chan struct{}
	errCh              chan error
	chResults          *ChannelResults
	env                map[string]interface{}
	s3DeviceManager    *S3DeviceManager
	nodeId             int
}

func (ctx *BuilderContext) FileKey() string {
	return ctx.cpConfig.CommonRuntimeArgs.FileKey
}

type PipeTransformationEvaluator interface {
	Apply(input *[]interface{}) error
	Done() error
	Finally()
}

type TransformationColumnEvaluator interface {
	InitializeCurrentValue(currentValue *[]interface{})
	Update(currentValue *[]interface{}, input *[]interface{}) error
	Done(currentValue *[]interface{}) error
}

type PipeSet map[*PipeSpec]bool
type Input2PipeSet map[string]*PipeSet

func (ctx *BuilderContext) BuildComputeGraph() error {

	for i := range ctx.cpConfig.PipesConfig {
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		input := pipeSpec.InputChannel.Name
		source, err := ctx.channelRegistry.GetInputChannel(input, pipeSpec.InputChannel.HasGroupedRows)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			// log.Println("**& starting PipeConfig", i, "fan_out", "on source", source.config.Name)
			// Create the writePartitionResultCh in case it contains a partition_writter,
			// it would write a single partition, the ch will contain the number of rows for the partition
			writePartitionsResultCh := make(chan ComputePipesResult, 10)
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			go ctx.StartFanOutPipe(pipeSpec, source, writePartitionsResultCh)

		case "splitter":
			// log.Println("**& starting PipeConfig", i, "splitter", "on source", source.config.Name)
			// Create the writePartitionResultCh that will contain the number of rows for each partition
			writePartitionsResultCh := make(chan ComputePipesResult, 15000) // NOTE Max number of partitions
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			go ctx.StartSplitterPipe(pipeSpec, source, writePartitionsResultCh)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	return nil
}

// Build the PipeTransformationEvaluator: one of map_record, aggregate, or partition_writer
// The partitionResultCh argument is used only by partition_writer to return the number of rows written and
// the error that might occur
func (ctx *BuilderContext) BuildPipeTransformationEvaluator(source *InputChannel, jetsPartitionKey interface{},
	partitionResultCh chan ComputePipesResult, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	// log.Println("**& BuildPipeTransformationEvaluator for", spec.Type, "source:", source.config.Name, "jetsPartitionKey:", jetsPartitionKey, "output:", spec.Output)

	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.OutputChannel.Name)
	if err != nil {
		err = fmt.Errorf("while in BuildPipeTransformationEvaluator for %s from source %s requesting output channel %s: %v",
			spec.Type, source.config.Name, spec.OutputChannel.Name, err)
		log.Println(err)
		return nil, err
	}
	switch spec.Type {
	case "map_record":
		return ctx.NewMapRecordTransformationPipe(source, outCh, spec)

	case "aggregate":
		return ctx.NewAggregateTransformationPipe(source, outCh, spec)

	case "partition_writer":
		return ctx.NewPartitionWriterTransformationPipe(source, jetsPartitionKey, outCh, partitionResultCh, spec)

	case "group_by":
		return ctx.NewGroupByTransformationPipe(source, outCh, spec)

	case "analyze":
		return ctx.NewAnalyzeTransformationPipe(source, outCh, spec)

	case "anonymize":
		return ctx.NewAnonymizeTransformationPipe(source, outCh, spec)

	case "distinct":
		return ctx.NewDistinctTransformationPipe(source, outCh, spec)

	case "high_freq":
		return ctx.NewHighFreqTransformationPipe(source, outCh, spec)

	case "shuffling":
		return ctx.NewShufflingTransformationPipe(source, outCh, spec)

	default:
		return nil, fmt.Errorf("error: unknown TransformationSpec type: %s", spec.Type)
	}
}
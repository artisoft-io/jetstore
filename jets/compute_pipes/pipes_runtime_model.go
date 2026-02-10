package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the Compute Pipes runtime data structures

// ChannelRegistry keeps track of all input and output channels
// inputRowChannel, called input_row correspond to the main input file.
// InputMergeChannels correspond to any merge input files.
// ComputeChannels correspond to all other channels created for intermediate
// steps in the compute graph.
// OutputTableChannels correspond to the output tables that need to be written
// ClosedChannels keeps track of which channels have been closed
type ChannelRegistry struct {
	InputRowChannel      *InputChannel
	ComputeChannels      map[string]*Channel
	OutputTableChannels  []string
	ClosedChannels       map[string]bool
	ClosedChMutex        sync.Mutex
	DistributionChannels map[string]*[]string
}

func (r *ChannelRegistry) AddDistributionChannel(input string) string {
	channels := r.DistributionChannels[input]
	if channels == nil {
		c := make([]string, 0)
		channels = &c
		r.DistributionChannels[input] = channels
	}
	echo := fmt.Sprintf("%s_%d", input, len(*channels))
	*channels = append(*channels, echo)
	// create the echo channel
	r.ComputeChannels[echo] = &Channel{
		Name:    echo,
		Channel: make(chan []any),
		Columns: r.ComputeChannels[input].Columns,
		Config:  r.ComputeChannels[input].Config,
	}
	log.Printf("AddDistributionChannel %s -> %s", input, echo)
	return echo
}

func (r *ChannelRegistry) CloseChannel(name string) {
	r.ClosedChMutex.Lock()
	defer r.ClosedChMutex.Unlock()
	if r.ClosedChannels[name] {
		return
	}
	c := r.ComputeChannels[name]
	if c != nil {
		// log.Println("** Closing channel", name)
		close(c.Channel)
	}
	r.ClosedChannels[name] = true
}

func (r *ChannelRegistry) GetInputChannel(name string, hasGroupedRows bool) (*InputChannel, error) {
	if name == "input_row" {
		if r.InputRowChannel.HasGroupedRows != hasGroupedRows {
			return &InputChannel{
				Name:           name,
				Channel:        r.InputRowChannel.Channel,
				Config:         r.InputRowChannel.Config,
				Columns:        r.InputRowChannel.Columns,
				DomainKeySpec:  r.InputRowChannel.DomainKeySpec,
				HasGroupedRows: hasGroupedRows,
			}, nil
		}
		return r.InputRowChannel, nil
	}
	ch, ok := r.ComputeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: input channel '%s' not found in ChannelRegistry", name)
	}
	return &InputChannel{
		Name:           name,
		Channel:        ch.Channel,
		Config:         ch.Config,
		Columns:        ch.Columns,
		DomainKeySpec:  ch.DomainKeySpec,
		HasGroupedRows: hasGroupedRows,
	}, nil
}
func (r *ChannelRegistry) GetOutputChannel(name string) (*OutputChannel, error) {
	ch, ok := r.ComputeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: output channel '%s' not found in ChannelRegistry", name)
	}
	return &OutputChannel{
		Name:    name,
		Channel: ch.Channel,
		Config:  ch.Config,
		Columns: ch.Columns,
	}, nil
}

type Channel struct {
	Name          string
	Channel       chan []any
	Columns       *map[string]int
	DomainKeySpec *DomainKeysSpec
	Config        *ChannelSpec
}
type InputChannel struct {
	Name           string
	Channel        <-chan []any
	Columns        *map[string]int
	DomainKeySpec  *DomainKeysSpec
	Config         *ChannelSpec
	HasGroupedRows bool
}
type OutputChannel struct {
	Name    string
	Channel chan<- []any
	Columns *map[string]int
	Config  *ChannelSpec
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
	inputParquetSchema *ParquetSchemaInfo
	jetRules           JetRulesProxy
	done               chan struct{}
	errCh              chan error
	chResults          *ChannelResults
	env                map[string]any
	s3DeviceManager    *S3DeviceManager
	nodeId             int
}

func (ctx *BuilderContext) FileKey() string {
	return ctx.cpConfig.CommonRuntimeArgs.FileKey
}

// Delegate to ExprBuilderContext
func (ctx *BuilderContext) BuildExprNodeEvaluator(sourceName string, columns map[string]int, spec *ExpressionNode) (evalExpression, error) {
	if ctx == nil {
		m := make(map[string]any)
		return ExprBuilderContext(m).BuildExprNodeEvaluator(sourceName, columns, spec)
	}
	return ExprBuilderContext(ctx.env).BuildExprNodeEvaluator(sourceName, columns, spec)
}

// Delegate to ExprBuilderContext
func (ctx *BuilderContext) parseValue(expr *string) (any, error) {
	if ctx == nil {
		m := make(map[string]any)
		return ExprBuilderContext(m).parseValue(expr)
	}
	return ExprBuilderContext(ctx.env).parseValue(expr)
}

type PipeTransformationEvaluator interface {
	Apply(input *[]any) error
	Done() error
	Finally()
}

// Initialize and Done are intended for aggregate transformations column evaluators
type TransformationColumnEvaluator interface {
	InitializeCurrentValue(currentValue *[]any)
	Update(currentValue *[]any, input *[]any) error
	Done(currentValue *[]any) error
}

type PipeSet map[*PipeSpec]bool
type Input2PipeSet map[string]*PipeSet

func (ctx *BuilderContext) BuildComputeGraph() error {
	if ctx.cpConfig.ClusterConfig.IsDebugMode {
		b, _ := json.Marshal(ctx.env)
		log.Printf("Entering BuildComputeGraph: ctx env var: %s\n", string(b))
	}

	var wg sync.WaitGroup
	for i := range ctx.cpConfig.PipesConfig {
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		input := pipeSpec.InputChannel.Name
		source, err := ctx.channelRegistry.GetInputChannel(input, pipeSpec.InputChannel.HasGroupedRows)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			// log.Println("**& starting PipeConfig", i, "fan_out", "on source", source.name)
			// Create the writePartitionResultCh in case it contains a partition_writter,
			// it would write a single partition, the ch will contain the number of rows for the partition
			writePartitionsResultCh := make(chan ComputePipesResult, 10)
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx.StartFanOutPipe(pipeSpec, source, writePartitionsResultCh)
			}()

		case "splitter":
			// log.Println("**& starting PipeConfig", i, "splitter", "on source", source.name)
			// Create the writePartitionResultCh that will contain the number of rows for each partition
			writePartitionsResultCh := make(chan ComputePipesResult, 15000) // NOTE Max number of partitions
			ctx.chResults.WritePartitionsResultCh <- writePartitionsResultCh
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx.StartSplitterPipe(pipeSpec, source, writePartitionsResultCh)
			}()

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	// Wait for the graph to build
	// log.Println("Waiting for the graph to be build")
	wg.Wait()
	// log.Println("Waiting for the graph to be build DONE")
	return nil
}

// Build the PipeTransformationEvaluator: one of map_record, aggregate, or partition_writer
// The partitionResultCh argument is used only by partition_writer to return the number of rows written and
// the error that might occur
func (ctx *BuilderContext) BuildPipeTransformationEvaluator(source *InputChannel, jetsPartitionKey any,
	partitionResultCh chan ComputePipesResult, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	// log.Println("**& BuildPipeTransformationEvaluator for", spec.Type, "source:", source.name, "jetsPartitionKey:", jetsPartitionKey, "output:", spec.Output)

	// Get the output channel
	var outCh *OutputChannel
	var err error
	if len(spec.OutputChannel.Name) > 0 {
		outCh, err = ctx.channelRegistry.GetOutputChannel(spec.OutputChannel.Name)
		if err != nil {
			err = fmt.Errorf("while in BuildPipeTransformationEvaluator for %s from source %s requesting output channel %s: %v",
				spec.Type, source.Name, spec.OutputChannel.Name, err)
			log.Println(err)
			return nil, err
		}
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

	case "merge":
		return ctx.NewMergeTransformationPipe(source, outCh, spec)

	case "distinct":
		return ctx.NewDistinctTransformationPipe(source, outCh, spec)

	case "filter":
		return ctx.NewFilterTransformationPipe(source, outCh, spec)

	case "sort":
		return ctx.NewSortTransformationPipe(source, outCh, spec)

	case "jetrules":
		return ctx.NewJetrulesTransformationPipe(source, outCh, spec)

	case "analyze":
		return ctx.NewAnalyzeTransformationPipe(source, outCh, spec)

	case "anonymize":
		return ctx.NewAnonymizeTransformationPipe(source, outCh, spec)

	case "clustering":
		return ctx.NewClusteringTransformationPipe(source, outCh, spec)

	case "high_freq":
		return ctx.NewHighFreqTransformationPipe(source, outCh, spec)

	case "shuffling":
		return ctx.NewShufflingTransformationPipe(source, outCh, spec)

	default:
		return nil, fmt.Errorf("error: unknown TransformationSpec type: %s", spec.Type)
	}
}

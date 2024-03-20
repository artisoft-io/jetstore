package compute_pipes

import (
	"fmt"
	"sync"
)

// This file contains the Compute Pipes runtime data structures

type ChannelRegistry struct {
	// Compute Pipes input channel, called input_row
	computePipesInputCh <-chan []interface{}
	inputChannelSpec    *ChannelSpec
	inputColumns        map[string]int
	computeChannels     map[string]*Channel
	outputTableChannels []string
	closedChannels      map[string]bool
	closedChMutex       sync.Mutex
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
	cpConfig             *ComputePipesConfig
	channelRegistry      *ChannelRegistry
	done                 chan struct{}
	errCh                chan error
	computePipesResultCh chan chan ComputePipesResult
	env map[string]interface{}
}

type PipeTransformationEvaluator interface {
	apply(input *[]interface{}) error
	done() error
}

type TransformationColumnEvaluator interface {
	initializeCurrentValue(currentValue *[]interface{})
	update(currentValue *[]interface{}, input *[]interface{}) error
	done(currentValue *[]interface{}) error
}

func (ctx *BuilderContext) buildComputeGraph() error {

	// Construct the in-memory compute graph
	// Build the Pipes
	fmt.Println("**& Start ComputeGraph")
	for i := range ctx.cpConfig.PipesConfig {
		// fmt.Println("**& PipeConfig", i, "type", ctx.cpConfig.PipesConfig[i].Type)
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		source, err := ctx.channelRegistry.GetInputChannel(pipeSpec.Input)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			// fmt.Println("**& starting PipeConfig", i, "fan_out", "on source", source.config.Name)
			go ctx.startFanOutPipe(pipeSpec, source)

		case "splitter":
			// fmt.Println("**& starting PipeConfig", i, "splitter", "on source", source.config.Name)
			go ctx.startSplitterPipe(pipeSpec, source)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	fmt.Println("**& Start ComputeGraph DONE")
	return nil
}

func (ctx *BuilderContext) buildPipeTransformationEvaluator(source *InputChannel, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	// fmt.Println("**& buildPipeTransformationEvaluator for", spec.Type,"source:",source.config.Name,"output:", spec.Output)

	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
	if err != nil {
		err = fmt.Errorf("while in buildPipeTransformationEvaluator for %s from source %s requesting output channel %s: %v", spec.Type, source.config.Name, spec.Output, err)
		fmt.Println(err)
		return nil, err
	}
	switch spec.Type {
	case "map_record":
		return ctx.NewMapRecordTransformationPipe(source, outCh, spec)

	case "aggregate":
		return ctx.NewAggregateTransformationPipe(source, outCh, spec)

	default:
		return nil, fmt.Errorf("error: unknown TransformationSpec type: %s", spec.Type)
	}
}

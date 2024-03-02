package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
)

// This file contains the Compute Pipes runtime data structures

type ChannelRegistry struct {
	// Compute Pipes input channel, called input_row
	computePipesInputCh <-chan []interface{}
	inputChannelSpec    *ChannelSpec
	inputColumns        map[string]int
	computeChannels     map[string]*Channel
}

func (r *ChannelRegistry) GetInputChannel(name string) (*InputChannel, error) {
	if name == "input_row" {
		return &InputChannel{
			channel: r.computePipesInputCh,
			config:  r.inputChannelSpec,
		}, nil
	}
	ch, ok := r.computeChannels[name]
	if !ok {
		return nil, fmt.Errorf("error: input channel '%s' not found in ChannelRegistry", name)
	}
	return &InputChannel{
		channel: ch.channel,
		config:  ch.config,
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
	computePipesResultCh chan<- ComputePipesResult
}

type TransformationColumnEvaluator interface {
	update(currentValue *[]interface{}, input *[]interface{}) error
}

func (ctx BuilderContext) buildComputeGraph() error {

	// Construct the in-memory compute graph
	// Build the Pipes
	for i := range ctx.cpConfig.PipesConfig {
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		source, err := ctx.channelRegistry.GetInputChannel(pipeSpec.Input)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}
		switch pipeSpec.Type {

		case "fan_out":
			// Apply each transformation to the input source
			for j := range pipeSpec.Apply {
				err = ctx.buildPipeTransformation(source, &pipeSpec.Apply[j])
				if err != nil {
					return fmt.Errorf("while calling buildPipeTransformation for fan_out: %v", err)
				}
			}

		case "splitter":
			go ctx.startSplitterPipe(source, pipeSpec)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	return nil
}

func (ctx BuilderContext) buildPipeTransformation(source *InputChannel, spec *TransformationSpec) error {

	// Construct the pipe transformation
	switch spec.Type {
	case "map_record":

	case "aggregate":
		go ctx.startAggregateTransform(source, spec)

	default:
		return fmt.Errorf("error: unknown TransformationSpec type: %s", spec.Type)
	}
	return nil
}

// start an aggregate pipe transformation
func (ctx BuilderContext) startAggregateTransform(source *InputChannel, spec *TransformationSpec) {
	var cpErr error
	// compile the TransformationColumnSpec into runtime evaluators
	evaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	var currentValues []interface{}
	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
	if err != nil {
		cpErr = fmt.Errorf("while requesting output channel %s", spec.Output)
		goto gotError
	}
	currentValues = make([]interface{}, len(outCh.config.Columns))
	for i := range spec.Columns {
		evaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outCh, &spec.Columns[i])
		if err != nil {
			cpErr = fmt.Errorf("while buildTransformationColumnEvaluator %v", err)
			goto gotError	
		}
	}
	// setup the aggregate loop on the source
	for inRow := range source.channel {
		for i := range spec.Columns {
			err = evaluators[i].update(&currentValues, &inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling update on TransformationColumnEvaluator: %v", err)
				goto gotError	
			}
		}
	}

	// Send the result to output
	select {
	case outCh.channel <- currentValues:
	case <-ctx.done:
		log.Println("AggregateTransform interrupted")
	}

	// All good!
	return

gotError:
	log.Println(cpErr)
	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
	close(ctx.done)
}

// start the splitter on the source based on the pipeSpec
func (ctx BuilderContext) startSplitterPipe(source *InputChannel, pipeSpec *PipeSpec) {
	// the map containing all the channels corresponding to values @ spliterColumnIdx
	chanState := make(map[string]chan []interface{})
	var cpErr error
	spliterColumnIdx, ok := source.columns[*pipeSpec.Column]
	if !ok {
		cpErr = fmt.Errorf("error: invalid column name %s for channel %s", *pipeSpec.Column, source.config.Name)
		goto gotError
	}

	for inRow := range source.channel {
		var key string
		v := inRow[spliterColumnIdx]
		if v != nil {
			// improve this by supporting different types in the splitting column
			switch vv := v.(type) {
			case string:
				key = vv
			case int:
				key = strconv.Itoa(vv)
			}
			if len(key) > 0 {
				splitCh := chanState[key]
				if splitCh == nil {
					splitCh = make(chan []interface{})
					chanState[key] = splitCh
					// configure the pipe around the channel
					// Apply each transformation to the input source
					for j := range pipeSpec.Apply {
						err := ctx.buildPipeTransformation(source, &pipeSpec.Apply[j])
						if err != nil {
							cpErr = fmt.Errorf("while calling buildPipeTransformation for splitter on %s for channel %s", *pipeSpec.Column, source.config.Name)
							goto gotError
						}
					}
				}
			}
		}
	}
	// All good!
	return

gotError:
	log.Println(cpErr)
	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
	close(ctx.done)
}

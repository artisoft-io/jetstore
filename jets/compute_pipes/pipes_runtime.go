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
	computePipesResultCh chan<- ComputePipesResult
	env map[string]interface{}
}

type PipeTransformationEvaluator interface {
	apply(input *[]interface{}) error
	done() error
}

type TransformationColumnEvaluator interface {
	initializeCurrentValue(currentValue *[]interface{})
	update(currentValue *[]interface{}, input *[]interface{}) error
}

func (ctx BuilderContext) buildComputeGraph() error {

	// Construct the in-memory compute graph
	// Build the Pipes
	fmt.Println("**& Start ComputeGraph")
	for i := range ctx.cpConfig.PipesConfig {
		fmt.Println("**& PipeConfig", i, "type", ctx.cpConfig.PipesConfig[i].Type)
		pipeSpec := &ctx.cpConfig.PipesConfig[i]
		source, err := ctx.channelRegistry.GetInputChannel(pipeSpec.Input)
		if err != nil {
			return fmt.Errorf("while building Pipe: %v", err)
		}

		switch pipeSpec.Type {
		case "fan_out":
			fmt.Println("**& starting PipeConfig", i, "fan_out", "on source", source.config.Name)
			go ctx.startFanOutPipe(pipeSpec, source)

		case "splitter":
			fmt.Println("**& starting PipeConfig", i, "splitter", "on source", source.config.Name)
			go ctx.startSplitterPipe(pipeSpec, source)

		default:
			return fmt.Errorf("error: unknown PipeSpec type: %s", pipeSpec.Type)
		}
	}
	fmt.Println("**& Start ComputeGraph DONE")
	return nil
}

func (ctx BuilderContext) buildPipeTransformationEvaluator(source *InputChannel, spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Construct the pipe transformation
	fmt.Println("**& buildPipeTransformationEvaluator for", spec.Type,"source:",source.config.Name)

	// Get the output channel
	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
	if err != nil {
		err = fmt.Errorf("while requesting output channel %s: %v", spec.Output, err)
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

// // start a map_record pipe transformation
// func (ctx BuilderContext) startMapRecordTransform(source *InputChannel, spec *TransformationSpec) {
// 	fmt.Println("**! startMapRecordTransform called for",spec.Type)
// 	var cpErr error
// 	// compile the TransformationColumnSpec into runtime evaluators
// 	evaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
// 	var currentValues []interface{}
// 	// Get the output channel
// 	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
// 	if err != nil {
// 		cpErr = fmt.Errorf("while requesting output channel %s: %v", spec.Output, err)
// 		goto gotError
// 	}
// 	for i := range spec.Columns {
// 		fmt.Printf("**! build TransformationColumn[%d] of type %s in aggregate loop frm source %s", i, spec.Type,source.config.Name)
// 		evaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outCh, &spec.Columns[i])
// 		if err != nil {
// 			cpErr = fmt.Errorf("while buildTransformationColumnEvaluator (in MapRecordTransform) %v", err)
// 			goto gotError	
// 		}
// 		evaluators[i].initializeCurrentValue(&currentValues)
// 	}
// 	// setup the map_record (from fan_out) loop on the source
// 	fmt.Println("**! start map_record loop", spec.Type,"source:",source.config.Name)
// 	for inRow := range source.channel {
// 		fmt.Println("**! map_record loop row:", inRow,"from source:",source.config.Name)
// 		currentValues = make([]interface{}, len(outCh.config.Columns))
// 		for i := range spec.Columns {
// 			err = evaluators[i].update(&currentValues, &inRow)
// 			if err != nil {
// 				cpErr = fmt.Errorf("while calling update on TransformationColumnEvaluator (in MapRecordTransform): %v", err)
// 				goto gotError	
// 			}
// 		}
// 		// Send the result to output
// 		fmt.Println("**! map_record loop out row:", currentValues, "from",source.config.Name,"to outCh:",outCh.config.Name)
// 		select {
// 		case outCh.channel <- currentValues:
// 		case <-ctx.done:
// 			log.Println("MapRecordTransform interrupted")
// 			return
// 		}
// 	}

// 	// All good!
// 	return

// gotError:
// 	log.Println(cpErr)
// 	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
// 	close(ctx.done)
// }

// // start an aggregate pipe transformation
// func (ctx BuilderContext) startAggregateTransform(source *InputChannel, spec *TransformationSpec) {
// 	var cpErr error
// 	// compile the TransformationColumnSpec into runtime evaluators
// 	evaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
// 	var currentValues []interface{}
// 	// Get the output channel
// 	outCh, err := ctx.channelRegistry.GetOutputChannel(spec.Output)
// 	if err != nil {
// 		cpErr = fmt.Errorf("while requesting output channel %s", spec.Output)
// 		goto gotError
// 	}
// 	currentValues = make([]interface{}, len(outCh.config.Columns))
// 	for i := range spec.Columns {
// 		evaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outCh, &spec.Columns[i])
// 		if err != nil {
// 			cpErr = fmt.Errorf("while buildTransformationColumnEvaluator %v", err)
// 			goto gotError	
// 		}
// 		evaluators[i].initializeCurrentValue(&currentValues)
// 	}
// 	// setup the aggregate loop on the source
// 	for inRow := range source.channel {
// 		for i := range spec.Columns {
// 			err = evaluators[i].update(&currentValues, &inRow)
// 			if err != nil {
// 				cpErr = fmt.Errorf("while calling update on TransformationColumnEvaluator: %v", err)
// 				goto gotError	
// 			}
// 		}
// 	}

// 	// Send the result to output
// 	select {
// 	case outCh.channel <- currentValues:
// 	case <-ctx.done:
// 		log.Println("AggregateTransform interrupted")
// 	}

// 	// All good!
// 	return

// gotError:
// 	log.Println(cpErr)
// 	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
// 	close(ctx.done)
// }

// // start the splitter on the source based on the pipeSpec
// func (ctx BuilderContext) startSplitterPipe(source *InputChannel, pipeSpec *PipeSpec) {
// 	// the map containing all the channels corresponding to values @ spliterColumnIdx
// 	chanState := make(map[string]chan []interface{})
// 	var cpErr error
// 	spliterColumnIdx, ok := source.columns[*pipeSpec.Column]
// 	if !ok {
// 		cpErr = fmt.Errorf("error: invalid column name %s for channel %s", *pipeSpec.Column, source.config.Name)
// 		goto gotError
// 	}

// 	for inRow := range source.channel {
// 		var key string
// 		v := inRow[spliterColumnIdx]
// 		if v != nil {
// 			// improve this by supporting different types in the splitting column
// 			switch vv := v.(type) {
// 			case string:
// 				key = vv
// 			case int:
// 				key = strconv.Itoa(vv)
// 			}
// 			if len(key) > 0 {
// 				splitCh := chanState[key]
// 				if splitCh == nil {
// 					splitCh = make(chan []interface{})
// 					chanState[key] = splitCh
// 					// configure the pipe around the channel
// 					// Apply each transformation to the input source
// 					for j := range pipeSpec.Apply {
// 						err := ctx.buildPipeTransformation(source, &pipeSpec.Apply[j])
// 						if err != nil {
// 							cpErr = fmt.Errorf("while calling buildPipeTransformation for splitter on %s for channel %s", *pipeSpec.Column, source.config.Name)
// 							goto gotError
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	// All good!
// 	return

// gotError:
// 	log.Println(cpErr)
// 	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
// 	close(ctx.done)
// }

package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
)

func (ctx *BuilderContext) startSplitterPipe(spec *PipeSpec, source *InputChannel) {
	var cpErr error
	// the map containing all the channels corresponding to values @ spliterColumnIdx
	chanState := make(map[string]chan []interface{})
	spliterColumnIdx, ok := source.columns[*spec.Column]
	if !ok {
		cpErr = fmt.Errorf("error: invalid column name %s for splitter with source channel %s", *spec.Column, source.config.Name)
		goto gotError
	}

	defer func() {
		fmt.Println("Closing startSplitterPipe")
		// Close all the intermediate channels after the splitterPipes are done
		for _, ch := range chanState {
			close(ch)
		}
	}()

	// fmt.Println("**! start splitter loop on source:",source.config.Name)
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
					// start a goroutine to manage the channel
					// the input channel to the goroutine is splitCh
					go ctx.startSplitterChannelHandler(spec, &InputChannel{
						channel: splitCh,
						columns: source.columns,
						config: &ChannelSpec{Name: "splitter_generated"},
					})
				}
				// Send the record to the intermediate channel
				// fmt.Println("**! splitter loop, sending record to intermediate channel:", key)
				select {
				case splitCh <- inRow:
				case <-ctx.done:
					log.Println("startSplitterPipe interrupted")
					return
				}				
			}
		}
	}
	// All good!
	return
gotError:
	log.Println(cpErr)
	// fmt.Println("**! gotError, writting to computePipesResultCh (ComputePipesResult)")
	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
	close(ctx.done)
}

func (ctx *BuilderContext) startSplitterChannelHandler(spec *PipeSpec, source *InputChannel) {
	var cpErr, err error
	var evaluators []PipeTransformationEvaluator
	defer func() {
		for i := range spec.Apply {
			if evaluators[i] != nil {
				err = evaluators[i].done()
				if err != nil {
					log.Printf("while calling done on PipeTransformationEvaluator (in splitter): %v", err)
				}	
			}
		}
		// Closing the output channels
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].Output] = true
		}
		for i := range oc {
			ctx.channelRegistry.CloseChannel(i)
		}
	}()
	// Build the PipeTransformationEvaluator
	evaluators = make([]PipeTransformationEvaluator, len(spec.Apply))
	for j := range spec.Apply {
		eval, err := ctx.buildPipeTransformationEvaluator(source, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling buildPipeTransformationEvaluator for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
		evaluators[j] = eval
  }
	// Process the channel
	for inRow := range source.channel {
		for i := range evaluators {
			err = evaluators[i].apply(&inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling apply on PipeTransformationEvaluator (in splitter): %v", err)
				goto gotError	
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
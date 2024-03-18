package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
	"sync"
)

func (ctx *BuilderContext) startSplitterPipe(spec *PipeSpec, source *InputChannel) {
	var cpErr error
	var wg sync.WaitGroup
	var oc map[string]bool
	// the map containing all the channels corresponding to values @ spliterColumnIdx
	chanState := make(map[string]chan []interface{})
	spliterColumnIdx, ok := source.columns[*spec.Column]
	if !ok {
		cpErr = fmt.Errorf("error: invalid column name %s for splitter with source channel %s", *spec.Column, source.config.Name)
		goto gotError
	}

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
					wg.Add(1)
					go ctx.startSplitterChannelHandler(spec, &InputChannel{
						channel: splitCh,
						columns: source.columns,
						config: &ChannelSpec{Name: fmt.Sprintf("splitter channel from %s", source.config.Name)},
					}, &wg)
				}
				// Send the record to the intermediate channel
				// fmt.Println("**! splitter loop, sending record to intermediate channel:", key)
				select {
				case splitCh <- inRow:
				case <-ctx.done:
					// log.Printf("startSplitterPipe writting to splitter intermediate channel with key %s from '%s' interrupted", key, source.config.Name)
					goto doneSplitterLoop
				}				
			}
		}
	}
	doneSplitterLoop:
	// Close all the intermediate channels
	for _, ch := range chanState {
		// fmt.Println("**! startSplitterPipe closing intermediate channel", key)
		close(ch)
	}

	// Close the output channels once all ch handlers are done
	wg.Wait()
	// Closing the output channels
	oc = make(map[string]bool)
	for i := range spec.Apply {
		oc[spec.Apply[i].Output] = true
	}
	for i := range oc {
		fmt.Println("**! SplitterPipe: Closing Output Channel",i)
		ctx.channelRegistry.CloseChannel(i)
	}
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
}

func (ctx *BuilderContext) startSplitterChannelHandler(spec *PipeSpec, source *InputChannel, wg *sync.WaitGroup) {
	var cpErr, err error
	var evaluators []PipeTransformationEvaluator
	defer func() {
		wg.Done()
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
				cpErr = fmt.Errorf("while calling apply on PipeTransformationEvaluator (in startSplitterChannelHandler): %v", err)
				goto gotError	
			}
		}
	}
	// Done, close the evaluators
	for i := range spec.Apply {
		if evaluators[i] != nil {
			err = evaluators[i].done()
			if err != nil {
				log.Printf("while calling done on PipeTransformationEvaluator (in startSplitterChannelHandler): %v", err)
			}	
		}
	}
	// All good!
	return

gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
}
package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
	"sync"
)

func (ctx *BuilderContext) StartSplitterPipe(spec *PipeSpec, source *InputChannel, writePartitionsResultCh chan chan ComputePipesResult) {
	var cpErr error
	var wg sync.WaitGroup
	defer func() {
		close(writePartitionsResultCh)
		// Closing the output channels
		fmt.Println("**! SPLITTER: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].Output] = true
		}
		for i := range oc {
			fmt.Println("**! SPLITTER: Closing Output Channel", i)
			ctx.channelRegistry.CloseChannel(i)
		}
	}()

	// the map containing all the intermediate channels corresponding to values @ spliterColumnIdx
	chanState := make(map[string]chan []interface{})
	spliterColumnIdx, ok := source.columns[*spec.Column]
	if !ok {
		cpErr = fmt.Errorf("error: invalid column name %s for splitter with source channel %s", *spec.Column, source.config.Name)
		goto gotError
	}

	fmt.Println("**! start splitter loop on source:",source.config.Name)
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
					splitCh = make(chan []interface{}, 1)
					chanState[key] = splitCh
					// start a goroutine to manage the channel
					// the input channel to the goroutine is splitCh
					wg.Add(1)								
					go ctx.startSplitterChannelHandler(spec, &InputChannel{
						channel: splitCh,
						columns: source.columns,
						config:  &ChannelSpec{Name: fmt.Sprintf("splitter channel from %s", source.config.Name)},
					}, writePartitionsResultCh, &key, &wg)
				}
				// Send the record to the intermediate channel
				fmt.Println("**! splitter loop, sending record to intermediate channel:", key)
				select {
				case splitCh <- inRow:
				case <-ctx.done:
					log.Printf("startSplitterPipe writing to splitter intermediate channel with key %s from '%s' interrupted", key, source.config.Name)
					goto doneSplitterLoop
				}
			}
		}
	}
doneSplitterLoop:
	// Close all the intermediate channels
	for key, ch := range chanState {
		fmt.Println("**! startSplitterPipe closing intermediate channel", key)
		close(ch)
	}
	// Close the output channels once all ch handlers are done
	fmt.Println("**!@@ Splitter loop done, ABOUT to wait on wg")
	wg.Wait()
	fmt.Println("**!@@ Splitter loop done, DONE waiting on wg!")
	// Closing the output channels via the defer above
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
}

func (ctx *BuilderContext) startSplitterChannelHandler(spec *PipeSpec, source *InputChannel, writePartitionsResultCh chan chan ComputePipesResult,
	splitterKey *string, wg *sync.WaitGroup) {
	var cpErr, err error
	var evaluators []PipeTransformationEvaluator
	defer func() {
		wg.Done()
	}()
	// fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ Called")
	// Build the PipeTransformationEvaluator
	evaluators = make([]PipeTransformationEvaluator, len(spec.Apply))
	for j := range spec.Apply {
		// partitionResultCh will have the aggregated count of files written by the partition writer
		partitionResultCh := make(chan ComputePipesResult, 1)
		writePartitionsResultCh <- partitionResultCh
		eval, err := ctx.buildPipeTransformationEvaluator(source, splitterKey, partitionResultCh, &spec.Apply[j])
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
			evaluators[i].finally()
		}
	}
	fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ All good!")
	// All good!
	return

gotError:
	for i := range spec.Apply {
		if evaluators[i] != nil {
			evaluators[i].finally()
		}
	}
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
}

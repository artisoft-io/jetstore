package compute_pipes

import (
	"fmt"
	"log"
	"math/rand/v2"
	"runtime/debug"
	"strings"
	"sync"
)

func (ctx *BuilderContext) StartSplitterPipe(spec *PipeSpec, source *InputChannel, writePartitionsResultCh chan ComputePipesResult) {
	var cpErr error
	var wg sync.WaitGroup
	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			cpErr := fmt.Errorf("StartSplitterPipe: recovered error: %v", r)
			log.Println(cpErr)
			debug.PrintStack()
			ctx.errCh <- cpErr
			close(ctx.done)
		}
		close(writePartitionsResultCh)
		// Closing the output channels
		// fmt.Println("**!@@ SPLITTER: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].OutputChannel.Name] = true
		}
		for i := range oc {
			// fmt.Println("**!@@ SPLITTER: Closing Output Channel", i)
			ctx.channelRegistry.CloseChannel(i)
		}
	}()
	var chanState map[interface{}]chan []interface{}
	var spliterColumnIdx int
	var ok bool
	var config *SplitterSpec
	var key interface{}

	if spec.SplitterConfig == nil {
		cpErr = fmt.Errorf("error: missing splitter_config for splitter with source channel %s", source.config.Name)
		goto gotError
	}
	config = spec.SplitterConfig
	if len(config.Column) == 0 && len(config.DefaultSplitterValue) == 0 && config.RandSuffix == 0 {
		cpErr = fmt.Errorf("error: invalid splitter_config for splitter with source channel %s", source.config.Name)
		goto gotError
	}

	// the map containing all the intermediate channels corresponding to values @ spliterColumnIdx
	chanState = make(map[interface{}]chan []interface{})
	if len(config.Column) > 0 {
		spliterColumnIdx, ok = source.columns[config.Column]
		if !ok {
			cpErr = fmt.Errorf("error: invalid column name %s for splitter with source channel %s", config.Column, source.config.Name)
			goto gotError
		}
	} else {
		spliterColumnIdx = -1
	}
	if len(config.DefaultSplitterValue) > 0 {
		if strings.Contains(config.DefaultSplitterValue, "$") {
			for key, v := range ctx.env {
				value, ok := v.(string)
				if ok {
					config.DefaultSplitterValue = strings.ReplaceAll(config.DefaultSplitterValue, key, value)
				}
			}
		}
	}

	// fmt.Println("**!@@ start splitter loop on source:",source.config.Name)
	for inRow := range source.channel {
		key = nil
		if spliterColumnIdx >= 0 {
			key = inRow[spliterColumnIdx]
		}
		if key == nil && len(config.DefaultSplitterValue) > 0 {
				key = config.DefaultSplitterValue
		}
		if config.RandSuffix > 0 {
			if key != nil {
				key = fmt.Sprintf("%v|%d", key, rand.IntN(config.RandSuffix))
			} else {
				key = rand.IntN(config.RandSuffix)
			}
		}
		if key == nil {
			log.Println(ctx.sessionId, "node", ctx.nodeId, "*WARNING* splitter with nil key on source", source.config.Name)
		}
		splitCh := chanState[key]
		if splitCh == nil {
			// unseen value, create an slot with an intermediate channel
			// log.Printf("**!@@ SPLITTER NEW KEY: %v", key)
			splitCh = make(chan []interface{}, 1)
			chanState[key] = splitCh

			if ctx.cpConfig.ClusterConfig.IsDebugMode {
				if len(chanState)%5 == 0 {
					log.Println(ctx.sessionId, "node", ctx.nodeId, "splitter size:", len(chanState), " on source", source.config.Name)
				}
			}

			// start a goroutine to manage the channel
			// the input channel to the goroutine is splitCh
			wg.Add(1)
			go ctx.startSplitterChannelHandler(spec, &InputChannel{
				channel: splitCh,
				columns: source.columns,
				config:  &ChannelSpec{Name: fmt.Sprintf("splitter channel from %s", source.config.Name)},
			}, writePartitionsResultCh, key, &wg)
		}
		// Send the record to the intermediate channel
		// fmt.Println("**!@@ splitter loop, sending record to intermediate channel:", key)
		select {
		case splitCh <- inRow:
		case <-ctx.done:
			log.Printf("startSplitterPipe writing to splitter intermediate channel with key %s from '%s' interrupted", key, source.config.Name)
			goto doneSplitterLoop
		}
	}
doneSplitterLoop:
	// Close all the intermediate channels
	for _, ch := range chanState {
		// fmt.Println("**!@@ startSplitterPipe closing intermediate channel", key)
		close(ch)
	}
	// Close the output channels once all ch handlers are done
	// fmt.Println("**!@@ Splitter loop done, ABOUT to wait on wg")
	wg.Wait()
	// fmt.Println("**!@@ Splitter loop done, DONE waiting on wg!")
	// Closing the output channels via the defer above
	// All good!
	return
gotError:
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
}

func (ctx *BuilderContext) startSplitterChannelHandler(spec *PipeSpec, source *InputChannel, partitionResultCh chan ComputePipesResult,
	jetsPartitionKey interface{}, wg *sync.WaitGroup) {
	var cpErr, err error
	var evaluators []PipeTransformationEvaluator
	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			cpErr := fmt.Errorf("startSplitterChannelHandler: recovered error: %v", r)
			log.Println(cpErr)
			debug.PrintStack()
			ctx.errCh <- cpErr
			close(ctx.done)
		}
		wg.Done()
	}()
	// fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ Called")
	// Build the PipeTransformationEvaluator
	evaluators = make([]PipeTransformationEvaluator, len(spec.Apply))
	for j := range spec.Apply {
		evaluators[j], err = ctx.buildPipeTransformationEvaluator(source, jetsPartitionKey, partitionResultCh, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling buildPipeTransformationEvaluator for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
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
	// fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ All good!")
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

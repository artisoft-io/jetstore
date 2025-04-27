package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/dolthub/swiss"
)

type ChannelState struct {
	rowCount int
	extShard int
	data     chan []interface{}
}

func (ctx *BuilderContext) StartSplitterPipe(spec *PipeSpec, source *InputChannel, writePartitionsResultCh chan ComputePipesResult) {
	var cpErr error
	var wg sync.WaitGroup
	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("StartSplitterPipe: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			cpErr := errors.New(buf.String())
			ctx.errCh <- cpErr
			// Avoid closing a closed channel
			select {
			case <-ctx.done:
			default:
				close(ctx.done)
			}
		}
		close(writePartitionsResultCh)
		// Closing the output channels
		fmt.Println("**!@@ SPLITTER: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			// Make sure the output chan config is used
			if len(spec.Apply[i].OutputChannel.Name) > 0 {
				oc[spec.Apply[i].OutputChannel.Name] = true
			}
			if spec.Apply[i].Type == "jetrules" {
				// Get the output channels of jetrules
				for j := range spec.Apply[i].JetrulesConfig.OutputChannels {
					oc[spec.Apply[i].JetrulesConfig.OutputChannels[j].Name] = true
				}
			}
		}
		for i := range oc {
			fmt.Println("**!@@ SPLITTER: Closing Output Channel", i)
			ctx.channelRegistry.CloseChannel(i)
		}
	}()
	var err error
	var mapSize uint32 = 1000
	var chanState *swiss.Map[interface{}, *ChannelState]
	var spliterColumnIdx int
	var ok bool
	var config *SplitterSpec
	var baseKey interface{}
	var jetsPartitionKey interface{}
	var splitOnColumn, splitOnDefault, splitOnHash bool
	var hashEvaluator *HashEvaluator

	if spec.SplitterConfig == nil {
		cpErr = fmt.Errorf("error: missing splitter_config for splitter with source channel %s", source.name)
		goto gotError
	}
	config = spec.SplitterConfig
	if config.Type == "" {
		config.Type = "standard"
	}
	if config.Column == "" && config.DefaultSplitterValue == "" && config.ShardOn == nil {
		cpErr = fmt.Errorf(
			"error: invalid splitter_config for splitter with source channel %s, must specify column, shard_on or default_splitter_value",
			source.name)
		goto gotError
	}
	switch config.Type {
	case "standard":
	case "ext_count":
		if config.PartitionRowCount == 0 {
			cpErr = fmt.Errorf(
				"error: splitter config type ext_count, with source channel %s must have partition_row_count specified",
				source.name)
			goto gotError
		}
	default:
		cpErr = fmt.Errorf("error: unknown splitter config type %s, with source channel %s", config.Type, source.name)
		goto gotError
	}

	// the map containing all the intermediate channels corresponding to values @ spliterColumnIdx
	if len(config.Column) > 0 {
		splitOnColumn = true
		spliterColumnIdx, ok = (*source.columns)[config.Column]
		if !ok {
			cpErr = fmt.Errorf("error: invalid column name %s for splitter with source channel %s", config.Column, source.name)
			goto gotError
		}
	} else {
		spliterColumnIdx = -1
	}
	if len(config.DefaultSplitterValue) > 0 {
		lc := 0
		for strings.Contains(config.DefaultSplitterValue, "$") && lc < 5 && ctx.env != nil {
			lc += 1
			for key, v := range ctx.env {
				value, ok := v.(string)
				if ok {
					config.DefaultSplitterValue = strings.ReplaceAll(config.DefaultSplitterValue, key, value)
				}
			}
		}
		mapSize = 1
		splitOnDefault = true
	}
	if config.ShardOn != nil {
		hashEvaluator, err = ctx.NewHashEvaluator(source, config.ShardOn)
		if err != nil {
			cpErr = fmt.Errorf("while calling NewHashEvaluator for splitter with source channel %s: %v", source.name, err)
			goto gotError
		}
		splitOnHash = true
		mapSize = uint32(hashEvaluator.partitions)
	}
	chanState = swiss.NewMap[interface{}, *ChannelState](mapSize)

	// fmt.Println("**!@@ start splitter loop on source:",source.name)
	for inRow := range source.channel {
		baseKey = nil
		switch {

		case splitOnColumn:
			baseKey = inRow[spliterColumnIdx]

		case splitOnHash:
			baseKey, err = hashEvaluator.ComputeHash(inRow)
			if err != nil {
				cpErr = fmt.Errorf("while computing hash key on input record in splitter with source channel %s: %v", source.name, err)
				goto gotError
			}

		case splitOnDefault:
			baseKey = config.DefaultSplitterValue
		}

		splitCh, ok := chanState.Get(baseKey)
		if !ok {
			// unseen value, create an slot with an intermediate channel
			// log.Printf("**!@@ SPLITTER NEW KEY: %v", baseKey)
			splitCh = &ChannelState{data: make(chan []interface{}, 1)}
			chanState.Put(baseKey, splitCh)
			if ctx.cpConfig.ClusterConfig.IsDebugMode {
				if chanState.Count()%5 == 0 {
					log.Println(ctx.sessionId, "node", ctx.nodeId, "splitter size:", chanState.Count(), " on source", source.name)
				}
			}
			// start a goroutine to manage the channel
			// the input channel to the goroutine is splitCh
			// The channel jetsPartitionKey
			switch config.Type {
			case "standard":
				jetsPartitionKey = baseKey
			case "ext_count":
				if baseKey != nil {
					jetsPartitionKey = fmt.Sprintf("%v|0", baseKey)
				} else {
					jetsPartitionKey = 0
				}
			default:
				cpErr = fmt.Errorf("error: unknown splitter config type %s, with source channel %s", config.Type, source.name)
				goto gotError
			}
			if jetsPartitionKey == nil {
				log.Println(ctx.sessionId, "node", ctx.nodeId, "*WARNING* splitter with nil jetsPartitionKey, with source channel", source.name)
			}
			wg.Add(1)
			go ctx.startSplitterChannelHandler(spec, &InputChannel{
				channel: splitCh.data,
				columns: source.columns,
				domainKeySpec: source.domainKeySpec,
				hasGroupedRows: source.hasGroupedRows,
				config: &ChannelSpec{
					Name:      fmt.Sprintf("splitter channel from %s", source.name),
					ClassName: source.config.ClassName,
				},
			}, writePartitionsResultCh, jetsPartitionKey, &wg)
		}

		if config.Type == "ext_count" {
			if splitCh.rowCount >= config.PartitionRowCount {
				// Cut a new channel and associated jetsPartitionKey
				close(splitCh.data)
				splitCh.data = make(chan []interface{}, 1)
				splitCh.extShard += 1
				splitCh.rowCount = 0
				jetsPartitionKey = fmt.Sprintf("%v|%d", baseKey, splitCh.extShard)
				// syart a go routine to manage the new channel
				wg.Add(1)
				go ctx.startSplitterChannelHandler(spec, &InputChannel{
					channel: splitCh.data,
					columns: source.columns,
					config: &ChannelSpec{
						Name:      fmt.Sprintf("splitter channel from %s", source.name),
						ClassName: source.config.ClassName,
					},
				}, writePartitionsResultCh, jetsPartitionKey, &wg)
			}
		}
		// Send the record to the intermediate channel
		// fmt.Println("**!@@ splitter loop, sending record to intermediate channel:", key)
		select {
		case splitCh.data <- inRow:
		case <-ctx.done:
			log.Printf(
				"startSplitterPipe writing to splitter intermediate channel with baseKey %v (jetsPartitionKey %v) from '%v' interrupted",
				baseKey, jetsPartitionKey, source.name)
			goto doneSplitterLoop
		}
		splitCh.rowCount += 1
	}
doneSplitterLoop:
	// Close all the opened intermediate channels
	chanState.Iter(func(key interface{}, v *ChannelState) (stop bool) {
		// fmt.Println("**!@@ startSplitterPipe closing intermediate channel", key)
		close(v.data)
		return false
	})
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
	// Avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
}

func (ctx *BuilderContext) startSplitterChannelHandler(spec *PipeSpec, source *InputChannel, partitionResultCh chan ComputePipesResult,
	jetsPartitionKey interface{}, wg *sync.WaitGroup) {
	var cpErr, err error
	var evaluators []PipeTransformationEvaluator
	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("startSplitterChannelHandler: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			cpErr := errors.New(buf.String())
			log.Println(cpErr)
			ctx.errCh <- cpErr
			// Avoid closing a closed channel
			select {
			case <-ctx.done:
			default:
				close(ctx.done)
			}
		}
		wg.Done()
	}()
	// fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ Called")
	// Build the PipeTransformationEvaluator
	evaluators = make([]PipeTransformationEvaluator, len(spec.Apply))
	for j := range spec.Apply {
		evaluators[j], err = ctx.BuildPipeTransformationEvaluator(source, jetsPartitionKey, partitionResultCh, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling BuildPipeTransformationEvaluator for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
	}
	// Process the channel
	for inRow := range source.channel {
		for i := range evaluators {
			err = evaluators[i].Apply(&inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling Apply on PipeTransformationEvaluator (in startSplitterChannelHandler): %v", err)
				goto gotError
			}
		}
	}
	// Done, close the evaluators
	for i := range spec.Apply {
		if evaluators[i] != nil {
			err = evaluators[i].Done()
			if err != nil {
				cpErr = fmt.Errorf("while calling done on PipeTransformationEvaluator (in startSplitterChannelHandler): %v", err)
				log.Println(cpErr)
				goto gotError
			}
		}
	}
	for i := range evaluators {
		if evaluators[i] != nil {
			evaluators[i].Finally()
		}
	}
	// fmt.Println("**!@@ SPLITTER *1 startSplitterChannelHandler ~ All good!")
	// All good!
	return

gotError:
	for i := range spec.Apply {
		if evaluators[i] != nil {
			evaluators[i].Finally()
		}
	}
	log.Println(cpErr)
	ctx.errCh <- cpErr
	// Avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
}

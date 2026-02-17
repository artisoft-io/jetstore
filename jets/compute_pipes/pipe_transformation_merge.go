package compute_pipes

import (
	"fmt"
	"log"
)

// Merge operator. Merge the input records from multiple input channels into one output channel.
type MergeTransformationPipe struct {
	cpConfig            *ComputePipesConfig
	mainSource          *InputChannel
	mainHashEvaluator   *HashEvaluator
	mergeSources        []*InputChannel
	mergeHashEvaluators []*HashEvaluator
	outputCh            *OutputChannel
	currentValue        any
	currentBundle       []any
	mergeCurrentValues  []*MergeCurrentValue
	spec                *TransformationSpec
	env                 map[string]any
	doneCh              chan struct{}
}

type MergeCurrentValue struct {
	value      any
	valueTxt	string
	pendingRow []any
}

// Implementing interface PipeTransformationEvaluator
func (ctx *MergeTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in MergeTransformationPipe")
	}
	// Compute the main hash key
	mainKey, err := mergeKeyOf(ctx.mainHashEvaluator, input)
	if err != nil {
		return fmt.Errorf("while computing main merge key in MergeTransformationPipe: %v", err)
	}
	if ctx.spec.MergeConfig.IsDebug {
		log.Printf("mainKey=%v (%T), currentValue=%v", mainKey, mainKey, ctx.currentValue)
	}

	switch {

	case ctx.currentValue == nil || ctx.currentValue != mainKey:
		// Got value past end of bundle
		ctx.sendBundle()
		// Start new bundle
		ctx.currentValue = mainKey
		ctx.currentBundle = make([]any, 0, len(ctx.currentBundle))
		ctx.currentBundle = append(ctx.currentBundle, *input)
		if ctx.spec.MergeConfig.IsDebug {
			log.Printf("Starting newBundle with mainKey=%v", mainKey)
		}

		// Add from merge sources
		mainKeyTxt, ok := mainKey.(string)
		if !ok {
			return fmt.Errorf("error: MergeTransformationPipe mainKey is not a string: %v", mainKey)
		}
	nextMergeSource:
		for i, mergeSource := range ctx.mergeSources {
			currentValue := ctx.mergeCurrentValues[i]
			if ctx.spec.MergeConfig.IsDebug {
				log.Printf("-- Merge Source %d: merge key currentValue=%v", i, currentValue.value)
			}

			// Check if have a pending value from previous iteration
			if currentValue.value != nil {
				if currentValue.valueTxt > mainKeyTxt {
					// Pending value is past the main key, so no matching row in merge source
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("mergeSource=%s, pending mergeKey=%v > mainKey=%v (waiting pending)", mergeSource.Name, currentValue.value, mainKey)
					}
					continue nextMergeSource
				}
				if currentValue.valueTxt == mainKeyTxt {
					// Matching row found in merge source
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("mergeSource=%s, mergeKey=%v == %v (adding pending)", mergeSource.Name, currentValue.value, mainKey)
					}
					// Consume the pending row
					ctx.currentBundle = append(ctx.currentBundle, currentValue.pendingRow)
					currentValue.value = nil
					currentValue.valueTxt = ""
					currentValue.pendingRow = nil
				}
			}
			// Advance merge source until we find a matching row or pass the main key
			for {
				mergeInput := ctx.readRecord(mergeSource)
				if mergeInput == nil {
					// End of merge source
					continue nextMergeSource
				}
				mergeKey, err := mergeKeyOf(ctx.mergeHashEvaluators[i], mergeInput)
				if err != nil {
					return fmt.Errorf("while computing merge key in MergeTransformationPipe: %v", err)
				}
				if ctx.spec.MergeConfig.IsDebug {
					log.Printf("mergeSource=%s, mergeKey=%v", mergeSource.Name, mergeKey)
				}
				mergeKeyTxt, ok := mergeKey.(string)
				if !ok {
					return fmt.Errorf("error: MergeTransformationPipe mergeKey is not a string: %v", mergeKey)
				}
				switch {
				case mergeKeyTxt < mainKeyTxt:
					// Advance merge source
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("mergeSource=%s, mergeKey=%v < %v (skipping)", mergeSource.Name, mergeKey, mainKey)
					}

				case mergeKeyTxt == mainKeyTxt:
					// Matching row found in merge source
					ctx.currentBundle = append(ctx.currentBundle, *mergeInput)
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("mergeSource=%s, mergeKey=%v == %v (adding)", mergeSource.Name, mergeKey, mainKey)
					}

				default:
					// Passed main key, store pending row
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("mergeSource=%s, mergeKey=%v > %v (storing pending)", mergeSource.Name, mergeKey, mainKey)
					}
					currentValue.value = mergeKey
					currentValue.valueTxt = mergeKeyTxt
					currentValue.pendingRow = *mergeInput
					continue nextMergeSource
				}
			}
		}

	default:
		// Same bundle
		ctx.currentBundle = append(ctx.currentBundle, *input)
	}

	return nil

}
func (ctx *MergeTransformationPipe) Done() error {
	if ctx.spec.MergeConfig.IsDebug {
		log.Println("MergeTransformationPipe DONE")
	}
	// Send the last bundle
	ctx.sendBundle()
	return nil
}

func (ctx *MergeTransformationPipe) Finally() {}

func mergeKeyOf(hashEval *HashEvaluator, input *[]any) (any, error) {
	return hashEval.ComputeHash(*input)
}

func (ctx *MergeTransformationPipe) sendBundle() {
	// Send the bundle out
	if len(ctx.currentBundle) > 0 {
		if ctx.spec.MergeConfig.IsDebug {
			log.Printf("MergeTransformationPipe sendBundle: currentValue=%v, bundle size=%d", ctx.currentValue, len(ctx.currentBundle))
		}
		select {
		case ctx.outputCh.Channel <- ctx.currentBundle:
		case <-ctx.doneCh:
			log.Println("MergeTransformationPipe interrupted")
		}
	}
}

func (ctx *MergeTransformationPipe) readRecord(mergeSource *InputChannel) *[]any {
	mergeInput, ok := <-mergeSource.Channel
	if !ok {
		return nil
	}
	return &mergeInput
}

// Builder function for MergeTransformationPipe
func (ctx *BuilderContext) NewMergeTransformationPipe(
	source *InputChannel,
	outCh *OutputChannel,
	spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	if spec.MergeConfig == nil {
		return nil, fmt.Errorf("error: MergeTransformationPipe requires MainHashExpr")
	}
	if spec.MergeConfig.IsDebug {
		log.Println("**!@@ ** Creating MergeTransformationPipe")
	}
	// The merge operator must be the first transformation operator since it needs multiple input sources
	inputChannel := ctx.cpConfig.PipesConfig[0].InputChannel
	mergeSources := make([]*InputChannel, 0, len(inputChannel.MergeChannels))
	for _, inputChannelConfig := range inputChannel.MergeChannels {
		mergeSource, err := ctx.channelRegistry.GetInputChannel(inputChannelConfig.Name, inputChannelConfig.HasGroupedRows)
		if err != nil {
			return nil, fmt.Errorf("while getting MergeTransformationPipe merge source channel %s: %v",
				inputChannelConfig.Name, err)
		}
		mergeSources = append(mergeSources, mergeSource)
	}
	return ctx.MakeMergeTransformationPipe(source, mergeSources, outCh, spec)
}

func (ctx *BuilderContext) MakeMergeTransformationPipe(
	mainSource *InputChannel,
	mergeSources []*InputChannel,
	outCh *OutputChannel,
	spec *TransformationSpec) (PipeTransformationEvaluator, error) {

	// Make sure the hash keys are actually the raw text value since the sources are ordered by column values
	if mainSource.DomainKeySpec == nil {
		return nil, fmt.Errorf("error: MergeTransformationPipe main source does not have DomainKeySpec")
	}
	mainSource.DomainKeySpec.HashingOverride = "none"
	mainHashExpression, err := MakeHashExpressionFromGroupByConfig(*mainSource.Columns, &spec.MergeConfig.MainGroupBy)
	if err != nil {
		return nil, fmt.Errorf("while creating main HashExpression for MergeTransformationPipe: %v", err)
	}
	if spec.MergeConfig.IsDebug {
		log.Printf("**MergeTransformationPipe: main HashExpression: %s", mainHashExpression.String())
	}

	mainHashEvaluator, err := ctx.NewHashEvaluator(mainSource, mainHashExpression)
	if err != nil {
		return nil, fmt.Errorf("while creating main HashEvaluator for MergeTransformationPipe: %v", err)
	}
	if spec.MergeConfig.IsDebug {
		log.Printf("**MergeTransformationPipe: main HashEvaluator: %s", mainHashEvaluator.String())
	}

	l := len(mergeSources)
	mergeHashEvaluators := make([]*HashEvaluator, 0, l)
	for i, mergeSource := range mergeSources {
		var mergeHashEvaluator *HashEvaluator
		if mergeSource.DomainKeySpec == nil {
			return nil, fmt.Errorf("error: MergeTransformationPipe merge source %d does not have DomainKeySpec", i)
		}
		mergeSource.DomainKeySpec.HashingOverride = "none"
		if len(spec.MergeConfig.MergeGroupBy) < l {
			mergeHashEvaluator, err = ctx.NewHashEvaluator(mergeSource, mainHashExpression)
		} else {
			mergeHashExpression, err2 := MakeHashExpressionFromGroupByConfig(*mergeSource.Columns, spec.MergeConfig.MergeGroupBy[i])
			if err2 != nil {
				return nil, fmt.Errorf("while creating merge HashExpression for MergeTransformationPipe: %v", err2)
			}
			mergeHashEvaluator, err = ctx.NewHashEvaluator(mergeSource, mergeHashExpression)
		}
		if err != nil {
			return nil, fmt.Errorf("while creating merge HashEvaluator for MergeTransformationPipe: %v", err)
		}
		mergeHashEvaluators = append(mergeHashEvaluators, mergeHashEvaluator)
		if spec.MergeConfig.IsDebug {
			log.Printf("**MergeTransformationPipe: merge source %d HashEvaluator: %s", i, mergeHashEvaluator.String())
		}
	}

	mergeCurrentValues := make([]*MergeCurrentValue, len(mergeSources))
	for i := range mergeSources {
		mergeCurrentValues[i] = &MergeCurrentValue{}
	}
	mergePipe := &MergeTransformationPipe{
		cpConfig:            ctx.cpConfig,
		mainSource:          mainSource,
		mainHashEvaluator:   mainHashEvaluator,
		mergeSources:        mergeSources,
		mergeHashEvaluators: mergeHashEvaluators,
		mergeCurrentValues:  mergeCurrentValues,
		outputCh:            outCh,
		spec:                spec,
		env:                 ctx.env,
		doneCh:              ctx.done,
	}
	return mergePipe, nil
}

func MakeHashExpressionFromGroupByConfig(column map[string]int, config *GroupBySpec) (*HashExpression, error) {
	var expr string
	var compositeExpr []string

	// Convert GroupByPos to GroupByName in spec.MergeConfig
	if len(config.GroupByPos) > 0 && len(config.GroupByName) > 0 {
		return nil, fmt.Errorf("error: MergeTransformationPipe cannot have both GroupByPos and GroupByName")
	}
	if len(config.GroupByPos) > 0 {
		// Convert positions to names
		config.GroupByName = make([]string, 0, len(config.GroupByPos))
		for _, pos := range config.GroupByPos {
			if pos < 0 || pos >= len(column) {
				return nil, fmt.Errorf("error: MergeTransformationPipe GroupByPos %d out of range for source with %d columns",
					pos, len(column))
			}
			for name, ps := range column {
				if ps == pos {
					config.GroupByName = append(config.GroupByName, name)
					break
				}
			}
		}
	}
	switch {
	case len(config.GroupByName) == 1:
		expr = config.GroupByName[0]
	case len(config.GroupByName) > 1:
		compositeExpr = config.GroupByName
	default:
		if len(config.DomainKey) == 0 {
			return nil, fmt.Errorf("error: MergeTransformationPipe requires GroupByName, GroupByPos, or DomainKey")
		}
	}
	var hashExpression = &HashExpression{
		Expr:             expr,
		CompositeExpr:    compositeExpr,
		DomainKey:        config.DomainKey,
		NoPartitions:     true,
		ComputeDomainKey: true, // to make sure the hashing is based on the raw value of the domain key column for correct merging, since the sources are ordered by column values
	}
	return hashExpression, nil
}

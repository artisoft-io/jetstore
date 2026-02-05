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
		log.Printf("MergeTransformationPipe input: mainKey=%v, currentValue=%v", mainKey, ctx.currentValue)
	}

	switch {

	case ctx.currentValue == nil || ctx.currentValue != mainKey:
		// Got value past end of bundle
		ctx.sendBundle()
		// Start new bundle
		ctx.currentValue = mainKey
		ctx.currentBundle = ctx.currentBundle[:0]
		ctx.currentBundle = append(ctx.currentBundle, *input)

		// Add from merge sources
		mainKeyTxt, ok := mainKey.(string)
		if !ok {
			return fmt.Errorf("error: MergeTransformationPipe mainKey is not a string: %v", mainKey)
		}
	nextMergeSource:
		for i, mergeSource := range ctx.mergeSources {
			currentValue := ctx.mergeCurrentValues[i]
			if currentValue.value == mainKey {
				// Consume the pending row
				ctx.currentBundle = append(ctx.currentBundle, currentValue.pendingRow)
				currentValue.value = nil
				currentValue.pendingRow = nil
			}
			// Advance merge source until we find a matching row or pass the main key
			for {
				mergeInput := ctx.readRecord(mergeSource)
				if mergeInput == nil {
					// End of merge source
					goto nextMergeSource
				}
				mergeKey, err := mergeKeyOf(ctx.mergeHashEvaluators[i], mergeInput)
				if err != nil {
					return fmt.Errorf("while computing merge key in MergeTransformationPipe: %v", err)
				}
				if ctx.spec.MergeConfig.IsDebug {
					log.Printf("MergeTransformationPipe input: mergeSource=%s, mergeKey=%v", mergeSource.Name, mergeKey)
				}
				mergeKeyTxt, ok := mergeKey.(string)
				if !ok {
					return fmt.Errorf("error: MergeTransformationPipe mergeKey is not a string: %v", mergeKey)
				}
				switch {
				case mergeKeyTxt < mainKeyTxt:
					// Advance merge source
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("MergeTransformationPipe input: mergeSource=%s, mergeKey=%v < %v (skipping)", mergeSource.Name, mergeKey, mainKey)
					}

				case mergeKeyTxt == mainKeyTxt:
					// Matching row found in merge source
					ctx.currentBundle = append(ctx.currentBundle, *mergeInput)
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("MergeTransformationPipe input: mergeSource=%s, mergeKey=%v == %v (adding)", mergeSource.Name, mergeKey, mainKey)
					}

				default:
					// Passed main key, store pending row
					if ctx.spec.MergeConfig.IsDebug {
						log.Printf("MergeTransformationPipe input: mergeSource=%s, mergeKey=%v > %v (storing pending)", mergeSource.Name, mergeKey, mainKey)
					}
					currentValue.value = mergeKey
					currentValue.pendingRow = *mergeInput
					goto nextMergeSource
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
	log.Println("**!@@ ** MergeTransformationPipe DONE")
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
	log.Println("**!@@ ** Creating MergeTransformationPipe")

	if spec.MergeConfig == nil {
		return nil, fmt.Errorf("error: MergeTransformationPipe requires MainHashExpr")
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
	mainSource.DomainKeySpec.HashingOverride = "none"
	mainHashExpression, err := MakeHashExpressionFromGroupByConfig(*mainSource.Columns, &spec.MergeConfig.MainGroupBy)
	if err != nil {
		return nil, fmt.Errorf("while creating main HashExpression for MergeTransformationPipe: %v", err)
	}
	mainHashEvaluator, err := ctx.NewHashEvaluator(mainSource, mainHashExpression)
	if err != nil {
		return nil, fmt.Errorf("while creating main HashEvaluator for MergeTransformationPipe: %v", err)
	}
	// log.Printf("**!@@ ** Created main HashEvaluator for MergeTransformationPipe: %+v", mainHashEvaluator)
	l := len(mergeSources)
	mergeHashEvaluators := make([]*HashEvaluator, 0, l)
	for i, mergeSource := range mergeSources {
		var mergeHashEvaluator *HashEvaluator
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
		// log.Printf("**!@@ ** Created merge HashEvaluator for MergeTransformationPipe: %+v", mergeHashEvaluator)
	}

	mergePipe := &MergeTransformationPipe{
		cpConfig:            ctx.cpConfig,
		mainSource:          mainSource,
		mainHashEvaluator:   mainHashEvaluator,
		mergeSources:        mergeSources,
		mergeHashEvaluators: mergeHashEvaluators,
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
		ComputeDomainKey: true, // to make sure the hashing is based on the raw value of the domain key column for correct merging, since the sources are ordered by column values
	}
	return hashExpression, nil
}

package compute_pipes

import (
	"fmt"
	"log"
	"math/rand"
)

// Note: No columnEvaluators is used by this operator.
type ShufflingTransformationPipe struct {
	cpConfig      *ComputePipesConfig
	source        *InputChannel
	outputCh      *OutputChannel
	sourceData    [][]interface{}
	maxInputCount int
	spec          *TransformationSpec
	env           map[string]interface{}
	doneCh        chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *ShufflingTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in ShufflingTransformationPipe")
	}
	if len(ctx.sourceData) < ctx.maxInputCount {
		ctx.sourceData = append(ctx.sourceData, *input)
	}
	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
func (ctx *ShufflingTransformationPipe) done() error {
	nbrRecIn := len(ctx.sourceData)
	for range ctx.spec.ShufflingConfig.OutputSampleSize {
		outputRow := make([]interface{}, len(ctx.outputCh.columns))
		// For each column take a random value from the sourceData set
		for jcol := range len(ctx.outputCh.columns) {
			outputRow[jcol] = ctx.sourceData[rand.Intn(nbrRecIn)][jcol]
		}
		// Send the result to output
		// log.Println("**!@@ ** Send SHUFFLING Result to", ctx.outputCh.config.Name)
		select {
		case ctx.outputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("ShufflingTransform interrupted")
		}
	}
	return nil
}

func (ctx *ShufflingTransformationPipe) finally() {}

func (ctx *BuilderContext) NewShufflingTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*ShufflingTransformationPipe, error) {
	if spec == nil || spec.ShufflingConfig == nil {
		return nil, fmt.Errorf("error: Shuffling Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	nsize := 1000

	if spec.ShufflingConfig.MaxInputSampleSize > 0 {
		nsize = spec.ShufflingConfig.MaxInputSampleSize
	}

	return &ShufflingTransformationPipe{
		cpConfig:      ctx.cpConfig,
		source:        source,
		outputCh:      outputCh,
		sourceData:    make([][]interface{}, 0, nsize),
		maxInputCount: spec.ShufflingConfig.MaxInputSampleSize,
		spec:          spec,
		env:           ctx.env,
		doneCh:        ctx.done,
	}, nil
}

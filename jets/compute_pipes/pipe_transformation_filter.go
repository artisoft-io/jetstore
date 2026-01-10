package compute_pipes

import (
	"fmt"
	"log"
)

// Filter operator. Filter input records based on filter criteria.
// Note: Automatically filter the bad rows (ie rows w/o the expected number of columns)
type FilterTransformationPipe struct {
	cpConfig    *ComputePipesConfig
	source      *InputChannel
	outputCh    *OutputChannel
	whenExpr    evalExpression
	nbrSentRows int
	spec        *TransformationSpec
	env         map[string]any
	doneCh      chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *FilterTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in FilterTransformationPipe")
	}
	inputLen := len(*input)
	expectedLen := len(ctx.source.Config.Columns)
	if inputLen != expectedLen {
		// Skip the row
		return nil
	}

	// Check if we reached the max nbr of rows to sent
	if ctx.spec.FilterConfig.MaxOutputCount > 0 && ctx.nbrSentRows >= ctx.spec.FilterConfig.MaxOutputCount {
		return nil
	}
	resp, err := ctx.whenExpr.Eval(*input)
	if err != nil {
		return fmt.Errorf("while evaluating when clause of filter: %v", err)
	}
	v, ok := resp.(int)
	if ok && v == 1 {
		// Filter out the row
		// log.Println("*** ROW FILTERED OUT: ", *input)
		return nil
	}
	// Send out the row
	// log.Println("*** KEEP ROW: ", *input)
	select {
	case ctx.outputCh.Channel <- *input:
	case <-ctx.doneCh:
		log.Println("FilterTransform interrupted")
	}
	ctx.nbrSentRows += 1
	return nil
}

func (ctx *FilterTransformationPipe) Done() error {
	return nil
}

func (ctx *FilterTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewFilterTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*FilterTransformationPipe, error) {
	if spec == nil || spec.FilterConfig == nil {
		return nil, fmt.Errorf("error: Filter Pipe Transformation spec is missing filter_config settings")
	}
	config := spec.FilterConfig
	whenExpr, err := ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, &config.When)
	if err != nil {
		return nil, fmt.Errorf("while building when clause: %v", err)
	}

	return &FilterTransformationPipe{
		cpConfig: ctx.cpConfig,
		source:   source,
		outputCh: outputCh,
		whenExpr: whenExpr,
		spec:     spec,
		env:      ctx.env,
		doneCh:   ctx.done,
	}, nil
}

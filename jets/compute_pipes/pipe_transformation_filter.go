package compute_pipes

import (
	"fmt"
	"log"
)

// Filter operator. Filter input records based on filter criteria.
// Note: Automatically filter the bad rows (ie rows w/o the expected number of columns)
type FilterTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	source           *InputChannel
	outputCh         *OutputChannel
	rowLengthStrict  bool
	whenExpr         *evalExpression
	maxOutputCount   int
	nbrSentRows      int
	spec             *TransformationSpec
	env              map[string]any
	doneCh           chan struct{}
	columnEvaluators []TransformationColumnEvaluator
}

// Implementing interface PipeTransformationEvaluator
func (ctx *FilterTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in FilterTransformationPipe")
	}
	if ctx.rowLengthStrict {
		inputLen := len(*input)
		expectedLen := len(ctx.source.Config.Columns)
		if inputLen != expectedLen {
			// Skip the row
			return nil
		}
	}

	// Check if we reached the max nbr of rows to sent
	if ctx.maxOutputCount > 0 && ctx.nbrSentRows >= ctx.maxOutputCount {
		return nil
	}
	if ctx.whenExpr != nil {
		resp, err := (*ctx.whenExpr).Eval(*input)
		if err != nil {
			return fmt.Errorf("while evaluating when clause of filter: %v", err)
		}
		v, ok := resp.(int)
		if ok && v == 1 {
			// Filter out the row
			// log.Println("*** ROW FILTERED OUT: ", *input)
			return nil
		}
	}

	var currentValues *[]any
	if ctx.spec.NewRecord {
		v := make([]any, len(ctx.outputCh.Config.Columns))
		currentValues = &v
		// initialize the column evaluators
		for i := range ctx.columnEvaluators {
			ctx.columnEvaluators[i].InitializeCurrentValue(currentValues)
		}
	} else {
		currentValues = input
	}
	// Apply the column transformation for each column
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Update(currentValues, input)
		if err != nil {
			err = fmt.Errorf("while calling column transformation from filter: %v", err)
			log.Println(err)
			return err
		}
	}
	if !ctx.spec.NewRecord {
		// resize the slice in case we're dropping column on the output
		if len(*currentValues) > len(ctx.outputCh.Config.Columns) {
			*currentValues = (*currentValues)[:len(ctx.outputCh.Config.Columns)]
		}
	}

	// Send out the row
	// log.Println("*** KEEP ROW: ", *currentValues)
	select {
	case ctx.outputCh.Channel <- *currentValues:
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
	config := spec.FilterConfig
	var whenExpr *evalExpression
	var strict bool
	var maxCount int
	if config != nil {
		strict = config.RowLengthStrict
		maxCount = config.MaxOutputCount
		if config.When != nil {
			ex, err := ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, config.When)
			if err != nil {
				return nil, fmt.Errorf("while building when clause: %v", err)
			}
			whenExpr = &ex
		}
	}
	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, 0, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		ce, err := ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in FilterTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
		columnEvaluators = append(columnEvaluators, ce)
	}

	return &FilterTransformationPipe{
		cpConfig:         ctx.cpConfig,
		source:           source,
		outputCh:         outputCh,
		rowLengthStrict:  strict,
		whenExpr:         whenExpr,
		maxOutputCount:   maxCount,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		env:              ctx.env,
		doneCh:           ctx.done,
	}, nil
}

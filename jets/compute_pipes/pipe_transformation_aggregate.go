package compute_pipes

import (
	"fmt"
	"log"
)

// aggregate TransformationSpec implementing PipeTransformationEvaluator interface

type AggregateTransformationPipe struct {
	outputCh *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	currentValues []interface{}
	doneCh chan struct{}
}
// Implementing interface PipeTransformationEvaluator
func (ctx *AggregateTransformationPipe) apply(input *[]interface{}) error {
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].update(&ctx.currentValues, input)
		if err != nil {
			return fmt.Errorf("while calling update on TransformationColumnEvaluator: %v", err)
		}
	}
	return nil
}
func (ctx *AggregateTransformationPipe) done() error {
	// Notify the column evaluator that we're done
	// fmt.Println("**! calling done on column evaluator from AggregateTransformationPipe for output", ctx.outputCh.config.Name)
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].done(&ctx.currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from AggregateTransformationPipe: %v", err)
		}
	}
	// Send the result to output
	// fmt.Println("**! ** Send AGGREGATE Result to", ctx.outputCh.config.Name)
	select {
	case ctx.outputCh.channel <- ctx.currentValues:
	case <-ctx.doneCh:
		log.Println("AggregateTransform interrupted")
	}
	// fmt.Println("**! ** Send AGGREGATE Result to", ctx.outputCh.config.Name,"DONE")
	return nil
}

func (ctx *BuilderContext) NewAggregateTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AggregateTransformationPipe, error) {
	// Prepare the column evaluators
	var err error
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build aggregate TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in buildPipeTransformationEvaluator) %v", err)
			fmt.Println(err)
			return nil, err
		}
	}

	currentValues := make([]interface{}, len(outputCh.config.Columns))
	for i := range columnEvaluators {
		columnEvaluators[i].initializeCurrentValue(&currentValues)
	}
	return &AggregateTransformationPipe{
		outputCh: outputCh,
		columnEvaluators: columnEvaluators,
		currentValues: currentValues,
	}, nil
}


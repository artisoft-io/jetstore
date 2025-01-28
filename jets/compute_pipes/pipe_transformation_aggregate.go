package compute_pipes

import (
	"fmt"
	"log"
)

// aggregate TransformationSpec implementing PipeTransformationEvaluator interface

type AggregateTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	outputCh         *OutputChannel
	columnEvaluators []TransformationColumnEvaluator
	currentValues    []interface{}
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AggregateTransformationPipe) Apply(input *[]interface{}) error {
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Update(&ctx.currentValues, input)
		if err != nil {
			return fmt.Errorf("while calling update on TransformationColumnEvaluator: %v", err)
		}
	}
	return nil
}
func (ctx *AggregateTransformationPipe) Done() error {
	// Notify the column evaluator that we're done
	// fmt.Println("**!@@ calling done on column evaluator from AggregateTransformationPipe for output", ctx.outputCh.name)
	for i := range ctx.columnEvaluators {
		err := ctx.columnEvaluators[i].Done(&ctx.currentValues)
		if err != nil {
			return fmt.Errorf("while calling done on column evaluator from AggregateTransformationPipe: %v", err)
		}
	}
	// Send the result to output
	// fmt.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.name)
	select {
	case ctx.outputCh.channel <- ctx.currentValues:
	case <-ctx.doneCh:
		log.Println("AggregateTransform interrupted")
	}
	// fmt.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.name,"DONE")
	return nil
}

func (ctx *AggregateTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewAggregateTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AggregateTransformationPipe, error) {
	// Prepare the column evaluators
	var err error
	// Validate the config: must have NewRecord set to true
	if !spec.NewRecord {
		err = fmt.Errorf("error: must have new_record set to true for aggregate transform")
		fmt.Println(err)
		return nil, err
	}
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build aggregate TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewAggregateTransformationPipe) %v", err)
			fmt.Println(err)
			return nil, err
		}
	}

	currentValues := make([]interface{}, len(outputCh.config.Columns))
	for i := range columnEvaluators {
		columnEvaluators[i].InitializeCurrentValue(&currentValues)
	}
	return &AggregateTransformationPipe{
		cpConfig:         ctx.cpConfig,
		outputCh:         outputCh,
		columnEvaluators: columnEvaluators,
		currentValues:    currentValues,
	}, nil
}

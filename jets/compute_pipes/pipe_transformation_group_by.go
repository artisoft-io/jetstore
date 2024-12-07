package compute_pipes

import (
	"fmt"
	"log"
	"strings"
)

// Group By operator. Group the input records into bundles, where each
// record of the bundle is a rule session.
type GroupByTransformationPipe struct {
	cpConfig      *ComputePipesConfig
	source        *InputChannel
	outputCh      *OutputChannel
	currentValue  any
	currentBundle []any
	groupByPos    []int
	spec          *TransformationSpec
	env           map[string]interface{}
	doneCh        chan struct{}
}

func (ctx *GroupByTransformationPipe) groupValueOf(input *[]interface{}) any {
	if len(ctx.groupByPos) == 1 {
		return (*input)[ctx.groupByPos[0]]
	}
	var buf strings.Builder
	for _, i := range ctx.groupByPos {
		if (*input)[i] != nil {
			buf.WriteString(fmt.Sprintf("%v", (*input)[i]))
		}
	}
	return buf.String()
}

// Implementing interface PipeTransformationEvaluator
func (ctx *GroupByTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in GroupByTransformationPipe")
	}
	groupByValue := ctx.groupValueOf(input)
	switch {
	case ctx.currentValue == nil:
		// First value of bundle
		ctx.currentValue = groupByValue
		ctx.currentBundle = append(ctx.currentBundle, *input)

	case ctx.currentValue != groupByValue:
		// Got value past end of bundle
		// Send the bundle out
		select {
		case ctx.outputCh.channel <- ctx.currentBundle:
		case <-ctx.doneCh:
			log.Println("GroupByTransform interrupted")
		}
		// Prepare the next bundle
		ctx.currentBundle = make([]any, 0)
		ctx.currentBundle = append(ctx.currentBundle, *input)

	default:
		// Adding to the bundle
		ctx.currentBundle = append(ctx.currentBundle, *input)
	}
	return nil
}

func (ctx *GroupByTransformationPipe) done() error {
	// Send the last bundle
	if len(ctx.currentBundle)	> 0 {
		// Send the bundle out the last bundle
		select {
		case ctx.outputCh.channel <- ctx.currentBundle:
		case <-ctx.doneCh:
			log.Println("GroupByTransform interrupted")
		}
	}
	// log.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.config.Name, "DONE")
	return nil
}

func (ctx *GroupByTransformationPipe) finally() {}

func (ctx *BuilderContext) NewGroupByTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*GroupByTransformationPipe, error) {
	if spec == nil || spec.GroupByConfig == nil {
		return nil, fmt.Errorf("error: GroupBy Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	config := spec.GroupByConfig
	groupByPos := config.GroupByPos
	if len(config.GroupByName) > 0 {
		groupByPos = make([]int, 0)
		for _, name := range config.GroupByName {
			groupByPos = append(groupByPos, source.columns[name])
		}
	}
	if groupByPos == nil {
		groupByPos = []int{1}
	}

	return &GroupByTransformationPipe{
		cpConfig:      ctx.cpConfig,
		source:        source,
		outputCh:      outputCh,
		groupByPos:    groupByPos,
		currentBundle: make([]any, 0),
		spec:          spec,
		env:           ctx.env,
		doneCh:        ctx.done,
	}, nil
}

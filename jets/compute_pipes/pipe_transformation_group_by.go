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
	groupByCount  int
	groupByPos    []int
	spec          *TransformationSpec
	env           map[string]any
	doneCh        chan struct{}
}

func (ctx *GroupByTransformationPipe) groupValueOf(input *[]any) any {
	if len(ctx.groupByPos) == 1 {
		return (*input)[ctx.groupByPos[0]]
	}
	var buf strings.Builder
	for _, i := range ctx.groupByPos {
		if (*input)[i] != nil {
			fmt.Fprintf(&buf, "%v", (*input)[i])
		}
	}
	return buf.String()
}

// Implementing interface PipeTransformationEvaluator
func (ctx *GroupByTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in GroupByTransformationPipe")
	}
	if ctx.groupByCount > 0 {
		// Group row by count
		if len(ctx.currentBundle) < ctx.groupByCount {
			ctx.currentBundle = append(ctx.currentBundle, *input)
			return nil
		}
		// Got value past end of bundle
		ctx.sendBundle(input)
		return nil
	}
	// Group by value
	groupByValue := ctx.groupValueOf(input)
	if ctx.spec.GroupByConfig != nil && ctx.spec.GroupByConfig.IsDebug {
		log.Printf("GroupByTransformationPipe input: groupByValue=%v, currentValue=%v", groupByValue, ctx.currentValue)
	}
	switch {
	case ctx.currentValue == nil:
		// First value of bundle
		ctx.currentValue = groupByValue
		ctx.currentBundle = append(ctx.currentBundle, *input)

	case ctx.currentValue != groupByValue:
		// Got value past end of bundle
		if ctx.spec.GroupByConfig != nil && ctx.spec.GroupByConfig.IsDebug {
			log.Printf("GroupByTransformationPipe output: sending bundle of size %d with currentValue=%v", len(ctx.currentBundle), ctx.currentValue)
		}
		ctx.sendBundle(input)
		ctx.currentValue = groupByValue

	default:
		// Adding to the bundle
		ctx.currentBundle = append(ctx.currentBundle, *input)
	}
	return nil
}

func (ctx *GroupByTransformationPipe) sendBundle(input *[]any) {
	// Send the bundle out
	select {
	case ctx.outputCh.Channel <- ctx.currentBundle:
	case <-ctx.doneCh:
		log.Println("GroupByTransform interrupted")
	}
	// Prepare the next bundle
	ctx.currentBundle = make([]any, 0)
	ctx.currentBundle = append(ctx.currentBundle, *input)
}

func (ctx *GroupByTransformationPipe) Done() error {
	// Send the last bundle
	if len(ctx.currentBundle) > 0 {
		// Send the bundle out the last bundle
		select {
		case ctx.outputCh.Channel <- ctx.currentBundle:
		case <-ctx.doneCh:
			log.Println("GroupByTransform interrupted")
		}
	}
	// log.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.name, "DONE")
	return nil
}

func (ctx *GroupByTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewGroupByTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*GroupByTransformationPipe, error) {
	if spec == nil || spec.GroupByConfig == nil {
		return nil, fmt.Errorf("error: GroupBy Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	config := spec.GroupByConfig
	groupByCount := config.GroupByCount
	var groupByPos []int

	// Check if group by domain_key
	if len(config.DomainKey) > 0 {
		dk := source.DomainKeySpec
		if dk == nil {
			return nil, fmt.Errorf("error: group_by operator is configured with domain key but no domain key spec available")
		}
		info, ok := dk.DomainKeys[config.DomainKey]
		if ok {
			// use config.GroupByName to hold the source of the domain key
			config.GroupByName = info.KeyExpr
			if config.IsDebug {
				log.Printf("GroupByTransformationPipe using domain key '%s' with key expr: %v", config.DomainKey, info.KeyExpr)
			}
		} else {
			return nil, fmt.Errorf("error: group_by operator is configured with domain key, but no domain key defined for %s", config.DomainKey)
		}
	}
	
	if groupByCount == 0 {
		groupByPos = config.GroupByPos
		l := len(config.GroupByName)
		if l > 0 {
			groupByPos = make([]int, 0, l)
			for _, name := range config.GroupByName {
				groupByPos = append(groupByPos, (*source.Columns)[name])
			}
		}
	}
	if groupByCount == 0 && len(groupByPos) == 0 {
		return nil, fmt.Errorf("error: group_by operator must specify one of: group_by_name, group_by_pos, group_by_count")
	}
	if config.IsDebug {
		log.Printf("GroupByTransformationPipe config: group_by_count=%d, group_by_pos=%v (name=%v)", groupByCount, groupByPos, config.GroupByName)
	}

	return &GroupByTransformationPipe{
		cpConfig:      ctx.cpConfig,
		source:        source,
		outputCh:      outputCh,
		groupByCount:  groupByCount,
		groupByPos:    groupByPos,
		currentBundle: make([]any, 0),
		spec:          spec,
		env:           ctx.env,
		doneCh:        ctx.done,
	}, nil
}

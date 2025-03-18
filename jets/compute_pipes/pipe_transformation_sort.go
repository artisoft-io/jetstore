package compute_pipes

import (
	"fmt"
	"log"
	"slices"
)

// sort operator.Sort the input records according to a composite key
type SortTransformationPipe struct {
	cpConfig     *ComputePipesConfig
	source       *InputChannel
	outputCh     *OutputChannel
	sortBy       []int
	inputRecords []*[]any
	spec         *TransformationSpec
	env          map[string]interface{}
	doneCh       chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *SortTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in SortTransformationPipe")
	}
	ctx.inputRecords = append(ctx.inputRecords, input)
	return nil
}

func (ctx *SortTransformationPipe) Done() error {
	// Sort the input records and send them
	slices.SortFunc(ctx.inputRecords, func(lhs, rhs *[]any) int {
		if lhs == nil || rhs == nil {
			return 0
		}
		sz := len(*lhs)
		r := len(*rhs)
		if r < sz {
			sz = r
		}
		for _, pos := range ctx.sortBy {
			if pos >= sz {
				return 0
			}
			c := CmpRecord((*lhs)[pos], (*rhs)[pos])
			switch c {
			case 0:
				continue
			default:
				return c
			}
		}
		return 0
	})

	// Send out the records
	for _, inputRecord := range ctx.inputRecords {
		select {
		case ctx.outputCh.channel <- *inputRecord:
		case <-ctx.doneCh:
			log.Println("SortTransform interrupted")
		}
	}
	return nil
}

func (ctx *SortTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewSortTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*SortTransformationPipe, error) {
	if spec == nil || spec.SortConfig == nil {
		return nil, fmt.Errorf("error: Sort Pipe Transformation spec is missing sort_config settings")
	}
	config := spec.SortConfig

	// Check if group by domain_key
	if len(config.DomainKey) > 0 {
		dk := source.domainKeySpec
		if dk == nil {
			return nil, fmt.Errorf("error: sort operator is configured with domain key but no domain key spec available")
		}
		info, ok := dk.DomainKeys[config.DomainKey]
		if ok {
			// use config.SortByColumn to hold the source of the domain key
			config.SortByColumn = info.KeyExpr
		} else {
			return nil, fmt.Errorf("error: sort operator is configured with domain key, but no domain key defined for %s", config.DomainKey)
		}
	}

	sortBy := make([]int, 0, len(config.SortByColumn))
	for _, c := range config.SortByColumn {
		pos, ok := (*source.columns)[c]
		if !ok {
			return nil, fmt.Errorf("error: sort key '%s' is not an input column to %s", c, source.name)
		}
		sortBy = append(sortBy, pos)
	}

	return &SortTransformationPipe{
		cpConfig:     ctx.cpConfig,
		inputRecords: make([]*[]any, 0, 2048),
		source:       source,
		outputCh:     outputCh,
		sortBy:       sortBy,
		spec:         spec,
		env:          ctx.env,
		doneCh:       ctx.done,
	}, nil
}

package compute_pipes

import (
	"fmt"
	"log"
	"strings"

	"github.com/dolthub/swiss"
)

type DistinctTransformationPipe struct {
	cpConfig     *ComputePipesConfig
	source       *InputChannel
	outputCh     *OutputChannel
	compositeKey []int
	distinctRows *swiss.Map[string, bool]
	spec         *TransformationSpec
	env          map[string]interface{}
	doneCh       chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *DistinctTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in DistinctTransformationPipe")
	}
	// make the key
	ckeys := make([]string, 0, len(ctx.compositeKey))
	for i := range ctx.compositeKey {
		switch vv := (*input)[i].(type) {
		case string:
			ckeys = append(ckeys, vv)
		default:
			ckeys = append(ckeys, fmt.Sprintf("%v", vv))
		}
	}
	key := strings.Join(ckeys, "")
	_, ok := ctx.distinctRows.Get(key)
	if !ok {
		ctx.distinctRows.Put(key, true)
		select {
		case ctx.outputCh.channel <- *input:
		case <-ctx.doneCh:
			log.Println("Distinct Transform interrupted")
		}
	}
	return nil
}

func (ctx *DistinctTransformationPipe) Done() error {
	return nil
}

func (ctx *DistinctTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewDistinctTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*DistinctTransformationPipe, error) {
	if spec == nil || spec.DistinctConfig == nil {
		return nil, fmt.Errorf("error: Distinct Pipe Transformation spec is missing distinct_config element")
	}
	// Make the composite key
	compositeKey := make([]int, 0, len(spec.DistinctConfig.DistinctOn))
	for _, column := range spec.DistinctConfig.DistinctOn {
		pos, ok := (*source.columns)[column]
		if !ok {
			return nil, fmt.Errorf("error: key column %s is not in the input channel (distinct operator)", column)
		}
		compositeKey = append(compositeKey, pos)
	}
	return &DistinctTransformationPipe{
		cpConfig:     ctx.cpConfig,
		source:       source,
		outputCh:     outputCh,
		compositeKey: compositeKey,
		distinctRows: swiss.NewMap[string, bool](1000),
		spec:         spec,
		env:          ctx.env,
		doneCh:       ctx.done,
	}, nil
}
